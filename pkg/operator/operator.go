/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operator

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/operator"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"
)

func init() {
	lo.Must0(apis.AddToScheme(scheme.Scheme))
	corev1beta1.NormalizedLabels = lo.Assign(corev1beta1.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": corev1.LabelTopologyZone})
}

// Operator is injected into the AWS CloudProvider's factories
type Operator struct {
	*operator.Operator

	Session                   *session.Session
	UnavailableOfferingsCache *awscache.UnavailableOfferings
	EC2API                    ec2iface.EC2API
	SubnetProvider            *subnet.Provider
	SecurityGroupProvider     *securitygroup.Provider
	InstanceProfileProvider   *instanceprofile.Provider
	AMIProvider               *amifamily.Provider
	AMIResolver               *amifamily.Resolver
	LaunchTemplateProvider    *launchtemplate.Provider
	PricingProvider           *pricing.Provider
	VersionProvider           *version.Provider
	InstanceTypesProvider     *instancetype.Provider
	InstanceProvider          *instance.Provider
}

func NewOperator(ctx context.Context, operator *operator.Operator) (context.Context, *Operator) {
	config := &aws.Config{
		STSRegionalEndpoint: endpoints.RegionalSTSEndpoint,
	}

	if assumeRoleARN := options.FromContext(ctx).AssumeRoleARN; assumeRoleARN != "" {
		config.Credentials = stscreds.NewCredentials(session.Must(session.NewSession()), assumeRoleARN,
			func(provider *stscreds.AssumeRoleProvider) { setDurationAndExpiry(ctx, provider) })
	}

	sess := withUserAgent(session.Must(session.NewSession(
		request.WithRetryer(
			config,
			awsclient.DefaultRetryer{NumMaxRetries: awsclient.DefaultRetryerMaxNumRetries},
		),
	)))

	if *sess.Config.Region == "" {
		logging.FromContext(ctx).Debug("retrieving region from IMDS")
		region, err := ec2metadata.New(sess).Region()
		*sess.Config.Region = lo.Must(region, err, "failed to get region from metadata server")
	}
	ec2api := ec2.New(sess)
	if err := checkEC2Connectivity(ctx, ec2api); err != nil {
		logging.FromContext(ctx).Fatalf("Checking EC2 API connectivity, %s", err)
	}
	logging.FromContext(ctx).With("region", *sess.Config.Region).Debugf("discovered region")
	clusterEndpoint, err := ResolveClusterEndpoint(ctx, eks.New(sess))
	if err != nil {
		logging.FromContext(ctx).Fatalf("unable to detect the cluster endpoint, %s", err)
	} else {
		logging.FromContext(ctx).With("cluster-endpoint", clusterEndpoint).Debugf("discovered cluster endpoint")
	}
	// We perform best-effort on resolving the kube-dns IP
	kubeDNSIP, err := kubeDNSIP(ctx, operator.KubernetesInterface)
	if err != nil {
		// If we fail to get the kube-dns IP, we don't want to crash because this causes issues with custom DNS setups
		// https://github.com/aws/karpenter-provider-aws/issues/2787
		logging.FromContext(ctx).Debugf("unable to detect the IP of the kube-dns service, %s", err)
	} else {
		logging.FromContext(ctx).With("kube-dns-ip", kubeDNSIP).Debugf("discovered kube dns")
	}

	unavailableOfferingsCache := awscache.NewUnavailableOfferings()
	subnetProvider := subnet.NewProvider(ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	securityGroupProvider := securitygroup.NewProvider(ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	instanceProfileProvider := instanceprofile.NewProvider(*sess.Config.Region, iam.New(sess), cache.New(awscache.InstanceProfileTTL, awscache.DefaultCleanupInterval))
	pricingProvider := pricing.NewProvider(
		ctx,
		pricing.NewAPI(sess, *sess.Config.Region),
		ec2api,
		*sess.Config.Region,
	)
	versionProvider := version.NewProvider(operator.KubernetesInterface, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	amiProvider := amifamily.NewProvider(versionProvider, ssm.New(sess), ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	amiResolver := amifamily.New(amiProvider)
	launchTemplateProvider := launchtemplate.NewProvider(
		ctx,
		cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval),
		ec2api,
		amiResolver,
		securityGroupProvider,
		subnetProvider,
		instanceProfileProvider,
		lo.Must(getCABundle(ctx, operator.GetConfig())),
		operator.Elected(),
		kubeDNSIP,
		clusterEndpoint,
	)
	instanceTypeProvider := instancetype.NewProvider(
		*sess.Config.Region,
		cache.New(awscache.InstanceTypesAndZonesTTL, awscache.DefaultCleanupInterval),
		ec2api,
		subnetProvider,
		unavailableOfferingsCache,
		pricingProvider,
	)
	instanceProvider := instance.NewProvider(
		ctx,
		aws.StringValue(sess.Config.Region),
		ec2api,
		unavailableOfferingsCache,
		instanceTypeProvider,
		subnetProvider,
		launchTemplateProvider,
	)

	lo.Must0(operator.Manager.GetFieldIndexer().IndexField(ctx, &corev1beta1.NodeClaim{}, "spec.nodeClassRef.name", func(o client.Object) []string {
		nc := o.(*corev1beta1.NodeClaim)
		if nc.Spec.NodeClassRef == nil {
			return []string{}
		}
		return []string{nc.Spec.NodeClassRef.Name}
	}), "failed to setup nodeclaim indexer")

	return ctx, &Operator{
		Operator:                  operator,
		Session:                   sess,
		UnavailableOfferingsCache: unavailableOfferingsCache,
		EC2API:                    ec2api,
		SubnetProvider:            subnetProvider,
		SecurityGroupProvider:     securityGroupProvider,
		InstanceProfileProvider:   instanceProfileProvider,
		AMIProvider:               amiProvider,
		AMIResolver:               amiResolver,
		VersionProvider:           versionProvider,
		LaunchTemplateProvider:    launchTemplateProvider,
		PricingProvider:           pricingProvider,
		InstanceTypesProvider:     instanceTypeProvider,
		InstanceProvider:          instanceProvider,
	}
}

// withUserAgent adds a karpenter specific user-agent string to AWS session
func withUserAgent(sess *session.Session) *session.Session {
	userAgent := fmt.Sprintf("karpenter.sh-%s", operator.Version)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentFreeFormHandler(userAgent))
	return sess
}

// checkEC2Connectivity makes a dry-run call to DescribeInstanceTypes.  If it fails, we provide an early indicator that we
// are having issues connecting to the EC2 API.
func checkEC2Connectivity(ctx context.Context, api *ec2.EC2) error {
	_, err := api.DescribeInstanceTypesWithContext(ctx, &ec2.DescribeInstanceTypesInput{DryRun: aws.Bool(true)})
	var aerr awserr.Error
	if errors.As(err, &aerr) && aerr.Code() == "DryRunOperation" {
		return nil
	}
	return err
}

func ResolveClusterEndpoint(ctx context.Context, eksAPI eksiface.EKSAPI) (string, error) {
	clusterEndpointFromOptions := options.FromContext(ctx).ClusterEndpoint
	if clusterEndpointFromOptions != "" {
		return clusterEndpointFromOptions, nil // cluster endpoint is explicitly set
	}
	out, err := eksAPI.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
		Name: aws.String(options.FromContext(ctx).ClusterName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve cluster endpoint, %w", err)
	}
	return *out.Cluster.Endpoint, nil
}

func getCABundle(ctx context.Context, restConfig *rest.Config) (*string, error) {
	// Discover CA Bundle from the REST client. We could alternatively
	// have used the simpler client-go InClusterConfig() method.
	// However, that only works when Karpenter is running as a Pod
	// within the same cluster it's managing.
	if caBundle := options.FromContext(ctx).ClusterCABundle; caBundle != "" {
		return lo.ToPtr(caBundle), nil
	}
	transportConfig, err := restConfig.TransportConfig()
	if err != nil {
		return nil, fmt.Errorf("discovering caBundle, loading transport config, %w", err)
	}
	_, err = transport.TLSConfigFor(transportConfig) // fills in CAData!
	if err != nil {
		return nil, fmt.Errorf("discovering caBundle, loading TLS config, %w", err)
	}
	return ptr.String(base64.StdEncoding.EncodeToString(transportConfig.TLS.CAData)), nil
}

func kubeDNSIP(ctx context.Context, kubernetesInterface kubernetes.Interface) (net.IP, error) {
	if kubernetesInterface == nil {
		return nil, fmt.Errorf("no K8s client provided")
	}
	dnsService, err := kubernetesInterface.CoreV1().Services("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	kubeDNSIP := net.ParseIP(dnsService.Spec.ClusterIP)
	if kubeDNSIP == nil {
		return nil, fmt.Errorf("parsing cluster IP")
	}
	return kubeDNSIP, nil
}

func setDurationAndExpiry(ctx context.Context, provider *stscreds.AssumeRoleProvider) {
	provider.Duration = options.FromContext(ctx).AssumeRoleDuration
	provider.ExpiryWindow = time.Duration(10) * time.Second
}

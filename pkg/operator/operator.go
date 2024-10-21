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
	"os"

	configV2 "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	iamV2 "github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awsclient "github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	prometheusv1 "github.com/jonathan-innis/aws-sdk-go-prometheus/v1"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/log"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator"

	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	ssmp "github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"
)

func init() {
	karpv1.NormalizedLabels = lo.Assign(karpv1.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": corev1.LabelTopologyZone})
}

// Operator is injected into the AWS CloudProvider's factories
type Operator struct {
	*operator.Operator

	Session                   *session.Session
	UnavailableOfferingsCache *awscache.UnavailableOfferings
	SubnetProvider            subnet.Provider
	SecurityGroupProvider     securitygroup.Provider
	InstanceProfileProvider   instanceprofile.Provider
	AMIProvider               amifamily.Provider
	AMIResolver               amifamily.Resolver
	LaunchTemplateProvider    launchtemplate.Provider
	PricingProvider           pricing.Provider
	VersionProvider           version.Provider
	InstanceTypesProvider     *instancetype.DefaultProvider
	InstanceProvider          instance.Provider
	SSMProvider               ssmp.Provider
}

func NewOperator(ctx context.Context, operator *operator.Operator) (context.Context, *Operator) {
	//v1
	config := &aws.Config{
		STSRegionalEndpoint: endpoints.RegionalSTSEndpoint,
	}

	// prometheusv1.WithPrometheusMetrics is used until the upstream aws-sdk-go or aws-sdk-go-v2 supports
	// Prometheus metrics for client-side metrics out-of-the-box
	// See: https://github.com/aws/aws-sdk-go-v2/issues/1744
	sess := prometheusv1.WithPrometheusMetrics(WithUserAgent(session.Must(session.NewSession(
		request.WithRetryer(
			config,
			awsclient.DefaultRetryer{NumMaxRetries: awsclient.DefaultRetryerMaxNumRetries},
		),
	))), crmetrics.Registry)

	if *sess.Config.Region == "" {
		log.FromContext(ctx).V(1).Info("retrieving region from IMDS")
		region, err := ec2metadata.New(sess).Region()
		*sess.Config.Region = lo.Must(region, err, "failed to get region from metadata server")
	}
	ec2api := ec2.New(sess)
	if err := CheckEC2Connectivity(ctx, ec2api); err != nil {
		log.FromContext(ctx).Error(err, "ec2 api connectivity check failed")
		os.Exit(1)
	}
	log.FromContext(ctx).WithValues("region", *sess.Config.Region).V(1).Info("discovered region")
	clusterEndpoint, err := ResolveClusterEndpoint(ctx, eks.New(sess))
	if err != nil {
		log.FromContext(ctx).Error(err, "failed detecting cluster endpoint")
		os.Exit(1)
	} else {
		log.FromContext(ctx).WithValues("cluster-endpoint", clusterEndpoint).V(1).Info("discovered cluster endpoint")
	}
	// We perform best-effort on resolving the kube-dns IP
	kubeDNSIP, err := KubeDNSIP(ctx, operator.KubernetesInterface)
	if err != nil {
		// If we fail to get the kube-dns IP, we don't want to crash because this causes issues with custom DNS setups
		// https://github.com/aws/karpenter-provider-aws/issues/2787
		log.FromContext(ctx).V(1).Info(fmt.Sprintf("unable to detect the IP of the kube-dns service, %s", err))
	} else {
		log.FromContext(ctx).WithValues("kube-dns-ip", kubeDNSIP).V(1).Info("discovered kube dns")
	}

	//v2
	//Once everything is migrated we will need to update prometheus metrics to v2
	cfg := lo.Must(configV2.LoadDefaultConfig(ctx, configV2.WithRetryMaxAttempts(3)))
	if cfg.Region == "" {
		log.FromContext(ctx).V(1).Info("retrieving region from IMDS")
		metaDataClient := imds.NewFromConfig(cfg)
		cfg.Region = lo.Must(metaDataClient.GetRegion(ctx, nil)).Region
	}

	unavailableOfferingsCache := awscache.NewUnavailableOfferings()
	subnetProvider := subnet.NewDefaultProvider(ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval), cache.New(awscache.AvailableIPAddressTTL, awscache.DefaultCleanupInterval), cache.New(awscache.AssociatePublicIPAddressTTL, awscache.DefaultCleanupInterval))
	securityGroupProvider := securitygroup.NewDefaultProvider(ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	instanceProfileProvider := instanceprofile.NewDefaultProvider(cfg.Region, iamV2.NewFromConfig(cfg), cache.New(awscache.InstanceProfileTTL, awscache.DefaultCleanupInterval))
	pricingProvider := pricing.NewDefaultProvider(
		ctx,
		pricing.NewAPI(sess, *sess.Config.Region),
		ec2api,
		*sess.Config.Region,
	)
	versionProvider := version.NewDefaultProvider(operator.KubernetesInterface, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	ssmProvider := ssmp.NewDefaultProvider(ssm.NewFromConfig(cfg), cache.New(awscache.SSMGetParametersByPathTTL, awscache.DefaultCleanupInterval))
	amiProvider := amifamily.NewDefaultProvider(operator.Clock, versionProvider, ssmProvider, ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	amiResolver := amifamily.NewDefaultResolver()
	launchTemplateProvider := launchtemplate.NewDefaultProvider(
		ctx,
		cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval),
		ec2api,
		eks.New(sess),
		amiResolver,
		securityGroupProvider,
		subnetProvider,
		lo.Must(GetCABundle(ctx, operator.GetConfig())),
		operator.Elected(),
		kubeDNSIP,
		clusterEndpoint,
	)
	instanceTypeProvider := instancetype.NewDefaultProvider(
		cache.New(awscache.InstanceTypesAndZonesTTL, awscache.DefaultCleanupInterval),
		cache.New(awscache.DiscoveredCapacityCacheTTL, awscache.DefaultCleanupInterval),
		ec2api,
		subnetProvider,
		instancetype.NewDefaultResolver(*sess.Config.Region, pricingProvider, unavailableOfferingsCache),
	)
	instanceProvider := instance.NewDefaultProvider(
		ctx,
		aws.StringValue(sess.Config.Region),
		ec2api,
		unavailableOfferingsCache,
		subnetProvider,
		launchTemplateProvider,
	)

	return ctx, &Operator{
		Operator:                  operator,
		Session:                   sess,
		UnavailableOfferingsCache: unavailableOfferingsCache,
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
		SSMProvider:               ssmProvider,
	}
}

// WithUserAgent adds a karpenter specific user-agent string to AWS session
func WithUserAgent(sess *session.Session) *session.Session {
	userAgent := fmt.Sprintf("karpenter.sh-%s", operator.Version)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentFreeFormHandler(userAgent))
	return sess
}

// CheckEC2Connectivity makes a dry-run call to DescribeInstanceTypes.  If it fails, we provide an early indicator that we
// are having issues connecting to the EC2 API.
func CheckEC2Connectivity(ctx context.Context, api ec2iface.EC2API) error {
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

func GetCABundle(ctx context.Context, restConfig *rest.Config) (*string, error) {
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
	return lo.ToPtr(base64.StdEncoding.EncodeToString(transportConfig.TLS.CAData)), nil
}

func KubeDNSIP(ctx context.Context, kubernetesInterface kubernetes.Interface) (net.IP, error) {
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

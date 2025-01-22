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
	stdlog "log"
	"net"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/middleware"
	config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/aws/smithy-go"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clinetconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator"

	prometheusv2 "github.com/jonathan-innis/aws-sdk-go-prometheus/v2"

	"sigs.k8s.io/karpenter/pkg/apis"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
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
	Config                    aws.Config
	UnavailableOfferingsCache *awscache.UnavailableOfferings
	SSMCache                  *cache.Cache
	SubnetProvider            subnet.Provider
	SecurityGroupProvider     securitygroup.Provider
	InstanceProfileProvider   instanceprofile.Provider
	AMIProvider               amifamily.Provider
	AMIResolver               amifamily.Resolver
	LaunchTemplateProvider    launchtemplate.Provider
	PricingProvider           pricing.Provider
	VersionProvider           *version.DefaultProvider
	InstanceTypesProvider     *instancetype.DefaultProvider
	InstanceProvider          instance.Provider
	EC2API                    ec2.Client
}

func NewOperator(ctx context.Context, operator *operator.Operator) (context.Context, *Operator) {
	kubeletCompatibilityAnnotationKey := fmt.Sprintf("%s/%s", apis.CompatibilityGroup, "v1beta1-kubelet-conversion")
	// we are going to panic if any of the customer nodepools contain
	// compatibility.karpenter.sh/v1beta1-kubelet-conversion
	restConfig := clinetconfig.GetConfigOrDie()
	kubeClient := lo.Must(client.New(restConfig, client.Options{}))
	nodePoolList := &karpv1.NodePoolList{}
	lo.Must0(kubeClient.List(ctx, nodePoolList))
	npNames := lo.FilterMap(nodePoolList.Items, func(np karpv1.NodePool, _ int) (string, bool) {
		_, ok := np.Annotations[kubeletCompatibilityAnnotationKey]
		return np.Name, ok
	})
	if len(npNames) != 0 {
		stdlog.Fatalf("The kubelet compatibility annotation, %s, is not supported on Karpenter v1.1+. Please refer to the upgrade guide in the docs. The following NodePools still have the compatibility annotation: %s", kubeletCompatibilityAnnotationKey, strings.Join(npNames, ", "))
	}

	cfg := prometheusv2.WithPrometheusMetrics(WithUserAgent(lo.Must(config.LoadDefaultConfig(ctx))), crmetrics.Registry)
	if cfg.Region == "" {
		log.FromContext(ctx).V(1).Info("retrieving region from IMDS")
		region := lo.Must(imds.NewFromConfig(cfg).GetRegion(ctx, nil))
		cfg.Region = region.Region
	}
	ec2api := ec2.NewFromConfig(cfg)
	eksapi := eks.NewFromConfig(cfg)
	log.FromContext(ctx).WithValues("region", cfg.Region).V(1).Info("discovered region")
	if err := CheckEC2Connectivity(ctx, ec2api); err != nil {
		log.FromContext(ctx).Error(err, "ec2 api connectivity check failed")
		os.Exit(1)
	}
	log.FromContext(ctx).WithValues("region", cfg.Region).V(1).Info("discovered region")
	clusterEndpoint, err := ResolveClusterEndpoint(ctx, eksapi)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed detecting cluster endpoint")
		os.Exit(1)
	} else {
		log.FromContext(ctx).WithValues("cluster-endpoint", clusterEndpoint).V(1).Info("discovered cluster endpoint")
	}
	kubeDNSIP, err := KubeDNSIP(ctx, operator.KubernetesInterface)
	if err != nil {
		// If we fail to get the kube-dns IP, we don't want to crash because this causes issues with custom DNS setups
		// https://github.com/aws/karpenter-provider-aws/issues/2787
		log.FromContext(ctx).V(1).Info(fmt.Sprintf("unable to detect the IP of the kube-dns service, %s", err))
	} else {
		log.FromContext(ctx).WithValues("kube-dns-ip", kubeDNSIP).V(1).Info("discovered kube dns")
	}
	unavailableOfferingsCache := awscache.NewUnavailableOfferings()
	ssmCache := cache.New(awscache.SSMCacheTTL, awscache.DefaultCleanupInterval)

	subnetProvider := subnet.NewDefaultProvider(ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval), cache.New(awscache.AvailableIPAddressTTL, awscache.DefaultCleanupInterval), cache.New(awscache.AssociatePublicIPAddressTTL, awscache.DefaultCleanupInterval))
	securityGroupProvider := securitygroup.NewDefaultProvider(ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	instanceProfileProvider := instanceprofile.NewDefaultProvider(cfg.Region, iam.NewFromConfig(cfg), cache.New(awscache.InstanceProfileTTL, awscache.DefaultCleanupInterval))
	pricingProvider := pricing.NewDefaultProvider(
		ctx,
		pricing.NewAPI(cfg),
		ec2api,
		cfg.Region,
	)
	versionProvider := version.NewDefaultProvider(operator.KubernetesInterface, eksapi)
	// Ensure we're able to hydrate the version before starting any reliant controllers.
	// Version updates are hydrated asynchronously after this, in the event of a failure
	// the previously resolved value will be used.
	lo.Must0(versionProvider.UpdateVersion(ctx))
	ssmProvider := ssmp.NewDefaultProvider(ssm.NewFromConfig(cfg), ssmCache)
	amiProvider := amifamily.NewDefaultProvider(operator.Clock, versionProvider, ssmProvider, ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	amiResolver := amifamily.NewDefaultResolver()
	launchTemplateProvider := launchtemplate.NewDefaultProvider(
		ctx,
		cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval),
		ec2api,
		eksapi,
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
		instancetype.NewDefaultResolver(cfg.Region, pricingProvider, unavailableOfferingsCache),
	)
	instanceProvider := instance.NewDefaultProvider(
		ctx,
		cfg.Region,
		ec2api,
		unavailableOfferingsCache,
		subnetProvider,
		launchTemplateProvider,
	)

	return ctx, &Operator{
		Operator:                  operator,
		Config:                    cfg,
		UnavailableOfferingsCache: unavailableOfferingsCache,
		SSMCache:                  ssmCache,
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
		EC2API:                    *ec2api,
	}
}

// WithUserAgent adds a karpenter specific user-agent string to AWS session
func WithUserAgent(cfg aws.Config) aws.Config {
	userAgent := fmt.Sprintf("karpenter.sh-%s", operator.Version)
	cfg.APIOptions = append(cfg.APIOptions,
		middleware.AddUserAgentKey(userAgent),
	)
	return cfg
}

// CheckEC2Connectivity makes a dry-run call to DescribeInstanceTypes.  If it fails, we provide an early indicator that we
// are having issues connecting to the EC2 API.
func CheckEC2Connectivity(ctx context.Context, api sdk.EC2API) error {
	_, err := api.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		DryRun: aws.Bool(true),
	})
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "DryRunOperation" {
		return nil
	}
	return err
}

func ResolveClusterEndpoint(ctx context.Context, eksAPI sdk.EKSAPI) (string, error) {
	clusterEndpointFromOptions := options.FromContext(ctx).ClusterEndpoint
	if clusterEndpointFromOptions != "" {
		return clusterEndpointFromOptions, nil // cluster endpoint is explicitly set
	}
	out, err := eksAPI.DescribeCluster(ctx, &eks.DescribeClusterInput{
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

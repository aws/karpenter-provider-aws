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

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/awslabs/operatorpkg/aws/middleware"
	"github.com/awslabs/operatorpkg/option"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/aws/smithy-go"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator"

	prometheusv2 "github.com/jonathan-innis/aws-sdk-go-prometheus/v2"

	kwokec2 "github.com/aws/karpenter-provider-aws/kwok/ec2"
	"github.com/aws/karpenter-provider-aws/kwok/strategy"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	ssmp "github.com/aws/karpenter-provider-aws/pkg/providers/ssm"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

func init() {
	karpv1.NormalizedLabels = lo.Assign(karpv1.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": corev1.LabelTopologyZone})
}

// Operator is injected into the AWS CloudProvider's factories
type Operator struct {
	*operator.Operator
	Config                      aws.Config
	UnavailableOfferingsCache   *awscache.UnavailableOfferings
	SSMCache                    *cache.Cache
	ValidationCache             *cache.Cache
	RecreationCache             *cache.Cache
	SubnetProvider              subnet.Provider
	SecurityGroupProvider       securitygroup.Provider
	InstanceProfileProvider     instanceprofile.Provider
	AMIProvider                 amifamily.Provider
	AMIResolver                 amifamily.Resolver
	LaunchTemplateProvider      launchtemplate.Provider
	PricingProvider             pricing.Provider
	VersionProvider             *version.DefaultProvider
	InstanceTypesProvider       *instancetype.DefaultProvider
	InstanceProvider            instance.Provider
	SSMProvider                 ssmp.Provider
	CapacityReservationProvider capacityreservation.Provider
	EC2API                      *kwokec2.Client
}

func NewOperator(ctx context.Context, operator *operator.Operator) (context.Context, *Operator) {
	cfg := prometheusv2.WithPrometheusMetrics(WithUserAgent(lo.Must(config.LoadDefaultConfig(ctx))), crmetrics.Registry)
	cfg.APIOptions = append(cfg.APIOptions, middleware.StructuredErrorHandler)
	if cfg.Region == "" {
		log.FromContext(ctx).V(1).Info("retrieving region from IMDS")
		region := lo.Must(imds.NewFromConfig(cfg).GetRegion(ctx, nil))
		cfg.Region = region.Region
	}
	ec2api := kwokec2.NewClient(cfg.Region, option.MustGetEnv("SYSTEM_NAMESPACE"), ec2.NewFromConfig(cfg), kwokec2.NewNopRateLimiterProvider(), strategy.NewLowestPrice(pricing.NewAPI(cfg), ec2.NewFromConfig(cfg), cfg.Region), operator.GetClient(), operator.Clock)

	eksapi := eks.NewFromConfig(cfg)
	log.FromContext(ctx).WithValues("region", cfg.Region).V(1).Info("discovered region")
	clusterEndpoint := lo.Must(ResolveClusterEndpoint(ctx, eksapi))
	log.FromContext(ctx).WithValues("cluster-endpoint", clusterEndpoint).V(1).Info("discovered cluster endpoint")
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
	validationCache := cache.New(awscache.ValidationTTL, awscache.DefaultCleanupInterval)
	recreationCache := cache.New(awscache.RecreationTTL, awscache.DefaultCleanupInterval)

	subnetProvider := subnet.NewDefaultProvider(ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval), cache.New(awscache.AvailableIPAddressTTL, awscache.DefaultCleanupInterval), cache.New(awscache.AssociatePublicIPAddressTTL, awscache.DefaultCleanupInterval))
	securityGroupProvider := securitygroup.NewDefaultProvider(ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	instanceProfileProvider := instanceprofile.NewDefaultProvider(iam.NewFromConfig(cfg), cache.New(awscache.InstanceProfileTTL, awscache.DefaultCleanupInterval), cache.New(awscache.ProtectedProfilesTTL, awscache.DefaultCleanupInterval), cfg.Region)
	pricingProvider := pricing.NewDefaultProvider(
		pricing.NewAPI(cfg),
		ec2api,
		cfg.Region,
		false,
	)
	versionProvider := version.NewDefaultProvider(operator.KubernetesInterface, eksapi)
	// Ensure we're able to hydrate the version before starting any reliant controllers.
	// Version updates are hydrated asynchronously after this, in the event of a failure
	// the previously resolved value will be used.
	lo.Must0(versionProvider.UpdateVersion(ctx))
	ssmProvider := ssmp.NewDefaultProvider(ssm.NewFromConfig(cfg), ssmCache)
	amiProvider := amifamily.NewDefaultProvider(operator.Clock, versionProvider, ssmProvider, ec2api, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	amiResolver := amifamily.NewDefaultResolver(cfg.Region)
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
	capacityReservationProvider := capacityreservation.NewProvider(
		ec2api,
		operator.Clock,
		cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval),
		cache.New(awscache.CapacityReservationAvailabilityTTL, awscache.DefaultCleanupInterval),
	)
	instanceTypeProvider := instancetype.NewDefaultProvider(
		cache.New(awscache.InstanceTypesZonesAndOfferingsTTL, awscache.DefaultCleanupInterval),
		cache.New(awscache.InstanceTypesZonesAndOfferingsTTL, awscache.DefaultCleanupInterval),
		cache.New(awscache.DiscoveredCapacityCacheTTL, awscache.DefaultCleanupInterval),
		ec2api,
		subnetProvider,
		pricingProvider,
		capacityReservationProvider,
		unavailableOfferingsCache,
		instancetype.NewDefaultResolver(cfg.Region),
	)
	// Ensure we're able to hydrate instance types before starting any reliant controllers.
	// Instance type updates are hydrated asynchronously after this by controllers.
	lo.Must0(instanceTypeProvider.UpdateInstanceTypes(ctx))
	lo.Must0(instanceTypeProvider.UpdateInstanceTypeOfferings(ctx))
	instanceProvider := instance.NewDefaultProvider(
		ctx,
		cfg.Region,
		operator.EventRecorder,
		ec2api,
		unavailableOfferingsCache,
		subnetProvider,
		launchTemplateProvider,
		capacityReservationProvider,
		cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval),
	)

	// Setup field indexers on instanceID -- specifically for the interruption controller
	if options.FromContext(ctx).InterruptionQueue != "" {
		SetupIndexers(ctx, operator.Manager)
	}
	return ctx, &Operator{
		Operator:                    operator,
		Config:                      cfg,
		UnavailableOfferingsCache:   unavailableOfferingsCache,
		SSMCache:                    ssmCache,
		ValidationCache:             validationCache,
		RecreationCache:             recreationCache,
		SubnetProvider:              subnetProvider,
		SecurityGroupProvider:       securityGroupProvider,
		InstanceProfileProvider:     instanceProfileProvider,
		AMIProvider:                 amiProvider,
		AMIResolver:                 amiResolver,
		VersionProvider:             versionProvider,
		LaunchTemplateProvider:      launchTemplateProvider,
		PricingProvider:             pricingProvider,
		InstanceTypesProvider:       instanceTypeProvider,
		InstanceProvider:            instanceProvider,
		SSMProvider:                 ssmProvider,
		CapacityReservationProvider: capacityReservationProvider,
		EC2API:                      ec2api,
	}
}

// WithUserAgent adds a karpenter specific user-agent string to AWS session
func WithUserAgent(cfg aws.Config) aws.Config {
	userAgent := fmt.Sprintf("karpenter.sh-%s", operator.Version)
	cfg.APIOptions = append(cfg.APIOptions,
		awsmiddleware.AddUserAgentKey(userAgent),
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

func SetupIndexers(ctx context.Context, mgr manager.Manager) {
	lo.Must0(mgr.GetFieldIndexer().IndexField(ctx, &karpv1.NodeClaim{}, "status.instanceID", func(o client.Object) []string {
		if o.(*karpv1.NodeClaim).Status.ProviderID == "" {
			return nil
		}
		id, e := utils.ParseInstanceID(o.(*karpv1.NodeClaim).Status.ProviderID)
		if e != nil || id == "" {
			return nil
		}
		return []string{id}
	}), "failed to setup nodeclaim instanceID indexer")
	lo.Must0(mgr.GetFieldIndexer().IndexField(ctx, &corev1.Node{}, "spec.instanceID", func(o client.Object) []string {
		if o.(*corev1.Node).Spec.ProviderID == "" {
			return nil
		}
		id, e := utils.ParseInstanceID(o.(*corev1.Node).Spec.ProviderID)
		if e != nil || id == "" {
			return nil
		}
		return []string{id}
	}), "failed to setup node instanceID indexer")
}

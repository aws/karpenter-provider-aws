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

package test

import (
	"context"
	"net"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/ptr"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/operator/scheme"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"

	coretest "sigs.k8s.io/karpenter/pkg/test"

	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	lo.Must0(apis.AddToScheme(scheme.Scheme))
	corev1beta1.NormalizedLabels = lo.Assign(corev1beta1.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": corev1.LabelTopologyZone})
}

type Environment struct {
	// API
	EC2API     *fake.EC2API
	SSMAPI     *fake.SSMAPI
	IAMAPI     *fake.IAMAPI
	PricingAPI *fake.PricingAPI

	// Cache
	EC2Cache                  *cache.Cache
	KubernetesVersionCache    *cache.Cache
	InstanceTypeCache         *cache.Cache
	UnavailableOfferingsCache *awscache.UnavailableOfferings
	LaunchTemplateCache       *cache.Cache
	SubnetCache               *cache.Cache
	SecurityGroupCache        *cache.Cache
	InstanceProfileCache      *cache.Cache

	// Providers
	InstanceTypesProvider   *instancetype.Provider
	InstanceProvider        *instance.Provider
	SubnetProvider          *subnet.Provider
	SecurityGroupProvider   *securitygroup.Provider
	InstanceProfileProvider *instanceprofile.Provider
	PricingProvider         *pricing.Provider
	AMIProvider             *amifamily.Provider
	AMIResolver             *amifamily.Resolver
	VersionProvider         *version.Provider
	LaunchTemplateProvider  *launchtemplate.Provider
}

func NewEnvironment(ctx context.Context, env *coretest.Environment) *Environment {
	// API
	ec2api := fake.NewEC2API()
	ssmapi := fake.NewSSMAPI()
	iamapi := fake.NewIAMAPI()

	// cache
	ec2Cache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	kubernetesVersionCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	instanceTypeCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	unavailableOfferingsCache := awscache.NewUnavailableOfferings()
	launchTemplateCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	subnetCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	securityGroupCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	instanceProfileCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	fakePricingAPI := &fake.PricingAPI{}

	// Providers
	pricingProvider := pricing.NewProvider(ctx, fakePricingAPI, ec2api, fake.DefaultRegion)
	subnetProvider := subnet.NewProvider(ec2api, subnetCache)
	securityGroupProvider := securitygroup.NewProvider(ec2api, securityGroupCache)
	versionProvider := version.NewProvider(env.KubernetesInterface, kubernetesVersionCache)
	instanceProfileProvider := instanceprofile.NewProvider(fake.DefaultRegion, iamapi, instanceProfileCache)
	amiProvider := amifamily.NewProvider(versionProvider, ssmapi, ec2api, ec2Cache)
	amiResolver := amifamily.New(amiProvider)
	instanceTypesProvider := instancetype.NewProvider(fake.DefaultRegion, instanceTypeCache, ec2api, subnetProvider, unavailableOfferingsCache, pricingProvider)
	launchTemplateProvider :=
		launchtemplate.NewProvider(
			ctx,
			launchTemplateCache,
			ec2api,
			amiResolver,
			securityGroupProvider,
			subnetProvider,
			instanceProfileProvider,
			ptr.String("ca-bundle"),
			make(chan struct{}),
			net.ParseIP("10.0.100.10"),
			"https://test-cluster",
		)
	instanceProvider :=
		instance.NewProvider(ctx,
			"",
			ec2api,
			unavailableOfferingsCache,
			instanceTypesProvider,
			subnetProvider,
			launchTemplateProvider,
		)

	return &Environment{
		EC2API:     ec2api,
		SSMAPI:     ssmapi,
		IAMAPI:     iamapi,
		PricingAPI: fakePricingAPI,

		EC2Cache:                  ec2Cache,
		KubernetesVersionCache:    kubernetesVersionCache,
		InstanceTypeCache:         instanceTypeCache,
		LaunchTemplateCache:       launchTemplateCache,
		SubnetCache:               subnetCache,
		SecurityGroupCache:        securityGroupCache,
		InstanceProfileCache:      instanceProfileCache,
		UnavailableOfferingsCache: unavailableOfferingsCache,

		InstanceTypesProvider:   instanceTypesProvider,
		InstanceProvider:        instanceProvider,
		SubnetProvider:          subnetProvider,
		SecurityGroupProvider:   securityGroupProvider,
		LaunchTemplateProvider:  launchTemplateProvider,
		InstanceProfileProvider: instanceProfileProvider,
		PricingProvider:         pricingProvider,
		AMIProvider:             amiProvider,
		AMIResolver:             amiResolver,
		VersionProvider:         versionProvider,
	}
}

func (env *Environment) Reset() {
	env.EC2API.Reset()
	env.SSMAPI.Reset()
	env.IAMAPI.Reset()
	env.PricingAPI.Reset()
	env.PricingProvider.Reset()

	env.EC2Cache.Flush()
	env.KubernetesVersionCache.Flush()
	env.InstanceTypeCache.Flush()
	env.UnavailableOfferingsCache.Flush()
	env.LaunchTemplateCache.Flush()
	env.SubnetCache.Flush()
	env.SecurityGroupCache.Flush()
	env.InstanceProfileCache.Flush()

	mfs, err := crmetrics.Registry.Gather()
	if err != nil {
		for _, mf := range mfs {
			for _, metric := range mf.GetMetric() {
				if metric != nil {
					metric.Reset()
				}
			}
		}
	}
}

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
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	clock "k8s.io/utils/clock/testing"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
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

	coretest "sigs.k8s.io/karpenter/pkg/test"

	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	karpv1.NormalizedLabels = lo.Assign(karpv1.NormalizedLabels, map[string]string{"topology.ebs.csi.aws.com/zone": corev1.LabelTopologyZone})
	coretest.SetDefaultNodeClassType(&v1.EC2NodeClass{})
}

type Environment struct {
	// Mock
	Clock *clock.FakeClock

	// API
	EC2API     *fake.EC2API
	EKSAPI     *fake.EKSAPI
	SSMAPI     *fake.SSMAPI
	IAMAPI     *fake.IAMAPI
	PricingAPI *fake.PricingAPI

	// Cache
	AMICache                      *cache.Cache
	EC2Cache                      *cache.Cache
	InstanceTypeCache             *cache.Cache
	UnavailableOfferingsCache     *awscache.UnavailableOfferings
	LaunchTemplateCache           *cache.Cache
	SubnetCache                   *cache.Cache
	AvailableIPAdressCache        *cache.Cache
	AssociatePublicIPAddressCache *cache.Cache
	SecurityGroupCache            *cache.Cache
	InstanceProfileCache          *cache.Cache
	SSMCache                      *cache.Cache
	DiscoveredCapacityCache       *cache.Cache

	// Providers
	InstanceTypesResolver   *instancetype.DefaultResolver
	InstanceTypesProvider   *instancetype.DefaultProvider
	InstanceProvider        *instance.DefaultProvider
	SubnetProvider          *subnet.DefaultProvider
	SecurityGroupProvider   *securitygroup.DefaultProvider
	InstanceProfileProvider *instanceprofile.DefaultProvider
	PricingProvider         *pricing.DefaultProvider
	AMIProvider             *amifamily.DefaultProvider
	AMIResolver             *amifamily.DefaultResolver
	VersionProvider         *version.DefaultProvider
	LaunchTemplateProvider  *launchtemplate.DefaultProvider
}

func NewEnvironment(ctx context.Context, env *coretest.Environment) *Environment {
	// Mock
	clock := &clock.FakeClock{}

	// API
	ec2api := fake.NewEC2API()
	eksapi := fake.NewEKSAPI()
	ssmapi := fake.NewSSMAPI()
	iamapi := fake.NewIAMAPI()

	// cache
	amiCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	ec2Cache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	instanceTypeCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	discoveredCapacityCache := cache.New(awscache.DiscoveredCapacityCacheTTL, awscache.DefaultCleanupInterval)
	unavailableOfferingsCache := awscache.NewUnavailableOfferings()
	launchTemplateCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	subnetCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	availableIPAdressCache := cache.New(awscache.AvailableIPAddressTTL, awscache.DefaultCleanupInterval)
	associatePublicIPAddressCache := cache.New(awscache.AssociatePublicIPAddressTTL, awscache.DefaultCleanupInterval)
	securityGroupCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	instanceProfileCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	ssmCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	fakePricingAPI := &fake.PricingAPI{}

	// Providers
	pricingProvider := pricing.NewDefaultProvider(ctx, fakePricingAPI, ec2api, fake.DefaultRegion)
	subnetProvider := subnet.NewDefaultProvider(ec2api, subnetCache, availableIPAdressCache, associatePublicIPAddressCache)
	securityGroupProvider := securitygroup.NewDefaultProvider(ec2api, securityGroupCache)
	versionProvider := version.NewDefaultProvider(env.KubernetesInterface, eksapi)
	// Ensure we're able to hydrate the version before starting any reliant controllers.
	// Version updates are hydrated asynchronously after this, in the event of a failure
	// the previously resolved value will be used.
	lo.Must0(versionProvider.UpdateVersion(ctx))
	instanceProfileProvider := instanceprofile.NewDefaultProvider(fake.DefaultRegion, iamapi, instanceProfileCache)
	ssmProvider := ssmp.NewDefaultProvider(ssmapi, ssmCache)
	amiProvider := amifamily.NewDefaultProvider(clock, versionProvider, ssmProvider, ec2api, amiCache)
	amiResolver := amifamily.NewDefaultResolver()
	instanceTypesResolver := instancetype.NewDefaultResolver(fake.DefaultRegion, pricingProvider, unavailableOfferingsCache)
	instanceTypesProvider := instancetype.NewDefaultProvider(instanceTypeCache, discoveredCapacityCache, ec2api, subnetProvider, instanceTypesResolver)
	launchTemplateProvider :=
		launchtemplate.NewDefaultProvider(
			ctx,
			launchTemplateCache,
			ec2api,
			eksapi,
			amiResolver,
			securityGroupProvider,
			subnetProvider,
			lo.ToPtr("ca-bundle"),
			make(chan struct{}),
			net.ParseIP("10.0.100.10"),
			"https://test-cluster",
		)
	instanceProvider :=
		instance.NewDefaultProvider(ctx,
			"",
			ec2api,
			unavailableOfferingsCache,
			subnetProvider,
			launchTemplateProvider,
		)

	return &Environment{
		Clock: clock,

		EC2API:     ec2api,
		EKSAPI:     eksapi,
		SSMAPI:     ssmapi,
		IAMAPI:     iamapi,
		PricingAPI: fakePricingAPI,

		AMICache:                      amiCache,
		EC2Cache:                      ec2Cache,
		InstanceTypeCache:             instanceTypeCache,
		LaunchTemplateCache:           launchTemplateCache,
		SubnetCache:                   subnetCache,
		AvailableIPAdressCache:        availableIPAdressCache,
		AssociatePublicIPAddressCache: associatePublicIPAddressCache,
		SecurityGroupCache:            securityGroupCache,
		InstanceProfileCache:          instanceProfileCache,
		UnavailableOfferingsCache:     unavailableOfferingsCache,
		SSMCache:                      ssmCache,
		DiscoveredCapacityCache:       discoveredCapacityCache,

		InstanceTypesResolver:   instanceTypesResolver,
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
	env.Clock.SetTime(time.Time{})
	env.EC2API.Reset()
	env.EKSAPI.Reset()
	env.SSMAPI.Reset()
	env.IAMAPI.Reset()
	env.PricingAPI.Reset()
	env.PricingProvider.Reset()
	env.InstanceTypesProvider.Reset()

	env.AMICache.Flush()
	env.EC2Cache.Flush()
	env.UnavailableOfferingsCache.Flush()
	env.LaunchTemplateCache.Flush()
	env.SubnetCache.Flush()
	env.AssociatePublicIPAddressCache.Flush()
	env.AvailableIPAdressCache.Flush()
	env.SecurityGroupCache.Flush()
	env.InstanceProfileCache.Flush()
	env.SSMCache.Flush()
	env.DiscoveredCapacityCache.Flush()
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

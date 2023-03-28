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

	"knative.dev/pkg/ptr"

	"github.com/patrickmn/go-cache"

	awscache "github.com/aws/karpenter/pkg/cache"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/instance"
	"github.com/aws/karpenter/pkg/providers/instancetype"
	"github.com/aws/karpenter/pkg/providers/launchtemplate"
	"github.com/aws/karpenter/pkg/providers/pricing"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"

	coretest "github.com/aws/karpenter-core/pkg/test"
)

type Environment struct {
	// API
	EC2API     *fake.EC2API
	SSMAPI     *fake.SSMAPI
	PricingAPI *fake.PricingAPI

	// Cache
	SSMCache                  *cache.Cache
	EC2Cache                  *cache.Cache
	KubernetesVersionCache    *cache.Cache
	InstanceTypeCache         *cache.Cache
	UnavailableOfferingsCache *awscache.UnavailableOfferings
	LaunchTemplateCache       *cache.Cache
	SubnetCache               *cache.Cache
	SecurityGroupCache        *cache.Cache

	// Providers
	InstanceTypesProvider  *instancetype.Provider
	InstanceProvider       *instance.Provider
	SubnetProvider         *subnet.Provider
	SecurityGroupProvider  *securitygroup.Provider
	PricingProvider        *pricing.Provider
	AMIProvider            *amifamily.Provider
	AMIResolver            *amifamily.Resolver
	LaunchTemplateProvider *launchtemplate.Provider
}

func NewEnvironment(ctx context.Context, env *coretest.Environment) *Environment {
	// API
	ec2api := &fake.EC2API{}
	ssmapi := &fake.SSMAPI{}

	// cache
	ssmCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	ec2Cache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	kubernetesVersionCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	instanceTypeCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	unavailableOfferingsCache := awscache.NewUnavailableOfferings()
	launchTemplateCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	subnetCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	securityGroupCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	fakePricingAPI := &fake.PricingAPI{}

	// Providers
	pricingProvider := pricing.NewProvider(ctx, fakePricingAPI, ec2api, "")
	subnetProvider := subnet.NewProvider(ec2api, subnetCache)
	securityGroupProvider := securitygroup.NewProvider(ec2api, securityGroupCache)
	amiProvider := amifamily.NewProvider(env.Client, env.KubernetesInterface, ssmapi, ec2api, ssmCache, ec2Cache, kubernetesVersionCache)
	amiResolver := amifamily.New(env.Client, amiProvider)
	instanceTypesProvider := instancetype.NewProvider("", instanceTypeCache, ec2api, subnetProvider, unavailableOfferingsCache, pricingProvider)
	launchTemplateProvider :=
		launchtemplate.NewProvider(
			ctx,
			launchTemplateCache,
			ec2api,
			amiResolver,
			securityGroupProvider,
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
		PricingAPI: fakePricingAPI,

		SSMCache:                  ssmCache,
		EC2Cache:                  ec2Cache,
		KubernetesVersionCache:    kubernetesVersionCache,
		InstanceTypeCache:         instanceTypeCache,
		LaunchTemplateCache:       launchTemplateCache,
		SubnetCache:               subnetCache,
		SecurityGroupCache:        securityGroupCache,
		UnavailableOfferingsCache: unavailableOfferingsCache,

		InstanceTypesProvider:  instanceTypesProvider,
		InstanceProvider:       instanceProvider,
		SubnetProvider:         subnetProvider,
		SecurityGroupProvider:  securityGroupProvider,
		PricingProvider:        pricingProvider,
		AMIProvider:            amiProvider,
		AMIResolver:            amiResolver,
		LaunchTemplateProvider: launchTemplateProvider,
	}
}

func (env *Environment) Reset() {
	env.EC2API.Reset()
	env.SSMAPI.Reset()
	env.PricingAPI.Reset()
	env.PricingProvider.Reset()

	env.SSMCache.Flush()
	env.EC2Cache.Flush()
	env.KubernetesVersionCache.Flush()
	env.InstanceTypeCache.Flush()
	env.UnavailableOfferingsCache.Flush()
	env.LaunchTemplateCache.Flush()
	env.SubnetCache.Flush()
	env.SecurityGroupCache.Flush()
}

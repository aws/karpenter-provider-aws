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
	"fmt"
	"net"

	"k8s.io/utils/clock"
	"knative.dev/pkg/ptr"

	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/imdario/mergo"
	"github.com/patrickmn/go-cache"
	"k8s.io/client-go/tools/record"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/events"

	awscache "github.com/aws/karpenter/pkg/cache"
	awscontext "github.com/aws/karpenter/pkg/context"
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

type ContextOptions struct {
	SSMCache                  *cache.Cache
	EC2Cache                  *cache.Cache
	KubernetesVersionCache    *cache.Cache
	InstanceTypeCache         *cache.Cache
	UnavailableOfferingsCache *awscache.UnavailableOfferings
	LaunchTemplateCache       *cache.Cache
	PricingAPI                *fake.PricingAPI
}

func Context(ctx context.Context, ec2api ec2iface.EC2API, ssmapi ssmiface.SSMAPI,
	env *coretest.Environment, clock clock.Clock, overrides ...ContextOptions) *awscontext.Context {
	options := ContextOptions{}
	for _, override := range overrides {
		if err := mergo.Merge(&options, override, mergo.WithOverride); err != nil {
			panic(fmt.Sprintf("Failed to merge settings: %s", err))
		}
	}

	// cache
	ssmCache := OptionOR(options.SSMCache, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	ec2Cache := OptionOR(options.EC2Cache, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	kubernetesVersionCache := OptionOR(options.KubernetesVersionCache, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	instanceTypeCache := OptionOR(options.InstanceTypeCache, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	unavailableOfferingsCache := OptionOR(options.UnavailableOfferingsCache, awscache.NewUnavailableOfferings())
	launchTemplateCache := OptionOR(options.LaunchTemplateCache, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval))
	pricingAPI := OptionOR(options.PricingAPI, &fake.PricingAPI{})

	// Providers
	pricingProvider := pricing.NewProvider(ctx, pricingAPI, ec2api, "", make(chan struct{}))
	subnetProvider := subnet.NewProvider(ec2api)
	securityGroupProvider := securitygroup.NewProvider(ec2api)
	amiProvider := amifamily.NewProvider(env.Client, env.KubernetesInterface, &fake.SSMAPI{}, ec2api, ssmCache, ec2Cache, kubernetesVersionCache)
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

	return &awscontext.Context{
		Context: cloudprovider.Context{
			Context:             ctx,
			RESTConfig:          env.Config,
			KubernetesInterface: env.KubernetesInterface,
			KubeClient:          env.Client,
			EventRecorder:       events.NewRecorder(&record.FakeRecorder{}),
			Clock:               clock,
			StartAsync:          nil,
		},
		Session:                   mock.Session,
		EC2API:                    ec2api,
		UnavailableOfferingsCache: unavailableOfferingsCache,
		InstanceTypesProvider:     instanceTypesProvider,
		InstanceProvider:          instanceProvider,
		SubnetProvider:            subnetProvider,
		SecurityGroupProvider:     securityGroupProvider,
		PricingProvider:           pricingProvider,
		AMIProvider:               amiProvider,
		AMIResolver:               amiResolver,
		LaunchTemplateProvider:    launchTemplateProvider,
	}
}

func OptionOR[T any](x *T, fallback *T) *T {
	if x == nil {
		return fallback
	}

	return x
}

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

	clock "k8s.io/utils/clock/testing"
	"knative.dev/pkg/ptr"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"k8s.io/client-go/tools/record"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
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
)

type ContextOptions struct {
	Session                   *session.Session
	UnavailableOfferingsCache *awscache.UnavailableOfferings
	SubnetProvider            *subnet.Provider
	SecurityGroupProvider     *securitygroup.Provider
	AMIProvider               *amifamily.Provider
	AMIResolver               *amifamily.Resolver
	LaunchTemplateProvider    *launchtemplate.Provider
	PricingProvider           *pricing.Provider
	InstanceTypesProvider     *instancetype.Provider
	InstanceProvider          *instance.Provider
}

func Context(ec2api ec2iface.EC2API, overrides ContextOptions) awscontext.Context {
	env := coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	var ctx context.Context
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, Settings())
	// ctx, stop := context.WithCancel(ctx)

	// cache
	ssmCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	ec2Cache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	kubernetesVersionCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	instanceTypeCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)
	unavailableOfferingsCache := awscache.NewUnavailableOfferings()
	launchTemplateCache := cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval)

	// Providers
	pricingProvider := pricing.NewProvider(ctx, &fake.PricingAPI{}, ec2api, "", make(chan struct{}))
	subnetProvider := subnet.NewProvider(ec2api)
	securityGroupProvider := securitygroup.NewProvider(ec2api)
	amiProvider := amifamily.NewProvider(env.Client, env.KubernetesInterface, &fake.SSMAPI{}, ec2api, ssmCache, ec2Cache, kubernetesVersionCache)
	amiResolver := amifamily.New(env.Client, amiProvider)
	instanceTypesProvider := instancetype.NewProvider("", instanceTypeCache, ec2api, subnetProvider, unavailableOfferingsCache, pricingProvider)
	launchTemplateProvider := launchtemplate.NewProvider(
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
	instanceProvider := instance.NewProvider(ctx,
		"",
		ec2api,
		unavailableOfferingsCache,
		instanceTypesProvider,
		subnetProvider,
		launchTemplateProvider,
	)

	return awscontext.Context{
		Context: cloudprovider.Context{
			Context:             ctx,
			RESTConfig:          env.Config,
			KubernetesInterface: env.KubernetesInterface,
			KubeClient:          env.Client,
			EventRecorder:       events.NewRecorder(&record.FakeRecorder{}),
			Clock:               &clock.FakeClock{},
			StartAsync:          nil,
		},
		Session:                   lo.FromPtrOr(&overrides.Session, mock.Session),
		EC2API:                    ec2api,
		UnavailableOfferingsCache: lo.FromPtrOr(&overrides.UnavailableOfferingsCache, unavailableOfferingsCache),
		InstanceTypesProvider:     lo.FromPtrOr(&overrides.InstanceTypesProvider, instanceTypesProvider),
		InstanceProvider:          lo.FromPtrOr(&overrides.InstanceProvider, instanceProvider),
		SubnetProvider:            lo.FromPtrOr(&overrides.SubnetProvider, subnetProvider),
		SecurityGroupProvider:     lo.FromPtrOr(&overrides.SecurityGroupProvider, securityGroupProvider),
		PricingProvider:           lo.FromPtrOr(&overrides.PricingProvider, pricingProvider),
		AMIProvider:               lo.FromPtrOr(&overrides.AMIProvider, amiProvider),
		AMIResolver:               lo.FromPtrOr(&overrides.AMIResolver, amiResolver),
		LaunchTemplateProvider:    lo.FromPtrOr(&overrides.LaunchTemplateProvider, launchTemplateProvider),
	}
}

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

package controllers

import (
	"context"

	"github.com/awslabs/operatorpkg/controller"
	"github.com/awslabs/operatorpkg/status"
	"github.com/patrickmn/go-cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/aws-sdk-go-v2/aws"
	servicesqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	crcapacitytype "github.com/aws/karpenter-provider-aws/pkg/controllers/capacityreservation/capacitytype"
	crexpiration "github.com/aws/karpenter-provider-aws/pkg/controllers/capacityreservation/expiration"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/metrics"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	nodeclasshash "github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/hash"
	controllersinstancetype "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/instancetype"
	controllersinstancetypecapacity "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/instancetype/capacity"
	controllerspricing "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/pricing"
	ssminvalidation "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/ssm/invalidation"
	controllersversion "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/version"
	capacityreservationprovider "github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/events"

	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption"
	nodeclaimgarbagecollection "github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclaim/garbagecollection"
	nodeclaimtagging "github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclaim/tagging"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/sqs"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
)

func NewControllers(
	ctx context.Context,
	mgr manager.Manager,
	cfg aws.Config,
	clk clock.Clock,
	ec2api sdk.EC2API,
	kubeClient client.Client,
	recorder events.Recorder,
	unavailableOfferings *awscache.UnavailableOfferings,
	ssmCache *cache.Cache,
	validationCache *cache.Cache,
	cloudProvider cloudprovider.CloudProvider,
	subnetProvider subnet.Provider,
	securityGroupProvider securitygroup.Provider,
	instanceProfileProvider instanceprofile.Provider,
	instanceProvider instance.Provider,
	pricingProvider pricing.Provider,
	amiProvider amifamily.Provider,
	launchTemplateProvider launchtemplate.Provider,
	versionProvider *version.DefaultProvider,
	instanceTypeProvider *instancetype.DefaultProvider,
	capacityReservationProvider capacityreservationprovider.Provider,
	amiResolver amifamily.Resolver,
) []controller.Controller {
	controllers := []controller.Controller{
		nodeclasshash.NewController(kubeClient),
		nodeclass.NewController(clk, kubeClient, cloudProvider, recorder, cfg.Region, subnetProvider, securityGroupProvider, amiProvider, instanceProfileProvider, instanceTypeProvider, launchTemplateProvider, capacityReservationProvider, ec2api, validationCache, amiResolver),
		nodeclaimgarbagecollection.NewController(kubeClient, cloudProvider),
		nodeclaimtagging.NewController(kubeClient, cloudProvider, instanceProvider),
		controllerspricing.NewController(pricingProvider),
		controllersinstancetype.NewController(instanceTypeProvider),
		controllersinstancetypecapacity.NewController(kubeClient, cloudProvider, instanceTypeProvider),
		ssminvalidation.NewController(ssmCache, amiProvider),
		status.NewController[*v1.EC2NodeClass](kubeClient, mgr.GetEventRecorderFor("karpenter"), status.EmitDeprecatedMetrics),
		controllersversion.NewController(versionProvider, versionProvider.UpdateVersionWithValidation),
		crcapacitytype.NewController(kubeClient, cloudProvider),
		crexpiration.NewController(clk, kubeClient, cloudProvider, capacityReservationProvider),
		metrics.NewController(kubeClient, cloudProvider),
	}
	if options.FromContext(ctx).InterruptionQueue != "" {
		sqsAPI := servicesqs.NewFromConfig(cfg)
		prov, _ := sqs.NewSQSProvider(ctx, sqsAPI)
		controllers = append(controllers, interruption.NewController(kubeClient, cloudProvider, clk, recorder, prov, sqsAPI, unavailableOfferings))
	}
	return controllers
}

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

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	nodeclass "github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	nodeclasshash "github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass/hash"
	controllersinstancetype "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/instancetype"
	controllersinstancetypecapacity "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/instancetype/capacity"
	controllerspricing "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/pricing"
	ssminvalidation "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/ssm/invalidation"
	controllersversion "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/version"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"

	servicesqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/samber/lo"
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
	kubeClient client.Client,
	recorder events.Recorder,
	unavailableOfferings *awscache.UnavailableOfferings,
	ssmCache *cache.Cache,
	cloudProvider cloudprovider.CloudProvider,
	subnetProvider subnet.Provider,
	securityGroupProvider securitygroup.Provider,
	instanceProfileProvider instanceprofile.Provider,
	instanceProvider instance.Provider,
	pricingProvider pricing.Provider,
	amiProvider amifamily.Provider,
	launchTemplateProvider launchtemplate.Provider,
	versionProvider *version.DefaultProvider,
	instanceTypeProvider *instancetype.DefaultProvider) []controller.Controller {
	controllers := []controller.Controller{
		nodeclasshash.NewController(kubeClient),
		nodeclass.NewController(kubeClient, recorder, subnetProvider, securityGroupProvider, amiProvider, instanceProfileProvider, launchTemplateProvider),
		nodeclaimgarbagecollection.NewController(kubeClient, cloudProvider),
		nodeclaimtagging.NewController(kubeClient, cloudProvider, instanceProvider),
		controllerspricing.NewController(pricingProvider),
		controllersinstancetype.NewController(instanceTypeProvider),
		controllersinstancetypecapacity.NewController(kubeClient, cloudProvider, instanceTypeProvider),
		ssminvalidation.NewController(ssmCache, amiProvider),
		status.NewController[*v1.EC2NodeClass](kubeClient, mgr.GetEventRecorderFor("karpenter"), status.EmitDeprecatedMetrics),
		controllersversion.NewController(versionProvider, versionProvider.UpdateVersionWithValidation),
	}
	if options.FromContext(ctx).InterruptionQueue != "" {
		sqsapi := servicesqs.NewFromConfig(cfg)
		out := lo.Must(sqsapi.GetQueueUrl(ctx, &servicesqs.GetQueueUrlInput{QueueName: lo.ToPtr(options.FromContext(ctx).InterruptionQueue)}))
		controllers = append(controllers, interruption.NewController(kubeClient, cloudProvider, clk, recorder, lo.Must(sqs.NewDefaultProvider(sqsapi, lo.FromPtr(out.QueueUrl))), unavailableOfferings))
	}
	return controllers
}

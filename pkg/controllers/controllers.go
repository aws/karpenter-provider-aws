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

	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	controllerspricing "github.com/aws/karpenter-provider-aws/pkg/controllers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"

	"github.com/aws/aws-sdk-go/aws/session"
	servicesqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/samber/lo"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/operator/controller"

	"github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption"
	nodeclaimgarbagecollection "github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclaim/garbagecollection"
	nodeclaimtagging "github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclaim/tagging"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/securitygroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/sqs"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"
)

func NewControllers(ctx context.Context, sess *session.Session, clk clock.Clock, kubeClient client.Client, recorder events.Recorder,
	unavailableOfferings *cache.UnavailableOfferings, cloudProvider cloudprovider.CloudProvider, subnetProvider *subnet.Provider,
	securityGroupProvider *securitygroup.Provider, instanceProfileProvider *instanceprofile.Provider, instanceProvider *instance.Provider,
	pricingProvider *pricing.Provider, amiProvider *amifamily.Provider, launchTemplateProvider *launchtemplate.Provider) []controller.Controller {

	controllers := []controller.Controller{
		nodeclass.NewController(kubeClient, recorder, subnetProvider, securityGroupProvider, amiProvider, instanceProfileProvider, launchTemplateProvider),
		nodeclaimgarbagecollection.NewController(kubeClient, cloudProvider),
		nodeclaimtagging.NewController(kubeClient, instanceProvider),
		controllerspricing.NewController(pricingProvider),
	}
	if options.FromContext(ctx).InterruptionQueue != "" {
		controllers = append(controllers, interruption.NewController(kubeClient, clk, recorder, lo.Must(sqs.NewProvider(ctx, servicesqs.New(sess), options.FromContext(ctx).InterruptionQueue)), unavailableOfferings))
	}
	return controllers
}

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

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/cache"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/interruption"
	machinegarbagecollection "github.com/aws/karpenter/pkg/controllers/machine/garbagecollection"
	machinelink "github.com/aws/karpenter/pkg/controllers/machine/link"
	"github.com/aws/karpenter/pkg/controllers/nodetemplate"
	"github.com/aws/karpenter/pkg/providers/amifamily"
	"github.com/aws/karpenter/pkg/providers/pricing"
	"github.com/aws/karpenter/pkg/providers/securitygroup"
	"github.com/aws/karpenter/pkg/providers/subnet"
	"github.com/aws/karpenter/pkg/utils/project"

	"github.com/aws/karpenter-core/pkg/operator/controller"
)

func NewControllers(ctx context.Context, sess *session.Session, clk clock.Clock, kubeClient client.Client, recorder events.Recorder,
	unavailableOfferings *cache.UnavailableOfferings, cloudProvider *cloudprovider.CloudProvider, subnetProvider *subnet.Provider,
	securityGroupProvider *securitygroup.Provider, pricingProvider *pricing.Provider, amiProvider *amifamily.Provider) []controller.Controller {

	logging.FromContext(ctx).With("version", project.Version).Debugf("discovered version")

	linkController := machinelink.NewController(kubeClient, cloudProvider)
	controllers := []controller.Controller{
		nodetemplate.NewNodeTemplateController(kubeClient, subnetProvider, securityGroupProvider, amiProvider),
		linkController,
		machinegarbagecollection.NewController(kubeClient, cloudProvider, linkController),
	}
	if settings.FromContext(ctx).InterruptionQueueName != "" {
		controllers = append(controllers, interruption.NewController(kubeClient, clk, recorder, interruption.NewSQSProvider(sqs.New(sess)), unavailableOfferings))
	}
	if settings.FromContext(ctx).IsolatedVPC {
		logging.FromContext(ctx).Infof("assuming isolated VPC, pricing information will not be updated")
	} else {
		controllers = append(controllers, pricing.NewController(pricingProvider))
	}
	return controllers
}

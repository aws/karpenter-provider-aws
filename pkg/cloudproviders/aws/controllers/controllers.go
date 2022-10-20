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
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
	"knative.dev/pkg/logging"

	awscontext "github.com/aws/karpenter/pkg/cloudproviders/aws/context"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/nodetemplate"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/providers"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/events"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/operator"
)

func GetControllers(ctx awscontext.Context, cluster *state.Cluster) []operator.Controller {
	rec := events.NewRecorder(ctx.EventRecorder)

	sqsProvider := providers.NewSQS(ctx, sqs.New(ctx.Session))
	eventBridgeProvider := providers.NewEventBridge(eventbridge.New(ctx.Session), sqsProvider)

	// Only enable spot interruption handling controllers when the feature flag is enabled
	//if options.Config.EnableInterruptionHandling() {
	logging.FromContext(ctx).Infof("Enabling interruption handling")

	nodeTemplateController := nodetemplate.NewController(ctx.KubeClient, sqsProvider, eventBridgeProvider)
	interruptionController := interruption.NewController(ctx.KubeClient, ctx.Clock, rec, cluster, sqsProvider, ctx.UnavailableOfferingsCache)
	//}
	return []operator.Controller{
		nodeTemplateController,
		interruptionController,
	}
}

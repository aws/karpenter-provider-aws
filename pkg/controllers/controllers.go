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

	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/operator/controller"
	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/pkg/controllers/nodetemplate"
	"github.com/aws/karpenter/pkg/controllers/providers"
	"github.com/aws/karpenter/pkg/events"
)

func NewControllers(ctx awscontext.Context, cluster *state.Cluster) []controller.Controller {
	rec := events.NewRecorder(ctx.EventRecorder)
	sqsProvider := providers.NewSQS(ctx, sqs.New(ctx.Session))
	eventBridgeProvider := providers.NewEventBridge(eventbridge.New(ctx.Session), sqsProvider)

	return []controller.Controller{
		nodetemplate.NewController(ctx.KubeClient, sqsProvider, eventBridgeProvider),
		interruption.NewController(ctx.KubeClient, ctx.Clock, rec, cluster, sqsProvider, ctx.UnavailableOfferingsCache),
	}
}

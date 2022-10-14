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

	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
	"k8s.io/utils/clock"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/cloudproviders/aws"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/events"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/infrastructure"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/notification"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/providers"
	"github.com/aws/karpenter/pkg/config"
	"github.com/aws/karpenter/pkg/controllers/state"
	coreevents "github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/operator"
)

type Options struct {
	aws.Options

	Config     config.Config
	Clock      clock.Clock
	Cluster    *state.Cluster
	Recorder   coreevents.Recorder
	KubeClient client.Client
}

func GetControllers(ctx context.Context, options Options) []operator.Controller {
	var ret []operator.Controller
	rec := events.NewRecorder(options.Recorder)

	sqsProvider := providers.NewSQSProvider(ctx, sqs.New(options.Session))
	eventBridgeProvider := providers.NewEventBridgeProvider(eventbridge.New(options.Session), sqsProvider)

	// Only enable spot interruption handling controllers when the feature flag is enabled
	if options.Config.EnableInterruptionHandling() {
		logging.FromContext(ctx).Infof("Enabling interruption handling")

		infraController := infrastructure.NewController(options.KubeClient, sqsProvider, eventBridgeProvider)
		notificationController := notification.NewController(options.KubeClient, options.Clock, rec, options.Cluster, sqsProvider, options.UnavailableOfferingsCache)
		ret = append(ret, infraController, notificationController)
	}
	return ret
}

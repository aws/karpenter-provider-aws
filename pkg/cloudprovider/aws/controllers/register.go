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

	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/infrastructure"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/nodetemplate"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/events"
	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/controllers/polling"
)

func Register(ctx context.Context, provider *aws.CloudProvider, opts *controllers.ControllerOptions) (ret []controllers.Controller) {
	rec := events.NewRecorder(opts.Recorder)

	// Only enable spot interruption handling controllers when the feature flag is enabled
	if opts.Config.EnableInterruptionHandling() {
		logging.FromContext(ctx).Infof("Enabling interruption handling")

		infraProvider := infrastructure.NewProvider(provider.SQSProvider(), provider.EventBridgeProvider())
		infraController := polling.NewController(infrastructure.NewReconciler(infraProvider))
		notificationController := polling.NewController(notification.NewReconciler(opts.KubeClient, rec, opts.Cluster, provider.SQSProvider(), provider.InstanceTypeProvider(), infraController))
		nodeTemplateController := nodetemplate.NewController(opts.KubeClient, infraProvider, infraController, notificationController)

		infraController.OnHealthy = notificationController.Start
		infraController.OnUnhealthy = notificationController.Stop
		ret = append(ret, infraController, notificationController, nodeTemplateController)
	}
	return ret
}

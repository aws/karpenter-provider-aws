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

	"github.com/aws/karpenter/pkg/operator"
)

func GetControllers(ctx context.Context, opts *operator.Options) []operator.Controller {
	var ret []operator.Controller
	//rec := events.NewRecorder(opts.Recorder)

	// Only enable spot interruption handling controllers when the feature flag is enabled
	//if opts.Config.EnableInterruptionHandling() {
	//	logging.FromContext(ctx).Infof("Enabling interruption handling")
	//
	//	infraController := infrastructure.NewController(opts.KubeClient, provider.SQSProvider(), provider.EventBridgeProvider())
	//	notificationController := notification.NewController(opts.KubeClient, opts.Clock, rec, opts.Cluster, provider.SQSProvider(), provider.InstanceTypeProvider())
	//	ret = append(ret, infraController, notificationController)
	//}
	return ret
}

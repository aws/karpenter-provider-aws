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
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/events"
	"github.com/aws/karpenter/pkg/controllers"
)

func Register(ctx context.Context, provider *aws.CloudProvider, opts *controllers.ControllerOptions) <-chan struct{} {
	rec := events.NewRecorder(opts.Recorder)
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("aws"))

	// Injecting the AWS-specific controllers that will start when opts.StartAsync is triggered
	infraController := infrastructure.NewController(ctx, opts.KubeClient, opts.Clock, rec, provider.SQSProvider(), provider.EventBridgeProvider(), opts.StartAsync, opts.CleanupAsync)
	notification.NewController(ctx, opts.KubeClient, opts.Clock, rec, opts.Cluster, provider.SQSProvider(), provider.InstanceTypeProvider(), infraController, opts.StartAsync)
	return infraController.Done()
}

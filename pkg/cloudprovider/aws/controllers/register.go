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
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/deployment"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/infrastructure"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/events"
	"github.com/aws/karpenter/pkg/controllers"
)

func Register(ctx context.Context, provider *aws.CloudProvider, manager manager.Manager, opts *controllers.ControllerOptions) {
	rec := events.NewRecorder(opts.Recorder)
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("aws"))

	// Injecting the cloudprovider-specific controllers that will start when opts.StartAsync is triggered
	// All these controllers should run with the same context since they rely on each other
	infraCtx, cancel := context.WithCancel(ctx)
	deploymentController := deployment.NewController(opts.KubeClient, cancel, provider.SQSProvider(), provider.EventBridgeProvider())
	infraController := infrastructure.NewController(infraCtx, opts.KubeClient, opts.Clock, rec, provider.SQSProvider(), provider.EventBridgeProvider(), opts.StartAsync)
	notification.NewController(infraCtx, opts.KubeClient, opts.Clock, provider.SQSProvider(), rec, opts.Provisioner, opts.Cluster, opts.StartAsync, infraController.Ready)

	// Register the controller-runtime controller with the global manager
	if err := deploymentController.Register(infraCtx, manager); err != nil {
		panic(err)
	}
}

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

package main

import (
	"github.com/samber/lo"
	"k8s.io/utils/clock"

	awscloudprovider "github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/context"
	awscontrollers "github.com/aws/karpenter/pkg/controllers"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/cloudprovider/metrics"
	"github.com/aws/karpenter-core/pkg/controllers"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/operator"
	"github.com/aws/karpenter-core/pkg/webhooks"
)

func main() {
	ctx, operator := operator.NewOperator()
	awsCtx := context.NewOrDie(cloudprovider.Context{
		Context:             ctx,
		Clock:               operator.Clock,
		RESTConfig:          operator.RESTConfig,
		KubeClient:          operator.GetClient(),
		KubernetesInterface: operator.KubernetesInterface,
		EventRecorder:       operator.EventRecorder,
		StartAsync:          operator.Elected(),
	})
	awsCloudProvider := awscloudprovider.New(awsCtx)
	lo.Must0(operator.AddHealthzCheck("cloud-provider", awsCloudProvider.LivenessProbe))
	cloudProvider := metrics.Decorate(awsCloudProvider)

	clusterState := state.NewCluster(operator.SettingsStore.InjectSettings(ctx), operator.Clock, operator.GetClient(), cloudProvider)
	operator.
		WithControllers(ctx, controllers.NewControllers(
			ctx,
			clock.RealClock{},
			operator.GetClient(),
			operator.KubernetesInterface,
			clusterState,
			operator.EventRecorder,
			operator.SettingsStore,
			cloudProvider,
		)...).
		WithControllers(ctx, awscontrollers.NewControllers(
			awsCtx,
			clusterState,
		)...).
		WithWebhooks(webhooks.NewWebhooks()...).
		Start(ctx)
}

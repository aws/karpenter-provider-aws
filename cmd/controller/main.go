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

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/cloudprovider/metrics"
	"github.com/aws/karpenter-core/pkg/controllers"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/operator"
	"github.com/aws/karpenter-core/pkg/webhooks"
	awscloudprovider "github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/context"
)

func main() {
	ctx, operator := operator.NewOperator()
	awscloudProvider := awscloudprovider.New(context.NewOrDie(cloudprovider.Context{
		Context:             ctx,
		Clock:               operator.Clock,
		KubeClient:          operator.GetClient(),
		KubernetesInterface: operator.KubernetesInterface,
		EventRecorder:       operator.EventRecorder,
		StartAsync:          operator.Elected(),
	}))
	lo.Must0(operator.AddHealthzCheck("cloud-provider", awscloudProvider.LivenessProbe))
	cloudProvider := metrics.Decorate(awscloudProvider)

	operator.
		WithControllers(ctx, controllers.NewControllers(
			ctx,
			clock.RealClock{},
			operator.GetClient(),
			operator.KubernetesInterface,
			state.NewCluster(operator.SettingsStore.InjectSettings(ctx), operator.Clock, operator.GetClient(), cloudProvider),
			operator.Recorder,
			operator.SettingsStore,
			cloudProvider,
		)...).
		WithWebhooks(webhooks.NewWebhooks()...).
		Start(ctx)
}

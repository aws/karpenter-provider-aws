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

	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/webhooks"

	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	"github.com/aws/karpenter-core/pkg/cloudprovider/metrics"
	corecontrollers "github.com/aws/karpenter-core/pkg/controllers"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/operator"
	corewebhooks "github.com/aws/karpenter-core/pkg/webhooks"
)

func main() {
	ctx, operator := operator.NewOperator()
	awsCtx := context.NewOrDie(corecloudprovider.Context{
		Context:             ctx,
		Clock:               operator.Clock,
		RESTConfig:          operator.RESTConfig,
		KubeClient:          operator.GetClient(),
		KubernetesInterface: operator.KubernetesInterface,
		EventRecorder:       operator.EventRecorder,
		StartAsync:          operator.Elected(),
	})
	awsCloudProvider := cloudprovider.New(awsCtx)
	lo.Must0(operator.AddHealthzCheck("cloud-provider", awsCloudProvider.LivenessProbe))
	cloudProvider := metrics.Decorate(awsCloudProvider)

	operator.
		WithControllers(ctx, corecontrollers.NewControllers(
			ctx,
			operator.Clock,
			operator.GetClient(),
			operator.KubernetesInterface,
			state.NewCluster(ctx, operator.Clock, operator.GetClient(), cloudProvider),
			operator.EventRecorder,
			operator.SettingsStore,
			cloudProvider,
		)...).
		WithWebhooks(corewebhooks.NewWebhooks()...).
		WithControllers(ctx, controllers.NewControllers(
			awsCtx,
		)...).
		WithWebhooks(webhooks.NewWebhooks()...).
		Start(ctx)
}

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
	"fmt"

	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/aws/karpenter-core/pkg/cloudprovider"
	cloudprovidermetrics "github.com/aws/karpenter-core/pkg/cloudprovider/metrics"
	"github.com/aws/karpenter-core/pkg/controllers"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/operator"
	awscloudprovider "github.com/aws/karpenter/pkg/cloudproviders/aws/cloudprovider"
	awscontext "github.com/aws/karpenter/pkg/cloudproviders/aws/context"
	awscontrollers "github.com/aws/karpenter/pkg/cloudproviders/aws/controllers"
)

func main() {
	ctx, manager := operator.NewOrDie()
	awsCtx := awscontext.NewOrDie(cloudprovider.Context{
		Context:       ctx,
		Clock:         ctx.Clock,
		ClientSet:     ctx.Clientset,
		KubeClient:    ctx.KubeClient,
		EventRecorder: ctx.BaseEventRecorder,
		StartAsync:    ctx.StartAsync,
	})
	awsCloudProvider := awscloudprovider.New(awsCtx)
	runtime.Must(manager.AddHealthzCheck("cloud-provider", awsCloudProvider.LivenessProbe))
	cloudProvider := cloudprovidermetrics.Decorate(awsCloudProvider)

	cluster := state.NewCluster(ctx.Clock, ctx.Config, ctx.KubeClient, cloudProvider)

	var conts []operator.Controller
	conts = append(conts, controllers.GetControllers(ctx, cluster, cloudProvider)...)
	conts = append(conts, awscontrollers.GetControllers(awsCtx, cluster)...)

	if err := operator.RegisterControllers(ctx,
		manager,
		conts...,
	).Start(ctx); err != nil {
		panic(fmt.Sprintf("Unable to start manager, %s", err))
	}
}

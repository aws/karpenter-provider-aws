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

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	"github.com/aws/karpenter-core/pkg/cloudprovider"
	cloudprovidermetrics "github.com/aws/karpenter-core/pkg/cloudprovider/metrics"
	"github.com/aws/karpenter-core/pkg/controllers"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/operator"
	"github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/settingsstore"
	awscloudprovider "github.com/aws/karpenter/pkg/cloudprovider"
	awscontext "github.com/aws/karpenter/pkg/context"
	awscontrollers "github.com/aws/karpenter/pkg/controllers"
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

	settingsStore := settingsstore.WatchSettings(ctx, ctx.ConfigMapWatcher, settings.Registration)
	if err := ctx.ConfigMapWatcher.Start(ctx.Done()); err != nil {
		panic(fmt.Errorf("starting ConfigMap watcher, %w", err))
	}

	// TODO: Remove settings injection once nominationPeriod no longer relies on it
	cluster := state.NewCluster(settingsStore.InjectSettings(ctx), ctx.Clock, ctx.KubeClient, cloudProvider)

	if err := operator.RegisterControllers(ctx,
		settingsStore,
		manager,
		lo.Flatten([][]controller.Controller{
			controllers.GetControllers(ctx, cluster, settingsStore, cloudProvider),
			awscontrollers.GetControllers(awsCtx, cluster),
		})...,
	).Start(ctx); err != nil {
		panic(fmt.Sprintf("Unable to start manager, %s", err))
	}
}

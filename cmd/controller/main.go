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

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/aws/karpenter/pkg/cloudproviders/aws"
	awscloudprovider "github.com/aws/karpenter/pkg/cloudproviders/aws/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider"
	cloudprovidermetrics "github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider/metrics"
	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/operator"
)

func main() {
	ctx, manager := operator.NewOrDie()
	awsCtx := aws.NewContextOrDie(cloudprovider.Context{
		Context:       ctx,
		ClientSet:     ctx.Clientset,
		KubeClient:    ctx.KubeClient,
		EventRecorder: ctx.BaseEventRecorder,
		StartAsync:    ctx.StartAsync,
	})
	awsCloudProvider := awscloudprovider.New(awsCtx)
	utilruntime.Must(manager.AddHealthzCheck("cloud-provider", awsCloudProvider.LivenessProbe))

	cloudProvider := cloudprovidermetrics.Decorate(awsCloudProvider)
	if err := operator.RegisterControllers(ctx,
		manager,
		controllers.GetControllers(ctx, cloudProvider)...,
	).Start(ctx); err != nil {
		panic(fmt.Sprintf("Unable to start manager, %s", err))
	}
}

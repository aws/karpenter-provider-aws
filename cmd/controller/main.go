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
	awscontrollers "github.com/aws/karpenter/pkg/cloudproviders/aws/controllers"
	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider"
	cloudprovidermetrics "github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider/metrics"
	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/operator"
)

func main() {
	options, manager := operator.NewOptionsWithManagerOrDie()
	awsOptions := aws.NewOptionsOrDie(options.Ctx, cloudprovider.Options{
		ClientSet:     options.Clientset,
		KubeClient:    options.KubeClient,
		EventRecorder: options.BaseEventRecorder,
		StartAsync:    options.StartAsync,
	})
	cloudProvider := cloudprovider.CloudProvider(awscloudprovider.New(options.Ctx, awsOptions))
	if hp, ok := cloudProvider.(operator.HealthCheck); ok {
		utilruntime.Must(manager.AddHealthzCheck("cloud-provider", hp.LivenessProbe))
	}
	cloudProvider = cloudprovidermetrics.Decorate(cloudProvider)

	cluster := state.NewCluster(options.Clock, options.Config, options.KubeClient, cloudProvider)

	var c []operator.Controller
	c = append(c, controllers.GetControllers(options, cluster, cloudProvider)...)
	c = append(c, awscontrollers.GetControllers(options.Ctx, awscontrollers.Options{
		Options:    awsOptions,
		Config:     options.Config,
		Clock:      options.Clock,
		Cluster:    cluster,
		KubeClient: options.KubeClient,
	})...)

	if err := operator.RegisterControllers(options.Ctx,
		manager,
		c...,
	).Start(options.Ctx); err != nil {
		panic(fmt.Sprintf("Unable to start manager, %s", err))
	}
}

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
	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/consolidation"
	"github.com/aws/karpenter/pkg/controllers/counter"
	metricspod "github.com/aws/karpenter/pkg/controllers/metrics/pod"
	metricsprovisioner "github.com/aws/karpenter/pkg/controllers/metrics/provisioner"
	metricsstate "github.com/aws/karpenter/pkg/controllers/metrics/state"
	"github.com/aws/karpenter/pkg/controllers/node"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/controllers/termination"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/operator"
)

func init() {
	metrics.MustRegister() // Registers cross-controller metrics
}

func GetControllers(ctx operator.Context, cloudProvider cloudprovider.CloudProvider) []operator.Controller {
	cluster := state.NewCluster(ctx.Clock, ctx.Config, ctx.KubeClient, cloudProvider)
	provisioner := provisioning.NewProvisioner(ctx, ctx.Config, ctx.KubeClient, ctx.Clientset.CoreV1(), ctx.EventRecorder, cloudProvider, cluster)

	metricsstate.StartMetricScraper(ctx, cluster)

	return []operator.Controller{
		provisioning.NewController(ctx.KubeClient, provisioner, ctx.EventRecorder),
		state.NewNodeController(ctx.KubeClient, cluster),
		state.NewPodController(ctx.KubeClient, cluster),
		state.NewProvisionerController(ctx.KubeClient, cluster),
		node.NewController(ctx.Clock, ctx.KubeClient, cloudProvider, cluster),
		termination.NewController(ctx, ctx.Clock, ctx.KubeClient, ctx.Clientset.CoreV1(), ctx.EventRecorder, cloudProvider),
		metricspod.NewController(ctx.KubeClient),
		metricsprovisioner.NewController(ctx.KubeClient),
		counter.NewController(ctx.KubeClient, cluster),
		consolidation.NewController(ctx.Clock, ctx.KubeClient, provisioner, cloudProvider, ctx.EventRecorder, cluster),
	}
}

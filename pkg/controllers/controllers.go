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
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider"
	"github.com/aws/karpenter/pkg/config"
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

func GetControllers(opts operator.Options, cloudProvider cloudprovider.CloudProvider) []operator.Controller {
	cfg, err := config.New(opts.Ctx, opts.Clientset, opts.Cmw)
	if err != nil {
		// this does not happen if the config map is missing or invalid, only if some other error occurs
		logging.FromContext(opts.Ctx).Fatalf("unable to load config, %s", err)
	}
	cluster := state.NewCluster(opts.Clock, cfg, opts.KubeClient, cloudProvider)
	provisioner := provisioning.NewProvisioner(opts.Ctx, cfg, opts.KubeClient, opts.Clientset.CoreV1(), opts.Recorder, cloudProvider, cluster)

	metricsstate.StartMetricScraper(opts.Ctx, cluster)

	return []operator.Controller{
		provisioning.NewController(opts.KubeClient, provisioner, opts.Recorder),
		state.NewNodeController(opts.KubeClient, cluster),
		state.NewPodController(opts.KubeClient, cluster),
		state.NewProvisionerController(opts.KubeClient, cluster),
		node.NewController(opts.Clock, opts.KubeClient, cloudProvider, cluster),
		termination.NewController(opts.Ctx, opts.Clock, opts.KubeClient, opts.Clientset.CoreV1(), opts.Recorder, cloudProvider),
		metricspod.NewController(opts.KubeClient),
		metricsprovisioner.NewController(opts.KubeClient),
		counter.NewController(opts.KubeClient, cluster),
		consolidation.NewController(opts.Clock, opts.KubeClient, provisioner, cloudProvider, opts.Recorder, cluster),
	}
}

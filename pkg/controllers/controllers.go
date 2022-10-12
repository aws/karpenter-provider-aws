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

	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/clock"
	"knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/cloudprovider"
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
	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/startup"
)

func init() {
	metrics.MustRegister() // Registers cross-controller metrics
}

func GetControllers(ctx context.Context, clk clock.Clock, cmw *informer.InformedWatcher, recorder events.Recorder, kubeClient client.Client, clientSet *kubernetes.Clientset, cloudProvider cloudprovider.CloudProvider) []startup.Controller {
	cfg, err := config.New(ctx, clientSet, cmw)
	if err != nil {
		// this does not happen if the config map is missing or invalid, only if some other error occurs
		logging.FromContext(ctx).Fatalf("unable to load config, %s", err)
	}
	cluster := state.NewCluster(clk, cfg, kubeClient, cloudProvider)
	provisioner := provisioning.NewProvisioner(ctx, cfg, kubeClient, clientSet.CoreV1(), recorder, cloudProvider, cluster)

	metricsstate.StartMetricScraper(ctx, cluster)

	return []startup.Controller{
		provisioning.NewController(kubeClient, provisioner, recorder),
		state.NewNodeController(kubeClient, cluster),
		state.NewPodController(kubeClient, cluster),
		state.NewProvisionerController(kubeClient, cluster),
		node.NewController(clk, kubeClient, cloudProvider, cluster),
		termination.NewController(ctx, clk, kubeClient, clientSet.CoreV1(), recorder, cloudProvider),
		metricspod.NewController(kubeClient),
		metricsprovisioner.NewController(kubeClient),
		counter.NewController(kubeClient, cluster),
		consolidation.NewController(clk, kubeClient, provisioner, cloudProvider, recorder, cluster),
	}
}

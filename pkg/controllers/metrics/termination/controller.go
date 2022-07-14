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

package termination

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/prometheus/client_golang/prometheus"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/functional"
)

const (
	controllerName = "terminationmetrics"
	provisioner    = "provisioner"
)

var (
	terminationProvisionerSummaryVec = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  "karpenter",
			Subsystem:  "nodes",
			Name:       "termination_provisioner_time_seconds",
			Help:       "The time taken between a node's deletion request and its termination, labeled by provisioner",
			Objectives: metrics.SummaryObjectives(),
		},
		[]string{provisioner},
	)

	terminationSummaryVec = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  "karpenter",
			Subsystem:  "nodes",
			Name:       "termination_time_seconds",
			Help:       "The time taken between a node's deletion request and its termination",
			Objectives: metrics.SummaryObjectives(),
		},
		[]string{},
	)
)

func init() {
	crmetrics.Registry.MustRegister(terminationProvisionerSummaryVec)
	crmetrics.Registry.MustRegister(terminationSummaryVec)
}

type Controller struct {
	kubeClient client.Client
	nodeRecord map[types.NamespacedName]*v1.Node
}

func NewController(kubeClient client.Client) *Controller {
	return &Controller{
		kubeClient: kubeClient,
		nodeRecord: make(map[types.NamespacedName]*v1.Node),
	}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName))

	// 1. Retrieve node from reconcile request
	node := &v1.Node{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			c.record(ctx, req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// 2. Check if the node is terminable
	if node.DeletionTimestamp.IsZero() || !functional.ContainsString(node.Finalizers, v1alpha5.TerminationFinalizer) {
		return reconcile.Result{}, nil
	}

	// 3. Record node object
	if _, ok := c.nodeRecord[req.NamespacedName]; !ok {
		c.nodeRecord[req.NamespacedName] = node
	}

	return reconcile.Result{}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1.Node{}).
		Complete(c)
}

func (c *Controller) record(ctx context.Context, nodeKey types.NamespacedName) {
	if node, ok := c.nodeRecord[nodeKey]; ok {
		duration := time.Since(node.DeletionTimestamp.Time)
		terminationProvisionerSummaryVec.With(c.getNodeLabels(node)).Observe(duration.Seconds())
		terminationSummaryVec.With(prometheus.Labels{}).Observe(duration.Seconds())
		delete(c.nodeRecord, nodeKey)
	}
}

func (c *Controller) getNodeLabels(node *v1.Node) prometheus.Labels {
	labels := prometheus.Labels{}
	for metricKey, nodeKey := range map[string]string{
		provisioner: v1alpha5.ProvisionerNameLabelKey,
	} {
		if value, ok := node.Labels[nodeKey]; ok {
			labels[metricKey] = value
		} else {
			labels[metricKey] = "N/A"
		}
	}
	return labels
}

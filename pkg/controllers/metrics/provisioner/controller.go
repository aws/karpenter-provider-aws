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

package provisioner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	"knative.dev/pkg/logging"

	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
)

const (
	resourceType    = "resource_type"
	provisionerName = "provisioner"
)

var (
	limitGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "provisioner",
			Name:      "limit",
			Help:      "The Provisioner Limits are the limits specified on the provisioner that restrict the quantity of resources provisioned. Labeled by provisioner name and resource type.",
		},
		labelNames(),
	)
	usageGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "provisioner",
			Name:      "usage",
			Help:      "The Provisioner Usage is the amount of resources that have been provisioned by a particular provisioner. Labeled by provisioner name and resource type.",
		},
		labelNames(),
	)
	usagePctGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "karpenter",
			Subsystem: "provisioner",
			Name:      "usage_pct",
			Help:      "The Provisioner Usage Percentage is the percentage of each resource used based on the resources provisioned and the limits that have been configured in the range [0,100].  Labeled by provisioner name and resource type.",
		},
		labelNames(),
	)
)

func init() {
	crmetrics.Registry.MustRegister(limitGaugeVec)
	crmetrics.Registry.MustRegister(usageGaugeVec)
	crmetrics.Registry.MustRegister(usagePctGaugeVec)
}

func labelNames() []string {
	return []string{
		resourceType,
		provisionerName,
	}
}

type Controller struct {
	kubeClient      client.Client
	labelCollection sync.Map
}

// NewController constructs a controller instance
func NewController(kubeClient client.Client) *Controller {
	return &Controller{
		kubeClient: kubeClient,
	}
}

// Reconcile executes a termination control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("provisionermetrics").With("provisioner", req.Name))

	// Remove the previous gauge after provisioner labels are updated
	c.cleanup(req.NamespacedName)

	// Retrieve provisioner from reconcile request
	provisioner := &v1alpha5.Provisioner{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, provisioner); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	c.record(ctx, provisioner)
	// periodically update our metrics per provisioner even if nothing has changed
	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named("provisionermetrics").
		For(&v1alpha5.Provisioner{}).
		Complete(c)
}

func (c *Controller) cleanup(provisionerName types.NamespacedName) {
	if labelSet, ok := c.labelCollection.Load(provisionerName); ok {
		for _, labels := range labelSet.([]prometheus.Labels) {
			limitGaugeVec.Delete(labels)
			usageGaugeVec.Delete(labels)
			usagePctGaugeVec.Delete(labels)
		}
	}
	c.labelCollection.Store(provisionerName, []prometheus.Labels{})
}

func (c *Controller) labels(provisioner *v1alpha5.Provisioner, resourceTypeName string) prometheus.Labels {
	metricLabels := prometheus.Labels{}
	metricLabels[resourceType] = resourceTypeName
	metricLabels[provisionerName] = provisioner.Name
	return metricLabels
}

func (c *Controller) record(ctx context.Context, provisioner *v1alpha5.Provisioner) {
	if err := c.set(provisioner.Status.Resources, provisioner, usageGaugeVec); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %s", err)
	}

	if provisioner.Spec.Limits == nil {
		// can't generate our limits or usagePct gauges if there are no limits
		return
	}

	if err := c.set(provisioner.Spec.Limits.Resources, provisioner, limitGaugeVec); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %s", err)
	}

	usage := v1.ResourceList{}
	for k, v := range provisioner.Spec.Limits.Resources {
		limitValue := v.AsApproximateFloat64()
		usedValue := provisioner.Status.Resources[k]
		if limitValue == 0 {
			usage[k] = *resource.NewQuantity(100, resource.DecimalSI)
		} else {
			usage[k] = *resource.NewQuantity(int64(usedValue.AsApproximateFloat64()/limitValue*100), resource.DecimalSI)
		}
	}

	if err := c.set(usage, provisioner, usagePctGaugeVec); err != nil {
		logging.FromContext(ctx).Errorf("Failed to generate gauge: %s", err)
	}
}

// set sets the value for the node gauge
func (c *Controller) set(resourceList v1.ResourceList, provisioner *v1alpha5.Provisioner, gaugeVec *prometheus.GaugeVec) error {
	for resourceName, quantity := range resourceList {
		resourceTypeName := strings.ReplaceAll(strings.ToLower(string(resourceName)), "-", "_")
		labels := c.labels(provisioner, resourceTypeName)

		provisionerName := types.NamespacedName{Name: provisioner.Name}
		existingLabels, _ := c.labelCollection.LoadOrStore(provisionerName, []prometheus.Labels{})
		existingLabels = append(existingLabels.([]prometheus.Labels), labels)
		c.labelCollection.Store(provisionerName, existingLabels)

		gauge, err := gaugeVec.GetMetricWith(labels)
		if err != nil {
			return fmt.Errorf("generate new gauge: %w", err)
		}
		if resourceName == v1.ResourceCPU {
			gauge.Set(float64(quantity.MilliValue()) / float64(1000))
		} else {
			gauge.Set(float64(quantity.Value()))
		}
	}
	return nil
}

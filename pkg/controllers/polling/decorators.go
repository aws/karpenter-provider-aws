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

package polling

import (
	"context"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter/pkg/metrics"
)

type ControllerWithHealthInterface interface {
	ControllerInterface

	Healthy() bool
}

// ControllerWithHealth is a Controller decorator that wraps a polling controller with health information
// on the success or failure of a reconciliation loop
type ControllerWithHealth struct {
	*Controller

	healthy atomic.Bool

	OnHealthy   func(context.Context)
	OnUnhealthy func(context.Context)
}

func NewControllerWithHealth(c *Controller) *ControllerWithHealth {
	return &ControllerWithHealth{
		Controller: c,
	}
}

func (c *ControllerWithHealth) Healthy() bool {
	return c.healthy.Load()
}

func (c *ControllerWithHealth) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	res, err := c.Controller.Reconcile(ctx, req)
	healthy := err == nil // The controller is considered healthy when it successfully reconciles
	if healthy {
		if c.OnHealthy != nil {
			c.OnHealthy(ctx)
		}
		c.healthyMetric().Set(1)
	} else {
		if c.OnUnhealthy != nil {
			c.OnUnhealthy(ctx)
		}
		c.healthyMetric().Set(0)
	}
	c.healthy.Store(healthy)
	return res, err
}

func (c *ControllerWithHealth) Builder(ctx context.Context, m manager.Manager) *controllerruntime.Builder {
	crmetrics.Registry.MustRegister(c.healthyMetric())
	return c.Controller.Builder(ctx, m)
}

func (c *ControllerWithHealth) Register(ctx context.Context, m manager.Manager) error {
	return c.Builder(ctx, m).Complete(c)
}

func (c *ControllerWithHealth) healthyMetric() prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: c.Controller.r.Metadata().MetricsSubsystem,
			Name:      "healthy",
			Help:      "Whether the controller is in a healthy state.",
		},
	)
}

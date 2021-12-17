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

package metrics

import (
	"context"
	"time"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/metrics"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	metricLabelComponent = "component"
	metricLabelMethod    = "method"
	metricLabelProvider  = "provider"
	metricLabelResult    = "result"
)

var methodDurationHistogramVec = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: metrics.Namespace,
		Subsystem: "cloudprovider",
		Name:      "duration_seconds",
		Help:      "Duration of cloud provider method calls.",
	},
	[]string{
		metricLabelComponent,
		metricLabelMethod,
		metricLabelProvider,
		metricLabelResult,
	},
)

func init() {
	crmetrics.Registry.MustRegister(methodDurationHistogramVec)
}

type decorator struct {
	cloudprovider.CloudProvider
}

// Decorate returns a new `CloudProvider` instance that will delegate all method
// calls to the argument, `cloudProvider`, and publish aggregated latency metrics. The
// value used for the metric label, "component", is taken from the `Context` object
// passed to the methods of `CloudProvider`.
func Decorate(cloudProvider cloudprovider.CloudProvider) cloudprovider.CloudProvider {
	switch c := cloudProvider.(type) {
	case *decorator:
		return c
	default:
		return &decorator{cloudProvider}
	}
}

func (d *decorator) Create(ctx context.Context, constraints *v1alpha5.Constraints, instanceTypes []cloudprovider.InstanceType, quantity int, callback func(*v1.Node) error) <-chan error {
	out := make(chan error)
	go func(startTime time.Time, in <-chan error) {
		select {
		case err := <-in:
			d.observe(ctx, "Create", time.Since(startTime), err)
			out <- err
		case <-ctx.Done():
		}
		close(out)
	}(time.Now(), d.CloudProvider.Create(ctx, constraints, instanceTypes, quantity, callback))
	return out
}

func (d *decorator) Delete(ctx context.Context, node *v1.Node) error {
	startTime := time.Now()
	err := d.CloudProvider.Delete(ctx, node)
	d.observe(ctx, "Delete", time.Since(startTime), err)
	return err
}

func (d *decorator) GetInstanceTypes(ctx context.Context, constraints *v1alpha5.Constraints) ([]cloudprovider.InstanceType, error) {
	startTime := time.Now()
	instanceTypes, err := d.CloudProvider.GetInstanceTypes(ctx, constraints)
	d.observe(ctx, "GetInstanceTypes", time.Since(startTime), err)
	return instanceTypes, err
}

func (d *decorator) Default(ctx context.Context, constraints *v1alpha5.Constraints) {
	startTime := time.Now()
	d.CloudProvider.Default(ctx, constraints)
	d.observe(ctx, "Default", time.Since(startTime), nil)
}

func (d *decorator) Validate(ctx context.Context, constraints *v1alpha5.Constraints) *apis.FieldError {
	startTime := time.Now()
	fieldErr := d.CloudProvider.Validate(ctx, constraints)
	d.observe(ctx, "Validate", time.Since(startTime), fieldErr)
	return fieldErr
}

func (d *decorator) observe(ctx context.Context, methodName string, duration time.Duration, err interface{}) {
	durationSeconds := duration.Seconds()

	labels := prometheus.Labels{
		metricLabelComponent: "unknown",
		metricLabelMethod:    methodName,
		metricLabelProvider:  d.Name(),
		metricLabelResult:    "success",
	}
	if componentName := injection.GetComponentName(ctx); componentName != "" {
		labels[metricLabelComponent] = componentName
	}
	if err != nil {
		labels[metricLabelResult] = "error"
	}
	observer, promErr := methodDurationHistogramVec.GetMetricWith(labels)
	if promErr != nil {
		logging.FromContext(ctx).Warnf(
			"Failed to record CloudProvider method duration metric [labels=%s, duration=%f]: error=%q",
			labels,
			durationSeconds,
			promErr.Error(),
		)
		return
	}

	observer.Observe(durationSeconds)
}

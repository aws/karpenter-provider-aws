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
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
)

// Metric contains the current value of a metric
type Metric struct {
	Name   string
	Labels map[string]string
	Value  float64
}

// Producer interface for all metrics implementations
type Producer interface {
	// Reconcile returns the current values for the set of metrics provided.
	Reconcile() error
}

// Client interface for all metrics implementations
type Client interface {
	// GetCurrentValues returns the current values for the specified metric.
	GetCurrentValue(v1alpha1.Metric) (Metric, error)
}

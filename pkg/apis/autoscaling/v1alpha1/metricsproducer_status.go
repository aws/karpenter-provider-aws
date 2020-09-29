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

package v1alpha1

import (
	"knative.dev/pkg/apis"
)

// MetricsProducerStatus defines the observed state of the resource.
// +kubebuilder:subresource:status
type MetricsProducerStatus struct {
	// Conditions is the set of conditions required for the metrics producer to
	// successfully public metrics to the metrics server
	Conditions apis.Conditions `json:"conditions,omitempty"`
	// LastUpdateTime is the last time the resource executed a control loop.
	LastUpdatedTime *apis.VolatileTime `json:"lastUpdatedTime,omitempty"`
}

var MetricsProducerConditions = apis.NewLivingConditionSet()

func (s *MetricsProducer) IsHappy() bool {
	return MetricsProducerConditions.Manage(s).IsHappy()
}

func (s *MetricsProducer) InitializeConditions() {
	MetricsProducerConditions.Manage(s).InitializeConditions()
}

func (s *MetricsProducer) GetConditions() apis.Conditions {
	return s.Status.Conditions
}

func (s *MetricsProducer) SetConditions(conditions apis.Conditions) {
	s.Status.Conditions = conditions
}

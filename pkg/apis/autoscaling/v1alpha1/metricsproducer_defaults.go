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

// +kubebuilder:webhook:verbs=create;update,path=/mutate-autoscaling-karpenter-sh-v1alpha1-metricsproducer,mutating=true,sideEffects=None,failurePolicy=fail,groups=autoscaling.karpenter.sh,resources=metricsproducers,versions=v1alpha1,name=mmetricsproducer.kb.io

package v1alpha1

import (
	"knative.dev/pkg/ptr"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ webhook.Defaulter = &MetricsProducer{}

type specDefaulter interface {
	defaultValues()
}

// Default implements webhook.Defaulter so a webhook will be registered
func (m *MetricsProducer) Default() {
	m.defaultValues()
}

// Default PendingCapacity
func (s *PendingCapacitySpec) defaultValues() {
}

// Default ReservedCapacity
func (s *ReservedCapacitySpec) defaultValues() {
}

// Default ScheduleSpec
func (s *ScheduleSpec) defaultValues() {
	if s.Timezone == nil {
		s.Timezone = ptr.String("UTC")
	}
}

func (s *QueueSpec) defaultValues() {
}

func (m *MetricsProducer) defaultValues() {
	for _, defaulter := range []specDefaulter{
		m.Spec.PendingCapacity,
		m.Spec.ReservedCapacity,
		m.Spec.Schedule,
		m.Spec.Queue,
	} {
		if !reflect.ValueOf(defaulter).IsNil() {
			defaulter.defaultValues()
		}
	}
}

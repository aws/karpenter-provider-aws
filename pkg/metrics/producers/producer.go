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

package producers

import (
	"github.com/ellistarn/karpenter/pkg/metrics"
	"k8s.io/client-go/informers"
)

// Producer interface for all metrics implementations
type Producer interface {
	// GetCurrentValues returns the current values for the set of metrics provided.
	GetCurrentValues() ([]metrics.Metric, error)
}

// Factory instantiates metrics producers
type Factory struct {
	InformerFactory informers.SharedInformerFactory
}

// NewPendingCapacityMetricsProducer instantiates a metrics producer
func (m *Factory) NewPendingCapacityMetricsProducer() Producer {
	return &PendingCapacity{
		Nodes: m.InformerFactory.Core().V1().Nodes().Lister(),
		Pods:  m.InformerFactory.Core().V1().Pods().Lister(),
	}
}

// NewReservedCapacityMetricsProducer instantiates a metrics producer
func (m *Factory) NewReservedCapacityMetricsProducer() Producer {
	return &ReservedCapacity{
		Nodes: m.InformerFactory.Core().V1().Nodes().Lister(),
		Pods:  m.InformerFactory.Core().V1().Pods().Lister(),
	}
}

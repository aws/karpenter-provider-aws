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

package pendingcapacity

import (
	"github.com/ellistarn/karpenter/pkg/metrics"
	v1 "k8s.io/client-go/listers/core/v1"
)

// MetricsProducer implements a Pending Capacity metric
type MetricsProducer struct {
	Nodes v1.NodeLister
	Pods  v1.PodLister
}

// GetCurrentValues of the metrics
func (m *MetricsProducer) GetCurrentValues() ([]metrics.Metric, error) {
	m.Nodes.Get("todo")
	m.Pods.Pods("namespace").Get("name")
	return nil, nil
}

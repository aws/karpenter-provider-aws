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

package reservedcapacity

import (
	"fmt"
	"github.com/ellistarn/karpenter/pkg/metrics/producers"
	v1 "k8s.io/api/core/v1"
)

const (
	Subsystem   string               = "reserved_capacity"
	Reserved    producers.MetricType = "reserved"
	Capacity    producers.MetricType = "capacity"
	Utilization producers.MetricType = "utilization"
)

func init() {
	for _, resource := range []v1.ResourceName{v1.ResourcePods, v1.ResourceCPU, v1.ResourceMemory} {
		for _, name := range []producers.MetricType{Reserved, Capacity, Utilization} {
			producers.RegisterNewGauge(Subsystem, FormatMetricString(resource, name))
		}
	}
}

func FormatMetricString(resource v1.ResourceName, metricType producers.MetricType) producers.MetricType {
	return producers.MetricType(fmt.Sprintf("%s_%s", string(resource), string(metricType)))
}

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
	"github.com/ellistarn/karpenter/pkg/metrics"
	v1 "k8s.io/api/core/v1"
)

const (
	Subsystem   = "reserved_capacity"
	Reserved    = "reserved"
	Capacity    = "capacity"
	Utilization = "utilization"
)

func init() {
	for _, resource := range []v1.ResourceName{v1.ResourcePods, v1.ResourceCPU, v1.ResourceMemory} {
		for _, name := range []string{Reserved, Capacity, Utilization} {
			metrics.RegisterNewGauge(Subsystem, fmt.Sprintf("%s_%s", resource, name))
		}
	}
}

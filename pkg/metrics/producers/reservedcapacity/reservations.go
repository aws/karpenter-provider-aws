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
	"math"
	"math/big"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Reservations struct {
	Resources map[v1.ResourceName]Reservation
}

func NewReservations() *Reservations {
	return &Reservations{
		Resources: map[v1.ResourceName]Reservation{
			v1.ResourceCPU: {
				Reserved: resource.NewQuantity(0, resource.DecimalSI),
				Capacity: resource.NewQuantity(0, resource.DecimalSI),
			},
			v1.ResourceMemory: {
				Reserved: resource.NewQuantity(0, resource.DecimalSI),
				Capacity: resource.NewQuantity(0, resource.DecimalSI),
			},
			v1.ResourcePods: {
				Reserved: resource.NewQuantity(0, resource.DecimalSI),
				Capacity: resource.NewQuantity(0, resource.DecimalSI),
			},
		},
	}
}

func (r *Reservations) Add(node *v1.Node, pods *v1.PodList) {
	for _, pod := range pods.Items {
		r.Resources[v1.ResourcePods].Reserved.Add(*resource.NewQuantity(1, resource.DecimalSI))
		for _, container := range pod.Spec.Containers {
			r.Resources[v1.ResourceCPU].Reserved.Add(*container.Resources.Requests.Cpu())
			r.Resources[v1.ResourceMemory].Reserved.Add(*container.Resources.Requests.Memory())
		}
	}
	r.Resources[v1.ResourcePods].Capacity.Add(*node.Status.Capacity.Pods())
	r.Resources[v1.ResourceCPU].Capacity.Add(*node.Status.Capacity.Cpu())
	r.Resources[v1.ResourceMemory].Capacity.Add(*node.Status.Capacity.Memory())
}

type Reservation struct {
	Reserved *resource.Quantity
	Capacity *resource.Quantity
}

func (r *Reservation) Compute() map[MetricType]float64 {
	var utilization = math.NaN()
	if r.Capacity.Value() != 0 {
		utilization, _ = big.NewRat(r.Reserved.Value(), r.Capacity.Value()).Float64()
	}

	return map[MetricType]float64{
		Reserved:    float64(r.Reserved.Value()),
		Capacity:    float64(r.Capacity.Value()),
		Utilization: utilization,
	}
}

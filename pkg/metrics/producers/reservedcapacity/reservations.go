package reservedcapacity

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Reservations struct {
	Resources map[v1.ResourceName]Reservation
}

func NewReservations(m v1alpha1.MetricsProducer) *Reservations {
	return &Reservations{
		Resources: map[v1.ResourceName]Reservation{
			v1.ResourceCPU: {
				Reserved: resource.NewQuantity(0, resource.DecimalSI),
				Total:    resource.NewQuantity(0, resource.DecimalSI),
				Gauge:    CpuGaugeVec.WithLabelValues(m.Name, m.Namespace),
			},
			v1.ResourceMemory: {
				Reserved: resource.NewQuantity(0, resource.DecimalSI),
				Total:    resource.NewQuantity(0, resource.DecimalSI),
				Gauge:    MemoryGaugeVec.WithLabelValues(m.Name, m.Namespace),
			},
			v1.ResourcePods: {
				Reserved: resource.NewQuantity(0, resource.DecimalSI),
				Total:    resource.NewQuantity(0, resource.DecimalSI),
				Gauge:    PodsGaugeVec.WithLabelValues(m.Name, m.Namespace),
			},
		},
	}
}

func (r *Reservations) Add(node *v1.Node, pods []*v1.Pod) {
	for _, pod := range pods {
		r.Resources[v1.ResourcePods].Reserved.Add(*resource.NewQuantity(1, resource.DecimalSI))
		for _, container := range pod.Spec.Containers {
			r.Resources[v1.ResourceCPU].Reserved.Add(*container.Resources.Requests.Cpu())
			r.Resources[v1.ResourceMemory].Reserved.Add(*container.Resources.Requests.Memory())
		}
	}
	r.Resources[v1.ResourceCPU].Total.Add(*node.Status.Capacity.Cpu())
	r.Resources[v1.ResourceMemory].Total.Add(*node.Status.Capacity.Memory())
	r.Resources[v1.ResourcePods].Total.Add(*node.Status.Capacity.Pods())
}

type Reservation struct {
	Reserved *resource.Quantity
	Total    *resource.Quantity
	Gauge    prometheus.Gauge
}

func (r *Reservation) Utilization() (float64, error) {
	reserved, _ := r.Reserved.AsInt64()
	total, _ := r.Total.AsInt64()
	if total == 0 {
		return 0, errors.Errorf("Unable to compute utilization of %d/%d", reserved, total)
	}
	return float64(reserved / total), nil
}

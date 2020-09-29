package reservedcapacity

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Reservations struct {
	CPU    Reservation
	Memory Reservation
	Pods   Reservation
}

func NewReservations(m v1alpha1.MetricsProducer) *Reservations {
	return &Reservations{
		CPU: Reservation{
			Reserved: resource.NewQuantity(0, resource.DecimalSI),
			Total:    resource.NewQuantity(0, resource.DecimalSI),
			Gauge:    CpuGaugeVec.WithLabelValues(m.Name, m.Namespace),
		},
		Memory: Reservation{
			Reserved: resource.NewQuantity(0, resource.DecimalSI),
			Total:    resource.NewQuantity(0, resource.DecimalSI),
			Gauge:    MemoryGaugeVec.WithLabelValues(m.Name, m.Namespace),
		},
		Pods: Reservation{
			Reserved: resource.NewQuantity(0, resource.DecimalSI),
			Total:    resource.NewQuantity(0, resource.DecimalSI),
			Gauge:    PodsGaugeVec.WithLabelValues(m.Name, m.Namespace),
		},
	}
}

func (r *Reservations) Add(node *v1.Node, pods []*v1.Pod) {
	for _, pod := range pods {
		r.Pods.Reserved.Add(*resource.NewQuantity(1, resource.DecimalSI))
		for _, container := range pod.Spec.Containers {
			r.CPU.Reserved.Add(*container.Resources.Requests.Cpu())
			r.Memory.Reserved.Add(*container.Resources.Requests.Memory())
		}
	}
	r.CPU.Total.Add(*node.Status.Capacity.Cpu())
	r.Memory.Total.Add(*node.Status.Capacity.Memory())
	r.Pods.Total.Add(*node.Status.Capacity.Pods())
}

func (r *Reservations) Record() {
	for _, reservation := range []Reservation{r.CPU, r.Memory, r.Pods} {
		reservation.Record()
	}
}

type Reservation struct {
	Reserved *resource.Quantity
	Total    *resource.Quantity
	Gauge    prometheus.Gauge
}

func (r *Reservation) Record() {
	reserved, _ := r.Reserved.AsInt64()
	total, _ := r.Total.AsInt64()
	if total != 0 {
		r.Gauge.Add(float64(reserved / total))
	}
}

package reservedcapacity

import (
	"fmt"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
)

// Producer implements a Reserved Capacity metric
type Producer struct {
	*v1alpha1.MetricsProducer
	Nodes listersv1.NodeLister
	Pods  listersv1.PodLister
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
	// 1. List Pods and Nodes
	nodes, err := p.Nodes.List(labels.Set(p.Spec.ReservedCapacity.NodeSelector).AsSelector())
	if err != nil {
		return errors.Wrapf(err, "Listing nodes for %s", p.Spec.ReservedCapacity.NodeSelector)
	}
	pods, err := p.Pods.Pods(metav1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return errors.Wrap(err, "Listing pods")
	}

	// 2. Calculate Pod Assignments,
	// TODO Make this an index for increased performance
	assignments := p.getAssignments(nodes, pods)

	// 3. Compute reservations
	reservations := p.getReservations(nodes, assignments)

	// 4. Record reservations and update status
	p.record(reservations)
	p.Status.LastUpdatedTime = &apis.VolatileTime{Inner: metav1.Now()}
	return nil
}

func (p *Producer) getAssignments(nodes []*v1.Node, pods []*v1.Pod) map[string][]*v1.Pod {
	assignments := map[string][]*v1.Pod{}
	for _, pod := range pods {
		assignments[pod.Spec.NodeName] = append(assignments[pod.Spec.NodeName], pod)
	}
	return assignments
}

func (p *Producer) getReservations(nodes []*v1.Node, assignments map[string][]*v1.Pod) *Reservations {
	reservations := NewReservations(p.MetricsProducer)
	for _, node := range nodes {
		reservations.Add(node, assignments[node.Name])
	}
	return reservations
}

func (p *Producer) record(reservations *Reservations) {
	if p.Status.ReservedCapacity == nil {
		p.Status.ReservedCapacity = &v1alpha1.ReservedCapacityStatus{
			Utilization: map[v1.ResourceName]string{},
		}
	}

	var result error
	for resource, reservation := range reservations.Resources {
		utilization, err := reservation.Utilization()
		if err != nil {
			result = multierr.Append(result, errors.Wrapf(err, "unable to compute utilization for %s", resource))
		} else {
			reservation.Gauge.Add(utilization)
			p.Status.ReservedCapacity.Utilization[resource] = fmt.Sprintf(
				"%d%%, %s/%s",
				int32(utilization*100),
				reservation.Reserved,
				reservation.Total,
			)
		}
	}
	if result != nil {
		p.MarkNotActive(result.Error())
	} else {
		p.MarkActive()
	}
}

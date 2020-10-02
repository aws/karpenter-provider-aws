package reservedcapacity

import (
	"fmt"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/apis"
)

// Producer implements a Reserved Capacity metric
type Producer struct {
	v1alpha1.MetricsProducer
	Nodes listersv1.NodeLister
	Pods  listersv1.PodLister
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
	// 1. List Pods and Nodes
	nodes, err := p.Nodes.List(labelSelector(p.Spec.ReservedCapacity.NodeSelector))
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

	var errors error
	for resource, reservation := range reservations.Resources {
		utilization, err := reservation.Utilization()
		if err != nil {
			errors = multierr.Append(errors, err)
		} else {
			reservation.Gauge.Add(utilization)
			p.Status.ReservedCapacity.Utilization[resource] = fmt.Sprintf(
				"%d, %s/%s",
				int32(utilization),
				reservation.Reserved,
				reservation.Total,
			)
		}
	}
	if errors != nil {
		p.MarkNotActive(errors.Error())
	} else {
		p.MarkActive()
	}
}

func labelSelector(selectorPairs map[string]string) labels.Selector {
	selector := labels.NewSelector()
	for key, value := range selectorPairs {
		if requirement, err := labels.NewRequirement(key, selection.Equals, []string{value}); err != nil {
			// Empty selector if unable to be parsed
			log.InvariantViolated(errors.Wrapf(err, "Failed to parse requirement from %s=%s", key, value).Error())
		} else {
			selector.Add(*requirement)
		}
	}
	return selector
}

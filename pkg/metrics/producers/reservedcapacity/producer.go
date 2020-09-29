package reservedcapacity

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"github.com/pkg/errors"
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
	// 1. List Nodes
	nodes, err := p.Nodes.List(labelSelectorOrDie(p.Spec.ReservedCapacity.NodeSelector))
	if err != nil {
		return errors.Wrapf(err, "Listing nodes for %s", p.Spec.ReservedCapacity.NodeSelector)
	}

	// 2. List Pods
	pods, err := p.Pods.List(labels.Everything())
	if err != nil {
		return errors.Wrap(err, "Listing pods")
	}

	// 3. Calculate Pod Assignments,
	// TODO Make this an index for increased performance
	assignments := map[string][]*v1.Pod{}
	for _, pod := range pods {
		assignments[pod.Spec.NodeName] = append(assignments[pod.Spec.NodeName], pod)
	}

	// 4. Sum Up Reservations
	reservations := NewReservations(p.MetricsProducer)
	for _, node := range nodes {
		reservations.Add(node, assignments[node.Name])
	}
	reservations.Record()

	p.Status.LastUpdatedTime = &apis.VolatileTime{Inner: metav1.Now()}
	return nil
}

func labelSelectorOrDie(selectorPairs map[string]string) labels.Selector {
	selector := labels.NewSelector()
	for key, value := range selectorPairs {
		if requirement, err := labels.NewRequirement(key, selection.Equals, []string{value}); err != nil {
			log.FatalInvariantViolated(errors.Wrapf(err, "Failed to parse requirement from %s=%s", key, value).Error())
		} else {
			selector.Add(*requirement)
		}
	}
	return selector
}

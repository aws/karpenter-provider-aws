package reservedcapacity

import (
	"context"
	"fmt"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Producer implements a Reserved Capacity metric
type Producer struct {
	*v1alpha1.MetricsProducer
	Client client.Client
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
	// 1. List nodes
	nodes := &v1.NodeList{}
	if err := p.Client.List(context.Background(), nodes, client.MatchingLabels(p.Spec.ReservedCapacity.NodeSelector)); err != nil {
		return fmt.Errorf("Listing nodes for %s, %w", p.Spec.ReservedCapacity.NodeSelector, err)
	}

	// 2. Compute reservations
	reservations := NewReservations(p.MetricsProducer)
	for _, node := range nodes.Items {
		pods := &v1.PodList{}
		if err := p.Client.List(context.Background(), pods, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
			return fmt.Errorf("Listing pods for %s, %w", node.Name, err)
		}
		reservations.Add(&node, pods)
	}

	// 3. Record reservations and update status
	p.record(reservations)
	return nil
}

func (p *Producer) record(reservations *Reservations) {
	if p.Status.ReservedCapacity == nil {
		p.Status.ReservedCapacity = map[v1.ResourceName]string{}
	}
	var errs error
	for resource, reservation := range reservations.Resources {
		utilization, err := reservation.Utilization()
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("unable to compute utilization for %s, %w", resource, err))
		} else {
			reservation.Gauge.Set(utilization)
			p.Status.ReservedCapacity[resource] = fmt.Sprintf(
				"%d%%, %s/%s",
				int32(utilization*100),
				reservation.Reserved,
				reservation.Total,
			)
		}
	}
	if errs != nil {
		p.StatusConditions().MarkFalse(v1alpha1.Calculable, "", errs.Error())
	} else {
		p.StatusConditions().MarkTrue(v1alpha1.Calculable)
	}
}

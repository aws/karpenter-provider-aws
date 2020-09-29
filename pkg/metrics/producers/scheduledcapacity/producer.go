package scheduledcapacity

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	v1 "k8s.io/client-go/listers/core/v1"
)

// Producer implements the ScheduledCapacity metric
type Producer struct {
	v1alpha1.ScheduledCapacitySpec
	Nodes v1.NodeLister
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
	return nil
}

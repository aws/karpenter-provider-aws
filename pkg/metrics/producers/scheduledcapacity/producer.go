package scheduledcapacity

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
)

// Producer implements the ScheduledCapacity metric
type Producer struct {
	*v1alpha1.MetricsProducer
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
	return nil
}

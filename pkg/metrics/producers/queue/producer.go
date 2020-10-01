package queue

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
)

// Producer implements a Pending Capacity metric
type Producer struct {
	v1alpha1.MetricsProducer
	Queue cloudprovider.Queue
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
	_, err := p.Queue.Length()
	if err != nil {
		return nil
	}
	return nil
}

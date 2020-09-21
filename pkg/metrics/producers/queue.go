package producers

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/metrics"
	"github.com/pkg/errors"
)

// Queue implements a Pending Capacity metric
type Queue struct {
	v1alpha1.QueueSpec
	Queue cloudprovider.Queue
}

// GetCurrentValues of the metrics
func (q *Queue) GetCurrentValues() ([]metrics.Metric, error) {
	length, err := q.Queue.Length()
	if err != nil {
		return nil, errors.Wrap(err, "retrieving queue length")
	}
	return []metrics.Metric{
		metrics.Metric{
			Value: float64(length),
		},
	}, nil
}

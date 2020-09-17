package queue

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/queue/aws"
	"go.uber.org/zap"
)

type Factory struct {
	// TODO dependencies
}

func (f *Factory) For(spec v1alpha1.QueueSpec) cloudprovider.Queue {
	switch spec.Type {
	case v1alpha1.AWSSQSQueueType:
		return &aws.SQSQueue{
			ARN: spec.ID,
		}
	}
	zap.S().Fatalf("Failed to instantiate queue: unexpected type  %s", spec.Type)
	return nil
}

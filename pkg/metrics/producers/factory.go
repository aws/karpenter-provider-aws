package producers

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	cloudproviderqueue "github.com/ellistarn/karpenter/pkg/cloudprovider/queue"
	"github.com/ellistarn/karpenter/pkg/metrics"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/pendingcapacity"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/queue"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/reservedcapacity"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/scheduledcapacity"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	v1 "k8s.io/client-go/listers/core/v1"
)

// Factory instantiates metrics producers
type Factory struct {
	NodeLister   v1.NodeLister
	PodLister    v1.PodLister
	QueueFactory cloudproviderqueue.Factory
}

func (f *Factory) For(mp v1alpha1.MetricsProducer) metrics.Producer {
	if mp.Spec.PendingCapacity != nil {
		return &pendingcapacity.Producer{
			MetricsProducer: mp,
			Nodes:           f.NodeLister,
			Pods:            f.PodLister,
		}
	}
	if mp.Spec.Queue != nil {
		return &queue.Producer{
			MetricsProducer: mp,
			Queue:           f.QueueFactory.For(*mp.Spec.Queue),
		}
	}
	if mp.Spec.ReservedCapacity != nil {
		return &reservedcapacity.Producer{
			MetricsProducer: mp,
			Nodes:           f.NodeLister,
			Pods:            f.PodLister,
		}
	}
	if mp.Spec.ScheduledCapacity != nil {
		return &scheduledcapacity.Producer{
			MetricsProducer: mp,
			Nodes:           f.NodeLister,
		}
	}
	log.InvariantViolated("Failed to instantiate metrics producer, no spec defined")
	return &metrics.NilProducer{}
}

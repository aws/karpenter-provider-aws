package producers

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	cloudproviderqueue "github.com/ellistarn/karpenter/pkg/cloudprovider/queue"
	"github.com/ellistarn/karpenter/pkg/metrics"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/pendingcapacity"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/queue"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/reservedcapacity"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/scheduledcapacity"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
)

// Factory instantiates metrics producers
type Factory struct {
	InformerFactory informers.SharedInformerFactory
	QueueFactory    cloudproviderqueue.Factory
}

func (f *Factory) For(mp v1alpha1.MetricsProducer) metrics.Producer {
	if mp.Spec.PendingCapacity != nil {
		return &pendingcapacity.Producer{
			PendingCapacitySpec: *mp.Spec.PendingCapacity,
			Nodes:               f.InformerFactory.Core().V1().Nodes().Lister(),
			Pods:                f.InformerFactory.Core().V1().Pods().Lister(),
		}
	}
	if mp.Spec.Queue != nil {
		return &queue.Producer{
			QueueSpec: *mp.Spec.Queue,
			Queue:     f.QueueFactory.For(*mp.Spec.Queue),
		}
	}
	if mp.Spec.ReservedCapacity != nil {
		return &reservedcapacity.Producer{
			MetricsProducer: mp,
			Nodes:           f.InformerFactory.Core().V1().Nodes().Lister(),
			Pods:            f.InformerFactory.Core().V1().Pods().Lister(),
		}
	}
	if mp.Spec.ScheduledCapacity != nil {
		return &scheduledcapacity.Producer{
			ScheduledCapacitySpec: *mp.Spec.ScheduledCapacity,
			Nodes:                 f.InformerFactory.Core().V1().Nodes().Lister(),
		}
	}
	zap.S().Fatal("Failed to instantiate metrics producer no spec defined. Is the validation webhook installed?")
	return nil
}

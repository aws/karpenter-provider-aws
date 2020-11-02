/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package producers

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/metrics"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/fake"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/pendingcapacity"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/queue"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/reservedcapacity"
	"github.com/ellistarn/karpenter/pkg/metrics/producers/scheduledcapacity"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory instantiates metrics producers
type Factory struct {
	Client               client.Client
	CloudProviderFactory cloudprovider.Factory
}

func (f *Factory) For(mp *v1alpha1.MetricsProducer) metrics.Producer {
	if mp.Spec.PendingCapacity != nil {
		if err := mp.Spec.PendingCapacity.ValidatePendingCapacity(); err != nil {
			return &fake.FakeProducer{Error: fake.InvalidPendingCapacityError}
		}
		return &pendingcapacity.Producer{
			MetricsProducer: mp,
			Client:          f.Client,
		}
	}
	if mp.Spec.Queue != nil {
		if err := mp.Spec.Queue.ValidateQueue(); err != nil {
			return &fake.FakeProducer{Error: fake.InvalidQueueError}
		}
		return &queue.Producer{
			MetricsProducer: mp,
			Queue:           f.CloudProviderFactory.QueueFor(mp.Spec.Queue),
		}
	}
	if mp.Spec.ReservedCapacity != nil {
		if err := mp.Spec.ReservedCapacity.ValidateReservedCapacity(); err != nil {
			return &fake.FakeProducer{Error: fake.InvalidReservedCapacityError}
		}
		return &reservedcapacity.Producer{
			MetricsProducer: mp,
			Client:          f.Client,
		}
	}
	if mp.Spec.ScheduledCapacity != nil {
		if err := mp.Spec.ScheduledCapacity.ValidateScheduledCapacity(); err != nil {
			return &fake.FakeProducer{Error: fake.InvalidScheduledCapacityError}
		}
		return &scheduledcapacity.Producer{
			MetricsProducer: mp,
		}
	}
	log.InvariantViolated("Failed to instantiate metrics producer, no spec defined")
	return &fake.FakeProducer{Error: fake.NotImplementedError}
}

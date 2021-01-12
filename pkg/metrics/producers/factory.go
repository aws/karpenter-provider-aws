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
	"github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/metrics"
	"github.com/awslabs/karpenter/pkg/metrics/producers/fake"
	"github.com/awslabs/karpenter/pkg/metrics/producers/pendingcapacity"
	"github.com/awslabs/karpenter/pkg/metrics/producers/queue"
	"github.com/awslabs/karpenter/pkg/metrics/producers/reservedcapacity"
	"github.com/awslabs/karpenter/pkg/metrics/producers/scheduledcapacity"
	"github.com/awslabs/karpenter/pkg/utils/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Factory instantiates metrics producers
type Factory struct {
	Client               client.Client
	CloudProviderFactory cloudprovider.Factory
}

func (f *Factory) For(mp *v1alpha1.MetricsProducer) metrics.Producer {
	if err := mp.Spec.Validate(); err != nil {
		return &fake.FakeProducer{WantErr: err}
	}
	if mp.Spec.PendingCapacity != nil {
		return &pendingcapacity.Producer{
			MetricsProducer: mp,
			Client:          f.Client,
		}
	}
	if mp.Spec.Queue != nil {
		return &queue.Producer{
			MetricsProducer: mp,
			Queue:           f.CloudProviderFactory.QueueFor(mp.Spec.Queue),
		}
	}
	if mp.Spec.ReservedCapacity != nil {
		return &reservedcapacity.Producer{
			MetricsProducer: mp,
			Client:          f.Client,
		}
	}
	if mp.Spec.Schedule != nil {
		return &scheduledcapacity.Producer{
			MetricsProducer: mp,
		}
	}
	log.InvariantViolated("Failed to instantiate metrics producer, no spec defined")
	return &fake.FakeProducer{WantErr: fake.NotImplementedError}
}

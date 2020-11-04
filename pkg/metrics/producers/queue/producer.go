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

package queue

import (
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/aws"
)

// Producer implements a Pending Capacity metric
type Producer struct {
	*v1alpha1.MetricsProducer
	Queue cloudprovider.Queue
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {

	p.setMetricType()
	length, err := p.Queue.Length()
	if err != nil {
		return err
	}
	oldestMessageAgeSeconds, err := p.Queue.OldestMessageAgeSeconds()
	if err != nil {
		return err
	}
	p.Status.Queue = &v1alpha1.QueueStatus{
		Length:                  length,
		OldestMessageAgeSeconds: oldestMessageAgeSeconds,
	}
	return nil

}

func (p *Producer) setMetricType() {
	if _, ok := p.Queue.(*aws.SQSQueue); ok {
		p.Status.MetricsType = v1alpha1.AWSSQSQueueMetricType
		return
	}
	p.Status.MetricsType = v1alpha1.UnknownMetricsType
}

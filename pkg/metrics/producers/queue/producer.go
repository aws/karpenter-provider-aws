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
	"fmt"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
)

// Producer implements a Pending Capacity metric
type Producer struct {
	*v1alpha1.MetricsProducer
	Queue cloudprovider.Queue
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
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

func (p *Producer) record(reservations *Reservations) {
	if p.Status.ReservedCapacity == nil {
		p.Status.ReservedCapacity = map[v1.ResourceName]string{}
	}
	var errs error
	for resource, reservation := range reservations.Resources {
		utilization, err := reservation.Utilization()
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("unable to compute utilization for %s, %w", resource, err))
		} else {
			reservation.Gauge.Set(utilization)
			p.Status.ReservedCapacity[resource] = fmt.Sprintf(
				"%d%%, %s/%s",
				int32(utilization*100),
				reservation.Reserved,
				reservation.Total,
			)
		}
	}
	if errs != nil {
		p.Status.Conditions[]
		MarkFalse(v1alpha1.Calculable, "", errs.Error())
	} else {
		p.StatusConditions().MarkTrue(v1alpha1.Calculable)
	}
}

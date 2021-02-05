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

package scheduledcapacity

import (
	"fmt"
	"github.com/awslabs/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/awslabs/karpenter/pkg/metrics"
	"time"
)

// Producer implements the ScheduledCapacity metric
type Producer struct {
	*v1alpha1.MetricsProducer
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
	var (
		loc *time.Location
		err error
	)
	// defaulting webhook ensures this is always defined
	loc, err = time.LoadLocation(*p.Spec.Schedule.Timezone)
	if err != nil {
		return fmt.Errorf("timezone was not a valid input")
	}
	now := time.Now().In(loc)

	p.Status.ScheduledCapacity = &v1alpha1.ScheduledCapacityStatus{
		CurrentValue: &p.Spec.Schedule.DefaultReplicas,
	}

	for _, behavior := range p.Spec.Schedule.Behaviors {
		// use Cron library to find the next time start and end next match
		startTime, err := crontabFrom(behavior.Start, loc).nextTime()
		if err != nil {
			return fmt.Errorf("start pattern is invalid: %w", err)
		}
		endTime, err := crontabFrom(behavior.End, loc).nextTime()
		if err != nil {
			return fmt.Errorf("end pattern is invalid: %w", err)
		}

		if !now.After(endTime) && (!endTime.After(startTime) || !startTime.After(now)) {
			// Since the way collisions are handled are by how they're ordered in the spec, stop on first match
			p.Status.ScheduledCapacity.CurrentValue = &behavior.Replicas
			break
		}
	}
	p.record()
	return nil
}

func (p *Producer) record() {
	metrics.Gauges[Subsystem][Value].
		WithLabelValues(p.Name, p.Namespace).
		Set(float64(*p.Status.ScheduledCapacity.CurrentValue))
}

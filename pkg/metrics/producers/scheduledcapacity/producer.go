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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
	"time"
)

// Producer implements the ScheduledCapacity metric
type Producer struct {
	*v1alpha1.MetricsProducer
	ResourceName string
}

// Reconcile of the metrics
func (p *Producer) Reconcile() error {
	var pastRecommendation, futureRecommendation int32 = 0, 0
	pastTime, futureTime := &apis.VolatileTime{Inner: metav1.Time{}}, &apis.VolatileTime{Inner: metav1.Time{}}

	timeRanges := p.Spec.ScheduledCapacity.Schedules

	for _, timeRange := range timeRanges {
		start, err := time.Parse(time.RFC3339, timeRange.StartTime)
		if err != nil {
			return fmt.Errorf("start of time range unable to be parsed: %w", err)
		}

		end, err := time.Parse(time.RFC3339, timeRange.EndTime)
		if err != nil {
			return fmt.Errorf("end of time range unable to be parsed: %w", err)
		}

		for _, s := range timeRange.States {
			// Parse crontab into set of bitsets
			schedSpec, err := parseCrontab(s.Crontab)
			if err != nil {
				return err
			}

			// Use ScheduleSpec to find the most recent matches in the future and past
			// Currently errors only if there's not a match in the past within the start and end range
			pastMatch, futureMatch, err := findMostRecentMatches(*schedSpec, start, end)
			if err != nil {
				return err
			}

			// If pastMatch is after the current pastTime, set it oto that
			if pastMatch.Inner.Time.After(pastTime.Inner.Time) {
				pastTime = pastMatch
				pastRecommendation = s.Replicas
			}
			if futureTime.Inner.Time.After(futureMatch.Inner.Time) {
				futureTime = futureMatch
				futureRecommendation = s.Replicas
			}
		}
	}
	p.Status.ScheduledCapacity = &v1alpha1.ScheduledCapacityStatus{
		LastChangeTime:        pastTime,
		CurrentRecommendation: ptr.Int32(pastRecommendation),
		NextChangeTime:        futureTime,
		NextRecommendation:    ptr.Int32(futureRecommendation),
	}
	p.record(float64(pastRecommendation))
	return nil
}

func (p *Producer) record(replicas float64) {
	metrics.Gauges[Subsystem]["recommendation"].
		WithLabelValues(p.Name, p.Namespace).
		Set(replicas)
}

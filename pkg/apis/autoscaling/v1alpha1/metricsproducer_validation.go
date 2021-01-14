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

package v1alpha1

import "fmt"

// +kubebuilder:object:generate=false
type QueueValidator func(*QueueSpec) error

var (
	queueValidator = map[QueueType]QueueValidator{}
)

func RegisterQueueValidator(queueType QueueType, validator QueueValidator) {
	queueValidator[queueType] = validator
}

// Validate Queue
func (mp *MetricsProducerSpec) Validate() error {
	if mp.Queue != nil {
		queueValidate, ok := queueValidator[mp.Queue.Type]
		if !ok {
			return fmt.Errorf("unexpected queue type %v", mp.Queue.Type)
		}
		if err := queueValidate(mp.Queue); err != nil {
			return fmt.Errorf("invalid Metrics Producer, %w", err)
		}
	}
	if mp.PendingCapacity != nil {
		return mp.PendingCapacity.validate()
	}
	if mp.ReservedCapacity != nil {
		return mp.ReservedCapacity.validate()
	}
	if mp.Schedule != nil {
		return mp.Schedule.validate()
	}
	return nil
}

// Validate PendingCapacity
func (s *PendingCapacitySpec) validate() error {
	return nil
}

// Validate ReservedCapacity
func (s *ReservedCapacitySpec) validate() error {
	if len(s.NodeSelector) != 1 {
		return fmt.Errorf("reserved capacity must refer to exactly one node selector")
	}
	return nil
}

// Validate ScheduledCapacity
func (s *ScheduleSpec) validate() error {
	return nil
}

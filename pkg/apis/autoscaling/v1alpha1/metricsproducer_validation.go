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

import (
	"fmt"
)

// +kubebuilder:object:generate=false
type QueueValidator func(*QueueSpec) error

// +kubebuilder:object:generate=false
type PendingCapacityValidator func(*PendingCapacitySpec) error

// +kubebuilder:object:generate=false
type ReservedCapacityValidator func(*ReservedCapacitySpec) error

// +kubebuilder:object:generate=false
type ScheduledCapacityValidator func(*ScheduledCapacitySpec) error

var (
	queueValidator = map[QueueType]QueueValidator{}
	// Currently keys are strings because this functionality has not been implemented yet
	pendingCapacityValidator   = map[string]PendingCapacityValidator{}
	reservedCapacityValidator  = map[string]ReservedCapacityValidator{}
	scheduledCapacityValidator = map[string]ScheduledCapacityValidator{}
)

func RegisterQueueValidator(queueType QueueType, validator QueueValidator) {
	queueValidator[queueType] = validator
}
func RegisterPendingCapacityValidator(pendingCapacity string, validator PendingCapacityValidator) {
	pendingCapacityValidator[pendingCapacity] = validator
}
func RegisterReservedCapacityValidator(reservedCapacity string, validator ReservedCapacityValidator) {
	reservedCapacityValidator[reservedCapacity] = validator
}
func RegisterScheduledCapacityValidator(scheduledCapacity string, validator ScheduledCapacityValidator) {
	scheduledCapacityValidator[scheduledCapacity] = validator
}

// Validate Queue
func (q *QueueSpec) ValidateQueue() error {
	queueValidate, ok := queueValidator[q.Type]
	if !ok {
		return fmt.Errorf("unexpected queue type %v", q.Type)
	}
	if err := queueValidate(q); err != nil {
		return fmt.Errorf("invalid Metrics Producer, %w", err)
	}
	return nil
}

// Validate PendingCapacity
func (p *PendingCapacitySpec) ValidatePendingCapacity() error {
	return nil
}

// Validate ReservedCapacity
func (p *ReservedCapacitySpec) ValidateReservedCapacity() error {
	return nil
}

// Validate ScheduledCapacity
func (p *ScheduledCapacitySpec) ValidateScheduledCapacity() error {
	return nil
}

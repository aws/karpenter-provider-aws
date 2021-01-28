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

// +kubebuilder:webhook:verbs=create;update,path=/validate-autoscaling-karpenter-sh-v1alpha1-metricsproducer,mutating=false,sideEffects=None,failurePolicy=fail,groups=autoscaling.karpenter.sh,resources=metricsproducers,versions=v1alpha1,name=vmetricsproducer.kb.io

package v1alpha1

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"regexp"
	"strings"
	"time"
)

// Only Validates ScheduledCapacity MetricsProducer right now
func (m *MetricsProducer) ValidateCreate() error {
	if err := m.ValidateSchedule(); err != nil  {
		return err
	}
	if m.Spec.ReservedCapacity != nil {
		return m.Spec.ReservedCapacity.validate()
	}
	if m.Spec.PendingCapacity != nil {
		return m.Spec.PendingCapacity.validate()
	}

	return nil
}

func (m *MetricsProducer) ValidateUpdate(old runtime.Object) error {
	return m.ValidateCreate()
}

func (m *MetricsProducer) ValidateDelete() error {
	return nil
}

func (m *MetricsProducer) ValidateSchedule() error {
	if m.Spec.Schedule != nil {
		for _, b := range m.Spec.Schedule.Behaviors {
			if err := b.Start.IsValidPattern(); err != nil {
				return fmt.Errorf("start pattern could not be parsed, %w", err)
			}
			if err := b.End.IsValidPattern(); err != nil {
				return fmt.Errorf("end pattern could not be parsed, %w", err)
			}
			if b.Replicas < 0 {
				return fmt.Errorf("behavior.replicas cannot be negative")
			}
		}
	}
	if m.Spec.Schedule.DefaultReplicas < 0 {
		return fmt.Errorf("defaultReplicas cannot be negative")
	}
	if m.Spec.Schedule.Timezone != nil {
		if _, err := time.LoadLocation(*m.Spec.Schedule.Timezone); err != nil {
			return fmt.Errorf("timezone region could not be parsed")
		}
	}
	return nil
}

// These regex patterns are meant to match to one element at a time
const (
	weekdayRegexPattern = "^((sun(day)?|0|7)|(mon(day)?|1)|(tue(sday)?|2)|(wed(nesday)?|3)|(thu(rsday)?|4)|(fri(day)?|5)|(sat(urday)?|6))$"
	monthRegexPattern = "^((jan(uary)?|1)|(feb(ruary)?|2)|(mar(ch)?|3)|(apr(il)?|4)|(may|5)|(june?|6)|(july?|7)|(aug(ust)?|8)|(sep(tember)?|9)|((oct(ober)?)|(10))|(nov(ember)?|(11))|(dec(ember)?|(12)))$"
	onlyNumbersPattern = `^\d+$`
)

func (p *Pattern) IsValidPattern() error {
	if !isValidField(p.Weekdays, weekdayRegexPattern) {
		return fmt.Errorf("weekdays field could not be parsed")
	}
	if !isValidField(p.Months, monthRegexPattern){
		return fmt.Errorf("months field could not be parsed")
	}
	if !isValidField(p.Days, onlyNumbersPattern){
		return fmt.Errorf("days field could not be parsed")
	}
	if !isValidField(p.Hours, onlyNumbersPattern) {
		return fmt.Errorf("hours field could not be parsed")
	}
	if !isValidField(p.Minutes, onlyNumbersPattern) {
		return fmt.Errorf("minutes field could not be parsed")
	}
	return nil
}

func isValidField(field *string, regexPattern string) bool {
	if field == nil {
		return true
	}
	elements := strings.Split(*field, ",")
	if len(elements) == 0 {
		return false
	}
	for _, elem := range elements {
		elem = strings.ToLower(strings.Trim(elem, " "))
		matched, _ := regexp.MatchString(regexPattern, elem)
		if !matched {
			return false
		}
	}
	return true
}

// +kubebuilder:object:generate=false
type QueueValidator func(*QueueSpec) error

var (
	queueValidator = map[QueueType]QueueValidator{}
)
func RegisterQueueValidator(queueType QueueType, validator QueueValidator) {
	queueValidator[queueType] = validator
}

// Validate CloudProvider Level
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
	if mp.ReservedCapacity != nil {
		return mp.ReservedCapacity.validate()
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
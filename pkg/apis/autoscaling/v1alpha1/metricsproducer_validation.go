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
	"reflect"
	"regexp"
	"strings"
	"time"
)
type specValidator interface {
	validate() error
}

// Only Validates ScheduledCapacity MetricsProducer right now
func (m *MetricsProducer) ValidateCreate() error {
	for _, validator := range []specValidator{
		m.Spec.PendingCapacity,
		m.Spec.ReservedCapacity,
		m.Spec.Schedule,
	} {
		if validator != nil {
			return validator.validate()
		}
	}
	return nil
}

func (m *MetricsProducer) ValidateUpdate(old runtime.Object) error {
	return m.ValidateCreate()
}

func (m *MetricsProducer) ValidateDelete() error {
	return nil
}

// Validate ScheduleSpec
func (s *ScheduleSpec) validate() error {
	for _, b := range s.Behaviors {
		if err := b.Start.validate(); err != nil {
			return fmt.Errorf("start pattern could not be parsed, %w", err)
		}
		if err := b.End.validate(); err != nil {
			return fmt.Errorf("end pattern could not be parsed, %w", err)
		}
		if b.Replicas < 0 {
			return fmt.Errorf("behavior.replicas cannot be negative")
		}
	}
	if s.DefaultReplicas < 0 {
		return fmt.Errorf("defaultReplicas cannot be negative")
	}
	if s.Timezone != nil {
		if _, err := time.LoadLocation(*s.Timezone); err != nil {
			return fmt.Errorf("timezone region could not be parsed")
		}
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

// These regex patterns are meant to match to one element at a time
const (
	weekdayRegexPattern = "^((sun(day)?|0|7)|(mon(day)?|1)|(tue(sday)?|2)|(wed(nesday)?|3)|(thu(rsday)?|4)|(fri(day)?|5)|(sat(urday)?|6))$"
	monthRegexPattern   = "^((jan(uary)?|1)|(feb(ruary)?|2)|(mar(ch)?|3)|(apr(il)?|4)|(may|5)|(june?|6)|(july?|7)|(aug(ust)?|8)|(sep(tember)?|9)|((oct(ober)?)|(10))|(nov(ember)?|(11))|(dec(ember)?|(12)))$"
	onlyNumbersPattern  = `^\d+$`
)

func (p *Pattern) validate() error {
	val := reflect.ValueOf(p)
	errorMessage := "%s field could not be parsed"
	for i, name := range []string{"Minutes","Hours","Days","Months","Weekdays"} {
		field := val.FieldByName(name).String()
		switch i {
		case 3:
			if !isValidField(&field, monthRegexPattern) {
				return fmt.Errorf(errorMessage, name)
			}
		case 4:
			if !isValidField(&field, weekdayRegexPattern) {
				return fmt.Errorf(errorMessage, name)
			}
		default:
			if !isValidField(&field, onlyNumbersPattern) {
				return fmt.Errorf(errorMessage, name)
			}
		}
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

// Validate at different level for cloud provider
func (mp *MetricsProducerSpec) ValidateQueue() error {
	queueValidate, ok := queueValidator[mp.Queue.Type]
	if !ok {
		return fmt.Errorf("unexpected queue type %v", mp.Queue.Type)
	}
	if err := queueValidate(mp.Queue); err != nil {
		return fmt.Errorf("invalid Metrics Producer, %w", err)
	}
	return nil
}

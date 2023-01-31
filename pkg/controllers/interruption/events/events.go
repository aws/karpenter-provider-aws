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

package events

import (
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/events"
)

func MachineSpotInterrupted(machine *v1alpha5.Machine) events.Event {
	return events.Event{
		InvolvedObject: machine,
		Type:           v1.EventTypeWarning,
		Reason:         "MachineSpotInterrupted",
		Message:        fmt.Sprintf("Machine %s event: A spot interruption warning was triggered for the machine", machine.Name),
		DedupeValues:   []string{machine.Name},
	}
}

func MachineRebalanceRecommendation(machine *v1alpha5.Machine) events.Event {
	return events.Event{
		InvolvedObject: machine,
		Type:           v1.EventTypeNormal,
		Reason:         "MachineSpotRebalanceRecommendation",
		Message:        fmt.Sprintf("Machine %s event: A spot rebalance recommendation was triggered for the machine", machine.Name),
		DedupeValues:   []string{machine.Name},
	}
}

func MachineStopping(machine *v1alpha5.Machine) events.Event {
	return events.Event{
		InvolvedObject: machine,
		Type:           v1.EventTypeWarning,
		Reason:         "InstanceStopping",
		Message:        fmt.Sprintf("Machine %s event: Machine is stopping", machine.Name),
		DedupeValues:   []string{machine.Name},
	}
}

func MachineTerminating(machine *v1alpha5.Machine) events.Event {
	return events.Event{
		InvolvedObject: machine,
		Type:           v1.EventTypeWarning,
		Reason:         "MachineTerminating",
		Message:        fmt.Sprintf("Machine %s event: Machine is terminating", machine.Name),
		DedupeValues:   []string{machine.Name},
	}
}

func MachineUnhealthy(machine *v1alpha5.Machine) events.Event {
	return events.Event{
		InvolvedObject: machine,
		Type:           v1.EventTypeWarning,
		Reason:         "MachineUnhealthy",
		Message:        fmt.Sprintf("Machine %s event: An unhealthy warning was triggered for the machine", machine.Name),
		DedupeValues:   []string{machine.Name},
	}
}

func MachineTerminatingOnInterruption(machine *v1alpha5.Machine) events.Event {
	return events.Event{
		InvolvedObject: machine,
		Type:           v1.EventTypeWarning,
		Reason:         "MachineTerminatingOnInterruption",
		Message:        fmt.Sprintf("Machine %s event: Interruption triggered termination for the machine", machine.Name),
		DedupeValues:   []string{machine.Name},
	}
}

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

func SpotInterrupted(node *v1.Node, machine *v1alpha5.Machine) []events.Event {
	evts := []events.Event{
		{
			InvolvedObject: machine,
			Type:           v1.EventTypeWarning,
			Reason:         "SpotInterrupted",
			Message:        fmt.Sprintf("Machine %s event: A spot interruption warning was triggered for the machine", machine.Name),
			DedupeValues:   []string{machine.Name},
		},
	}
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           v1.EventTypeWarning,
			Reason:         "SpotInterrupted",
			Message:        fmt.Sprintf("Node %s event: A spot interruption warning was triggered for the node", node.Name),
			DedupeValues:   []string{node.Name},
		})
	}
	return evts
}

func RebalanceRecommendation(node *v1.Node, machine *v1alpha5.Machine) []events.Event {
	evts := []events.Event{
		{
			InvolvedObject: machine,
			Type:           v1.EventTypeNormal,
			Reason:         "SpotRebalanceRecommendation",
			Message:        fmt.Sprintf("Machine %s event: A spot rebalance recommendation was triggered for the machine", machine.Name),
			DedupeValues:   []string{machine.Name},
		},
	}
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           v1.EventTypeNormal,
			Reason:         "SpotRebalanceRecommendation",
			Message:        fmt.Sprintf("Node %s event: A spot rebalance recommendation was triggered for the node", node.Name),
			DedupeValues:   []string{node.Name},
		})
	}
	return evts
}

func Stopping(node *v1.Node, machine *v1alpha5.Machine) []events.Event {
	evts := []events.Event{
		{
			InvolvedObject: machine,
			Type:           v1.EventTypeWarning,
			Reason:         "Stopping",
			Message:        fmt.Sprintf("Machine %s event: Machine is stopping", machine.Name),
			DedupeValues:   []string{machine.Name},
		},
	}
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           v1.EventTypeWarning,
			Reason:         "Stopping",
			Message:        fmt.Sprintf("Node %s event: Node is stopping", node.Name),
			DedupeValues:   []string{node.Name},
		})
	}
	return evts
}

func Terminating(node *v1.Node, machine *v1alpha5.Machine) []events.Event {
	evts := []events.Event{
		{
			InvolvedObject: machine,
			Type:           v1.EventTypeWarning,
			Reason:         "Terminating",
			Message:        fmt.Sprintf("Machine %s event: Machine is terminating", machine.Name),
			DedupeValues:   []string{machine.Name},
		},
	}
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           v1.EventTypeWarning,
			Reason:         "Terminating",
			Message:        fmt.Sprintf("Node %s event: Node is terminating", node.Name),
			DedupeValues:   []string{node.Name},
		})
	}
	return evts
}

func Unhealthy(node *v1.Node, machine *v1alpha5.Machine) []events.Event {
	evts := []events.Event{
		{
			InvolvedObject: machine,
			Type:           v1.EventTypeWarning,
			Reason:         "Unhealthy",
			Message:        fmt.Sprintf("Machine %s event: An unhealthy warning was triggered for the machine", machine.Name),
			DedupeValues:   []string{machine.Name},
		},
	}
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           v1.EventTypeWarning,
			Reason:         "Unhealthy",
			Message:        fmt.Sprintf("Node %s event: An unhealthy warning was triggered for the node", node.Name),
			DedupeValues:   []string{node.Name},
		})
	}
	return evts
}

func TerminatingOnInterruption(node *v1.Node, machine *v1alpha5.Machine) []events.Event {
	evts := []events.Event{
		{
			InvolvedObject: machine,
			Type:           v1.EventTypeWarning,
			Reason:         "TerminatingOnInterruption",
			Message:        fmt.Sprintf("Machine %s event: Interruption triggered termination for the machine", machine.Name),
			DedupeValues:   []string{machine.Name},
		},
	}
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           v1.EventTypeWarning,
			Reason:         "TerminatingOnInterruption",
			Message:        fmt.Sprintf("Node %s event: Interruption triggered termination for the node", node.Name),
			DedupeValues:   []string{node.Name},
		})
	}
	return evts
}

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

	"github.com/aws/karpenter/pkg/cloudproviders/aws/events"
)

func InstanceSpotInterrupted(n *v1.Node) events.Event {
	return events.Event{
		InvolvedObject: n,
		Type:           v1.EventTypeWarning,
		Reason:         "InstanceSpotInterrupted",
		Message:        fmt.Sprintf("Node %s event: A spot interruption warning was triggered for the node", n.Name),
	}
}

func InstanceRebalanceRecommendation(n *v1.Node) events.Event {
	return events.Event{
		InvolvedObject: n,
		Type:           v1.EventTypeNormal,
		Reason:         "InstanceSpotRebalanceRecommendation",
		Message:        fmt.Sprintf("Node %s event: A spot rebalance recommendation was triggered for the node", n.Name),
	}
}

func InstanceStopping(n *v1.Node) events.Event {
	return events.Event{
		InvolvedObject: n,
		Type:           v1.EventTypeWarning,
		Reason:         "InstanceStopping",
		Message:        fmt.Sprintf("Node %s event: Instance is stopping", n.Name),
	}
}

func InstanceTerminating(n *v1.Node) events.Event {
	return events.Event{
		InvolvedObject: n,
		Type:           v1.EventTypeWarning,
		Reason:         "InstanceTerminating",
		Message:        fmt.Sprintf("Node %s event: Instance is terminating", n.Name),
	}
}

func InstanceUnhealthy(n *v1.Node) events.Event {
	return events.Event{
		InvolvedObject: n,
		Type:           v1.EventTypeWarning,
		Reason:         "InstanceUnhealthy",
		Message:        fmt.Sprintf("Node %s event: An unhealthy warning was triggered for the node", n.Name),
	}
}

func NodeTerminatingOnInterruption(n *v1.Node) events.Event {
	return events.Event{
		InvolvedObject: n,
		Type:           v1.EventTypeWarning,
		Reason:         "NodeTerminatingOnInterruption",
		Message:        fmt.Sprintf("Node %s event: Interruption triggered termination for the node", n.Name),
	}
}

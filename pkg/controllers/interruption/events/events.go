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
	corev1 "k8s.io/api/core/v1"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
)

func SpotInterrupted(pods []*corev1.Pod, node *corev1.Node, nodeClaim *karpv1.NodeClaim) (evts []events.Event) {
	evts = append(evts, events.Event{
		InvolvedObject: nodeClaim,
		Type:           corev1.EventTypeWarning,
		Reason:         "SpotInterrupted",
		Message:        "Spot interruption warning was triggered",
		DedupeValues:   []string{string(nodeClaim.UID)},
	})
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           corev1.EventTypeWarning,
			Reason:         "SpotInterrupted",
			Message:        "Spot interruption warning was triggered",
			DedupeValues:   []string{string(node.UID)},
		})
	}
	for _, pod := range pods {
		evts = append(evts, events.Event{
			InvolvedObject: pod,
			Type:           corev1.EventTypeWarning,
			Reason:         "SpotInterrupted",
			Message:        "Pod will be evicted from the node due to a spot interruption warning",
			DedupeValues:   []string{string(pod.UID)},
		})
	}
	return evts
}

func RebalanceRecommendation(node *corev1.Node, nodeClaim *karpv1.NodeClaim) (evts []events.Event) {
	evts = append(evts, events.Event{
		InvolvedObject: nodeClaim,
		Type:           corev1.EventTypeNormal,
		Reason:         "SpotRebalanceRecommendation",
		Message:        "Spot rebalance recommendation was triggered",
		DedupeValues:   []string{string(nodeClaim.UID)},
	})
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           corev1.EventTypeNormal,
			Reason:         "SpotRebalanceRecommendation",
			Message:        "Spot rebalance recommendation was triggered",
			DedupeValues:   []string{string(node.UID)},
		})
	}
	return evts
}

func Stopping(node *corev1.Node, nodeClaim *karpv1.NodeClaim) (evts []events.Event) {
	evts = append(evts, events.Event{
		InvolvedObject: nodeClaim,
		Type:           corev1.EventTypeWarning,
		Reason:         "InstanceStopping",
		Message:        "Instance is stopping",
		DedupeValues:   []string{string(nodeClaim.UID)},
	})
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           corev1.EventTypeWarning,
			Reason:         "InstanceStopping",
			Message:        "Instance is stopping",
			DedupeValues:   []string{string(node.UID)},
		})
	}
	return evts
}

func Terminating(node *corev1.Node, nodeClaim *karpv1.NodeClaim) (evts []events.Event) {
	evts = append(evts, events.Event{
		InvolvedObject: nodeClaim,
		Type:           corev1.EventTypeWarning,
		Reason:         "InstanceTerminating",
		Message:        "Instance is terminating",
		DedupeValues:   []string{string(nodeClaim.UID)},
	})
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           corev1.EventTypeWarning,
			Reason:         "InstanceTerminating",
			Message:        "Instance is terminating",
			DedupeValues:   []string{string(node.UID)},
		})
	}
	return evts
}

func Unhealthy(node *corev1.Node, nodeClaim *karpv1.NodeClaim) (evts []events.Event) {
	evts = append(evts, events.Event{
		InvolvedObject: nodeClaim,
		Type:           corev1.EventTypeWarning,
		Reason:         "InstanceUnhealthy",
		Message:        "An unhealthy warning was triggered for the instance",
		DedupeValues:   []string{string(nodeClaim.UID)},
	})
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           corev1.EventTypeWarning,
			Reason:         "InstanceUnhealthy",
			Message:        "An unhealthy warning was triggered for the instance",
			DedupeValues:   []string{string(node.UID)},
		})
	}
	return evts
}

func TerminatingOnInterruption(node *corev1.Node, nodeClaim *karpv1.NodeClaim) (evts []events.Event) {
	evts = append(evts, events.Event{
		InvolvedObject: nodeClaim,
		Type:           corev1.EventTypeWarning,
		Reason:         "TerminatingOnInterruption",
		Message:        "Interruption triggered termination for the NodeClaim",
		DedupeValues:   []string{string(nodeClaim.UID)},
	})
	if node != nil {
		evts = append(evts, events.Event{
			InvolvedObject: node,
			Type:           corev1.EventTypeWarning,
			Reason:         "TerminatingOnInterruption",
			Message:        "Interruption triggered termination for the Node",
			DedupeValues:   []string{string(node.UID)},
		})
	}
	return evts
}

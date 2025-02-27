/*
Copyright The Kubernetes Authors.

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
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
)

func Launching(nodeClaim *v1.NodeClaim, reason string) events.Event {
	return events.Event{
		InvolvedObject: nodeClaim,
		Type:           corev1.EventTypeNormal,
		Reason:         events.DisruptionLaunching,
		Message:        fmt.Sprintf("Launching NodeClaim: %s", cases.Title(language.Und, cases.NoLower).String(reason)),
		DedupeValues:   []string{string(nodeClaim.UID), reason},
	}
}

func WaitingOnReadiness(nodeClaim *v1.NodeClaim) events.Event {
	return events.Event{
		InvolvedObject: nodeClaim,
		Type:           corev1.EventTypeNormal,
		Reason:         events.DisruptionWaitingReadiness,
		Message:        "Waiting on readiness to continue disruption",
		DedupeValues:   []string{string(nodeClaim.UID)},
	}
}

func Terminating(node *corev1.Node, nodeClaim *v1.NodeClaim, reason string) []events.Event {
	return []events.Event{
		{
			InvolvedObject: node,
			Type:           corev1.EventTypeNormal,
			Reason:         events.DisruptionTerminating,
			Message:        fmt.Sprintf("Disrupting Node: %s", cases.Title(language.Und, cases.NoLower).String(reason)),
			DedupeValues:   []string{string(node.UID), reason},
		},
		{
			InvolvedObject: nodeClaim,
			Type:           corev1.EventTypeNormal,
			Reason:         events.DisruptionTerminating,
			Message:        fmt.Sprintf("Disrupting NodeClaim: %s", cases.Title(language.Und, cases.NoLower).String(reason)),
			DedupeValues:   []string{string(nodeClaim.UID), reason},
		},
	}
}

// Unconsolidatable is an event that informs the user that a NodeClaim/Node combination cannot be consolidated
// due to the state of the NodeClaim/Node or due to some state of the pods that are scheduled to the NodeClaim/Node
func Unconsolidatable(node *corev1.Node, nodeClaim *v1.NodeClaim, msg string) []events.Event {
	return []events.Event{
		{
			InvolvedObject: node,
			Type:           corev1.EventTypeNormal,
			Reason:         events.Unconsolidatable,
			Message:        msg,
			DedupeValues:   []string{string(node.UID)},
			DedupeTimeout:  time.Minute * 15,
		},
		{
			InvolvedObject: nodeClaim,
			Type:           corev1.EventTypeNormal,
			Reason:         events.Unconsolidatable,
			Message:        msg,
			DedupeValues:   []string{string(nodeClaim.UID)},
			DedupeTimeout:  time.Minute * 15,
		},
	}
}

// Blocked is an event that informs the user that a NodeClaim/Node combination is blocked on deprovisioning
// due to the state of the NodeClaim/Node or due to some state of the pods that are scheduled to the NodeClaim/Node
func Blocked(node *corev1.Node, nodeClaim *v1.NodeClaim, msg string) (evs []events.Event) {
	if node != nil {
		evs = append(evs, events.Event{
			InvolvedObject: node,
			Type:           corev1.EventTypeNormal,
			Reason:         events.DisruptionBlocked,
			Message:        msg,
			DedupeValues:   []string{string(node.UID)},
		})
	}
	if nodeClaim != nil {
		evs = append(evs, events.Event{
			InvolvedObject: nodeClaim,
			Type:           corev1.EventTypeNormal,
			Reason:         events.DisruptionBlocked,
			Message:        msg,
			DedupeValues:   []string{string(nodeClaim.UID)},
		})
	}
	return evs
}

func NodePoolBlockedForDisruptionReason(nodePool *v1.NodePool, reason v1.DisruptionReason) events.Event {
	return events.Event{
		InvolvedObject: nodePool,
		Type:           corev1.EventTypeNormal,
		Reason:         events.DisruptionBlocked,
		Message:        fmt.Sprintf("No allowed disruptions for disruption reason %s due to blocking budget", reason),
		DedupeValues:   []string{string(nodePool.UID), string(reason)},
		DedupeTimeout:  1 * time.Minute,
	}
}

func NodePoolBlocked(nodePool *v1.NodePool) events.Event {
	return events.Event{
		InvolvedObject: nodePool,
		Type:           corev1.EventTypeNormal,
		Reason:         events.DisruptionBlocked,
		Message:        "No allowed disruptions due to blocking budget",
		DedupeValues:   []string{string(nodePool.UID)},
		// Set a small timeout as a NodePool's disruption budget can change every minute.
		DedupeTimeout: 1 * time.Minute,
	}
}

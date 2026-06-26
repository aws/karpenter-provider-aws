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

	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
)

func NodePoolFailedToResolveNodeClass(nodePool *karpv1.NodePool) events.Event {
	return events.Event{
		InvolvedObject: nodePool,
		Type:           corev1.EventTypeWarning,
		Message:        "Failed resolving NodeClass",
		DedupeValues:   []string{string(nodePool.UID)},
	}
}

func NodeClaimFailedToResolveNodeClass(nodeClaim *karpv1.NodeClaim) events.Event {
	return events.Event{
		InvolvedObject: nodeClaim,
		Type:           corev1.EventTypeWarning,
		Message:        "Failed resolving NodeClass",
		DedupeValues:   []string{string(nodeClaim.UID)},
	}
}

// NodePoolZonalShiftDetected is emitted when a zone the NodePool can provision into becomes shifted away from.
func NodePoolZonalShiftDetected(nodePool *karpv1.NodePool, zoneName, zoneID string) events.Event {
	return events.Event{
		InvolvedObject: nodePool,
		Type:           corev1.EventTypeWarning,
		Reason:         "ZonalShiftActive",
		Message:        fmt.Sprintf("Zonal shift detected: offerings in zone %s (%s) are unavailable for this NodePool", zoneName, zoneID),
		DedupeValues:   []string{string(nodePool.UID), zoneID},
	}
}

// NodePoolZonalShiftCleared is emitted when a previously shifted zone the NodePool can provision into is restored.
func NodePoolZonalShiftCleared(nodePool *karpv1.NodePool, zoneName, zoneID string) events.Event {
	return events.Event{
		InvolvedObject: nodePool,
		Type:           corev1.EventTypeNormal,
		Reason:         "ZonalShiftCleared",
		Message:        fmt.Sprintf("Zonal shift cleared: offerings in zone %s (%s) are restored for this NodePool", zoneName, zoneID),
		DedupeValues:   []string{string(nodePool.UID), zoneID},
	}
}

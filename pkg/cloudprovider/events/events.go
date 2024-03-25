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

	"github.com/aws/karpenter-provider-aws/pkg/utils"

	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/events"
)

func NodePoolFailedToResolveNodeClass(nodePool *v1beta1.NodePool) events.Event {
	return events.Event{
		InvolvedObject: nodePool,
		Type:           v1.EventTypeWarning,
		Message:        "Failed resolving NodeClass",
		DedupeValues:   []string{string(nodePool.UID)},
	}
}

func NodeClaimFailedToResolveNodeClass(nodeClaim *v1beta1.NodeClaim) events.Event {
	return events.Event{
		InvolvedObject: nodeClaim,
		Type:           v1.EventTypeWarning,
		Message:        "Failed resolving NodeClass",
		DedupeValues:   []string{string(nodeClaim.UID)},
	}
}

func FilteredInstanceTypes(nodeClaim *v1beta1.NodeClaim, filteredITsRequirement int, filteredExoticITs int, filteredExpensiveSpot int, filteredITs []string) events.Event {
	return events.Event{
		InvolvedObject: nodeClaim,
		Type:           v1.EventTypeNormal,
		Message: fmt.Sprintf("Filtered %d instance types on requirements, filtered %d instance types from exotic instance types, filtered %d instance types with spot offerings more expensive than cheapest on-demand. Remaining instance types:: %s",
			filteredITsRequirement, filteredExoticITs, filteredExpensiveSpot, utils.PrettySlice(filteredITs, 5)),
		DedupeValues: []string{string(nodeClaim.UID)},
	}
}

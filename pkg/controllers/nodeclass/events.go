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

package nodeclass

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/karpenter/pkg/events"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

func IncompatibleSubnetSecurityGroupsEvent(nodeClass *v1.EC2NodeClass, message, fingerprint string) events.Event {
	return events.Event{
		InvolvedObject: nodeClass,
		Type:           corev1.EventTypeWarning,
		Reason:         "IncompatibleSubnetSecurityGroups",
		Message:        message,
		DedupeValues:   []string{string(nodeClass.UID), fingerprint},
	}
}

func NetworkCompatibilityCheckFailedEvent(nodeClass *v1.EC2NodeClass, message string) events.Event {
	return events.Event{
		InvolvedObject: nodeClass,
		Type:           corev1.EventTypeWarning,
		Reason:         "NetworkCompatibilityCheckFailed",
		Message:        message,
		// Changing AWS errors should not flood events for a single NodeClass.
		DedupeValues:  []string{string(nodeClass.UID)},
		DedupeTimeout: 10 * time.Minute,
	}
}

func NetworkCompatibilityRestoredEvent(nodeClass *v1.EC2NodeClass) events.Event {
	return events.Event{
		InvolvedObject: nodeClass,
		Type:           corev1.EventTypeNormal,
		Reason:         "NetworkCompatibilityRestored",
		Message:        "All resolved subnets are compatible with all resolved security groups",
		DedupeValues:   []string{string(nodeClass.UID)},
	}
}

func WaitingOnNodeClaimTerminationEvent(nodeClass *v1.EC2NodeClass, names []string) events.Event {
	return events.Event{
		InvolvedObject: nodeClass,
		Type:           corev1.EventTypeNormal,
		Reason:         "WaitingOnNodeClaimTermination",
		Message:        fmt.Sprintf("Waiting on NodeClaim termination for %s", utils.PrettySlice(names, 5)),
		DedupeValues:   []string{string(nodeClass.UID)},
	}
}

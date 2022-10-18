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

package consolidation

import (
	"bytes"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"

	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
)

// DeprovisioningResult is used to indicate the action of consolidating so we can optimize by not trying to consolidate if
// we were unable to consolidate the cluster and it hasn't changed state with respect to pods/nodes.
type DeprovisioningResult byte

const (
	DeprovisioningResultNothingToDo DeprovisioningResult = iota // there are no actions that can be performed given the current cluster state
	DeprovisioningResultRetry                                   // we attempted an action, but its validation failed so retry soon
	DeprovisioningResultFailed                                  // the action failed entirely
	DeprovisioningResultSuccess                                 // the action was successful
)

func (r DeprovisioningResult) String() string {
	switch r {
	case DeprovisioningResultNothingToDo:
		return "Nothing to do"
	case DeprovisioningResultRetry:
		return "Retry"
	case DeprovisioningResultFailed:
		return "Failed"
	case DeprovisioningResultSuccess:
		return "Success"
	default:
		return fmt.Sprintf("Unknown (%d)", r)
	}
}

type deprovisionAction byte

const (
	deprovisionActionUnknown deprovisionAction = iota
	deprovisionActionNotPossible
	deprovisionActionDelete
	deprovisionActionDeleteEmpty
	deprovisionActionReplace
	deprovisionActionDoNothing
	deprovisionActionFailed
)

func (a deprovisionAction) String() string {
	switch a {
	case deprovisionActionUnknown:
		return "Unknown"
	case deprovisionActionNotPossible:
		return "Not Possible"
	case deprovisionActionDelete:
		return "Delete"
	case deprovisionActionDeleteEmpty:
		return "Delete (empty node)"
	case deprovisionActionReplace:
		return "Replace"
	case deprovisionActionDoNothing:
		return "NoAction"
	case deprovisionActionFailed:
		return "Failed"
	default:
		return fmt.Sprintf("Unknown (%d)", a)
	}
}

type lifecycleCommand struct {
	nodesToRemove   []*v1.Node
	action          deprovisionAction
	replacementNode *scheduling.Node
	created         time.Time
}

func (o lifecycleCommand) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s, terminating %d nodes ", o.action, len(o.nodesToRemove))
	for i, old := range o.nodesToRemove {
		if i != 0 {
			fmt.Fprint(&buf, ", ")
		}
		fmt.Fprintf(&buf, "%s", old.Name)
		if instanceType, ok := old.Labels[v1.LabelInstanceTypeStable]; ok {
			fmt.Fprintf(&buf, "/%s", instanceType)
		}
		if capacityType, ok := old.Labels[v1alpha5.LabelCapacityType]; ok {
			fmt.Fprintf(&buf, "/%s", capacityType)
		}
	}
	if o.replacementNode != nil {
		ct := o.replacementNode.Requirements.Get(v1alpha5.LabelCapacityType)
		nodeDesc := "node"
		// if there is a single capacity type value, we know what will launch. This makes it more clear
		// in logs why we replaced a node with one that at first glance appears more expensive when doing an OD->spot
		// replacement
		if ct.Len() == 1 {
			nodeDesc = fmt.Sprintf("%s node", ct.Any())
		}

		fmt.Fprintf(&buf, " and replacing with %s from types %s",
			nodeDesc,
			scheduling.InstanceTypeList(o.replacementNode.InstanceTypeOptions))
	}
	return buf.String()
}

func clamp(min, val, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

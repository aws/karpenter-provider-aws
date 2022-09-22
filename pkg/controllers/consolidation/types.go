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

	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
)

// ProcessResult is used to indicate the result of consolidating so we can optimize by not trying to consolidate if
// we were unable to consolidate the cluster and it hasn't changed state with respect to pods/nodes.
type ProcessResult byte

const (
	ProcessResultNothingToDo ProcessResult = iota
	ProcessResultFailed
	ProcessResultConsolidated
)

type consolidateResult byte

const (
	consolidateResultUnknown consolidateResult = iota
	consolidateResultNotPossible
	consolidateResultDelete
	consolidateResultDeleteEmpty
	consolidateResultReplace
	consolidateResultNoAction
)

func (r consolidateResult) String() string {
	switch r {
	case consolidateResultUnknown:
		return "Unknown"
	case consolidateResultNotPossible:
		return "Not Possible"
	case consolidateResultDelete:
		return "Delete"
	case consolidateResultDeleteEmpty:
		return "Delete (empty node)"
	case consolidateResultReplace:
		return "Replace"
	case consolidateResultNoAction:
		return "NoAction"
	default:
		return fmt.Sprintf("Unknown (%d)", r)
	}
}

type consolidationAction struct {
	oldNodes        []*v1.Node
	disruptionCost  float64
	result          consolidateResult
	replacementNode *scheduling.Node
}

func (o consolidationAction) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s, terminating %d nodes ", o.result, len(o.oldNodes))
	for i, old := range o.oldNodes {
		if i != 0 {
			fmt.Fprint(&buf, ", ")
		}
		fmt.Fprintf(&buf, "%s", old.Name)
		if instanceType, ok := old.Labels[v1.LabelInstanceTypeStable]; ok {
			fmt.Fprintf(&buf, "/%s", instanceType)
		}
	}
	if o.replacementNode != nil {
		fmt.Fprintf(&buf, " and replacing with a node from types %s",
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

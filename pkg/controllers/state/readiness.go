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

package state

import (
	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

func GetCondition(conditions []v1.NodeCondition, match v1.NodeConditionType) v1.NodeCondition {
	for _, condition := range conditions {
		if condition.Type == match {
			return condition
		}
	}
	return v1.NodeCondition{}
}

// isStartupTaintRemoved returns true if there are no startup taints registered for the provisioner, or if all startup
// taints have been removed from the node
func isStartupTaintRemoved(node *v1.Node, provisioner *v1alpha5.Provisioner) bool {
	if provisioner != nil {
		for _, startupTaint := range provisioner.Spec.StartupTaints {
			for i := 0; i < len(node.Spec.Taints); i++ {
				// if the node still has a startup taint applied, it's not ready
				if startupTaint.MatchTaint(&node.Spec.Taints[i]) {
					return false
				}
			}
		}
	}
	return true
}

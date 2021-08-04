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

package node

import (
	"time"

	v1 "k8s.io/api/core/v1"
)

func IsReady(node *v1.Node) bool {
	return getNodeCondition(node.Status.Conditions, v1.NodeReady).Status == v1.ConditionTrue
}

func FailedToJoin(node *v1.Node, gracePeriod time.Duration) bool {
	if time.Since(node.GetCreationTimestamp().Time) < gracePeriod {
		return false
	}
	condition := getNodeCondition(node.Status.Conditions, v1.NodeReady)
	return condition.LastHeartbeatTime.IsZero()
}

func getNodeCondition(conditions []v1.NodeCondition, match v1.NodeConditionType) v1.NodeCondition {
	for _, condition := range conditions {
		if condition.Type == match {
			return condition
		}
	}
	return v1.NodeCondition{}
}

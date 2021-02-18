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
	"fmt"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"time"
)

func IsReadyAndSchedulable(node v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			return condition.Status == v1.ConditionTrue && !node.Spec.Unschedulable
		}
	}
	return false
}

func IsPastTTL(node *v1.Node) (bool, error) {
	ttl, ok := node.Annotations[v1alpha1.ProvisionerTTLKey]
	if !ok {
		return false, nil
	}
	ttlTime, err := time.Parse(time.RFC3339, ttl)
	if err != nil {
		return false, fmt.Errorf("node %s did not have valid ttl", node.Name)
	}
	return time.Now().After(ttlTime), nil
}

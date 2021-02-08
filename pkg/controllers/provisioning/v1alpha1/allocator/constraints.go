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

package allocator

import (
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	v1 "k8s.io/api/core/v1"
)

type Constraints struct{}

func (c *Constraints) Group(pods []*v1.Pod) []*cloudprovider.Constraints {
	groups := []*cloudprovider.Constraints{}
	for _, pod := range pods {
		added := false
		for _, constraints := range groups {
			if matchesConstraints(constraints, pod) {
				constraints.Pods = append(constraints.Pods, pod)
				added = true
				break
			}
		}
		if added {
			continue
		}
		groups = append(groups, constraintsForPod(pod))
	}
	return groups
}

// TODO
func matchesConstraints(constraints *cloudprovider.Constraints, pod *v1.Pod) bool {
	return false
}

func constraintsForPod(pod *v1.Pod) *cloudprovider.Constraints {
	return &cloudprovider.Constraints{
		Overhead:     calculateOverheadResources(),
		Architecture: getSystemArchitecture(pod),
		Topology:     map[cloudprovider.TopologyKey]string{},
		Pods:         []*v1.Pod{pod},
	}
}

func calculateOverheadResources() v1.ResourceList {
	//TODO
	return v1.ResourceList{}
}

func getSystemArchitecture(pod *v1.Pod) cloudprovider.Architecture {
	return cloudprovider.ArchitectureLinux386
}

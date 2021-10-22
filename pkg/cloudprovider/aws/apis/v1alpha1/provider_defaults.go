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

package v1alpha1

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
)

var ClusterDiscoveryTagKeyFormat = "kubernetes.io/cluster/%s"

// Default the constraints.
func (c *Constraints) Default(ctx context.Context) {
	c.defaultCapacityTypes()
	c.defaultSubnets()
	c.defaultSecurityGroups()
	c.defaultTags()
}

func (c *Constraints) defaultCapacityTypes() {
	if _, ok := c.Labels[CapacityTypeLabel]; ok {
		return
	}
	for _, requirement := range c.Requirements {
		if requirement.Key == CapacityTypeLabel {
			return
		}
	}
	c.Requirements = append(c.Requirements, v1.NodeSelectorRequirement{
		Key:      CapacityTypeLabel,
		Operator: v1.NodeSelectorOpIn,
		Values:   []string{CapacityTypeOnDemand},
	})
}

func (c *Constraints) defaultSubnets() {
	if c.SubnetSelector != nil {
		return
	}
	c.SubnetSelector = map[string]string{fmt.Sprintf(ClusterDiscoveryTagKeyFormat, c.Cluster.Name): "*"}
}

func (c *Constraints) defaultTags() {
	if c.Tags != nil {
		return
	}
	c.Tags = make(map[string]string)
}

func (c *Constraints) defaultSecurityGroups() {
	if c.SecurityGroupSelector != nil {
		return
	}
	c.SecurityGroupSelector = map[string]string{fmt.Sprintf(ClusterDiscoveryTagKeyFormat, c.Cluster.Name): "*"}
}

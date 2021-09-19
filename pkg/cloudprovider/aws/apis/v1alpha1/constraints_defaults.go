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

	"knative.dev/pkg/ptr"
)

var ClusterDiscoveryTagKeyFormat = "kubernetes.io/cluster/%s"

// Default the constraints
func (c *Constraints) Default(ctx context.Context) {
	c.defaultCapacityType(ctx)
	c.defaultSubnets(ctx)
	c.defaultSecurityGroups(ctx)
}

func (c *Constraints) defaultCapacityType(ctx context.Context) {
	if capacityType, ok := c.Labels[CapacityTypeLabel]; ok {
		c.CapacityType = &capacityType
	}
	if c.CapacityType != nil {
		return
	}
	c.CapacityType = ptr.String(CapacityTypeOnDemand)
}

func (c *Constraints) defaultSubnets(ctx context.Context) {
	if c.SubnetSelector != nil {
		return
	}
	c.SubnetSelector = map[string]string{fmt.Sprintf(ClusterDiscoveryTagKeyFormat, c.Cluster.Name): ""}
}

func (c *Constraints) defaultSecurityGroups(ctx context.Context) {
	if c.SecurityGroupsSelector != nil {
		return
	}
	c.SecurityGroupsSelector = map[string]string{fmt.Sprintf(ClusterDiscoveryTagKeyFormat, c.Cluster.Name): ""}
}

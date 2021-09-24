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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	"github.com/awslabs/karpenter/pkg/scheduling"
	v1 "k8s.io/api/core/v1"
)

var ClusterDiscoveryTagKeyFormat = "kubernetes.io/cluster/%s"

// Default the constraints.
func (c *Constraints) Default(ctx context.Context) {
	c.defaultCapacityTypes(ctx)
	c.defaultSubnets(ctx)
	c.defaultSecurityGroups(ctx)
}

func (c *Constraints) defaultCapacityTypes(ctx context.Context) {
	if len(c.CapacityTypes) != 0 {
		return
	}
	c.CapacityTypes = []string{CapacityTypeOnDemand}
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

// Constrain applies the pod's scheduling constraints to the constraints.
// Returns an error if the constraints cannot be applied.
func (c *Constraints) Constrain(ctx context.Context, pods ...*v1.Pod) error {
	nodeAffinity := scheduling.NodeAffinityFor(pods...)
	capacityTypes := nodeAffinity.GetLabelValues(CapacityTypeLabel, c.CapacityTypes, v1alpha4.WellKnownLabels[CapacityTypeLabel])
	if len(capacityTypes) == 0 {
		return fmt.Errorf("no valid capacity types")
	}
	c.CapacityTypes = capacityTypes
	return nil
}

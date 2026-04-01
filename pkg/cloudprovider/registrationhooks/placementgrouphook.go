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

package registrationhooks

import (
	"context"
	"fmt"
	"strconv"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

// PlacementGroupRegistrationHook gates node registration until the placement group partition
// label is populated. For partition placement groups, EC2 auto-assigns the partition number
// and it is only discoverable via DescribeInstances after launch. This hook ensures the
// karpenter.k8s.aws/placement-group-partition label is set before the karpenter.sh/unregistered
// taint is removed, so that TopologySpreadConstraints using the partition topology key always
// see accurate partition data.
type PlacementGroupRegistrationHook struct {
	instanceProvider instance.Provider
}

func NewPlacementGroupRegistrationHook(instanceProvider instance.Provider) *PlacementGroupRegistrationHook {
	return &PlacementGroupRegistrationHook{
		instanceProvider: instanceProvider,
	}
}

func (h *PlacementGroupRegistrationHook) Name() string {
	return "PlacementGroupRegistrationHook"
}

func (h *PlacementGroupRegistrationHook) Registered(ctx context.Context, nodeClaim *karpv1.NodeClaim) (cloudprovider.NodeLifecycleHookResult, error) {
	if _, ok := nodeClaim.Labels[v1.LabelPlacementGroupID]; !ok {
		return cloudprovider.NodeLifecycleHookResult{}, nil
	}

	if _, ok := nodeClaim.Labels[v1.LabelPlacementGroupPartition]; ok {
		return cloudprovider.NodeLifecycleHookResult{}, nil
	}

	if nodeClaim.Status.ProviderID == "" {
		return cloudprovider.NodeLifecycleHookResult{Requeue: true}, nil
	}

	instanceID, err := utils.ParseInstanceID(nodeClaim.Status.ProviderID)
	if err != nil {
		return cloudprovider.NodeLifecycleHookResult{}, fmt.Errorf("parsing instance ID from provider ID, %w", err)
	}

	inst, err := h.instanceProvider.Get(ctx, instanceID, instance.SkipCache)
	if err != nil {
		return cloudprovider.NodeLifecycleHookResult{}, fmt.Errorf("describing instance for partition number, %w", err)
	}

	// If the instance doesn't have a partition number, it's not in a partition placement group
	// (e.g., cluster or spread PGs don't have partitions). Nothing to gate.
	if inst.PartitionNumber == nil {
		return cloudprovider.NodeLifecycleHookResult{}, nil
	}

	nodeClaim.Labels[v1.LabelPlacementGroupPartition] = strconv.FormatInt(int64(*inst.PartitionNumber), 10)
	return cloudprovider.NodeLifecycleHookResult{}, nil
}

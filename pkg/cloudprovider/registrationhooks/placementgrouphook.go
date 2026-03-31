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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

// PlacementGroupRegistrationHook gates node registration until the placement group partition
// label is populated. For partition placement groups, EC2 auto-assigns the partition number
// and it is only discoverable via DescribeInstances after launch. This hook ensures the
// karpenter.k8s.aws/placement-group-partition label is set before the karpenter.sh/unregistered
// taint is removed, so that TopologySpreadConstraints using the partition topology key always
// see accurate partition data.
type PlacementGroupRegistrationHook struct {
	kubeClient             client.Client
	instanceProvider       instance.Provider
	placementGroupProvider placementgroup.Provider
}

func NewPlacementGroupRegistrationHook(kubeClient client.Client, instanceProvider instance.Provider, placementGroupProvider placementgroup.Provider) *PlacementGroupRegistrationHook {
	return &PlacementGroupRegistrationHook{
		kubeClient:             kubeClient,
		instanceProvider:       instanceProvider,
		placementGroupProvider: placementGroupProvider,
	}
}

func (h *PlacementGroupRegistrationHook) Name() string {
	return "PlacementGroupRegistrationHook"
}

func (h *PlacementGroupRegistrationHook) Registered(ctx context.Context, nodeClaim *karpv1.NodeClaim) (cloudprovider.NodeLifecycleHookResult, error) {
	// Resolve the EC2NodeClass from the NodeClaim's nodeClassRef
	nodeClass := &v1.EC2NodeClass{}
	if err := h.kubeClient.Get(ctx, types.NamespacedName{Name: nodeClaim.Spec.NodeClassRef.Name}, nodeClass); err != nil {
		return cloudprovider.NodeLifecycleHookResult{}, fmt.Errorf("resolving ec2nodeclass for placement group hook, %w", err)
	}

	// Check if the EC2NodeClass has a partition placement group
	pg := h.placementGroupProvider.GetForNodeClass(nodeClass)
	if pg == nil || pg.Strategy != placementgroup.StrategyPartition {
		return cloudprovider.NodeLifecycleHookResult{}, nil
	}

	// Check if the partition label is already populated on the NodeClaim
	if _, ok := nodeClaim.Labels[v1.LabelPlacementGroupPartition]; ok {
		return cloudprovider.NodeLifecycleHookResult{}, nil
	}

	// We need the providerID to look up the instance
	if nodeClaim.Status.ProviderID == "" {
		return cloudprovider.NodeLifecycleHookResult{Requeue: true}, nil
	}

	// Parse the instance ID from the provider ID
	instanceID, err := utils.ParseInstanceID(nodeClaim.Status.ProviderID)
	if err != nil {
		return cloudprovider.NodeLifecycleHookResult{Requeue: true}, fmt.Errorf("parsing instance ID from provider ID, %w", err)
	}

	// Get the instance details, skipping cache to get fresh partition data
	inst, err := h.instanceProvider.Get(ctx, instanceID, instance.SkipCache)
	if err != nil {
		return cloudprovider.NodeLifecycleHookResult{Requeue: true}, fmt.Errorf("describing instance for partition number, %w", err)
	}

	// Check if the partition number has been assigned
	if inst.PartitionNumber == nil {
		return cloudprovider.NodeLifecycleHookResult{Requeue: true}, nil
	}

	// Set the partition label on the NodeClaim so it gets synced to the Node during registration
	nodeClaim.Labels[v1.LabelPlacementGroupPartition] = strconv.FormatInt(int64(*inst.PartitionNumber), 10)
	return cloudprovider.NodeLifecycleHookResult{}, nil
}

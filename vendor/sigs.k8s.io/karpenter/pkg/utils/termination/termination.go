/*
Copyright The Kubernetes Authors.

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

package termination

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

// EnsureTerminated is a helper function that takes a v1.NodeClaim and calls cloudProvider.Delete() if status condition
// on nodeClaim is not terminating. If it is terminating then it will call cloudProvider.Get() to check if the instance
// is terminated or not. It will return an error and a boolean that indicates if the instance is terminated or not. We simply return
// conflict or a NotFound error if we encounter it while updating the status on nodeClaim.
func EnsureTerminated(ctx context.Context, c client.Client, nodeClaim *v1.NodeClaim, cloudProvider cloudprovider.CloudProvider) (terminated bool, err error) {
	// Check if the status condition on nodeClaim is Terminating
	if !nodeClaim.StatusConditions().Get(v1.ConditionTypeInstanceTerminating).IsTrue() {
		// If not then call Delete on cloudProvider to trigger termination and always requeue reconciliation
		if err = cloudProvider.Delete(ctx, nodeClaim); err != nil {
			if cloudprovider.IsNodeClaimNotFoundError(err) {
				stored := nodeClaim.DeepCopy()
				updateStatusConditionsForDeleting(nodeClaim)
				// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
				// can cause races due to the fact that it fully replaces the list on a change
				// Here, we are updating the status condition list
				if err = c.Status().Patch(ctx, nodeClaim, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
					return false, err
				}
				// Instance is terminated
				return true, nil
			}
			return false, fmt.Errorf("terminating cloudprovider instance, %w", err)
		}

		stored := nodeClaim.DeepCopy()
		updateStatusConditionsForDeleting(nodeClaim)
		// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
		// can cause races due to the fact that it fully replaces the list on a change
		// Here, we are updating the status condition list
		if err = c.Status().Patch(ctx, nodeClaim, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); err != nil {
			return false, err
		}
		return false, nil
	}
	// Call Get on cloudProvider to check if the instance is terminated
	if _, err := cloudProvider.Get(ctx, nodeClaim.Status.ProviderID); err != nil {
		if cloudprovider.IsNodeClaimNotFoundError(err) {
			return true, nil
		}
		return false, fmt.Errorf("getting cloudprovider instance, %w", err)
	}
	return false, nil
}

func updateStatusConditionsForDeleting(nc *v1.NodeClaim) {
	// perform a no-op for whatever the status condition is currently set to
	// so that we bump the observed generation to the latest and prevent the nodeclaim
	// root status from entering an `Unknown` state
	for _, condition := range nc.Status.Conditions {
		nc.StatusConditions().Set(condition)
	}
	nc.StatusConditions().SetTrue(v1.ConditionTypeInstanceTerminating)
}

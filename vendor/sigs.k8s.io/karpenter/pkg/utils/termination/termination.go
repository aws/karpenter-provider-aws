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

	"k8s.io/apimachinery/pkg/api/equality"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

// EnsureTerminated is a helper function that takes a v1.NodeClaim and calls cloudProvider.Delete(). It will return an
// error and a boolean that indicates if the instance is terminated or not. We simply return conflict or a NotFound
// error if we encounter it while updating the status on nodeClaim.
func EnsureTerminated(ctx context.Context, c client.Client, nodeClaim *v1.NodeClaim, cloudProvider cloudprovider.CloudProvider) (terminated bool, err error) {
	stored := nodeClaim.DeepCopy()
	err = cloudProvider.Delete(ctx, nodeClaim)
	// Set InstanceTerminating to true when cloudProvider.Delete() returns -
	// 1. nodeClaim not found error which indicates the instance was terminated
	// 2. no error which indicates that Delete() has been processed but the instance has not terminated so requeue
	if err == nil || cloudprovider.IsNodeClaimNotFoundError(err) {
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeInstanceTerminating)
		if !equality.Semantic.DeepEqual(stored, nodeClaim) {
			// We use client.MergeFromWithOptimisticLock because patching a list with a JSON merge patch
			// can cause races due to the fact that it fully replaces the list on a change
			// Here, we are updating the status condition list
			if e := c.Status().Patch(ctx, nodeClaim, client.MergeFromWithOptions(stored, client.MergeFromWithOptimisticLock{})); e != nil {
				return false, e
			}
		}
		return cloudprovider.IsNodeClaimNotFoundError(err), nil
	}
	return false, fmt.Errorf("terminating cloudprovider instance, %w", err)
}

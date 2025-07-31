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

package nodeclass

import (
	"context"
	"fmt"
	"log"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
)

type InstanceProfile struct {
	instanceProfileProvider instanceprofile.Provider
	region                  string
	recreationCache         *cache.Cache
}

func NewInstanceProfileReconciler(instanceProfileProvider instanceprofile.Provider, region string, cache *cache.Cache) *InstanceProfile {
	return &InstanceProfile{
		instanceProfileProvider: instanceProfileProvider,
		region:                  region,
		recreationCache:         cache,
	}
}

func generateCacheKey(nodeClass *v1.EC2NodeClass) string {
	return fmt.Sprintf("%s/%s", nodeClass.Spec.Role, nodeClass.UID)
}

func (ip *InstanceProfile) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if nodeClass.Spec.Role != "" {
		var currentRole string
		var oldProfileName string

		// Use a short-lived cache to prevent instance profile recreation for the same role in the same EC2NodeClass
		// in case of a status patch error in the EC2NodeClass controller
		if profileName, ok := ip.recreationCache.Get(generateCacheKey(nodeClass)); ok {
			nodeClass.Status.InstanceProfile = profileName.(string)
		}

		// Get the current profile info if it exists
		if nodeClass.Status.InstanceProfile != "" {
			oldProfileName = nodeClass.Status.InstanceProfile
			profile, err := ip.instanceProfileProvider.Get(ctx, nodeClass.Status.InstanceProfile)
			if err != nil {
				if !awserrors.IsNotFound(err) {
					return reconcile.Result{}, fmt.Errorf("getting instance profile %s: %w", nodeClass.Status.InstanceProfile, err)
				}
			} else if len(profile.Roles) > 0 {
				currentRole = lo.FromPtr(profile.Roles[0].RoleName)
			}
		}

		// If role has changed, create new profile
		log.Printf("Current role: %s, new role: %s", currentRole, nodeClass.Spec.Role)
		if currentRole != nodeClass.Spec.Role {
			// Generate new profile name
			newProfileName := nodeClass.InstanceProfileName(options.FromContext(ctx).ClusterName, ip.region)
			log.Printf("Generated new profile name: %s", newProfileName)

			if err := ip.instanceProfileProvider.Create(
				ctx,
				newProfileName,
				nodeClass.InstanceProfileRole(),
				nodeClass.InstanceProfileTags(options.FromContext(ctx).ClusterName, ip.region),
				string(nodeClass.UID),
			); err != nil {
				return reconcile.Result{}, fmt.Errorf("creating instance profile, %w", err)
			}
			ip.recreationCache.SetDefault(generateCacheKey(nodeClass), newProfileName)

			// Mark the old profile as protected to prevent premature deletion by garbage collection.
			// This handles the case where a new NodeClaim is created but hasn't yet appeared in the
			// informer cache when garbage collection runs.
			if oldProfileName != "" {
				ip.instanceProfileProvider.SetProtectedState(oldProfileName, true)
			}
			// Similarly protect the new profile to prevent deletion if garbage collection runs
			// before this NodeClass appears in the informer cache.
			ip.instanceProfileProvider.SetProtectedState(newProfileName, true)
			nodeClass.Status.InstanceProfile = newProfileName
		}
	} else {
		// Ensure old profile is marked as protected in the event a customer switches from using
		// spec.role to spec.instanceProfile
		if nodeClass.Status.InstanceProfile != "" {
			ip.instanceProfileProvider.SetProtectedState(nodeClass.Status.InstanceProfile, true)
		}
		nodeClass.Status.InstanceProfile = lo.FromPtr(nodeClass.Spec.InstanceProfile)
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeInstanceProfileReady)
	log.Printf("Reconciled instance profile: %s", nodeClass.Status.InstanceProfile)
	return reconcile.Result{}, nil
}

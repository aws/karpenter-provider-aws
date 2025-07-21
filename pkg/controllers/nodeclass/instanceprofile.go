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

	// "github.com/patrickmn/go-cache"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
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

func (ip *InstanceProfile) PreventRecreation(nodeClass *v1.EC2NodeClass) (string, bool) {
	if profileName, found := ip.recreationCache.Get(fmt.Sprintf("%s/%s", nodeClass.Spec.Role, nodeClass.UID)); found {
		return profileName.(string), true
	}
	return "", false
}

func (ip *InstanceProfile) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if nodeClass.Spec.Role != "" {
		var currentRole string
		var oldProfileName string

		// Use a short-lived cache to prevent instance profile recreation for the same role in the same EC2NodeClass
		// in case of a status patch error in the EC2NodeClass controller
		if profileName, found := ip.PreventRecreation(nodeClass); found {
			nodeClass.Status.InstanceProfile = profileName
			currentRole = nodeClass.Spec.Role
		} else {
			// Get the current profile info if it exists
			if nodeClass.Status.InstanceProfile != "" {
				oldProfileName = nodeClass.Status.InstanceProfile
				if profile, err := ip.instanceProfileProvider.Get(ctx, nodeClass.Status.InstanceProfile); err == nil {
					if len(profile.Roles) > 0 {
						currentRole = lo.FromPtr(profile.Roles[0].RoleName)
					}
				}
			}
		}

		// If role has changed, create new profile
		log.Printf("Current role: %s, new role: %s", currentRole, nodeClass.Spec.Role)
		if currentRole != nodeClass.Spec.Role {
			// nodeClass.StatusConditions().SetFalse(v1.ConditionTypeInstanceProfileReady, "Reconciling", "Creating a new instance profile for role change")

			// Generate new profile name
			newProfileName := nodeClass.InstanceProfileName(options.FromContext(ctx).ClusterName, ip.region)
			log.Printf("Generated new profile name: %s", newProfileName)

			if err := ip.instanceProfileProvider.Create(
				ctx,
				oldProfileName,
				newProfileName,
				nodeClass.InstanceProfileRole(),
				nodeClass.InstanceProfileTags(options.FromContext(ctx).ClusterName, ip.region),
				string(nodeClass.UID),
			); err != nil {
				return reconcile.Result{}, fmt.Errorf("creating instance profile, %w", err)
			}
			ip.recreationCache.SetDefault(fmt.Sprintf("%s/%s", nodeClass.Spec.Role, nodeClass.UID), newProfileName)
			nodeClass.Status.InstanceProfile = newProfileName
		}
	} else {
		nodeClass.Status.InstanceProfile = lo.FromPtr(nodeClass.Spec.InstanceProfile)
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeInstanceProfileReady)
	log.Printf("Reconciled instance profile: %s", nodeClass.Status.InstanceProfile)
	return reconcile.Result{}, nil
}

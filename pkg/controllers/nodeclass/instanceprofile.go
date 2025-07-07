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

	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	//"sync"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
)

type InstanceProfile struct {
	instanceProfileProvider instanceprofile.Provider
	region                  string
}

func NewInstanceProfileReconciler(instanceProfileProvider instanceprofile.Provider, region string) *InstanceProfile {
	return &InstanceProfile{
		instanceProfileProvider: instanceProfileProvider,
		region:                  region,
	}
}

func (ip *InstanceProfile) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if nodeClass.Spec.Role != "" {
		var currentRole string
		var oldProfileName string

		// Get the current profile info if it exists
		if nodeClass.Status.InstanceProfile != "" {
			oldProfileName = nodeClass.Status.InstanceProfile
			if profile, err := ip.instanceProfileProvider.Get(ctx, nodeClass.Status.InstanceProfile); err == nil {
				log.Printf("PROFILE ROLES: %v", lo.FromPtr(profile.Roles[0].RoleName))
				if len(profile.Roles) > 0 {
					currentRole = lo.FromPtr(profile.Roles[0].RoleName)
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

			// Create new profile
			if err := ip.instanceProfileProvider.Create(
				ctx,
				newProfileName,
				nodeClass.InstanceProfileRole(),
				nodeClass.InstanceProfileTags(options.FromContext(ctx).ClusterName, ip.region),
			); err != nil {
				return reconcile.Result{}, fmt.Errorf("creating instance profile, %w", err)
			}

			// Track the replacement change
			if oldProfileName != "" {
				instanceprofile.TrackReplacement(oldProfileName)
			}

			nodeClass.Status.InstanceProfile = newProfileName
		}
	} else {
		nodeClass.Status.InstanceProfile = lo.FromPtr(nodeClass.Spec.InstanceProfile)
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeInstanceProfileReady)
	log.Printf("Reconciled instance profile: %s", nodeClass.Status.InstanceProfile)
	return reconcile.Result{}, nil
}

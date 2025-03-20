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
	"strings"

	"github.com/awslabs/operatorpkg/status"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
)

type InstanceProfile struct {
	instanceProfileProvider instanceprofile.Provider
	iamapi                  sdk.IAMAPI
	cache                   *cache.Cache
}

func NewInstanceProfileReconciler(instanceProfileProvider instanceprofile.Provider, iamapi sdk.IAMAPI, cache *cache.Cache) *InstanceProfile {
	return &InstanceProfile{
		instanceProfileProvider: instanceProfileProvider,
		iamapi:                  iamapi,
		cache:                   cache,
	}
}

func (ip *InstanceProfile) Reconcile(ctx context.Context, nodeClass *v1.EC2NodeClass) (reconcile.Result, error) {
	if nodeClass.Spec.Role != "" {
		name, err := ip.instanceProfileProvider.Create(ctx, nodeClass)
		if err != nil {
			//Create filters out instanceprofile not found errors so any is not found error will be referencing the role
			if awserrors.IsNotFound(err) || awserrors.IsUnauthorizedOperationError(err) {
				nodeClass.StatusConditions().SetFalse(v1.ConditionTypeInstanceProfileReady, "NodeRoleNotFound", "Failed to detect the NodeRole")
			}
			return reconcile.Result{}, fmt.Errorf("creating instance profile, %w", err)
		}
		nodeClass.Status.InstanceProfile = name
	} else {
		if val, ok := ip.cache.Get(ip.cacheKey(nodeClass)); ok {
			if val == metav1.ConditionTrue {
				nodeClass.Status.InstanceProfile = lo.FromPtr(nodeClass.Spec.InstanceProfile)
				nodeClass.StatusConditions().SetTrue(v1.ConditionTypeInstanceProfileReady)
			} else if val == metav1.ConditionFalse {
				nodeClass.StatusConditions().SetFalse(v1.ConditionTypeInstanceProfileReady, "InstanceProfileNotFound", "Failed to detect the Instance Profile")
			} else {
				nodeClass.StatusConditions().SetUnknown(v1.ConditionTypeInstanceProfileReady)
			}
			log.Print(nodeClass.StatusConditions().Get(v1.ConditionTypeInstanceProfileReady))
			log.Print(nodeClass.StatusConditions().Get(v1.ConditionTypeValidationSucceeded))
			log.Print(nodeClass.StatusConditions().Get(status.ConditionReady))
			return reconcile.Result{}, nil
		}
		_, err := ip.iamapi.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
			InstanceProfileName: nodeClass.Spec.InstanceProfile,
		})
		if err != nil {
			if awserrors.IsNotFound(err) || awserrors.IsUnauthorizedOperationError(err) {
				nodeClass.StatusConditions().SetFalse(v1.ConditionTypeInstanceProfileReady, "InstanceProfileNotFound", "Failed to detect the Instance Profile")
			}
			ip.cache.SetDefault(ip.cacheKey(nodeClass), nodeClass.StatusConditions().Get(v1.ConditionTypeInstanceProfileReady).Status)
			return reconcile.Result{}, fmt.Errorf("getting instance profile, %w", err)
		}
		nodeClass.Status.InstanceProfile = lo.FromPtr(nodeClass.Spec.InstanceProfile)
		ip.cache.SetDefault(ip.cacheKey(nodeClass), metav1.ConditionTrue)
	}
	nodeClass.StatusConditions().SetTrue(v1.ConditionTypeInstanceProfileReady)
	return reconcile.Result{}, nil
}

func (*InstanceProfile) cacheKey(nodeClass *v1.EC2NodeClass) string {
	hash := lo.Must(hashstructure.Hash([]interface{}{
		nodeClass.Spec.InstanceProfile,
	}, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true}))
	return fmt.Sprintf("%s:%016x", nodeClass.Name, hash)
}

// clearCacheEntries removes all cache entries associated with the given nodeclass from the instance profile cache
func (ip *InstanceProfile) clearCacheEntries(nodeClass *v1.EC2NodeClass) {
	var toDelete []string
	for key := range ip.cache.Items() {
		parts := strings.Split(key, ":")
		// NOTE: should never occur, indicates malformed cache key
		if len(parts) != 2 {
			continue
		}
		if parts[0] == nodeClass.Name {
			toDelete = append(toDelete, key)
		}
	}
	for _, key := range toDelete {
		ip.cache.Delete(key)
	}
}

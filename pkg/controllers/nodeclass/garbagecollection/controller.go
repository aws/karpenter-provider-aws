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

package garbagecollection

import (
	"context"
	"fmt"
	"time"

	"github.com/awslabs/operatorpkg/singleton"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
)

const (
	//InstanceProfileAnnotation = "karpenter.k8s.aws/instance-profile"
	// GarbageCollectionDelay = 24 * time.Hour
	GarbageCollectionDelay = 1 * time.Minute
)

type Controller struct {
	kubeClient              client.Client
	cloudProvider           cloudprovider.CloudProvider
	instanceProfileProvider instanceprofile.Provider
}

func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, instanceProfileProvider instanceprofile.Provider) *Controller {
	return &Controller{
		kubeClient:              kubeClient,
		cloudProvider:           cloudProvider,
		instanceProfileProvider: instanceProfileProvider,
	}
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("controller", "instanceprofile.garbagecollection"))

	// Get all NodeClaims to check which profiles are still in use
	nodeClaims, err := nodeclaimutils.ListManaged(ctx, c.kubeClient, c.cloudProvider)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("listing nodeclaims, %w", err)
	}

	// Build set of profiles currently in use
	activeProfiles := make(map[string]struct{})
	for _, nc := range nodeClaims {
		if profileName, ok := nc.Annotations[v1.AnnotationInstanceProfile]; ok {
			activeProfiles[profileName] = struct{}{}
		}
	}

	// Check each tracked profile
	instanceprofile.CreationTimeMap.Range(func(key, value interface{}) bool {
		profileName := key.(string)
		creationTime := value.(time.Time)

		// Skip if not old enough
		if time.Since(creationTime) < GarbageCollectionDelay {
			return true
		}

		// If profile isn't active and is old enough, delete it
		if _, isActive := activeProfiles[profileName]; !isActive {
			if err := c.instanceProfileProvider.Delete(ctx, profileName); err != nil {
				klog.Errorf("failed to delete instance profile %s: %v", profileName, err)
			} else {
				klog.V(2).Infof("garbage collected instance profile %s", profileName)
				instanceprofile.DeleteTracking(profileName)
			}
		}
		return true
	})

	// Requeue after 30 minutes
	return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("instanceprofile.garbagecollection").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}

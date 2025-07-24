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
	origlog "log"
	"time"

	"github.com/awslabs/operatorpkg/singleton"
	"k8s.io/apimachinery/pkg/util/sets"
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

type Controller struct {
	kubeClient              client.Client
	cloudProvider           cloudprovider.CloudProvider
	instanceProfileProvider instanceprofile.Provider
	region                  string
}

func NewController(kubeClient client.Client, cloudProvider cloudprovider.CloudProvider, instanceProfileProvider instanceprofile.Provider, region string) *Controller {
	return &Controller{
		kubeClient:              kubeClient,
		cloudProvider:           cloudProvider,
		instanceProfileProvider: instanceProfileProvider,
		region:                  region,
	}
}

func (c *Controller) getActiveProfiles(ctx context.Context) (sets.Set[string], error) {
	nodeClaims, err := nodeclaimutils.ListManaged(ctx, c.kubeClient, c.cloudProvider)
	if err != nil {
		return nil, fmt.Errorf("listing nodeclaims, %w", err)
	}

	activeProfiles := sets.New[string]()
	for _, nc := range nodeClaims {
		if profileName, ok := nc.Annotations[v1.AnnotationInstanceProfile]; ok {
			activeProfiles.Insert(profileName)
		}

	}
	origlog.Printf("length of active profiles: %d", len(activeProfiles))

	return activeProfiles, nil
}

func (c *Controller) getCurrentProfiles(ctx context.Context) (sets.Set[string], error) {
	nodeClasses := &v1.EC2NodeClassList{}
	if err := c.kubeClient.List(ctx, nodeClasses); err != nil {
		return nil, fmt.Errorf("listing nodeclasses, %w", err)
	}

	currentProfiles := sets.New[string]()

	for _, nc := range nodeClasses.Items {
		if nc.Status.InstanceProfile != "" {
			currentProfiles.Insert(nc.Status.InstanceProfile)
		}
	}
	origlog.Printf("length of current profiles: %d", len(currentProfiles))
	return currentProfiles, nil
}

func (c *Controller) shouldDeleteProfile(profileName string, currentProfiles sets.Set[string]) bool {
	// Skip if this is a current profile in any EC2NodeClass
	origlog.Printf("ENTERED SHOULD DELETE")
	if _, isCurrent := currentProfiles[profileName]; isCurrent {
		return false
	}
	origlog.Printf("NOT CURRENT PROFILE")

	if c.instanceProfileProvider.IsProtected(profileName) {
		return false
	}
	origlog.Printf("NOT PROTECTED PROFILE")

	return true
}

func (c *Controller) cleanupInactiveProfiles(ctx context.Context, activeProfiles sets.Set[string], currentProfiles sets.Set[string]) error {
	profiles, err := c.instanceProfileProvider.ListClusterProfiles(ctx)
	origlog.Printf("length of listed cluster profiles: %d", len(profiles))

	if err != nil {
		return fmt.Errorf("listing instance profiles, %w", err)
	}

	for _, profile := range profiles {
		profileName := *profile.InstanceProfileName

		shouldDelete := c.shouldDeleteProfile(profileName, currentProfiles)

		if !shouldDelete {
			origlog.Printf("WE CONTINUED FOR SOME REASON")
			continue
		}
		origlog.Printf("WE MADE IT HERE!")
		if _, isActive := activeProfiles[profileName]; !isActive {
			if err := c.instanceProfileProvider.Delete(ctx, profileName); err != nil {
				log.FromContext(ctx).Error(err, "failed to delete instance profile", "profile", profileName)
				return err
			}
			origlog.Printf("NOT ACTIVE PROFILE")
		}
	}
	return nil
}

func (c *Controller) Reconcile(ctx context.Context) (reconcile.Result, error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("controller", "instanceprofile.garbagecollection"))

	activeProfiles, err := c.getActiveProfiles(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	currentProfiles, err := c.getCurrentProfiles(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := c.cleanupInactiveProfiles(ctx, activeProfiles, currentProfiles); err != nil {
		return reconcile.Result{}, err
	}

	// Requeue after 30 minutes (1 minute for testing purposes)
	return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("instanceprofile.garbagecollection").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}

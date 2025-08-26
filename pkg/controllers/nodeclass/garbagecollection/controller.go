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

	"github.com/awslabs/operatorpkg/serrors"
	"github.com/awslabs/operatorpkg/singleton"
	"k8s.io/apimachinery/pkg/util/sets"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/awslabs/operatorpkg/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
	return currentProfiles, nil
}

func (c *Controller) shouldDeleteProfile(profileName string, currentProfiles sets.Set[string], activeProfiles sets.Set[string]) bool {
	if currentProfiles.Has(profileName) {
		return false
	}

	if c.instanceProfileProvider.IsProtected(profileName) {
		return false
	}

	if activeProfiles.Has(profileName) {
		return false
	}

	return true
}

func (c *Controller) cleanupInactiveProfiles(ctx context.Context) error {
	activeProfiles, err := c.getActiveProfiles(ctx)
	if err != nil {
		return err
	}

	currentProfiles, err := c.getCurrentProfiles(ctx)
	if err != nil {
		return err
	}

	profiles, err := c.instanceProfileProvider.ListClusterProfiles(ctx)
	if err != nil {
		return fmt.Errorf("listing instance profiles, %w", err)
	}

	for _, profile := range profiles {
		profileName := *profile.InstanceProfileName

		if !c.shouldDeleteProfile(profileName, currentProfiles, activeProfiles) {
			continue
		}

		if err := c.instanceProfileProvider.Delete(ctx, profileName); err != nil {
			return serrors.Wrap(fmt.Errorf("deleting instance profile, %w", err), "instance-profile", profileName)
		}
		log.FromContext(ctx).V(1).Info("deleted instance profile", "instance-profile", profileName)
	}
	return nil
}

func (c *Controller) Reconcile(ctx context.Context) (reconciler.Result, error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("controller", "instanceprofile.garbagecollection"))

	if err := c.cleanupInactiveProfiles(ctx); err != nil {
		return reconciler.Result{}, err
	}
	// Requeue after 30 minutes
	return reconciler.Result{RequeueAfter: 30 * time.Minute}, nil
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("instanceprofile.garbagecollection").
		WatchesRawSource(singleton.Source()).
		Complete(singleton.AsReconciler(c))
}

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
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"

	"github.com/mitchellh/hashstructure/v2"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instanceprofile"
)

const (
	// GarbageCollectionDelay = 24 * time.Hour
	GarbageCollectionDelay = 1 * time.Minute // Use 1 minute for testing purposes
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

func (c *Controller) getActiveProfiles(ctx context.Context) (map[string]struct{}, error) {
	nodeClaims, err := nodeclaimutils.ListManaged(ctx, c.kubeClient, c.cloudProvider)
	if err != nil {
		return nil, fmt.Errorf("listing nodeclaims, %w", err)
	}

	activeProfiles := make(map[string]struct{})
	for _, nc := range nodeClaims {
		if profileName, ok := nc.Annotations[v1.AnnotationInstanceProfile]; ok {
			activeProfiles[profileName] = struct{}{}
			continue
		}

		// Protect against migration where pre-upgrade NodeClaims do not have instanceprofile annotation
		clusterName := options.FromContext(ctx).ClusterName
		hash := lo.Must(hashstructure.Hash(fmt.Sprintf("%s%s", c.region, nc.Spec.NodeClassRef.Name), hashstructure.FormatV2, nil))
		oldProfileName := fmt.Sprintf("%s_%d", clusterName, hash)
		activeProfiles[oldProfileName] = struct{}{}
	}
	return activeProfiles, nil
}

func (c *Controller) getCurrentProfiles(ctx context.Context) (map[string]struct{}, error) {
	nodeClasses := &v1.EC2NodeClassList{}
	if err := c.kubeClient.List(ctx, nodeClasses); err != nil {
		return nil, fmt.Errorf("listing nodeclasses, %w", err)
	}

	currentProfiles := make(map[string]struct{})
	for _, nc := range nodeClasses.Items {
		if nc.Status.InstanceProfile != "" {
			currentProfiles[nc.Status.InstanceProfile] = struct{}{}
		}
	}
	return currentProfiles, nil
}

func (c *Controller) shouldDeleteProfile(ctx context.Context, profileName string, currentProfiles map[string]struct{}, clusterName string) (bool, error) {
	// Skip if this is a current profile in any EC2NodeClass
	if _, isCurrent := currentProfiles[profileName]; isCurrent {
		return false, nil
	}

	tags, err := c.instanceProfileProvider.ListTags(ctx, profileName)
	if err != nil {
		return false, fmt.Errorf("failed to list tags for instance profile %s: %w", profileName, err)
	}

	tagMap := make(map[string]string)
	for _, tag := range tags {
		tagMap[*tag.Key] = *tag.Value
	}

	clusterTag := fmt.Sprintf("kubernetes.io/cluster/%s", clusterName)
	if tagMap[clusterTag] != "owned" {
		return false, nil
	}
	if tagMap[v1.EKSClusterNameTagKey] != clusterName {
		return false, nil
	}

	creationTime, ok := instanceprofile.GetCreationTime(profileName)
	if !ok {
		creationTime = time.Now().Add(-GarbageCollectionDelay)
	}

	return time.Since(creationTime) >= GarbageCollectionDelay, nil
}

func (c *Controller) cleanupInactiveProfiles(ctx context.Context, activeProfiles, currentProfiles map[string]struct{}) error {
	clusterName := options.FromContext(ctx).ClusterName
	profiles, err := c.instanceProfileProvider.ListByPrefix(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("listing instance profiles, %w", err)
	}

	for _, profile := range profiles {
		profileName := *profile.InstanceProfileName

		shouldDelete, err := c.shouldDeleteProfile(ctx, profileName, currentProfiles, clusterName)
		if err != nil {
			return err
		}
		if !shouldDelete {
			continue
		}

		if _, isActive := activeProfiles[profileName]; !isActive {
			if err := c.instanceProfileProvider.Delete(ctx, profileName); err != nil {
				log.FromContext(ctx).Error(err, "failed to delete instance profile", "profile", profileName)
			} else {
				instanceprofile.DeleteTracking(profileName)
			}
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

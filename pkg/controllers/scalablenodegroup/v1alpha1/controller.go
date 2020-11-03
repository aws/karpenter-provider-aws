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

// +kubebuilder:rbac:groups=autoscaling.karpenter.sh,resources=scalablenodegroups;scalablenodegroups/status,verbs=get;list;watch;create;patch;delete

package v1alpha1

import (
	"fmt"
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/controllers"
)

// Controller for the resource
type Controller struct {
	CloudProvider cloudprovider.Factory
}

// For returns the resource this controller is for.
func (c *Controller) For() controllers.Object {
	return &v1alpha1.ScalableNodeGroup{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []controllers.Object {
	return []controllers.Object{}
}

func (c *Controller) Interval() time.Duration {
	return 60 * time.Second
}

// Reconcile executes a control loop for the resource
func (c *Controller) reconcile(resource *v1alpha1.ScalableNodeGroup) error {
	ng := c.CloudProvider.NodeGroupFor(&resource.Spec)
	observedReplicas, err := ng.GetReplicas()
	if err != nil {
		return fmt.Errorf("unable to get replica count for node group %v, %w", resource.Spec.ID, err)
	}
	resource.Status.Replicas = observedReplicas
	if resource.Spec.Replicas == nil || *resource.Spec.Replicas == observedReplicas {
		return nil
	}
	if err := ng.SetReplicas(*resource.Spec.Replicas); err != nil {
		return fmt.Errorf("unable to set replicas for node group %v, %w", resource.Spec.ID, err)
	}
	return nil
}

// Reconcile executes a control loop for the resource
func (c *Controller) Reconcile(object controllers.Object) (err error) {
	resource := object.(*v1alpha1.ScalableNodeGroup)
	if err = c.reconcile(resource); controllers.IsRetryable(err) {
		resource.StatusConditions().MarkFalse(v1alpha1.AbleToScale, "", controllers.ErrorCode(err))
		// We don't want to return an error here; that would cause the
		// resource to go out of Active mode, and would take longer
		// before the next reconciliation (which will most likely
		// work, next time around).
		return nil
	}
	resource.StatusConditions().MarkTrue(v1alpha1.AbleToScale)
	return err
}

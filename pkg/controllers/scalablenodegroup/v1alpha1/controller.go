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
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup"
	"github.com/ellistarn/karpenter/pkg/controllers"
	"github.com/pkg/errors"
	"knative.dev/pkg/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Controller for the resource
type Controller struct {
	client.Client
	NodeGroupFactory nodegroup.Factory
}

// For returns the resource this controller is for.
func (c *Controller) For() controllers.Resource {
	return &v1alpha1.ScalableNodeGroup{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []controllers.Resource {
	return []controllers.Resource{}
}

func (c *Controller) Interval() time.Duration {
	return 60 * time.Second
}

// Reconcile executes a control loop for the resource
func (c *Controller) Reconcile(object controllers.Resource) error {
	resource := object.(*v1alpha1.ScalableNodeGroup)
	ng := c.NodeGroupFactory.For(resource)
	replicas, err := ng.GetReplicas()
	if err != nil {
		return errors.Wrapf(err, "unable to get replica count for node group: %v", resource.Spec.ID)
	}
	resource.Status.Replicas = ptr.Int32(int32(replicas))
	if resource.Spec.Replicas == nil || *resource.Spec.Replicas == int32(replicas) {
		return nil
	}
	if err := ng.SetReplicas(replicas); err != nil {
		return errors.Wrapf(err, "unable to set replicas for node group: %v", resource.Spec.ID)
	}
	return nil

}

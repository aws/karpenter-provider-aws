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

// +kubebuilder:rbac:groups=autoscaling.karpenter.sh,resources=horizontalautoscalers;horizontalautoscalers/status,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=autoscaling.karpenter.sh,resources=scalablenodegroups/scale,verbs=get;update;patch

package v1alpha1

import (
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/autoscaler"
	"github.com/ellistarn/karpenter/pkg/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Controller reconciles a HorizontalAutoscaler object
type Controller struct {
	client.Client
	AutoscalerFactory autoscaler.Factory
}

// For returns the resource this controller is for.
func (c *Controller) For() controllers.Resource {
	return &v1alpha1.HorizontalAutoscaler{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []controllers.Resource {
	return []controllers.Resource{}
}

func (c *Controller) Interval() time.Duration {
	return 10 * time.Second
}

// Reconcile executes a control loop for the HorizontalAutoscaler resource
// For now, assume a singleton architecture where all definitions are handled in a single shard.
// In the future, we may wish to do some sort of sharded assignment to spread definitions across many controller instances.
func (c *Controller) Reconcile(object controllers.Resource) error {
	autoscaler := c.AutoscalerFactory.For(object.(*v1alpha1.HorizontalAutoscaler))
	return autoscaler.Reconcile()
}

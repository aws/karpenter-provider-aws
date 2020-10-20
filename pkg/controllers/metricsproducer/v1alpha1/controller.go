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

// +kubebuilder:rbac:groups=autoscaling.karpenter.sh,resources=metricsproducers;metricsproducers/status,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes;pods,verbs=get;list;watch

package v1alpha1

import (
	"time"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/controllers"
	"github.com/ellistarn/karpenter/pkg/metrics/producers"
)

// Controller for the resource
type Controller struct {
	ProducerFactory *producers.Factory
}

// For returns the resource this controller is for.
func (c *Controller) For() controllers.Resource {
	return &v1alpha1.MetricsProducer{}
}

// Owns returns the resources owned by this controller's resource.
func (c *Controller) Owns() []controllers.Resource {
	return []controllers.Resource{}
}

func (c *Controller) Interval() time.Duration {
	return 5 * time.Second
}

// Reconcile executes a control loop for the resource
func (c *Controller) Reconcile(object controllers.Resource) error {
	return c.ProducerFactory.For(object.(*v1alpha1.MetricsProducer)).Reconcile()
}

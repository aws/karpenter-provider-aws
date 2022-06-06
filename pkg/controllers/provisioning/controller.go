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

package provisioning

import (
	"context"
	"time"

	"github.com/aws/karpenter/pkg/config"
	"github.com/aws/karpenter/pkg/utils/pod"

	"github.com/aws/karpenter/pkg/events"

	"github.com/aws/karpenter/pkg/controllers/state"

	"github.com/aws/karpenter/pkg/cloudprovider"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const controllerName = "provisioning"

// Controller for the resource
type Controller struct {
	kubeClient  client.Client
	provisioner *Provisioner
	recorder    events.Recorder
}

// NewController constructs a controller instance
func NewController(ctx context.Context, cfg config.Config, kubeClient client.Client, coreV1Client corev1.CoreV1Interface, recorder events.Recorder, cloudProvider cloudprovider.CloudProvider, cluster *state.Cluster) *Controller {
	return &Controller{
		kubeClient:  kubeClient,
		provisioner: NewProvisioner(ctx, cfg, kubeClient, coreV1Client, recorder, cloudProvider, cluster),
		recorder:    recorder,
	}
}

func (c *Controller) Recorder() events.Recorder {
	return c.recorder
}

// Reconcile the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	p := &v1.Pod{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, p); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	// Ensure the pod can be provisioned
	if !pod.IsProvisionable(p) {
		return reconcile.Result{}, nil
	}
	c.provisioner.Trigger()
	// TODO: This is only necessary due to a bug in the batcher. Ideally we should retrigger on provisioning error instead
	return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
}

// Deprecated: TriggerAndWait is used for unit testing purposes only
func (c *Controller) TriggerAndWait() {
	c.provisioner.TriggerAndWait()
}

func (c *Controller) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(c)
}

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

package persistentvolumeclaim

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/pod"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerName         = "volume"
	SelectedNodeAnnotation = "volume.kubernetes.io/selected-node"
)

// Controller for the resource
type Controller struct {
	kubeClient client.Client
}

// NewController is a constructor
func NewController(kubeClient client.Client) *Controller {
	return &Controller{kubeClient: kubeClient}
}

// Register the controller to the manager
func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(controllerName).
		For(&v1.PersistentVolumeClaim{}).
		Watches(&source.Kind{Type: &v1.Pod{}}, handler.EnqueueRequestsFromMapFunc(c.pvcForPod)).
		Complete(c)
}

// Reconcile a control loop for the resource
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(controllerName).With("resource", req.String()))
	ctx = injection.WithNamespacedName(ctx, req.NamespacedName)
	ctx = injection.WithControllerName(ctx, controllerName)

	pvc := &v1.PersistentVolumeClaim{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, pvc); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	pod, err := c.podForPvc(ctx, pvc)
	if err != nil {
		return reconcile.Result{}, err
	}
	if pod == nil {
		return reconcile.Result{}, nil
	}
	if nodeName, ok := pvc.Annotations[SelectedNodeAnnotation]; ok && nodeName == pod.Spec.NodeName {
		return reconcile.Result{}, nil
	}
	if !c.isBindable(pod) {
		return reconcile.Result{}, nil
	}
	pvc.Annotations = functional.UnionStringMaps(pvc.Annotations, map[string]string{SelectedNodeAnnotation: pod.Spec.NodeName})
	if err := c.kubeClient.Update(ctx, pvc); err != nil {
		return reconcile.Result{}, fmt.Errorf("binding persistent volume claim for pod %s/%s to node %q, %w", pod.Namespace, pod.Name, pod.Spec.NodeName, err)
	}
	logging.FromContext(ctx).Infof("Bound persistent volume claim to node %s", pod.Spec.NodeName)
	return reconcile.Result{}, nil
}

func (c *Controller) podForPvc(ctx context.Context, pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {
	pods := &v1.PodList{}
	if err := c.kubeClient.List(ctx, pods, client.InNamespace(pvc.Namespace)); err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				return &pod, nil
			}
		}
	}
	return nil, nil
}

func (c *Controller) pvcForPod(o client.Object) (requests []reconcile.Request) {
	if !c.isBindable(o.(*v1.Pod)) {
		return requests
	}
	for _, volume := range o.(*v1.Pod).Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: o.GetNamespace(), Name: volume.PersistentVolumeClaim.ClaimName}})
	}
	return requests
}

func (c *Controller) isBindable(p *v1.Pod) bool {
	return pod.IsScheduled(p) && !pod.IsTerminal(p) && !pod.IsTerminating(p)
}

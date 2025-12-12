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

package debug

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/karpenter/pkg/utils/pod"
)

type PodController struct {
	kubeClient client.Client
}

func NewPodController(kubeClient client.Client) *PodController {
	return &PodController{
		kubeClient: kubeClient,
	}
}

func (c *PodController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	p := &corev1.Pod{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, p); err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("[DELETED %s] POD %s\n", time.Now().Format(time.RFC3339), req.String())
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	fmt.Printf("[CREATED/UPDATED %s] POD %s %s\n", time.Now().Format(time.RFC3339), req.String(), c.GetInfo(p))
	return reconcile.Result{}, nil
}

func (c *PodController) GetInfo(p *corev1.Pod) string {
	var containerInfo strings.Builder
	for _, c := range p.Status.ContainerStatuses {
		if containerInfo.Len() > 0 {
			_ = lo.Must(fmt.Fprintf(&containerInfo, ", "))
		}
		_ = lo.Must(fmt.Fprintf(&containerInfo, "%s restarts=%d", c.Name, c.RestartCount))
	}
	return fmt.Sprintf("provisionable=%v phase=%s nodename=%s owner=%#v [%s]",
		pod.IsProvisionable(p), p.Status.Phase, p.Spec.NodeName, p.OwnerReferences, containerInfo.String())
}

func (c *PodController) Register(_ context.Context, m manager.Manager) error {
	return controllerruntime.NewControllerManagedBy(m).
		Named("pod").
		For(&corev1.Pod{}).
		WithEventFilter(predicate.And(
			predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldPod := e.ObjectOld.(*corev1.Pod)
					newPod := e.ObjectNew.(*corev1.Pod)
					return c.GetInfo(oldPod) != c.GetInfo(newPod)
				},
			},
			predicate.NewPredicateFuncs(func(o client.Object) bool {
				return o.GetNamespace() != "kube-system" && o.GetNamespace() != "prometheus"
			}),
		)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10, SkipNameValidation: lo.ToPtr(true)}).
		Complete(c)
}

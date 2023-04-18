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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
)

type MachineController struct {
	kubeClient client.Client
}

func NewMachineController(kubeClient client.Client) *MachineController {
	return &MachineController{
		kubeClient: kubeClient,
	}
}

func (c *MachineController) Name() string {
	return "machine"
}

func (c *MachineController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	m := &v1alpha5.Machine{}
	if err := c.kubeClient.Get(ctx, req.NamespacedName, m); err != nil {
		if errors.IsNotFound(err) {
			fmt.Printf("[DELETED %s] MACHINE %s\n", time.Now().Format(time.RFC3339), req.NamespacedName.String())
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	fmt.Printf("[CREATED/UPDATED %s] MACHINE %s %s\n", time.Now().Format(time.RFC3339), req.NamespacedName.Name, c.GetInfo(m))
	return reconcile.Result{}, nil
}

func (c *MachineController) GetInfo(m *v1alpha5.Machine) string {
	return fmt.Sprintf("ready=%t launched=%t registered=%t initialized=%t",
		m.StatusConditions().IsHappy(),
		m.StatusConditions().GetCondition(v1alpha5.MachineLaunched).IsTrue(),
		m.StatusConditions().GetCondition(v1alpha5.MachineRegistered).IsTrue(),
		m.StatusConditions().GetCondition(v1alpha5.MachineInitialized).IsTrue(),
	)
}

func (c *MachineController) Builder(_ context.Context, m manager.Manager) corecontroller.Builder {
	return corecontroller.Adapt(controllerruntime.
		NewControllerManagedBy(m).
		For(&v1alpha5.Machine{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldMachine := e.ObjectOld.(*v1alpha5.Machine)
				newMachine := e.ObjectNew.(*v1alpha5.Machine)
				return c.GetInfo(oldMachine) != c.GetInfo(newMachine)
			},
		}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}))
}

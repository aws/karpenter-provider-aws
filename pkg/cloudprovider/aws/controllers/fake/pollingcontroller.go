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

package fake

import (
	"context"
	"sync/atomic"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PollingController struct {
	TriggerCalls atomic.Int64
}

func (c *PollingController) Start(context.Context) {}

func (c *PollingController) Stop(context.Context) {}

func (c *PollingController) Trigger() {
	c.TriggerCalls.Add(1)
}

func (c *PollingController) Active() bool { return true }

func (c *PollingController) Healthy() bool { return true }

func (c *PollingController) Reconcile(context.Context, reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (c *PollingController) Register(context.Context, manager.Manager) error { return nil }

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
	"sync"

	"github.com/go-logr/zapr"
	"github.com/samber/lo"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
)

type Monitor struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mgr    manager.Manager
}

func New(ctx context.Context, config *rest.Config, kubeClient client.Client) *Monitor {
	logger := logging.FromContext(ctx)
	ctrl.SetLogger(zapr.NewLogger(logger.Desugar()))
	mgr := lo.Must(controllerruntime.NewManager(config, controllerruntime.Options{
		Scheme: scheme.Scheme,
		BaseContext: func() context.Context {
			ctx := context.Background()
			ctx = logging.WithLogger(ctx, logger)
			logger.WithOptions()
			return ctx
		},
		Metrics: server.Options{
			BindAddress: "0",
		},
	}))
	for _, c := range newControllers(kubeClient) {
		lo.Must0(c.Builder(ctx, mgr).Complete(c), "failed to register controller")
	}
	ctx, cancel := context.WithCancel(ctx) // this context is only meant for monitor start/stop
	return &Monitor{
		ctx:    ctx,
		cancel: cancel,
		mgr:    mgr,
	}
}

// MustStart starts the debug monitor
func (m *Monitor) MustStart() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		lo.Must0(m.mgr.Start(m.ctx))
	}()
}

// Stop stops the monitor
func (m *Monitor) Stop() {
	m.cancel()
	m.wg.Wait()
}

func newControllers(kubeClient client.Client) []controller.Controller {
	return []controller.Controller{
		NewMachineController(kubeClient),
		NewNodeClaimController(kubeClient),
		NewNodeController(kubeClient),
		NewPodController(kubeClient),
	}
}

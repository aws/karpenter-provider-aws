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

package test

import (
	"context"
	"fmt"
	"sync"

	"github.com/awslabs/karpenter/pkg/controllers"
	"github.com/awslabs/karpenter/pkg/utils/log"
	"github.com/awslabs/karpenter/pkg/utils/project"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

/*
Environment is for e2e local testing. It stands up an API Server, ETCD,
and a controller-runtime manager. It's possible to run multiple environments
simultaneously, as the ports are randomized. A common use case for this is
parallel tests using ginkgo's parallelization functionality. The environment is
typically instantiated once in a test file and re-used between different test
cases. Resources for each test should be isolated into its own namespace.
env := new Local(func(local *Local) {
	// Register test controller with manager
	controllerruntime.NewControllerManagedBy(local.Manager).For(...)
	return nil
})
BeforeSuite(func() { env.Start() })
AfterSuite(func() { env.Stop() })
*/
type Environment struct {
	envtest.Environment
	Manager controllers.Manager
	Client  client.Client

	options []EnvironmentOption
	ctx     context.Context
	stop    context.CancelFunc
	cleanup *sync.WaitGroup
}

// LocalOption passes the Local environment to an option function. This is
// useful for registering controllers with the controller-runtime manager or for
// customizing Client, Scheme, or other variables.
type EnvironmentOption func(env *Environment)

func NewEnvironment(options ...EnvironmentOption) *Environment {
	log.Setup(controllerruntimezap.UseDevMode(true), controllerruntimezap.ConsoleEncoder(), controllerruntimezap.StacktraceLevel(zapcore.DPanicLevel))
	ctx, stop := context.WithCancel(controllerruntime.SetupSignalHandler())
	return &Environment{
		Environment: envtest.Environment{
			CRDDirectoryPaths: []string{project.RelativeToRoot("charts/karpenter/templates/provisioning.karpenter.sh_provisioners.yaml")},
		},
		ctx:     ctx,
		stop:    stop,
		options: options,
		cleanup: &sync.WaitGroup{},
	}
}

func (e *Environment) Start() (err error) {
	// Environment
	if _, err := e.Environment.Start(); err != nil {
		return fmt.Errorf("starting environment, %w", err)
	}

	// Manager
	e.Manager = controllers.NewManagerOrDie(e.Config, controllerruntime.Options{
		MetricsBindAddress: "0", // Skip the metrics server to avoid port conflicts for parallel testing
	})

	// Client
	kubeClient, err := client.New(e.Manager.GetConfig(), client.Options{
		Scheme: e.Manager.GetScheme(),
		Mapper: e.Manager.GetRESTMapper(),
	})
	if err != nil {
		return err
	}
	e.Client = kubeClient

	// options
	for _, option := range e.options {
		option(e)
	}

	// Start manager
	go func() {
		if err := e.Manager.Start(e.ctx); err != nil {
			zap.S().Panic(err)
		}
	}()
	return nil
}

func (e *Environment) Stop() error {
	e.stop()
	e.cleanup.Wait()
	return e.Environment.Stop()
}

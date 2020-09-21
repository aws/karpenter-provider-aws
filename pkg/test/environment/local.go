package environment

import (
	"context"

	"github.com/ellistarn/karpenter/pkg/apis"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"github.com/ellistarn/karpenter/pkg/utils/project"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

/*
Local is an Environment for e2e local testing. It stands up an API Server, ETCD,
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
type Local struct {
	envtest.Environment
	Manager manager.Manager

	injectors []LocalInjector
	stopch    chan struct{}
}

// LocalInjector passes the Local environment to an injector function. This is
// useful for registering controllers with the controller-runtime manager or for
// customizing Client, Scheme, or other variables.
type LocalInjector func(env *Local) error

func NewLocal(injectors ...LocalInjector) Environment {
	log.Setup(controllerruntimezap.UseDevMode(true))
	return &Local{
		Environment: envtest.Environment{
			CRDDirectoryPaths: []string{project.RelativeToRoot("config/crd/bases")},
			WebhookInstallOptions: envtest.WebhookInstallOptions{
				DirectoryPaths: []string{project.RelativeToRoot("config/webhook")},
			},
		},
		stopch:    make(chan struct{}),
		injectors: injectors,
	}
}

func (e *Local) NewNamespace() (*Namespace, error) {
	ns := NewNamespace(e.Manager.GetClient())
	if err := e.Manager.GetClient().Create(context.Background(), &ns.Namespace); err != nil {
		return nil, err
	}

	go func() {
		<-e.stopch
		if err := e.Manager.GetClient().Delete(context.Background(), &ns.Namespace); err != nil {
			zap.S().Error(errors.Wrap(err, "Failed to tear down namespace"))
		}
	}()
	return ns, nil
}

func (e *Local) Start() (err error) {
	// Environment
	if _, err := e.Environment.Start(); err != nil {
		return err
	}

	// Scheme
	scheme := runtime.NewScheme()
	for _, AddToScheme := range []func(s *runtime.Scheme) error{
		apis.AddToScheme,
		clientgoscheme.AddToScheme,
	} {
		if err := AddToScheme(scheme); err != nil {
			return err
		}
	}

	// Manager
	if e.Manager, err = controllerruntime.NewManager(e.Config, controllerruntime.Options{
		CertDir: e.WebhookInstallOptions.LocalServingCertDir,
		Host:    e.WebhookInstallOptions.LocalServingHost,
		Port:    e.WebhookInstallOptions.LocalServingPort,
		Scheme:  scheme,
	}); err != nil {
		return err
	}

	// Injectors
	for _, injector := range e.injectors {
		if err := injector(e); err != nil {
			return err
		}
	}

	// Start manager
	go func() {
		if err := e.Manager.Start(e.stopch); err != nil {
			zap.S().Fatal(errors.Wrapf(err, "Failed to start manager"))
		}
	}()

	// Close on interrupt
	go func() {
		<-controllerruntime.SetupSignalHandler()
		close(e.stopch)
	}()

	return nil
}

func (e *Local) Stop() error {
	close(e.stopch)
	if err := e.Environment.Stop(); err != nil {
		return err
	}
	return nil
}

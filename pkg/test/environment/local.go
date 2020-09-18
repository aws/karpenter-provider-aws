package environment

import (
	"context"

	"github.com/ellistarn/karpenter/pkg/apis"
	"github.com/ellistarn/karpenter/pkg/test"
	"github.com/ellistarn/karpenter/pkg/utils/log"
	"github.com/ellistarn/karpenter/pkg/utils/project"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type LocalInjector func(env *Local) error

type Local struct {
	envtest.Environment
	Manager manager.Manager
	Client  client.Client

	injectors []LocalInjector
	stopch    chan struct{}
}

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

func (e *Local) NewNamespace() (*test.Namespace, error) {
	ns := test.NewNamespace()
	v1ns := v1.Namespace(*ns) // Convert to v1 API object to make API requests
	if err := e.GetClient().Create(context.Background(), &v1ns); err != nil {
		return nil, err
	}

	go func() {
		<-e.stopch
		if err := e.GetClient().Delete(context.Background(), &v1ns); err != nil {
			zap.S().Error(errors.Wrap(err, "Failed to tear down namespace"))
		}
	}()
	return ns, nil
}

func (e *Local) GetClient() client.Client {
	return e.Manager.GetClient()
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
	e.Client = e.Manager.GetClient()

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

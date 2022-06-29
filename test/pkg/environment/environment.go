package environment

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/environment"
	loggingtesting "knative.dev/pkg/logging/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/utils/env"
)

var EnvironmentName = flag.String("environment-name", env.WithDefaultString("ENVIRONMENT_NAME", ""), "Environment name enables discovery of the testing environment")

type Environment struct {
	context.Context
	Options *Options
	Client  client.Client
	Monitor *Monitor
}

func NewEnvironment(t *testing.T) (*Environment, error) {
	ctx := loggingtesting.TestContextWithLogger(t)
	client, err := NewLocalClient()
	if err != nil {
		return nil, err
	}
	options, err := NewOptions()
	if err != nil {
		return nil, err
	}
	gomega.SetDefaultEventuallyTimeout(5 * time.Minute)
	gomega.SetDefaultEventuallyPollingInterval(1 * time.Second)
	return &Environment{Context: ctx,
		Options: options,
		Client:  client,
		Monitor: NewClusterMonitor(ctx, client),
	}, nil
}

func NewLocalClient() (client.Client, error) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := apis.AddToScheme(scheme); err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	config, err := (&environment.ClientConfig{Kubeconfig: path.Join(home, ".kube/config")}).GetRESTConfig()
	if err != nil {
		return nil, err
	}
	return client.New(config, client.Options{Scheme: scheme})
}

type Options struct {
	EnvironmentName string
}

func NewOptions() (*Options, error) {
	options := &Options{
		EnvironmentName: *EnvironmentName,
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	return options, nil
}

func (o Options) Validate() error {
	if o.EnvironmentName == "" {
		return fmt.Errorf("--environment-name must be defined")
	}
	return nil
}

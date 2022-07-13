package environment

import (
	"context"
	"flag"
	"fmt"
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

var ClusterName = flag.String("cluster-name", env.WithDefaultString("CLUSTER_NAME", ""), "Cluster name enables discovery of the testing environment")

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
	config, err := (&environment.ClientConfig{}).GetRESTConfig()
	if err != nil {
		return nil, err
	}
	return client.New(config, client.Options{Scheme: scheme})
}

type Options struct {
	ClusterName string
}

func NewOptions() (*Options, error) {
	options := &Options{
		ClusterName: *ClusterName,
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	return options, nil
}

func (o Options) Validate() error {
	if o.ClusterName == "" {
		return fmt.Errorf("--cluster-name must be defined")
	}
	return nil
}

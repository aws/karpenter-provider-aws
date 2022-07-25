package environment

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	loggingtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/utils/env"
	"github.com/aws/karpenter/pkg/utils/project"
)

var clusterNameFlag = flag.String("cluster-name", env.WithDefaultString("CLUSTER_NAME", ""), "Cluster name enables discovery of the testing environment")

type Environment struct {
	context.Context
	ClusterName string
	Client      client.Client
	KubeClient  kubernetes.Interface
	EC2API      ec2.EC2
	SSMAPI      ssm.SSM
	Monitor     *Monitor
}

func NewEnvironment(t *testing.T) (*Environment, error) {
	ctx := loggingtesting.TestContextWithLogger(t)
	config := NewConfig()
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}
	clusterName, err := DiscoverClusterName(config)
	if err != nil {
		return nil, err
	}
	gomega.SetDefaultEventuallyTimeout(5 * time.Minute)
	gomega.SetDefaultEventuallyPollingInterval(1 * time.Second)
	ginkgoconfig.DefaultReporterConfig.Verbose = true
	session := session.Must(session.NewSession())
	return &Environment{Context: ctx,
		ClusterName: clusterName,
		Client:      client,
		KubeClient:  kubernetes.NewForConfigOrDie(config),
		EC2API:      *ec2.New(session),
		SSMAPI:      *ssm.New(session),
		Monitor:     NewMonitor(ctx, client),
	}, nil
}

func DiscoverClusterName(config *rest.Config) (string, error) {
	if ptr.StringValue(clusterNameFlag) != "" {
		return ptr.StringValue(clusterNameFlag), nil
	}
	if config.ExecProvider != nil && len(config.ExecProvider.Args) > 5 {
		return config.ExecProvider.Args[5], nil
	}
	return "", fmt.Errorf("-cluster-name is not set and could not be discovered")
}

func NewConfig() *rest.Config {
	config := controllerruntime.GetConfigOrDie()
	config.UserAgent = fmt.Sprintf("testing.karpenter.sh-%s", project.Version)
	return config
}

func NewClient(config *rest.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := apis.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return client.New(config, client.Options{Scheme: scheme})
}

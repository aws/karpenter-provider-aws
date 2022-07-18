package environment

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	loggingtesting "knative.dev/pkg/logging/testing"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/utils/env"
)

var ClusterName = flag.String("cluster-name", env.WithDefaultString("CLUSTER_NAME", ""), "Cluster name enables discovery of the testing environment")
var Region = flag.String("region", env.WithDefaultString("AWS_REGION", ""), "Region that your test cluster lives in.")

type Environment struct {
	context.Context
	Options    *Options
	Client     client.Client
	KubeClient *kubernetes.Clientset
	Ec2Api     ec2.EC2
	SsmApi     ssm.SSM
	Monitor    *Monitor
}

func NewEnvironment(t *testing.T) (*Environment, error) {
	ctx := loggingtesting.TestContextWithLogger(t)
	client, err := NewLocalClient()
	if err != nil {
		return nil, err
	}
	kubeClient, err := NewKubeClient()
	if err != nil {
		return nil, err
	}
	options, err := NewOptions()
	if err != nil {
		return nil, err
	}
	gomega.SetDefaultEventuallyTimeout(5 * time.Minute)
	gomega.SetDefaultEventuallyPollingInterval(1 * time.Second)
	session := *session.Must(session.NewSession(
		&aws.Config{STSRegionalEndpoint: endpoints.RegionalSTSEndpoint, Region: aws.String(options.Region)},
	))
	return &Environment{Context: ctx,
		Options:    options,
		Client:     client,
		KubeClient: kubeClient,
		Ec2Api:     *ec2.New(&session),
		SsmApi:     *ssm.New(&session),
		Monitor:    NewClusterMonitor(ctx, client),
	}, nil
}

func NewKubeClient() (*kubernetes.Clientset, error) {
	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	controllerRuntimeConfig := controllerruntime.GetConfigOrDie()
	controllerRuntimeConfig.UserAgent = "KarpenterIntegrationTest"
	return kubernetes.NewForConfigOrDie(config), nil
}

func NewLocalClient() (client.Client, error) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := apis.AddToScheme(scheme); err != nil {
		return nil, err
	}
	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	return client.New(config, client.Options{Scheme: scheme})
}

type Options struct {
	ClusterName string
	Region      string
}

func NewOptions() (*Options, error) {
	options := &Options{
		ClusterName: *ClusterName,
		Region:      *Region,
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
	if o.Region == "" {
		return fmt.Errorf("either specify --region, or set $AWS_REGION in your environment")
	}
	return nil
}

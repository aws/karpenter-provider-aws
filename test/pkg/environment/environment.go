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

package environment

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/ssm"

	// . "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	loggingtesting "knative.dev/pkg/logging/testing"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/utils/env"
	"github.com/aws/karpenter/pkg/utils/project"
)

type Environment struct {
	context.Context
	ClusterName       string
	Region            string
	Client            client.Client
	KubeClient        kubernetes.Interface
	EC2API            ec2.EC2
	SSMAPI            ssm.SSM
	IAMAPI            iam.IAM
	Monitor           *Monitor
	StartingNodeCount int
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
	session := session.Must(session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable}))

	return &Environment{Context: ctx,
		ClusterName: clusterName,
		Client:      client,
		KubeClient:  kubernetes.NewForConfigOrDie(config),
		EC2API:      *ec2.New(session),
		SSMAPI:      *ssm.New(session),
		IAMAPI:      *iam.New(session),
		Region:      *session.Config.Region,
		Monitor:     NewMonitor(ctx, client),
	}, nil
}

func DiscoverClusterName(config *rest.Config) (string, error) {
	if clusterName := env.WithDefaultString("CLUSTER_NAME", ""); clusterName != "" {
		return clusterName, nil
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

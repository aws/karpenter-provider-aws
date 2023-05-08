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

package common

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"
	loggingtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/system"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coreapis "github.com/aws/karpenter-core/pkg/apis"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/utils/project"
)

const (
	clusterMetricServiceEndpoint = "http://prometheus-kube-prometheus-prometheus.prometheus.svc.cluster.local:9090"
	localMetricServiceEndpoint   = "http://localhost:9090"
)

type Environment struct {
	context.Context

	Client     client.Client
	Config     *rest.Config
	KubeClient kubernetes.Interface
	Monitor    *Monitor
	PromClient v1.API

	StartingNodeCount int
}

func NewEnvironment(t *testing.T) *Environment {
	ctx := loggingtesting.TestContextWithLogger(t)
	config := NewConfig()
	client := lo.Must(NewClient(config))

	lo.Must0(os.Setenv(system.NamespaceEnvKey, "karpenter"))
	kubernetesInterface := kubernetes.NewForConfigOrDie(config)
	ctx = injection.WithSettingsOrDie(ctx, kubernetesInterface, apis.Settings...)

	gomega.SetDefaultEventuallyTimeout(5 * time.Minute)
	gomega.SetDefaultEventuallyPollingInterval(1 * time.Second)
	return &Environment{
		Context:    ctx,
		Config:     config,
		Client:     client,
		KubeClient: kubernetes.NewForConfigOrDie(config),
		Monitor:    NewMonitor(ctx, client),
		PromClient: getPromClient(ctx),
	}
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
	if err := coreapis.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return client.New(config, client.Options{Scheme: scheme})
}

// getPromClient performs a fallback mechanism for getting the prometheus client for prometheus queries starting
// with the cluster endpoint, falling back to a local endpoint, ending with no prometheus client at all
func getPromClient(ctx context.Context) v1.API {
	if _, err := http.DefaultClient.Get(clusterMetricServiceEndpoint); err == nil {
		logging.FromContext(ctx).With("endpoint", clusterMetricServiceEndpoint).Debugf("using cluster service prometheus client")
		return v1.NewAPI(lo.Must(api.NewClient(api.Config{Address: clusterMetricServiceEndpoint})))
	}
	// This assumes that you have done "kubectl port-forward -n <namespace> svc/<name> 9090"
	if _, err := http.DefaultClient.Get(localMetricServiceEndpoint); err == nil {
		logging.FromContext(ctx).With("endpoint", localMetricServiceEndpoint).Debugf("using local prometheus client")
		return v1.NewAPI(lo.Must(api.NewClient(api.Config{Address: localMetricServiceEndpoint})))
	}
	logging.FromContext(ctx).Debug("using no prometheus client")
	return nil
}

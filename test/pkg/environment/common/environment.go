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
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	loggingtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/system"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coreapis "github.com/aws/karpenter-core/pkg/apis"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/utils/project"
)

type ContextKey string

const (
	GitRefContextKey = ContextKey("gitRef")
)

type Environment struct {
	context.Context
	cancel context.CancelFunc

	Client     client.Client
	Config     *rest.Config
	KubeClient kubernetes.Interface
	Monitor    *Monitor

	StartingNodeCount int
}

func NewEnvironment(t *testing.T) *Environment {
	ctx := loggingtesting.TestContextWithLogger(t)
	ctx, cancel := context.WithCancel(ctx)
	config := NewConfig()
	client := NewClient(ctx, config)

	lo.Must0(os.Setenv(system.NamespaceEnvKey, "karpenter"))
	kubernetesInterface := kubernetes.NewForConfigOrDie(config)
	ctx = injection.WithSettingsOrDie(ctx, kubernetesInterface, apis.Settings...)
	if val, ok := os.LookupEnv("GIT_REF"); ok {
		ctx = context.WithValue(ctx, GitRefContextKey, val)
	}

	gomega.SetDefaultEventuallyTimeout(5 * time.Minute)
	gomega.SetDefaultEventuallyPollingInterval(1 * time.Second)
	return &Environment{
		Context:    ctx,
		cancel:     cancel,
		Config:     config,
		Client:     client,
		KubeClient: kubernetes.NewForConfigOrDie(config),
		Monitor:    NewMonitor(ctx, client),
	}
}

func (env *Environment) Stop() {
	env.cancel()
}

func NewConfig() *rest.Config {
	config := controllerruntime.GetConfigOrDie()
	config.UserAgent = fmt.Sprintf("testing-%s", project.Version)
	config.QPS = 1e6
	config.Burst = 1e6
	return config
}

func NewClient(ctx context.Context, config *rest.Config) client.Client {
	scheme := runtime.NewScheme()
	lo.Must0(clientgoscheme.AddToScheme(scheme))
	lo.Must0(apis.AddToScheme(scheme))
	lo.Must0(coreapis.AddToScheme(scheme))

	cache := lo.Must(cache.New(config, cache.Options{Scheme: scheme}))
	lo.Must0(cache.IndexField(ctx, &v1.Pod{}, "spec.nodeName", func(o client.Object) []string {
		pod := o.(*v1.Pod)
		return []string{pod.Spec.NodeName}
	}))
	lo.Must0(cache.IndexField(ctx, &v1.Event{}, "involvedObject.kind", func(o client.Object) []string {
		evt := o.(*v1.Event)
		return []string{evt.InvolvedObject.Kind}
	}))
	lo.Must0(cache.IndexField(ctx, &v1.Node{}, "spec.unschedulable", func(o client.Object) []string {
		node := o.(*v1.Node)
		return []string{strconv.FormatBool(node.Spec.Unschedulable)}
	}))
	c := lo.Must(client.NewDelegatingClient(client.NewDelegatingClientInput{
		CacheReader: cache,
		Client:      lo.Must(client.New(config, client.Options{Scheme: scheme})),
	}))
	go func() {
		lo.Must0(cache.Start(ctx))
	}()
	if !cache.WaitForCacheSync(ctx) {
		log.Fatalf("cache failed to sync")
	}
	return c
}

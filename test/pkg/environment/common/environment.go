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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	. "sigs.k8s.io/karpenter/pkg/utils/testing" //nolint:stylecheck,staticcheck

	"knative.dev/pkg/system"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coreapis "sigs.k8s.io/karpenter/pkg/apis"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/operator"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
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
	ctx := TestContextWithLogger(t)
	ctx, cancel := context.WithCancel(ctx)
	config := NewConfig()
	client := NewClient(ctx, config)

	lo.Must0(os.Setenv(system.NamespaceEnvKey, "kube-system"))
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
	config.UserAgent = fmt.Sprintf("testing-%s", operator.Version)
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
	lo.Must0(cache.IndexField(ctx, &v1.Node{}, "spec.taints[*].karpenter.sh/disruption", func(o client.Object) []string {
		node := o.(*v1.Node)
		t, _ := lo.Find(node.Spec.Taints, func(t v1.Taint) bool {
			return t.Key == corev1beta1.DisruptionTaintKey
		})
		return []string{t.Value}
	}))

	c := lo.Must(client.New(config, client.Options{Scheme: scheme, Cache: &client.CacheOptions{Reader: cache}}))

	go func() {
		lo.Must0(cache.Start(ctx))
	}()
	if !cache.WaitForCacheSync(ctx) {
		log.Fatalf("cache failed to sync")
	}
	return c
}

func (env *Environment) DefaultNodePool(nodeClass *v1beta1.EC2NodeClass) *corev1beta1.NodePool {
	nodePool := coretest.NodePool()
	nodePool.Spec.Template.Spec.NodeClassRef = &corev1beta1.NodeClassReference{
		Name: nodeClass.Name,
	}
	nodePool.Spec.Template.Spec.Requirements = []corev1beta1.NodeSelectorRequirementWithMinValues{
		{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{string(v1.Linux)},
			},
		},
		{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      corev1beta1.CapacityTypeLabelKey,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{corev1beta1.CapacityTypeOnDemand},
			},
		},
		{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceCategory,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"c", "m", "r"},
			},
		},
		{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceGeneration,
				Operator: v1.NodeSelectorOpGt,
				Values:   []string{"2"},
			},
		},
		// Filter out a1 instance types, which are incompatible with AL2023 AMIs
		{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceFamily,
				Operator: v1.NodeSelectorOpNotIn,
				Values:   []string{"a1"},
			},
		},
	}
	nodePool.Spec.Disruption.ConsolidateAfter = lo.ToPtr(corev1beta1.MustParseNillableDuration("Never"))
	nodePool.Spec.Disruption.ExpireAfter = corev1beta1.MustParseNillableDuration("Never")
	nodePool.Spec.Limits = corev1beta1.Limits(v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("1000"),
		v1.ResourceMemory: resource.MustParse("1000Gi"),
	})
	return nodePool
}

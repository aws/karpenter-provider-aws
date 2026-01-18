/*
Copyright The Kubernetes Authors.

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
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/awslabs/operatorpkg/object"
	"github.com/awslabs/operatorpkg/status"
	"github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	serializeryaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/utils/testing" //nolint:stylecheck,staticcheck
	"sigs.k8s.io/karpenter/test/pkg/debug"
)

type ContextKey string

const GitRefContextKey = ContextKey("gitRef")

// I need to add the the default kwok nodeclass path
// That way it's not defined in code but we use it when we initialize the nodeclass
var (
	//go:embed default_kowknodeclass.yaml
	defaultNodeClass []byte
	//go:embed default_nodepool.yaml
	defaultNodePool []byte
	nodeClassPath   = flag.String("default-nodeclass", "", "Pass in a default cloud specific node class")
	nodePoolPath    = flag.String("default-nodepool", "", "Pass in a default karpenter nodepool")
)

type Environment struct {
	context.Context
	cancel context.CancelFunc

	TimeIntervalCollector *debug.TimeIntervalCollector
	Client                client.Client
	Config                *rest.Config
	KubeClient            kubernetes.Interface
	Monitor               *Monitor
	DefaultNodeClass      *unstructured.Unstructured

	OutputDir         string
	StartingNodeCount int
}

func NewEnvironment(t *testing.T) *Environment {
	ctx := TestContextWithLogger(t)
	ctx, cancel := context.WithCancel(ctx)
	config := NewConfig()
	client := NewClient(ctx, config)

	if val, ok := os.LookupEnv("GIT_REF"); ok {
		ctx = context.WithValue(ctx, GitRefContextKey, val)
	}
	// Get the output dir if it's set
	outputDir, _ := os.LookupEnv("OUTPUT_DIR")

	gomega.SetDefaultEventuallyTimeout(10 * time.Minute)
	gomega.SetDefaultEventuallyPollingInterval(1 * time.Second)
	return &Environment{
		Context:               ctx,
		cancel:                cancel,
		Config:                config,
		Client:                client,
		KubeClient:            kubernetes.NewForConfigOrDie(config),
		Monitor:               NewMonitor(ctx, client),
		TimeIntervalCollector: debug.NewTimestampCollector(),
		OutputDir:             outputDir,
		DefaultNodeClass:      decodeNodeClass(),
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
	cache := lo.Must(cache.New(config, cache.Options{Scheme: scheme.Scheme}))
	lo.Must0(cache.IndexField(ctx, &corev1.Pod{}, "spec.nodeName", func(o client.Object) []string {
		pod := o.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}))
	lo.Must0(cache.IndexField(ctx, &corev1.Event{}, "involvedObject.kind", func(o client.Object) []string {
		evt := o.(*corev1.Event)
		return []string{evt.InvolvedObject.Kind}
	}))
	lo.Must0(cache.IndexField(ctx, &corev1.Node{}, "spec.unschedulable", func(o client.Object) []string {
		node := o.(*corev1.Node)
		return []string{strconv.FormatBool(node.Spec.Unschedulable)}
	}))
	lo.Must0(cache.IndexField(ctx, &corev1.Node{}, "spec.taints[*].karpenter.sh/disrupted", func(o client.Object) []string {
		node := o.(*corev1.Node)
		_, found := lo.Find(node.Spec.Taints, func(t corev1.Taint) bool {
			return t.Key == v1.DisruptedTaintKey
		})
		return []string{lo.Ternary(found, "true", "false")}
	}))
	lo.Must0(cache.IndexField(ctx, &v1.NodeClaim{}, "status.conditions[*].type", func(o client.Object) []string {
		nodeClaim := o.(*v1.NodeClaim)
		return lo.Map(nodeClaim.Status.Conditions, func(c status.Condition, _ int) string {
			return c.Type
		})
	}))

	c := lo.Must(client.New(config, client.Options{Scheme: scheme.Scheme, Cache: &client.CacheOptions{Reader: cache}}))

	go func() {
		lo.Must0(cache.Start(ctx))
	}()
	if !cache.WaitForCacheSync(ctx) {
		log.Fatalf("cache failed to sync")
	}
	return c
}

func (env *Environment) DefaultNodePool(nodeClass *unstructured.Unstructured) *v1.NodePool {
	nodePool := &v1.NodePool{}
	if lo.FromPtr(nodePoolPath) == "" {
		nodePool = object.Unmarshal[v1.NodePool](defaultNodePool)
	} else {
		file := lo.Must1(os.ReadFile(lo.FromPtr(nodePoolPath)))
		lo.Must0(yaml.Unmarshal(file, nodePool))
	}

	// Update to use the provided default nodeclass
	nodePool.Spec.Template.Spec.NodeClassRef = &v1.NodeClassReference{
		Kind:  nodeClass.GetObjectKind().GroupVersionKind().Kind,
		Group: nodeClass.GetObjectKind().GroupVersionKind().Group,
		Name:  nodeClass.GetName(),
	}
	nodePool.Labels = lo.Assign(nodePool.Labels, map[string]string{test.DiscoveryLabel: "unspecified"})
	nodePool.Spec.Template.Labels = lo.Assign(nodePool.Spec.Template.Labels, map[string]string{test.DiscoveryLabel: "unspecified"})
	nodePool.Name = fmt.Sprintf("%s-%s", nodePool.GetName(), test.RandomName())
	return nodePool
}

func (env *Environment) IsDefaultNodeClassKWOK() bool {
	return env.DefaultNodeClass.GetObjectKind().GroupVersionKind().Kind == "KWOKNodeClass"
}

func decodeNodeClass() *unstructured.Unstructured {
	// Open the file
	if lo.FromPtr(nodeClassPath) == "" {
		return object.Unmarshal[unstructured.Unstructured](defaultNodeClass)
	}

	file := lo.Must1(os.Open(lo.FromPtr(nodeClassPath)))
	content := lo.Must1(io.ReadAll(file))

	decoder := serializeryaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	u := &unstructured.Unstructured{}
	_, gvk, _ := decoder.Decode(content, nil, u)
	u.SetGroupVersionKind(lo.FromPtr(gvk))

	return u
}

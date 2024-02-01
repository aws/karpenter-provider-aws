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

package chaos_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	coretest "sigs.k8s.io/karpenter/pkg/test"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/test/pkg/debug"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *aws.Environment
var nodeClass *v1beta1.EC2NodeClass
var nodePool *corev1beta1.NodePool

func TestChaos(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Chaos")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Chaos", func() {
	Describe("Runaway Scale-Up", func() {
		It("should not produce a runaway scale-up when consolidation is enabled", Label(debug.NoWatch), Label(debug.NoEvents), func() {
			ctx, cancel := context.WithCancel(env.Context)
			defer cancel()

			nodePool = coretest.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
				NodeSelectorRequirement: v1.NodeSelectorRequirement{
					Key:      corev1beta1.CapacityTypeLabelKey,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{corev1beta1.CapacityTypeSpot},
				},
			})
			nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenUnderutilized
			nodePool.Spec.Disruption.ConsolidateAfter = nil

			numPods := 1
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "my-app"},
					},
					TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
				},
			})
			// Start a controller that adds taints to nodes after creation
			Expect(startTaintAdder(ctx, env.Config)).To(Succeed())
			startNodeCountMonitor(ctx, env.Client)

			// Create a deployment with a single pod
			env.ExpectCreated(nodeClass, nodePool, dep)

			// Expect that we never get over a high number of nodes
			Consistently(func(g Gomega) {
				list := &v1.NodeList{}
				g.Expect(env.Client.List(env.Context, list, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())
				g.Expect(len(list.Items)).To(BeNumerically("<", 35))
			}, time.Minute*5).Should(Succeed())
		})
		It("should not produce a runaway scale-up when emptiness is enabled", Label(debug.NoWatch), Label(debug.NoEvents), func() {
			ctx, cancel := context.WithCancel(env.Context)
			defer cancel()

			nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenEmpty
			nodePool.Spec.Disruption.ConsolidateAfter = &corev1beta1.NillableDuration{Duration: lo.ToPtr(30 * time.Second)}
			numPods := 1
			dep := coretest.Deployment(coretest.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: coretest.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "my-app"},
					},
					TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
				},
			})
			// Start a controller that adds taints to nodes after creation
			Expect(startTaintAdder(ctx, env.Config)).To(Succeed())
			startNodeCountMonitor(ctx, env.Client)

			// Create a deployment with a single pod
			env.ExpectCreated(nodeClass, nodePool, dep)

			// Expect that we never get over a high number of nodes
			Consistently(func(g Gomega) {
				list := &v1.NodeList{}
				g.Expect(env.Client.List(env.Context, list, client.HasLabels{coretest.DiscoveryLabel})).To(Succeed())
				g.Expect(len(list.Items)).To(BeNumerically("<", 35))
			}, time.Minute*5).Should(Succeed())
		})
	})
})

type taintAdder struct {
	kubeClient client.Client
}

func (t *taintAdder) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	node := &v1.Node{}
	if err := t.kubeClient.Get(ctx, req.NamespacedName, node); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	mergeFrom := client.MergeFrom(node.DeepCopy())
	taint := v1.Taint{
		Key:    "test",
		Value:  "true",
		Effect: v1.TaintEffectNoExecute,
	}
	if !lo.Contains(node.Spec.Taints, taint) {
		node.Spec.Taints = append(node.Spec.Taints, taint)
		if err := t.kubeClient.Patch(ctx, node, mergeFrom); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (t *taintAdder) Builder(mgr manager.Manager) *controllerruntime.Builder {
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&v1.Node{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			node := obj.(*v1.Node)
			if _, ok := node.Labels[coretest.DiscoveryLabel]; !ok {
				return false
			}
			return true
		}))
}

func startTaintAdder(ctx context.Context, config *rest.Config) error {
	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{})
	if err != nil {
		return err
	}
	adder := &taintAdder{kubeClient: mgr.GetClient()}
	if err = adder.Builder(mgr).Complete(adder); err != nil {
		return err
	}
	go func() {
		Expect(mgr.Start(ctx)).To(Succeed())
	}()
	return nil
}

func startNodeCountMonitor(ctx context.Context, kubeClient client.Client) {
	createdNodes := atomic.Int64{}
	deletedNodes := atomic.Int64{}

	factory := informers.NewSharedInformerFactoryWithOptions(env.KubeClient, time.Second*30,
		informers.WithTweakListOptions(func(l *metav1.ListOptions) { l.LabelSelector = corev1beta1.NodePoolLabelKey }))
	nodeInformer := factory.Core().V1().Nodes().Informer()
	_ = lo.Must(nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(_ interface{}) {
			createdNodes.Add(1)
		},
		DeleteFunc: func(_ interface{}) {
			deletedNodes.Add(1)
		},
	}))
	factory.Start(ctx.Done())
	go func() {
		for {
			list := &v1.NodeList{}
			if err := kubeClient.List(ctx, list, client.HasLabels{coretest.DiscoveryLabel}); err == nil {
				readyCount := lo.CountBy(list.Items, func(n v1.Node) bool {
					return nodeutils.GetCondition(&n, v1.NodeReady).Status == v1.ConditionTrue
				})
				fmt.Printf("[NODE COUNT] CURRENT: %d | READY: %d | CREATED: %d | DELETED: %d\n", len(list.Items), readyCount, createdNodes.Load(), deletedNodes.Load())
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second * 5):
			}
		}
	}()
}

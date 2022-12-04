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

package chaos

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	nodeutils "github.com/aws/karpenter-core/pkg/utils/node"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment/common"
)

var env *common.Environment

func TestChaos(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = common.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Chaos")
}

var _ = BeforeEach(func() { env.BeforeEach() })
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.ForceCleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Chaos", func() {
	Describe("Runaway Scale-Up", func() {
		It("should not produce a runaway scale-up when consolidation is enabled", Label(common.NoWatch), Label(common.NoEvents), func() {
			ctx, cancel := context.WithCancel(env.Context)
			defer cancel()

			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{v1alpha5.DiscoveryTagKey: settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{v1alpha5.DiscoveryTagKey: settings.FromContext(env.Context).ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha5.LabelCapacityType,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{v1alpha5.CapacityTypeSpot},
					},
				},
				Consolidation: &v1alpha5.Consolidation{
					Enabled: lo.ToPtr(true),
				},
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			})
			numPods := 1
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: test.PodOptions{
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
			env.ExpectCreated(provider, provisioner, dep)

			// Expect that we never get over a high number of nodes
			Consistently(func(g Gomega) {
				list := &v1.NodeList{}
				g.Expect(env.Client.List(env.Context, list, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
				g.Expect(len(list.Items)).To(BeNumerically("<", 35))
			}, time.Minute*5).Should(Succeed())
		})
		It("should not produce a runaway scale-up when ttlSecondsAfterEmpty is enabled", Label(common.NoWatch), Label(common.NoEvents), func() {
			ctx, cancel := context.WithCancel(env.Context)
			defer cancel()

			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{v1alpha5.DiscoveryTagKey: settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{v1alpha5.DiscoveryTagKey: settings.FromContext(env.Context).ClusterName},
			}})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha5.LabelCapacityType,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{v1alpha5.CapacityTypeSpot},
					},
				},
				TTLSecondsAfterEmpty: lo.ToPtr[int64](30),
				ProviderRef:          &v1alpha5.ProviderRef{Name: provider.Name},
			})
			numPods := 1
			dep := test.Deployment(test.DeploymentOptions{
				Replicas: int32(numPods),
				PodOptions: test.PodOptions{
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
			env.ExpectCreated(provider, provisioner, dep)

			// Expect that we never get over a high number of nodes
			Consistently(func(g Gomega) {
				list := &v1.NodeList{}
				g.Expect(env.Client.List(env.Context, list, client.HasLabels{test.DiscoveryLabel})).To(Succeed())
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
			if _, ok := node.Labels[test.DiscoveryLabel]; !ok {
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
		informers.WithTweakListOptions(func(l *metav1.ListOptions) { l.LabelSelector = v1alpha5.ProvisionerNameLabelKey }))
	nodeInformer := factory.Core().V1().Nodes().Informer()
	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(_ interface{}) {
			createdNodes.Add(1)
		},
		DeleteFunc: func(_ interface{}) {
			deletedNodes.Add(1)
		},
	})
	factory.Start(ctx.Done())
	go func() {
		for {
			list := &v1.NodeList{}
			if err := kubeClient.List(ctx, list, client.HasLabels{test.DiscoveryLabel}); err == nil {
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

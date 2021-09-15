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

package scheduling_test

import (
	"context"
	"testing"
	"time"

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha4"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers/allocation"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/binpacking"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/scheduling"
	"github.com/awslabs/karpenter/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var provisioner *v1alpha4.Provisioner
var controller *allocation.Controller
var env *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Allocation/Scheduling")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider := &fake.CloudProvider{}
		registry.RegisterOrDie(cloudProvider)
		controller = &allocation.Controller{
			Filter:        &allocation.Filter{KubeClient: e.Client},
			Binder:        &allocation.Binder{KubeClient: e.Client, CoreV1Client: corev1.NewForConfigOrDie(e.Config)},
			Batcher:       allocation.NewBatcher(1*time.Millisecond, 1*time.Millisecond),
			Scheduler:     scheduling.NewScheduler(cloudProvider, e.Client),
			Packer:        binpacking.NewPacker(),
			CloudProvider: cloudProvider,
			KubeClient:    e.Client,
		}
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	provisioner = &v1alpha4.Provisioner{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha4.DefaultProvisioner.Name},
		Spec: v1alpha4.ProvisionerSpec{},
	}
})

var _ = AfterEach(func() {
	ExpectCleanedUp(env.Client)
})

var _ = Describe("Topology", func() {
	labels := map[string]string{"test": "test"}

	It("should ignore unknown topology keys", func() {
		ExpectCreated(env.Client, provisioner)
		pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
			test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: []v1.TopologySpreadConstraint{{
				TopologyKey:       "unknown",
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}}),
		)
		Expect(pods[0].Spec.NodeName).To(BeEmpty())
	})

	Context("Zone", func() {
		It("should balance pods across zones", func() {
			ExpectCreated(env.Client, provisioner)
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(env.Client, v1.LabelTopologyZone).To(ConsistOf(1, 1, 2))
		})
		It("should respect provisioner zonal constraints", func() {
			provisioner.Spec.Constraints.Zones = []string{"test-zone-1", "test-zone-2"}
			ExpectCreated(env.Client, provisioner)
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(env.Client, v1.LabelTopologyZone).To(ConsistOf(2, 2))
		})
		It("should only count scheduled pods with matching labels scheduled to nodes with a corresponding domain", func() {
			firstNode := test.Node(test.NodeOptions{Labels: map[string]string{v1.LabelTopologyZone: "test-zone-1"}})
			secondNode := test.Node(test.NodeOptions{Labels: map[string]string{v1.LabelTopologyZone: "test-zone-2"}})
			thirdNode := test.Node(test.NodeOptions{}) // missing topology domain
			ExpectCreated(env.Client, provisioner, firstNode, secondNode, thirdNode)
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.Pod(test.PodOptions{Labels: labels}), // ignored, pending
				test.Pod(test.PodOptions{}),               // ignored, missing labels
				test.Pod(test.PodOptions{Labels: labels, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{Labels: labels, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{Labels: labels, NodeName: secondNode.Name}),
				test.Pod(test.PodOptions{Labels: labels, NodeName: thirdNode.Name}), //ignored, no domain
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
			)
			nodes := v1.NodeList{}
			Expect(env.Client.List(ctx, &nodes)).To(Succeed())
			ExpectSkew(env.Client, v1.LabelTopologyZone).To(ConsistOf(2, 2, 1))
		})
	})

	Context("Hostname", func() {
		It("should balance pods across nodes", func() {
			ExpectCreated(env.Client, provisioner)
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(env.Client, v1.LabelHostname).To(ConsistOf(1, 1, 1, 1))
		})
		It("should balance pods on the same hostname up to maxskew", func() {
			ExpectCreated(env.Client, provisioner)
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           4,
			}}
			ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{Labels: labels, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(env.Client, v1.LabelHostname).To(ConsistOf(4))
		})
	})

	Context("Combined Hostname and Topology", func() {
		It("should spread pods while respecting both constraints", func() {
			ExpectCreated(env.Client, provisioner)
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           3,
			}}
			ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				MakePods(2, test.PodOptions{Labels: labels, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(env.Client, v1.LabelTopologyZone).To(ConsistOf(1, 1))
			ExpectSkew(env.Client, v1.LabelHostname).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				MakePods(3, test.PodOptions{Labels: labels, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(env.Client, v1.LabelTopologyZone).To(ConsistOf(2, 2, 1))
			ExpectSkew(env.Client, v1.LabelHostname).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				MakePods(5, test.PodOptions{Labels: labels, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(env.Client, v1.LabelTopologyZone).To(ConsistOf(4, 3, 3))
			ExpectSkew(env.Client, v1.LabelHostname).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				MakePods(11, test.PodOptions{Labels: labels, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(env.Client, v1.LabelTopologyZone).To(ConsistOf(7, 7, 7))
			ExpectSkew(env.Client, v1.LabelHostname).ToNot(ContainElements(BeNumerically(">", 3)))
		})
	})
})

var _ = Describe("Taints", func() {
	It("should schedule pods that tolerate provisioner constraints", func() {
		provisioner.Spec.Taints = []v1.Taint{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}}
		schedulable := []client.Object{
			// Tolerates with OpExists
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoSchedule}}}),
			// Tolerates with OpEqual
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Value: "test-value", Operator: v1.TolerationOpEqual, Effect: v1.TaintEffectNoSchedule}}}),
		}
		unschedulable := []client.Object{
			// Missing toleration
			test.UnschedulablePod(),
			// key mismatch with OpExists
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "invalid", Operator: v1.TolerationOpExists}}}),
		}
		ExpectCreated(env.Client, provisioner)
		ExpectCreatedWithStatus(env.Client, schedulable...)
		ExpectCreatedWithStatus(env.Client, unschedulable...)
		ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

		nodes := &v1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(1))
		for _, pod := range schedulable {
			scheduled := ExpectPodExists(env.Client, pod.GetName(), pod.GetNamespace())
			ExpectNodeExists(env.Client, scheduled.Spec.NodeName)
		}
		for _, pod := range unschedulable {
			unscheduled := ExpectPodExists(env.Client, pod.GetName(), pod.GetNamespace())
			Expect(unscheduled.Spec.NodeName).To(BeEmpty())
		}
	})
	It("should not generate taints for OpExists", func() {
		pods := []client.Object{
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoExecute}}}),
		}
		ExpectCreated(env.Client, provisioner)
		ExpectCreatedWithStatus(env.Client, pods...)
		ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
		nodes := &v1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(1))
	})
	It("should generate taints for pod tolerations", func() {
		pods := []client.Object{
			// Matching pods schedule together on a node with a matching taint
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			}),
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			}),
			// Key is different, generate new node with a taint for this key
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "another-test-key", Operator: v1.TolerationOpEqual, Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			}),
			// Value is different, generate new node with a taint for this value
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "another-test-value", Effect: v1.TaintEffectNoSchedule}},
			}),
			// Effect is different, generate new node with a taint for this value
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "test-value", Effect: v1.TaintEffectNoExecute}},
			}),
			// Missing effect, generate a new node with a taints for all effects
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "test-value"}},
			}),
			// // No taint generated
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoExecute}}}),
		}
		ExpectCreated(env.Client, provisioner)
		ExpectCreatedWithStatus(env.Client, pods...)
		ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

		nodes := &v1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(6))

		for i, expectedTaintsPerNode := range [][]v1.Taint{
			{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			{{Key: "another-test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			{{Key: "test-key", Value: "another-test-value", Effect: v1.TaintEffectNoSchedule}},
			{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoExecute}},
			{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}, {Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoExecute}},
		} {
			pod := ExpectPodExists(env.Client, pods[i].GetName(), pods[i].GetNamespace())
			node := ExpectNodeExists(env.Client, pod.Spec.NodeName)
			for _, taint := range expectedTaintsPerNode {
				Expect(node.Spec.Taints).To(ContainElement(taint))
			}
		}

		pod := ExpectPodExists(env.Client, pods[len(pods)-1].GetName(), pods[len(pods)-1].GetNamespace())
		node := ExpectNodeExists(env.Client, pod.Spec.NodeName)
		Expect(node.Spec.Taints).To(HaveLen(2)) // Expect no taints generated beyond defaults
	})
})

func MakePods(count int, options test.PodOptions) (pods []*v1.Pod) {
	for i := 0; i < count; i++ {
		pods = append(pods, test.UnschedulablePod(options))
	}
	return pods
}

func ExpectSkew(c client.Client, topologyKey string) Assertion {
	nodes := &v1.NodeList{}
	Expect(c.List(ctx, nodes)).To(Succeed())
	pods := &v1.PodList{}
	Expect(c.List(ctx, pods)).To(Succeed())
	skew := map[string]int{}
	for _, pod := range pods.Items {
		for _, node := range nodes.Items {
			if pod.Spec.NodeName == node.Name {
				if topologyKey == v1.LabelHostname {
					skew[node.Name]++ // Check node name, since we strip placeholder hostname label
				} else if key, ok := node.Labels[topologyKey]; ok {
					skew[key]++
				}
			}
		}
	}
	return Expect(skew)
}

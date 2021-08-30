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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers/allocation"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/scheduling"
	"github.com/awslabs/karpenter/pkg/packing"
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
			Packer:        packing.NewPacker(),
			CloudProvider: cloudProvider,
			KubeClient:    e.Client,
		}
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Scheduling", func() {
	var provisioner *v1alpha3.Provisioner
	BeforeEach(func() {
		provisioner = &v1alpha3.Provisioner{ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.DefaultProvisioner.Name}}
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	Context("Topology", func() {
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

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
		registry.RegisterOrDie(ctx, cloudProvider)
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
		Spec:       v1alpha4.ProvisionerSpec{},
	}
})

var _ = AfterEach(func() {
	ExpectCleanedUp(env.Client)
})

var _ = Describe("NodeAffinity", func() {
	Context("Custom Labels", func() {
		It("should schedule pods that have matching node selectors", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should generate custom labels for node selectors", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{"another-key": "another-value"},
			}))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue("another-key", "another-value"))
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should not schedule pods that have conflicting node selectors", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{"test-key": "different-value"},
			}))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should schedule pods that have matching requirements", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value", "another-value"}},
				}},
			))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should not schedule pods that have conflicting requirements", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"another-value"}},
				}},
			))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should generate custom labels for requirements", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value", "another-value"}},
				}},
			))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", Or(Equal("test-value"), Equal("another-value"))))
		})
		It("should schedule pods that have matching preferences", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod(
				test.PodOptions{NodePreferences: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value", "another-value"}},
				}},
			))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should not schedule pods with have conflicting preferences", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod(
				test.PodOptions{NodePreferences: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-value"}},
				}},
			))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should generate custom labels for preferences", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod(
				test.PodOptions{NodePreferences: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value", "another-value"}},
				}},
			))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", Or(Equal("test-value"), Equal("another-value"))))
		})
		It("should generate custom labels with both requirements and preferences", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value", "another-value"}},
					},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: "test-key", Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-value"}},
					},
				},
			))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "another-value"))
		})
	})
	Context("Well Known Labels", func() {
		It("should use provisioner constraints", func() {
			provisioner.Spec.Zones = []string{"test-zone-2"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should use node selectors", func() {
			provisioner.Spec.Zones = []string{"test-zone-1", "test-zone-2"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-2"}}))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should not schedule the pod if nodeselector unknown", func() {
			provisioner.Spec.Zones = []string{"test-zone-1"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "unknown"}}))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should not schedule if node selector outside of provisioner constraints", func() {
			provisioner.Spec.Zones = []string{"test-zone-1"}
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-2"}}))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should schedule compatible requirements with Operator=In", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-3"}}},
				}))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
		})
		It("should not schedule incompatible preferences and requirements with Operator=In", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"unknown"}}},
				}))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should schedule compatible requirements with Operator=NotIn", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-1", "test-zone-2", "unknown"}}},
				}))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
		})
		It("should not schedule incompatible preferences and requirements with Operator=NotIn", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}}},
				}))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should schedule compatible preferences and requirements with Operator=In", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2", "unknown"}}},
				}))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should not schedule incompatible preferences and requirements with Operator=In", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"unknown"}}},
				}))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should schedule compatible preferences and requirements with Operator=NotIn", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-1", "test-zone-3"}}},
				}))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should not schedule incompatible preferences and requirements with Operator=NotIn", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}},
				}))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should schedule compatible node selectors, preferences and requirements", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-3"},
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}},
				}))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
		})
		It("should not schedule incompatible node selectors, preferences and requirements", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-3"},
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-2", "test-zone-3"}}},
				}))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
		It("should combine multidimensional node selectors, preferences and requirements", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeSelector: map[string]string{
						v1.LabelTopologyZone:       "test-zone-3",
						v1.LabelInstanceTypeStable: "arm-instance-type",
					},
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"}},
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"default-instance-type", "arm-instance-type"}},
					},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"unnknown"}},
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpNotIn, Values: []string{"unknown"}},
					},
				}))
			node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "arm-instance-type"))
		})
		It("should not combine incompatible multidimensional node selectors, preferences and requirements", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					NodeSelector: map[string]string{
						v1.LabelTopologyZone:       "test-zone-3",
						v1.LabelInstanceTypeStable: "arm-instance-type",
					},
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"}},
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"default-instance-type", "arm-instance-type"}},
					},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-3"}},
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpNotIn, Values: []string{"arm-instance-type"}},
					},
				}))
			Expect(pods[0].Spec.NodeName).To(BeEmpty())
		})
	})
})

var _ = Describe("Preferential Fallback", func() {
	Context("Required", func() {
		It("should not relax the final term", func() {
			provisioner.Spec.Zones = []string{"test-zone-1"}
			provisioner.Spec.InstanceTypes = []string{"default-instance-type"}
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}}, // Should not be relaxed
				}},
			}}}}
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, pod)
			// Don't relax
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
			pod = ExpectPodExists(env.Client, pod.Name, pod.Namespace)
			Expect(pod.Spec.NodeName).To(BeEmpty())
			// Don't relax
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
			pod = ExpectPodExists(env.Client, pod.Name, pod.Namespace)
			Expect(pod.Spec.NodeName).To(BeEmpty())
		})
		It("should relax multiple terms", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
				}},
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
				}},
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
				}},
			}}}}
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, pod)
			// Remove first term
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
			pod = ExpectPodExists(env.Client, pod.Name, pod.Namespace)
			Expect(pod.Spec.NodeName).To(BeEmpty())
			// Remove second term
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
			pod = ExpectPodExists(env.Client, pod.Name, pod.Namespace)
			Expect(pod.Spec.NodeName).To(BeEmpty())
			// Success
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
			pod = ExpectPodExists(env.Client, pod.Name, pod.Namespace)
			ExpectNodeExists(env.Client, pod.Spec.NodeName)
		})
	})
	Context("Preferred", func() {
		It("should relax all terms", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
				},
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
				},
			}}}
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, pod)
			// Remove first term
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
			pod = ExpectPodExists(env.Client, pod.Name, pod.Namespace)
			Expect(pod.Spec.NodeName).To(BeEmpty())
			// Remove second term
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
			pod = ExpectPodExists(env.Client, pod.Name, pod.Namespace)
			Expect(pod.Spec.NodeName).To(BeEmpty())
			// Success
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
			pod = ExpectPodExists(env.Client, pod.Name, pod.Namespace)
			ExpectNodeExists(env.Client, pod.Spec.NodeName)
		})
	})
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

	Context("Zonal", func() {
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

	Context("Combined Hostname and Zonal Topology", func() {
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
			// value mismatch
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpEqual, Effect: v1.TaintEffectNoSchedule}}}),
		}
		ExpectCreated(env.Client, provisioner)
		ExpectCreatedWithStatus(env.Client, schedulable...)
		ExpectCreatedWithStatus(env.Client, unschedulable...)
		ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

		nodes := &v1.NodeList{}
		Expect(env.Client.List(ctx, nodes)).To(Succeed())
		Expect(len(nodes.Items)).To(Equal(1))
		Expect(nodes.Items[0].Spec.Taints[0]).To(Equal(provisioner.Spec.Taints[0]))
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

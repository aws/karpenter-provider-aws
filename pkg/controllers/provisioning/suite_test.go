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

package provisioning_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/selection"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils/resources"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var provisioningController *provisioning.Controller
var selectionController *selection.Controller
var env *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Provisioning")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider := &fake.CloudProvider{}
		registry.RegisterOrDie(ctx, cloudProvider)
		provisioningController = provisioning.NewController(ctx, e.Client, corev1.NewForConfigOrDie(e.Config), cloudProvider)
		selectionController = selection.NewController(e.Client, provisioningController)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Provisioning", func() {
	var provisioner *v1alpha5.Provisioner
	BeforeEach(func() {
		provisioner = &v1alpha5.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec: v1alpha5.ProvisionerSpec{
				Limits: v1alpha5.Limits{
					Resources: v1.ResourceList{
						v1.ResourceCPU: *resource.NewScaledQuantity(10, 0),
					},
				},
			},
		}
		provisioner.SetDefaults(ctx)
	})

	AfterEach(func() {
		ExpectProvisioningCleanedUp(ctx, env.Client, provisioningController)
	})

	Context("Reconciliation", func() {
		It("should provision nodes", func() {
			pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod())
			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(1))
			for _, pod := range pods {
				ExpectScheduled(ctx, env.Client, pod)
			}
		})
		It("should provision nodes for pods with supported node selectors", func() {
			schedulable := []*v1.Pod{
				// Constrained by provisioner
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name}}),
				// Constrained by zone
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1"}}),
				// Constrained by instanceType
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "default-instance-type"}}),
				// Constrained by architecture
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: "arm64"}}),
				// Constrained by operatingSystem
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelOSStable: "linux"}}),
			}
			unschedulable := []*v1.Pod{
				// Ignored, matches another provisioner
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: "unknown"}}),
				// Ignored, invalid zone
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "unknown"}}),
				// Ignored, invalid instance type
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "unknown"}}),
				// Ignored, invalid architecture
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: "unknown"}}),
				// Ignored, invalid operating system
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelOSStable: "unknown"}}),
				// Ignored, invalid capacity type
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.LabelCapacityType: "unknown"}}),
				// Ignored, label selector does not match
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{"foo": "bar"}}),
			}
			for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, schedulable...) {
				ExpectScheduled(ctx, env.Client, pod)
			}
			for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, unschedulable...) {
				ExpectNotScheduled(ctx, env.Client, pod)
			}
		})
		It("should provision nodes for accelerators", func() {
			for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner,
				test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")}},
				}),
				test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{resources.AMDGPU: resource.MustParse("1")}},
				}),
				test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")}},
				}),
			) {
				ExpectScheduled(ctx, env.Client, pod)
			}
		})
		Context("Resource Limits", func() {
			It("should not schedule when limits are exceeded", func() {
				provisioner.Status = v1alpha5.ProvisionerStatus{
					Resources: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("100"),
					},
				}
				provisioner.Spec.Limits.Resources[v1.ResourceCPU] = resource.MustParse("20")
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod())[0]
				ExpectNotScheduled(ctx, env.Client, pod)
			})
		})
		Context("Daemonsets and Node Overhead", func() {
			It("should account for overhead", func() {
				ExpectCreated(ctx, env.Client, test.DaemonSet(
					test.DaemonSetOptions{PodOptions: test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					}},
				))
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod(
					test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					},
				))[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("4")))
				Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
			})
			It("should not schedule if overhead is too large", func() {
				ExpectCreated(ctx, env.Client, test.DaemonSet(
					test.DaemonSetOptions{PodOptions: test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10000"), v1.ResourceMemory: resource.MustParse("10000Gi")}},
					}},
				))
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod(test.PodOptions{}))[0]
				ExpectNotScheduled(ctx, env.Client, pod)
			})
			It("should ignore daemonsets without matching tolerations", func() {
				provisioner.Spec.Taints = v1alpha5.Taints{{Key: "foo", Value: "bar", Effect: v1.TaintEffectNoSchedule}}
				ExpectCreated(ctx, env.Client, test.DaemonSet(
					test.DaemonSetOptions{PodOptions: test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					}},
				))
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod(
					test.PodOptions{
						Tolerations:          []v1.Toleration{{Operator: v1.TolerationOperator(v1.NodeSelectorOpExists)}},
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					},
				))[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("2")))
				Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("2Gi")))
			})
			It("should ignore daemonsets with an invalid selector", func() {
				ExpectCreated(ctx, env.Client, test.DaemonSet(
					test.DaemonSetOptions{PodOptions: test.PodOptions{
						NodeSelector:         map[string]string{"node": "invalid"},
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					}},
				))
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod(
					test.PodOptions{
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					},
				))[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("2")))
				Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("2Gi")))
			})
			It("should ignore daemonsets that don't match pod constraints", func() {
				ExpectCreated(ctx, env.Client, test.DaemonSet(
					test.DaemonSetOptions{PodOptions: test.PodOptions{
						NodeRequirements:     []v1.NodeSelectorRequirement{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}}},
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					}},
				))
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod(
					test.PodOptions{
						NodeRequirements:     []v1.NodeSelectorRequirement{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2"}}},
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					},
				))[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("2")))
				Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("2Gi")))
			})
			It("should account daemonsets with NotIn operator and unspecified key", func() {
				ExpectCreated(ctx, env.Client, test.DaemonSet(
					test.DaemonSetOptions{PodOptions: test.PodOptions{
						NodeRequirements:     []v1.NodeSelectorRequirement{{Key: "foo", Operator: v1.NodeSelectorOpNotIn, Values: []string{"bar"}}},
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					}},
				))
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod(
					test.PodOptions{
						NodeRequirements:     []v1.NodeSelectorRequirement{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2"}}},
						ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
					},
				))[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(*node.Status.Allocatable.Cpu()).To(Equal(resource.MustParse("4")))
				Expect(*node.Status.Allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
			})
		})
		Context("Labels", func() {
			It("should label nodes", func() {
				provisioner.Spec.Labels = map[string]string{"test-key": "test-value", "test-key-2": "test-value-2"}
				for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod()) {
					node := ExpectScheduled(ctx, env.Client, pod)
					Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.ProvisionerNameLabelKey, provisioner.Name))
					Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
					Expect(node.Labels).To(HaveKeyWithValue("test-key-2", "test-value-2"))
					Expect(node.Labels).To(HaveKey(v1.LabelTopologyZone))
					Expect(node.Labels).To(HaveKey(v1.LabelInstanceTypeStable))
				}
			})
		})
		Context("Taints", func() {
			It("should apply unready taints", func() {
				ExpectCreated(ctx, env.Client, provisioner)
				for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioningController, provisioner, test.UnschedulablePod()) {
					node := ExpectScheduled(ctx, env.Client, pod)
					Expect(node.Spec.Taints).To(ContainElement(v1.Taint{Key: v1alpha5.NotReadyTaintKey, Effect: v1.TaintEffectNoSchedule}))
				}
			})
		})
	})
})

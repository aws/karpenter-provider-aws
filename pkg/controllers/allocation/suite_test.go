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

package allocation

import (
	"context"
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha2"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/test"
	"github.com/awslabs/karpenter/pkg/utils/resources"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provisioner/Allocator")
}

var controller *Controller
var env = test.NewEnvironment(func(e *test.Environment) {
	cloudProvider := &fake.CloudProvider{}
	registry.RegisterOrDie(cloudProvider)
	controller = NewController(
		e.Client,
		corev1.NewForConfigOrDie(e.Config),
		cloudProvider,
	)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Allocation", func() {
	var provisioner *v1alpha2.Provisioner
	var ctx context.Context
	BeforeEach(func() {
		provisioner = &v1alpha2.Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(randomdata.SillyName()),
				Namespace: "default",
			},
			Spec: v1alpha2.ProvisionerSpec{
				Cluster: &v1alpha2.Cluster{Name: "test-cluster", Endpoint: "http://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
			},
		}
		ctx = context.Background()
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	Context("Reconcilation", func() {
		Context("Zones", func() {
			It("should default to a cluster zone", func() {
				// Setup
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Spec.ProviderID).To(ContainSubstring("test-zone-1"))
			})
			It("should default to a provisioner's zone", func() {
				// Setup
				provisioner.Spec.Zones = []string{"test-zone-2"}
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner, test.PendingPod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Spec.ProviderID).To(ContainSubstring("test-zone-2"))
			})
			It("should allow a pod to override the zone", func() {
				// Setup
				provisioner.Spec.Zones = []string{"test-zone-1"}
				pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
					test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.ZoneLabelKey: "test-zone-2"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Spec.ProviderID).To(ContainSubstring("test-zone-2"))
			})
		})
		It("should provision nodes for unconstrained pods", func() {
			pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
				test.PendingPod(), test.PendingPod(),
			)
			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(1))
			for _, pod := range pods {
				Expect(pod.Spec.NodeName).To(Equal(nodes.Items[0].Name))
			}
		})
		It("should provision nodes for pods with supported node selectors", func() {
			coschedulable := []client.Object{
				// Unconstrained
				test.PendingPod(),
				// Constrained by provisioner
				test.PendingPod(test.PodOptions{
					NodeSelector: map[string]string{v1alpha2.ProvisionerNameLabelKey: provisioner.Name, v1alpha2.ProvisionerNamespaceLabelKey: provisioner.Namespace},
				}),
			}
			schedulable := []client.Object{
				// Constrained by zone
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.ZoneLabelKey: "test-zone-1"}}),
				// Constrained by instanceType
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.InstanceTypeLabelKey: "default-instance-type"}}),
				// Constrained by architecture
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.ArchitectureLabelKey: "arm64"}}),
				// Constrained by operating system
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.OperatingSystemLabelKey: "windows"}}),
				// Constrained by arbitrary label
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{"foo": "bar"}}),
			}
			unschedulable := []client.Object{
				// Ignored, matches another provisioner
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.ProvisionerNameLabelKey: "unknown", v1alpha2.ProvisionerNamespaceLabelKey: "unknown"}}),
				// Ignored, invalid zone
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.ZoneLabelKey: "unknown"}}),
				// Ignored, invalid instance type
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.InstanceTypeLabelKey: "unknown"}}),
				// Ignored, invalid architecture
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.ArchitectureLabelKey: "unknown"}}),
				// Ignored, invalid operating system
				test.PendingPod(test.PodOptions{NodeSelector: map[string]string{v1alpha2.OperatingSystemLabelKey: "unknown"}}),
			}
			ExpectCreatedWithStatus(env.Client, schedulable...)
			ExpectCreatedWithStatus(env.Client, coschedulable...)
			ExpectCreatedWithStatus(env.Client, unschedulable...)
			ExpectReconcileSucceeded(controller, provisioner)

			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(6)) // 5 schedulable -> 5 node, 2 coschedulable -> 1 node
			for _, pod := range append(schedulable, coschedulable...) {
				scheduled := ExpectPodExists(env.Client, pod.GetName(), pod.GetNamespace())
				node := ExpectNodeExists(env.Client, scheduled.Spec.NodeName)
				for key, value := range scheduled.Spec.NodeSelector {
					Expect(node.Labels[key]).To(Equal(value))
				}
			}
			for _, pod := range unschedulable {
				unscheduled := ExpectPodExists(env.Client, pod.GetName(), pod.GetNamespace())
				Expect(unscheduled.Spec.NodeName).To(Equal(""))
			}
		})
		It("should provision nodes for pods with tolerations", func() {
			provisioner.Spec.Taints = []v1.Taint{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}}
			schedulable := []client.Object{
				// Tolerates with OpExists
				test.PendingPod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists}},
				}),
				// Tolerates with OpEqual
				test.PendingPod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "test-value", Operator: v1.TolerationOpEqual}},
				}),
			}
			unschedulable := []client.Object{
				// Missing toleration
				test.PendingPod(),
				// key mismatch with OpExists
				test.PendingPod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "invalid", Operator: v1.TolerationOpExists}},
				}),
				// value mismatch with OpEqual
				test.PendingPod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "invalid", Operator: v1.TolerationOpEqual}},
				}),
				// key mismatch with OpEqual
				test.PendingPod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "invalid", Value: "test-value", Operator: v1.TolerationOpEqual}},
				}),
			}
			ExpectCreatedWithStatus(env.Client, schedulable...)
			ExpectCreatedWithStatus(env.Client, unschedulable...)
			ExpectReconcileSucceeded(controller, provisioner)

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
		It("should provision nodes for accelerators", func() {
			pods := ExpectProvisioningSucceeded(env.Client, controller, provisioner,
				test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")}},
				}),
				test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{resources.AMDGPU: resource.MustParse("1")}},
				}),
				test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{resources.AWSNeuron: resource.MustParse("1")}},
				}),
			)
			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(3))
			for _, pod := range pods {
				scheduled := ExpectPodExists(env.Client, pod.GetName(), pod.GetNamespace())
				ExpectNodeExists(env.Client, scheduled.Spec.NodeName)
			}
		})
		It("should account for daemonsets", func() {
			daemonsets := []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "daemons", Namespace: "default"},
					Spec: appsv1.DaemonSetSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
							Spec: test.PendingPod(test.PodOptions{
								ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
							}).Spec,
						}},
				},
			}
			schedulable := []client.Object{
				test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}),
				test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}),
				test.PendingPod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}),
			}
			ExpectCreatedWithStatus(env.Client, daemonsets...)
			ExpectCreatedWithStatus(env.Client, schedulable...)
			ExpectReconcileSucceeded(controller, provisioner)

			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(1))
			for _, pod := range schedulable {
				scheduled := ExpectPodExists(env.Client, pod.GetName(), pod.GetNamespace())
				ExpectNodeExists(env.Client, scheduled.Spec.NodeName)
			}
			Expect(*nodes.Items[0].Status.Allocatable.Cpu()).To(Equal(resource.MustParse("4")))
			Expect(*nodes.Items[0].Status.Allocatable.Memory()).To(Equal(resource.MustParse("4Gi")))
		})
	})
})

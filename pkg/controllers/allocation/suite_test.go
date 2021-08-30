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

package allocation_test

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
	"knative.dev/pkg/ptr"

	"github.com/awslabs/karpenter/pkg/utils/resources"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ctx context.Context
var controller *allocation.Controller
var env *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Allocation")
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

var _ = Describe("Allocation", func() {
	var provisioner *v1alpha3.Provisioner
	BeforeEach(func() {
		provisioner = &v1alpha3.Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha3.DefaultProvisioner.Name,
			},
			Spec: v1alpha3.ProvisionerSpec{
				Cluster: v1alpha3.Cluster{Name: ptr.String("test-cluster"), Endpoint: "http://test-cluster", CABundle: ptr.String("dGVzdC1jbHVzdGVyCg==")},
			},
		}
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	Context("Reconcilation", func() {
		Context("Zones", func() {
			It("should default to a cluster zone", func() {
				// Setup
				ExpectCreated(env.Client, provisioner)
				ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Spec.ProviderID).To(ContainSubstring("test-zone-1"))
			})
			It("should default to a provisioner's zone", func() {
				// Setup
				provisioner.Spec.Zones = []string{"test-zone-2"}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Spec.ProviderID).To(ContainSubstring("test-zone-2"))
			})
			It("should allow a pod to override the zone", func() {
				// Setup
				provisioner.Spec.Zones = []string{"test-zone-1"}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
					test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-2"}}),
				)
				// Assertions
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Spec.ProviderID).To(ContainSubstring("test-zone-2"))
			})
		})
		It("should provision nodes for unconstrained pods", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(), test.UnschedulablePod(),
			)
			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(1))
			for _, pod := range pods {
				Expect(pod.Spec.NodeName).To(Equal(nodes.Items[0].Name))
			}
		})
		It("should provision nodes for pods with supported node selectors", func() {
			schedulable := []client.Object{
				// Constrained by provisioner
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha3.ProvisionerNameLabelKey: provisioner.Name}}),
				// Constrained by zone
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1"}}),
				// Constrained by instanceType
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "default-instance-type"}}),
				// Constrained by architecture
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: "arm64"}}),
				// Constrained by operating system
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelOSStable: "windows"}}),
				// Constrained by arbitrary label
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{"foo": "bar"}}),
			}
			unschedulable := []client.Object{
				// Ignored, matches another provisioner
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha3.ProvisionerNameLabelKey: "unknown"}}),
				// Ignored, invalid zone
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "unknown"}}),
				// Ignored, invalid instance type
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "unknown"}}),
				// Ignored, invalid architecture
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: "unknown"}}),
				// Ignored, invalid operating system
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelOSStable: "unknown"}}),
			}
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, schedulable...)
			ExpectCreatedWithStatus(env.Client, unschedulable...)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(6)) // 5 schedulable -> 5 node, 2 coschedulable -> 1 node
			for _, pod := range schedulable {
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
				test.UnschedulablePod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists}},
				}),
				// Tolerates with OpEqual
				test.UnschedulablePod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "test-value", Operator: v1.TolerationOpEqual}},
				}),
			}
			unschedulable := []client.Object{
				// Missing toleration
				test.UnschedulablePod(),
				// key mismatch with OpExists
				test.UnschedulablePod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "invalid", Operator: v1.TolerationOpExists}},
				}),
				// value mismatch with OpEqual
				test.UnschedulablePod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "invalid", Operator: v1.TolerationOpEqual}},
				}),
				// key mismatch with OpEqual
				test.UnschedulablePod(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "invalid", Value: "test-value", Operator: v1.TolerationOpEqual}},
				}),
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
		It("should provision nodes for accelerators", func() {
			ExpectCreated(env.Client, provisioner)
			pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner,
				test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{resources.NvidiaGPU: resource.MustParse("1")}},
				}),
				test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Limits: v1.ResourceList{resources.AMDGPU: resource.MustParse("1")}},
				}),
				test.UnschedulablePod(test.PodOptions{
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
							Spec: test.UnschedulablePod(test.PodOptions{
								ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
							}).Spec,
						}},
				},
			}
			schedulable := []client.Object{
				test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}),
				test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}),
				test.UnschedulablePod(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}),
			}
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, daemonsets...)
			ExpectCreatedWithStatus(env.Client, schedulable...)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

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

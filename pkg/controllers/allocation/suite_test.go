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

	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers/allocation"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/binpacking"
	"github.com/awslabs/karpenter/pkg/controllers/allocation/scheduling"
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
		registry.RegisterOrDie(ctx, cloudProvider)
		controller = &allocation.Controller{
			CloudProvider: cloudProvider,
			Batcher:       allocation.NewBatcher(1*time.Millisecond, 1*time.Millisecond),
			Filter:        &allocation.Filter{KubeClient: e.Client},
			Scheduler:     scheduling.NewScheduler(e.Client, cloudProvider),
			Launcher: &allocation.Launcher{
				Packer:        &binpacking.Packer{},
				KubeClient:    e.Client,
				CoreV1Client:  corev1.NewForConfigOrDie(e.Config),
				CloudProvider: cloudProvider,
			},
			KubeClient: e.Client,
		}
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Allocation", func() {
	var provisioner *v1alpha5.Provisioner
	BeforeEach(func() {
		provisioner = &v1alpha5.Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha5.DefaultProvisioner.Name,
			},
			Spec: v1alpha5.ProvisionerSpec{},
		}
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	Context("Reconcilation", func() {
		It("should provision nodes for unschedulable pods", func() {
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
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name}}),
				// Constrained by zone
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1"}}),
				// Constrained by instanceType
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "default-instance-type"}}),
				// Constrained by architecture
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: "arm64"}}),
			}
			unschedulable := []client.Object{
				// Ignored, matches another provisioner
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: "unknown"}}),
				// Ignored, invalid zone
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "unknown"}}),
				// Ignored, invalid instance type
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "unknown"}}),
				// Ignored, invalid architecture
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: "unknown"}}),
				// Ignored, invalid capacity type
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.LabelCapacityType: "unknown"}}),
				// Ignored, label selector does not match
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{"foo": "bar"}}),
			}
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, schedulable...)
			ExpectCreatedWithStatus(env.Client, unschedulable...)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			for _, pod := range schedulable {
				scheduled := ExpectPodExists(env.Client, pod.GetName(), pod.GetNamespace())
				ExpectNodeExists(env.Client, scheduled.Spec.NodeName)
			}
			for _, pod := range unschedulable {
				ExpectNotScheduled(env.Client, pod.GetName(), pod.GetNamespace())
			}
			Expect(len(nodes.Items)).To(Equal(4))
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

		Context("Labels", func() {
			It("should label nodes", func() {
				provisioner.Spec.Labels = map[string]string{"test-key": "test-value", "test-key-2": "test-value-2"}
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Labels).To(HaveKeyWithValue(v1alpha5.ProvisionerNameLabelKey, provisioner.Name))
				Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
				Expect(node.Labels).To(HaveKeyWithValue("test-key-2", "test-value-2"))
				Expect(node.Labels).To(HaveKey(v1.LabelTopologyZone))
				Expect(node.Labels).To(HaveKey(v1.LabelInstanceTypeStable))
			})
		})
		Context("Taints", func() {
			It("should apply unready taints", func() {
				ExpectCreated(env.Client, provisioner)
				pods := ExpectProvisioningSucceeded(ctx, env.Client, controller, provisioner, test.UnschedulablePod())
				node := ExpectNodeExists(env.Client, pods[0].Spec.NodeName)
				Expect(node.Spec.Taints).To(ContainElement(v1.Taint{Key: v1alpha5.NotReadyTaintKey, Effect: v1.TaintEffectNoSchedule}))
			})
		})
	})
})

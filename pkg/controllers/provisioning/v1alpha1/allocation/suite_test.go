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
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/test"
	webhooksprovisioning "github.com/awslabs/karpenter/pkg/webhooks/provisioning/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Provisioner/Allocator",
		[]Reporter{printer.NewlineReporter{}})
}

var controller *Controller
var env = test.NewEnvironment(func(e *test.Environment) {
	cloudProvider := fake.NewFactory(cloudprovider.Options{})
	controller = NewController(
		e.Manager.GetClient(),
		corev1.NewForConfigOrDie(e.Manager.GetConfig()),
		cloudProvider,
	)
	e.Manager.RegisterWebhooks(
		&webhooksprovisioning.Validator{CloudProvider: cloudProvider},
		&webhooksprovisioning.Defaulter{},
	).RegisterControllers(controller)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Allocation", func() {
	var provisioner *v1alpha1.Provisioner
	var ctx context.Context
	BeforeEach(func() {
		provisioner = &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.ToLower(randomdata.SillyName()),
				Namespace: "default",
			},
			Spec: v1alpha1.ProvisionerSpec{
				Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster", Endpoint: "http://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
			},
		}
		ctx = context.Background()
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	Context("Reconcilation", func() {
		It("should provision nodes for unconstrained pods", func() {
			pods := []*v1.Pod{test.PendingPod(), test.PendingPod()}
			for _, pod := range pods {
				ExpectCreatedWithStatus(env.Client, pod)
			}
			ExpectCreated(env.Client, provisioner)
			ExpectEventuallyReconciled(env.Client, provisioner)

			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(1))
			for _, object := range pods {
				pod := ExpectPodExists(env.Client, object.GetName(), object.GetNamespace())
				Expect(pod.Spec.NodeName).To(Equal(nodes.Items[0].Name))
			}
		})
		It("should provision nodes for pods with supported node selectors", func() {
			coschedulable := []client.Object{
				// Unconstrained
				test.PendingPod(),
				// Constrained by provisioner
				test.PendingPodWith(test.PodOptions{
					NodeSelector: map[string]string{v1alpha1.ProvisionerNameLabelKey: provisioner.Name, v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace},
				}),
			}
			schedulable := []client.Object{
				// Constrained by zone
				test.PendingPodWith(test.PodOptions{
					NodeSelector: map[string]string{v1alpha1.ZoneLabelKey: "test-zone-1"},
				}),
				// Constrained by instanceType
				test.PendingPodWith(test.PodOptions{
					NodeSelector: map[string]string{v1alpha1.InstanceTypeLabelKey: "default-instance-type"},
				}),
				// Constrained by architecture
				test.PendingPodWith(test.PodOptions{
					NodeSelector: map[string]string{v1alpha1.ArchitectureLabelKey: "arm64"},
				}),
				// Constrained by operating system
				test.PendingPodWith(test.PodOptions{
					NodeSelector: map[string]string{v1alpha1.OperatingSystemLabelKey: "windows"},
				}),
				// Constrained by arbitrary label
				test.PendingPodWith(test.PodOptions{
					NodeSelector: map[string]string{"foo": "bar"},
				}),
			}
			unschedulable := []client.Object{
				// Ignored, matches another provisioner
				test.PendingPodWith(test.PodOptions{
					NodeSelector: map[string]string{v1alpha1.ProvisionerNameLabelKey: "test", v1alpha1.ProvisionerNamespaceLabelKey: "test"},
				}),
			}
			ExpectCreatedWithStatus(env.Client, schedulable...)
			ExpectCreatedWithStatus(env.Client, coschedulable...)
			ExpectCreatedWithStatus(env.Client, unschedulable...)
			ExpectCreated(env.Client, provisioner)
			ExpectEventuallyReconciled(env.Client, provisioner)

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
				test.PendingPodWith(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists}},
				}),
				// Tolerates with OpEqual
				test.PendingPodWith(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "test-value", Operator: v1.TolerationOpEqual}},
				}),
			}
			unschedulable := []client.Object{
				// Missing toleration
				test.PendingPod(),
				// key mismatch with OpExists
				test.PendingPodWith(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "invalid", Operator: v1.TolerationOpExists}},
				}),
				// value mismatch with OpEqual
				test.PendingPodWith(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "invalid", Operator: v1.TolerationOpEqual}},
				}),
				// key mismatch with OpEqual
				test.PendingPodWith(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "invalid", Value: "test-value", Operator: v1.TolerationOpEqual}},
				}),
			}
			ExpectCreatedWithStatus(env.Client, schedulable...)
			ExpectCreatedWithStatus(env.Client, unschedulable...)
			ExpectCreated(env.Client, provisioner)
			ExpectEventuallyReconciled(env.Client, provisioner)

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
		It("should account for daemonsets", func() {
			daemonsets := []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "daemons", Namespace: "default"},
					Spec: appsv1.DaemonSetSpec{
						Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
							Spec: test.PendingPodWith(test.PodOptions{
								ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
							}).Spec,
						}},
				},
			}
			schedulable := []client.Object{
				test.PendingPodWith(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}),
				test.PendingPodWith(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}),
				test.PendingPodWith(test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"), v1.ResourceMemory: resource.MustParse("1Gi")}},
				}),
			}
			ExpectCreatedWithStatus(env.Client, daemonsets...)
			ExpectCreatedWithStatus(env.Client, schedulable...)
			ExpectCreated(env.Client, provisioner)
			ExpectEventuallyReconciled(env.Client, provisioner)

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

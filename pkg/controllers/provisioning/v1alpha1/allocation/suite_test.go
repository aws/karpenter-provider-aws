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
	"github.com/awslabs/karpenter/pkg/test/pods"
	webhooksprovisioning "github.com/awslabs/karpenter/pkg/webhooks/provisioning/v1alpha1"
	v1 "k8s.io/api/core/v1"
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
			ps := []*v1.Pod{pods.Pending(), pods.Pending()}
			for _, pod := range ps {
				ExpectCreatedWithStatus(env.Client, pod)
			}
			ExpectCreated(env.Client, provisioner)
			ExpectEventuallyReconciled(env.Client, provisioner)

			nodes := &v1.NodeList{}
			Expect(env.Client.List(ctx, nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(1))
			for _, object := range ps {
				pod := v1.Pod{}
				Expect(env.Client.Get(ctx, client.ObjectKey{Name: object.GetName(), Namespace: object.GetNamespace()}, &pod)).To(Succeed())
				Expect(pod.Spec.NodeName).To(Equal(nodes.Items[0].Name))
			}
		})
		It("should provision nodes for pods with supported node selectors", func() {
			coschedulable := []client.Object{
				// Unconstrained
				pods.Pending(),
				// Constrained by provisioner
				pods.PendingWith(pods.Options{
					NodeSelector: map[string]string{v1alpha1.ProvisionerNameLabelKey: provisioner.Name, v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace},
				}),
			}
			schedulable := []client.Object{
				// Constrained by zone
				pods.PendingWith(pods.Options{
					NodeSelector: map[string]string{v1alpha1.ZoneLabelKey: "test-zone-1"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Constrained by instanceType
				pods.PendingWith(pods.Options{
					NodeSelector: map[string]string{v1alpha1.InstanceTypeLabelKey: "test-instance-type-1"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Constrained by architecture
				pods.PendingWith(pods.Options{
					NodeSelector: map[string]string{v1alpha1.ArchitectureLabelKey: "test-architecture-1"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Constrained by operating system
				pods.PendingWith(pods.Options{
					NodeSelector: map[string]string{v1alpha1.OperatingSystemLabelKey: "test-operating-system-1"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Constrained by arbitrary label
				pods.PendingWith(pods.Options{
					NodeSelector: map[string]string{"foo": "bar"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
			}
			unschedulable := []client.Object{
				// Ignored, matches another provisioner
				pods.PendingWith(pods.Options{
					NodeSelector: map[string]string{v1alpha1.ProvisionerNameLabelKey: "test", v1alpha1.ProvisionerNamespaceLabelKey: "test"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
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
				scheduled := &v1.Pod{}
				Expect(env.Client.Get(ctx, client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
				node := &v1.Node{}
				Expect(env.Client.Get(ctx, client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
				for key, value := range scheduled.Spec.NodeSelector {
					Expect(node.Labels[key]).To(Equal(value))
				}
			}
			for _, pod := range unschedulable {
				unscheduled := &v1.Pod{}
				Expect(env.Client.Get(ctx, client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, unscheduled)).To(Succeed())
				Expect(unscheduled.Spec.NodeName).To(Equal(""))
			}
		})
		It("should provision nodes for pods with tolerations", func() {
			provisioner.Spec.Taints = []v1.Taint{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}}
			schedulable := []client.Object{
				// Tolerates with OpExists
				pods.PendingWith(pods.Options{
					Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists}},
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Tolerates with OpEqual
				pods.PendingWith(pods.Options{
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "test-value", Operator: v1.TolerationOpEqual}},
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
			}
			unschedulable := []client.Object{
				// Missing toleration
				pods.PendingWith(pods.Options{
					Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// key mismatch with OpExists
				pods.PendingWith(pods.Options{
					Tolerations: []v1.Toleration{{Key: "invalid", Operator: v1.TolerationOpExists}},
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// value mismatch with OpEqual
				pods.PendingWith(pods.Options{
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "invalid", Operator: v1.TolerationOpEqual}},
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// key mismatch with OpEqual
				pods.PendingWith(pods.Options{
					Tolerations: []v1.Toleration{{Key: "invalid", Value: "test-value", Operator: v1.TolerationOpEqual}},
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
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
				scheduled := &v1.Pod{}
				Expect(env.Client.Get(ctx, client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
				node := &v1.Node{}
				Expect(env.Client.Get(ctx, client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
			}
			for _, pod := range unschedulable {
				unscheduled := &v1.Pod{}
				Expect(env.Client.Get(ctx, client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, unscheduled)).To(Succeed())
				Expect(unscheduled.Spec.NodeName).To(Equal(""))
			}
		})
	})
})

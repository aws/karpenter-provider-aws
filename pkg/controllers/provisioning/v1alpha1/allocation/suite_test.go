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
	"github.com/awslabs/karpenter/pkg/test/environment"
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
var env environment.Environment = environment.NewLocal(func(e *environment.Local) {
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
	var ns *environment.Namespace
	var provisioner *v1alpha1.Provisioner

	BeforeEach(func() {
		var err error
		ns, err = env.NewNamespace()
		Expect(err).NotTo(HaveOccurred())
		provisioner = &v1alpha1.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()), Namespace: ns.Name},
			Spec: v1alpha1.ProvisionerSpec{
				Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster", Endpoint: "http://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
			},
		}
	})

	AfterEach(func() {
		ExpectCleanedUp(ns.Client)
	})

	Context("Reconcilation", func() {
		It("should provision nodes for unconstrained pods", func() {
			pods := []*v1.Pod{
				test.PodWith(test.PodOptions{
					Namespace:  ns.Name,
					Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				test.PodWith(test.PodOptions{
					Namespace:  ns.Name,
					Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
			}
			for _, pod := range pods {
				ExpectCreatedWithStatus(ns.Client, pod)
			}
			ExpectCreated(ns.Client, provisioner)
			ExpectEventuallyReconciled(ns.Client, provisioner)

			nodes := &v1.NodeList{}
			Expect(ns.Client.List(context.Background(), nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(1))
			for _, object := range pods {
				pod := v1.Pod{}
				Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: object.GetName(), Namespace: object.GetNamespace()}, &pod)).To(Succeed())
				Expect(pod.Spec.NodeName).To(Equal(nodes.Items[0].Name))
			}
		})
		It("should provision nodes for pods with supported node selectors", func() {
			coschedulable := []client.Object{
				// Unconstrained
				test.PodWith(test.PodOptions{
					Namespace:  ns.Name,
					Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Constrained by provisioner
				test.PodWith(test.PodOptions{
					Namespace:    ns.Name,
					NodeSelector: map[string]string{v1alpha1.ProvisionerNameLabelKey: provisioner.Name, v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
			}
			schedulable := []client.Object{
				// Constrained by zone
				test.PodWith(test.PodOptions{
					Namespace:    ns.Name,
					NodeSelector: map[string]string{v1alpha1.ZoneLabelKey: "test-zone-1"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Constrained by instanceType
				test.PodWith(test.PodOptions{
					Namespace:    ns.Name,
					NodeSelector: map[string]string{v1alpha1.InstanceTypeLabelKey: "test-instance-type-1"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Constrained by architecture
				test.PodWith(test.PodOptions{
					Namespace:    ns.Name,
					NodeSelector: map[string]string{v1alpha1.ArchitectureLabelKey: "test-architecture-1"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Constrained by operating system
				test.PodWith(test.PodOptions{
					Namespace:    ns.Name,
					NodeSelector: map[string]string{v1alpha1.OperatingSystemLabelKey: "test-operating-system-1"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Constrained by arbitrary label
				test.PodWith(test.PodOptions{
					Namespace:    ns.Name,
					NodeSelector: map[string]string{"foo": "bar"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
			}
			unschedulable := []client.Object{
				// Ignored, matches another provisioner
				test.PodWith(test.PodOptions{
					Namespace:    ns.Name,
					NodeSelector: map[string]string{v1alpha1.ProvisionerNameLabelKey: "test", v1alpha1.ProvisionerNamespaceLabelKey: "test"},
					Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
			}
			ExpectCreatedWithStatus(ns.Client, schedulable...)
			ExpectCreatedWithStatus(ns.Client, coschedulable...)
			ExpectCreatedWithStatus(ns.Client, unschedulable...)
			ExpectCreated(ns.Client, provisioner)
			ExpectEventuallyReconciled(ns.Client, provisioner)

			nodes := &v1.NodeList{}
			Expect(ns.Client.List(context.Background(), nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(6)) // 5 schedulable -> 5 node, 2 coschedulable -> 1 node
			for _, pod := range append(schedulable, coschedulable...) {
				scheduled := &v1.Pod{}
				Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
				node := &v1.Node{}
				Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
				for key, value := range scheduled.Spec.NodeSelector {
					Expect(node.Labels[key]).To(Equal(value))
				}
			}
			for _, pod := range unschedulable {
				unscheduled := &v1.Pod{}
				Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, unscheduled)).To(Succeed())
				Expect(unscheduled.Spec.NodeName).To(Equal(""))
			}
		})
		It("should provision nodes for pods with tolerations", func() {
			provisioner.Spec.Taints = []v1.Taint{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}}
			schedulable := []client.Object{
				// Tolerates with OpExists
				test.PodWith(test.PodOptions{
					Namespace:   ns.Name,
					Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists}},
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// Tolerates with OpEqual
				test.PodWith(test.PodOptions{
					Namespace:   ns.Name,
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "test-value", Operator: v1.TolerationOpEqual}},
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
			}
			unschedulable := []client.Object{
				// Missing toleration
				test.PodWith(test.PodOptions{
					Namespace:  ns.Name,
					Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// key mismatch with OpExists
				test.PodWith(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "invalid", Operator: v1.TolerationOpExists}},
					Namespace:   ns.Name,
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// value mismatch with OpEqual
				test.PodWith(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "test-key", Value: "invalid", Operator: v1.TolerationOpEqual}},
					Namespace:   ns.Name,
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
				// key mismatch with OpEqual
				test.PodWith(test.PodOptions{
					Tolerations: []v1.Toleration{{Key: "invalid", Value: "test-value", Operator: v1.TolerationOpEqual}},
					Namespace:   ns.Name,
					Conditions:  []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
				}),
			}
			ExpectCreatedWithStatus(ns.Client, schedulable...)
			ExpectCreatedWithStatus(ns.Client, unschedulable...)
			ExpectCreated(ns.Client, provisioner)
			ExpectEventuallyReconciled(ns.Client, provisioner)

			nodes := &v1.NodeList{}
			Expect(ns.Client.List(context.Background(), nodes)).To(Succeed())
			Expect(len(nodes.Items)).To(Equal(1))
			for _, pod := range schedulable {
				scheduled := &v1.Pod{}
				Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, scheduled)).To(Succeed())
				node := &v1.Node{}
				Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: scheduled.Spec.NodeName}, node)).To(Succeed())
			}
			for _, pod := range unschedulable {
				unscheduled := &v1.Pod{}
				Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: pod.GetName(), Namespace: pod.GetNamespace()}, unscheduled)).To(Succeed())
				Expect(unscheduled.Spec.NodeName).To(Equal(""))
			}
		})
	})
})

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

package reallocation

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	"strings"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha1"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/test"
	"github.com/awslabs/karpenter/pkg/test/environment"
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
		"Provisioner/Reallocator",
		[]Reporter{printer.NewlineReporter{}})
}

var controller *Controller
var env environment.Environment = environment.NewLocal(func(e *environment.Local) {
	controller = NewController(
		e.Manager.GetClient(),
		corev1.NewForConfigOrDie(e.Manager.GetConfig()),
		fake.NewFactory(cloudprovider.Options{}),
	)
	e.Manager.Register(controller)
})

var _ = BeforeSuite(func() {
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Reallocation", func() {
	var ns *environment.Namespace

	BeforeEach(func() {
		var err error
		ns, err = env.NewNamespace()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ExpectCleanedUp(ns.Client)
	})

	Context("Reconciliation", func() {
		It("should label nodes as underutilized", func() {
			provisioner := &v1alpha1.Provisioner{
				ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()), Namespace: ns.Name},
				Spec: v1alpha1.ProvisionerSpec{
					Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster", Endpoint: "http://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
				},
			}
			node := test.NodeWith(test.NodeOptions{
				Labels: map[string]string{
					v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
					v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
				},
			})
			ExpectCreatedWithStatus(ns.Client, node)

			ExpectCreated(ns.Client, provisioner)
			ExpectEventuallyReconciled(ns.Client, provisioner)

			Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: node.Name}, node)).To(Succeed())
			Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.ProvisionerPhaseLabel, v1alpha1.ProvisionerUnderutilizedPhase))
			Expect(node.Annotations).To(HaveKey(v1alpha1.ProvisionerTTLKey))
		})

		It("should remove labels from utilized nodes", func() {
			provisioner := &v1alpha1.Provisioner{
				ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()), Namespace: ns.Name},
				Spec: v1alpha1.ProvisionerSpec{
					Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster", Endpoint: "http://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
				},
			}
			node := test.NodeWith(test.NodeOptions{
				Labels: map[string]string{
					v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
					v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
					v1alpha1.ProvisionerPhaseLabel:        v1alpha1.ProvisionerUnderutilizedPhase,
				},
				Annotations: map[string]string{
					v1alpha1.ProvisionerTTLKey: time.Now().Add(time.Duration(100) * time.Second).Format(time.RFC3339),
				},
			})
			pod := test.PodWith(test.PodOptions{
				Name:       strings.ToLower(randomdata.SillyName()),
				Namespace:  provisioner.Namespace,
				NodeName:   node.Name,
				Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}},
			})

			ExpectCreatedWithStatus(ns.Client, node)
			ExpectCreatedWithStatus(ns.Client, pod)

			ExpectCreated(ns.Client, provisioner)
			ExpectEventuallyReconciled(ns.Client, provisioner)

			updatedNode := &v1.Node{}
			Expect(ns.Client.Get(context.Background(), client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Labels).ToNot(HaveKey(v1alpha1.ProvisionerPhaseLabel))
			Expect(updatedNode.Annotations).ToNot(HaveKey(v1alpha1.ProvisionerTTLKey))
		})

		It("should terminate nodes with expired TTL", func() {
			provisioner := &v1alpha1.Provisioner{
				ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()), Namespace: ns.Name},
				Spec: v1alpha1.ProvisionerSpec{
					Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster", Endpoint: "http://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
				},
			}
			node := test.NodeWith(test.NodeOptions{
				Labels: map[string]string{
					v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
					v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
					v1alpha1.ProvisionerPhaseLabel:        v1alpha1.ProvisionerUnderutilizedPhase,
				},
				Annotations: map[string]string{
					v1alpha1.ProvisionerTTLKey: time.Now().Add(time.Duration(-100) * time.Second).Format(time.RFC3339),
				},
			})
			ExpectCreatedWithStatus(ns.Client, node)

			ExpectCreated(ns.Client, provisioner)
			ExpectEventuallyReconciled(ns.Client, provisioner)

			err := ns.Client.Get(context.Background(), client.ObjectKey{Name: node.Name}, node)
			Expect(err).ToNot(Succeed())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should terminate nodes marked terminable", func() {
			provisioner := &v1alpha1.Provisioner{
				ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName()), Namespace: ns.Name},
				Spec: v1alpha1.ProvisionerSpec{
					Cluster: &v1alpha1.ClusterSpec{Name: "test-cluster", Endpoint: "http://test-cluster", CABundle: "dGVzdC1jbHVzdGVyCg=="},
				},
			}
			node := test.NodeWith(test.NodeOptions{
				Labels: map[string]string{
					v1alpha1.ProvisionerNameLabelKey:      provisioner.Name,
					v1alpha1.ProvisionerNamespaceLabelKey: provisioner.Namespace,
					v1alpha1.ProvisionerPhaseLabel:        v1alpha1.ProvisionerTerminablePhase,
				},
				Annotations: map[string]string{
					v1alpha1.ProvisionerTTLKey: time.Now().Add(time.Duration(-100) * time.Second).Format(time.RFC3339),
				},
			})
			ExpectCreatedWithStatus(ns.Client, node)

			ExpectCreated(ns.Client, provisioner)
			ExpectEventuallyReconciled(ns.Client, provisioner)

			err := ns.Client.Get(context.Background(), client.ObjectKey{Name: node.Name}, node)
			Expect(err).ToNot(Succeed())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})

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

package reallocation_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/awslabs/karpenter/pkg/apis/provisioning/v1alpha3"
	"github.com/awslabs/karpenter/pkg/cloudprovider/fake"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers/reallocation"
	"github.com/awslabs/karpenter/pkg/test"

	"bou.ke/monkey"
	. "github.com/awslabs/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	. "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ctx context.Context
var controller *reallocation.Controller
var env *test.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provisioner/Reallocator")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider := &fake.CloudProvider{}
		registry.RegisterOrDie(cloudProvider)
		controller = &reallocation.Controller{
			Utilization:   &reallocation.Utilization{KubeClient: e.Client},
			CloudProvider: cloudProvider,
			KubeClient:    e.Client,
		}
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Reallocation", func() {
	var provisioner *v1alpha3.Provisioner

	BeforeEach(func() {
		provisioner = &v1alpha3.Provisioner{
			ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.DefaultProvisioner.Name},
			Spec: v1alpha3.ProvisionerSpec{
				Cluster:              v1alpha3.Cluster{Name: ptr.String("test-cluster"), Endpoint: "http://test-cluster", CABundle: ptr.String("dGVzdC1jbHVzdGVyCg==")},
				TTLSecondsAfterEmpty: ptr.Int64(300),
			},
		}
	})

	AfterEach(func() {
		ExpectCleanedUp(env.Client)
	})

	Context("Reconciliation", func() {
		It("should not TTL nodes that have ready status unknown", func() {
			node := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionUnknown,
			})

			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Labels).ToNot(HaveKey(v1alpha3.ProvisionerUnderutilizedLabelKey))
			Expect(updatedNode.Annotations).ToNot(HaveKey(v1alpha3.ProvisionerTTLAfterEmptyKey))
		})
		It("should not TTL nodes that have ready status false", func() {
			node := test.Node(test.NodeOptions{
				ReadyStatus: v1.ConditionFalse,
			})

			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Labels).ToNot(HaveKey(v1alpha3.ProvisionerUnderutilizedLabelKey))
			Expect(updatedNode.Annotations).ToNot(HaveKey(v1alpha3.ProvisionerTTLAfterEmptyKey))
		})
		It("should label nodes as underutilized and add TTL", func() {
			node := test.Node(test.NodeOptions{
				Labels: map[string]string{
					v1alpha3.ProvisionerNameLabelKey: provisioner.Name,
				},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Labels).To(HaveKey(v1alpha3.ProvisionerUnderutilizedLabelKey))
			Expect(updatedNode.Annotations).To(HaveKey(v1alpha3.ProvisionerTTLAfterEmptyKey))
		})
		It("should remove labels from utilized nodes", func() {
			node := test.Node(test.NodeOptions{
				Labels: map[string]string{
					v1alpha3.ProvisionerNameLabelKey:          provisioner.Name,
					v1alpha3.ProvisionerUnderutilizedLabelKey: "true",
				},
				Annotations: map[string]string{
					v1alpha3.ProvisionerTTLAfterEmptyKey: time.Now().Add(100 * time.Second).Format(time.RFC3339),
				},
			})
			ExpectCreated(env.Client, provisioner)
			ExpectCreatedWithStatus(env.Client, node)
			ExpectCreatedWithStatus(env.Client, test.Pod(test.PodOptions{
				Name:       strings.ToLower(randomdata.SillyName()),
				Namespace:  provisioner.Namespace,
				NodeName:   node.Name,
				Conditions: []v1.PodCondition{{Type: v1.PodReady, Status: v1.ConditionTrue}},
			}))
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.Labels).ToNot(HaveKey(v1alpha3.ProvisionerUnderutilizedLabelKey))
			Expect(updatedNode.Annotations).ToNot(HaveKey(v1alpha3.ProvisionerTTLAfterEmptyKey))
		})
		It("should terminate underutilized nodes past their TTL", func() {
			node := test.Node(test.NodeOptions{
				Finalizers: []string{v1alpha3.TerminationFinalizer},
				Labels: map[string]string{
					v1alpha3.ProvisionerNameLabelKey:          provisioner.Name,
					v1alpha3.ProvisionerUnderutilizedLabelKey: "true",
				},
				Annotations: map[string]string{
					v1alpha3.ProvisionerTTLAfterEmptyKey: time.Now().Add(-100 * time.Second).Format(time.RFC3339),
				},
			})
			ExpectCreated(env.Client, provisioner, node)
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			updatedNode := &v1.Node{}
			Expect(env.Client.Get(ctx, client.ObjectKey{Name: node.Name}, updatedNode)).To(Succeed())
			Expect(updatedNode.DeletionTimestamp.IsZero()).To(BeFalse())
		})
		It("should only terminate nodes that failed to join with all pods terminating after 5 minutes", func() {
			node := test.Node(test.NodeOptions{
				Finalizers: []string{v1alpha3.TerminationFinalizer},
				Labels: map[string]string{
					v1alpha3.ProvisionerNameLabelKey:          provisioner.Name,
					v1alpha3.ProvisionerUnderutilizedLabelKey: "true",
				},
				ReadyStatus: v1.ConditionUnknown,
			})
			pod := test.Pod(test.PodOptions{
				Finalizers: []string{"fake.sh/finalizer"},
				NodeName:   node.Name,
			})
			ExpectCreated(env.Client, provisioner, pod)
			ExpectCreatedWithStatus(env.Client, node)

			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			// Expect node not deleted
			updatedNode := ExpectNodeExists(env.Client, node.Name)
			Expect(updatedNode.DeletionTimestamp.IsZero()).To(BeTrue())

			// Set pod DeletionTimestamp and do another reconcile
			Expect(env.Client.Delete(ctx, pod)).To(Succeed())
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			// Expect node not deleted
			updatedNode = ExpectNodeExists(env.Client, node.Name)
			Expect(updatedNode.DeletionTimestamp.IsZero()).To(BeTrue())

			// Simulate time passing and a node failing to join
			future := time.Now().Add(reallocation.FailedToJoinTimeout)
			monkey.Patch(time.Now, func() time.Time {
				return future
			})
			ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(provisioner))

			updatedNode = ExpectNodeExists(env.Client, node.Name)
			Expect(updatedNode.DeletionTimestamp.IsZero()).To(BeFalse())
		})
	})
})

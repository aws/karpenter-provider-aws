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

package interruption_test

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/utils"
)

var _ = Describe("Machine/InterruptionHandling", func() {
	var node *v1.Node
	var machine *v1alpha5.Machine
	BeforeEach(func() {
		machine, node = coretest.MachineAndNode(v1alpha5.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			Status: v1alpha5.MachineStatus{
				ProviderID: fake.RandomProviderID(),
			},
		})
	})
	Context("Processing Messages", func() {
		It("should delete the machine when receiving a spot interruption warning", func() {
			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(machine.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, machine, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, machine)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete the machine when receiving a scheduled change message", func() {
			ExpectMessagesCreated(scheduledChangeMessage(lo.Must(utils.ParseInstanceID(machine.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, machine, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, machine)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete the machine when receiving a state change message", func() {
			var machines []*v1alpha5.Machine
			var messages []interface{}
			for _, state := range []string{"terminated", "stopped", "stopping", "shutting-down"} {
				instanceID := fake.InstanceID()
				m, n := coretest.MachineAndNode(v1alpha5.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1alpha5.ProvisionerNameLabelKey: "default",
						},
					},
					Status: v1alpha5.MachineStatus{
						ProviderID: fake.ProviderID(instanceID),
					},
				})
				ExpectApplied(ctx, env.Client, m, n)
				machines = append(machines, m)
				messages = append(messages, stateChangeMessage(instanceID, state))
			}
			ExpectMessagesCreated(messages...)
			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, lo.Map(machines, func(m *v1alpha5.Machine, _ int) client.Object { return m })...)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(4))
		})
		It("should handle multiple messages that cause machine deletion", func() {
			var machines []*v1alpha5.Machine
			var instanceIDs []string
			for i := 0; i < 100; i++ {
				instanceID := fake.InstanceID()
				m, n := coretest.MachineAndNode(v1alpha5.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1alpha5.ProvisionerNameLabelKey: "default",
						},
					},
					Status: v1alpha5.MachineStatus{
						ProviderID: fake.ProviderID(instanceID),
					},
				})
				ExpectApplied(ctx, env.Client, m, n)
				instanceIDs = append(instanceIDs, instanceID)
				machines = append(machines, machine)
			}

			var messages []interface{}
			for _, id := range instanceIDs {
				messages = append(messages, spotInterruptionMessage(id))
			}
			ExpectMessagesCreated(messages...)
			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, lo.Map(machines, func(m *v1alpha5.Machine, _ int) client.Object { return m })...)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(100))
		})
		It("should delete a message when the message can't be parsed", func() {
			badMessage := &sqs.Message{
				Body: aws.String(string(lo.Must(json.Marshal(map[string]string{
					"field1": "value1",
					"field2": "value2",
				})))),
				MessageId: aws.String(string(uuid.NewUUID())),
			}

			ExpectMessagesCreated(badMessage)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete a state change message when the state isn't in accepted states", func() {
			ExpectMessagesCreated(stateChangeMessage(lo.Must(utils.ParseInstanceID(machine.Status.ProviderID)), "creating"))
			ExpectApplied(ctx, env.Client, machine, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectExists(ctx, env.Client, machine)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should mark the ICE cache for the offering when getting a spot interruption warning", func() {
			machine.Labels = lo.Assign(machine.Labels, map[string]string{
				v1.LabelTopologyZone:       "coretest-zone-1a",
				v1.LabelInstanceTypeStable: "t3.large",
				v1alpha5.LabelCapacityType: v1alpha1.CapacityTypeSpot,
			})
			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(machine.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, machine, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, machine)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))

			// Expect a t3.large in coretest-zone-1a to be added to the ICE cache
			Expect(unavailableOfferingsCache.IsUnavailable("t3.large", "coretest-zone-1a", v1alpha1.CapacityTypeSpot)).To(BeTrue())
		})
	})
})

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

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/utils"
)

var _ = Describe("NodeClaim/InterruptionHandling", func() {
	var node *v1.Node
	var nodeClaim *corev1beta1.NodeClaim
	BeforeEach(func() {
		nodeClaim, node = coretest.NodeClaimAndNode(corev1beta1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					corev1beta1.NodePoolLabelKey: "default",
				},
			},
			Status: corev1beta1.NodeClaimStatus{
				ProviderID: fake.RandomProviderID(),
			},
		})
	})
	Context("Processing Messages", func() {
		It("should delete the NodeClaim when receiving a spot interruption warning", func() {
			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, nodeClaim)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete the NodeClaim when receiving a scheduled change message", func() {
			ExpectMessagesCreated(scheduledChangeMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, nodeClaim)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete the NodeClaim when receiving a state change message", func() {
			var nodeClaims []*corev1beta1.NodeClaim
			var messages []interface{}
			for _, state := range []string{"terminated", "stopped", "stopping", "shutting-down"} {
				instanceID := fake.InstanceID()
				nc, n := coretest.NodeClaimAndNode(corev1beta1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							corev1beta1.NodePoolLabelKey: "default",
						},
					},
					Status: corev1beta1.NodeClaimStatus{
						ProviderID: fake.ProviderID(instanceID),
					},
				})
				ExpectApplied(ctx, env.Client, nc, n)
				nodeClaims = append(nodeClaims, nc)
				messages = append(messages, stateChangeMessage(instanceID, state))
			}
			ExpectMessagesCreated(messages...)
			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, lo.Map(nodeClaims, func(nc *corev1beta1.NodeClaim, _ int) client.Object { return nc })...)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(4))
		})
		It("should handle multiple messages that cause nodeClaim deletion", func() {
			var nodeClaims []*corev1beta1.NodeClaim
			var instanceIDs []string
			for i := 0; i < 100; i++ {
				instanceID := fake.InstanceID()
				nc, n := coretest.NodeClaimAndNode(corev1beta1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							corev1beta1.NodePoolLabelKey: "default",
						},
					},
					Status: corev1beta1.NodeClaimStatus{
						ProviderID: fake.ProviderID(instanceID),
					},
				})
				ExpectApplied(ctx, env.Client, nc, n)
				instanceIDs = append(instanceIDs, instanceID)
				nodeClaims = append(nodeClaims, nc)
			}

			var messages []interface{}
			for _, id := range instanceIDs {
				messages = append(messages, spotInterruptionMessage(id))
			}
			ExpectMessagesCreated(messages...)
			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, lo.Map(nodeClaims, func(nc *corev1beta1.NodeClaim, _ int) client.Object { return nc })...)
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
			ExpectMessagesCreated(stateChangeMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID)), "creating"))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectExists(ctx, env.Client, nodeClaim)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should mark the ICE cache for the offering when getting a spot interruption warning", func() {
			nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{
				v1.LabelTopologyZone:             "coretest-zone-1a",
				v1.LabelInstanceTypeStable:       "t3.large",
				corev1beta1.CapacityTypeLabelKey: corev1beta1.CapacityTypeSpot,
			})
			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, nodeClaim)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))

			// Expect a t3.large in coretest-zone-1a to be added to the ICE cache
			Expect(unavailableOfferingsCache.IsUnavailable("t3.large", "coretest-zone-1a", corev1beta1.CapacityTypeSpot)).To(BeTrue())
		})
	})
})

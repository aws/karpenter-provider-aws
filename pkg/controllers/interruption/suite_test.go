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
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	servicesqs "github.com/aws/aws-sdk-go/service/sqs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/events"
	coreoptions "github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	awscache "github.com/aws/karpenter/pkg/cache"
	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/scheduledchange"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/spotinterruption"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/statechange"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/providers/sqs"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils"
)

const (
	defaultAccountID = "000000000000"
	ec2Source        = "aws.ec2"
	healthSource     = "aws.health"
)

var ctx context.Context
var env *coretest.Environment
var sqsapi *fake.SQSAPI
var sqsProvider *sqs.Provider
var unavailableOfferingsCache *awscache.UnavailableOfferings
var fakeClock *clock.FakeClock
var controller *interruption.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSInterruption")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	fakeClock = &clock.FakeClock{}
	unavailableOfferingsCache = awscache.NewUnavailableOfferings()
	sqsapi = &fake.SQSAPI{}
	sqsProvider = lo.Must(sqs.NewProvider(ctx, sqsapi, "test-cluster"))
	controller = interruption.NewController(env.Client, fakeClock, events.NewRecorder(&record.FakeRecorder{}), sqsProvider, unavailableOfferingsCache)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = settings.ToContext(ctx, test.Settings())
	unavailableOfferingsCache.Flush()
	sqsapi.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("InterruptionHandling", func() {
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
			badMessage := &servicesqs.Message{
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

var _ = Describe("Error Handling", func() {
	It("should send an error on polling when QueueNotExists", func() {
		sqsapi.ReceiveMessageBehavior.Error.Set(awsErrWithCode(servicesqs.ErrCodeQueueDoesNotExist), fake.MaxCalls(0))
		ExpectReconcileFailed(ctx, controller, types.NamespacedName{})
	})
	It("should send an error on polling when AccessDenied", func() {
		sqsapi.ReceiveMessageBehavior.Error.Set(awsErrWithCode("AccessDenied"), fake.MaxCalls(0))
		ExpectReconcileFailed(ctx, controller, types.NamespacedName{})
	})
	It("should not return an error when deleting a nodeClaim that is already deleted", func() {
		ExpectMessagesCreated(spotInterruptionMessage(fake.InstanceID()))
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
	})
})

func ExpectMessagesCreated(messages ...interface{}) {
	raw := lo.Map(messages, func(m interface{}, _ int) *servicesqs.Message {
		return &servicesqs.Message{
			Body:      aws.String(string(lo.Must(json.Marshal(m)))),
			MessageId: aws.String(string(uuid.NewUUID())),
		}
	})
	sqsapi.ReceiveMessageBehavior.Output.Set(
		&servicesqs.ReceiveMessageOutput{
			Messages: raw,
		},
	)
}

func awsErrWithCode(code string) awserr.Error {
	return awserr.New(code, "", fmt.Errorf(""))
}

func spotInterruptionMessage(involvedInstanceID string) spotinterruption.Message {
	return spotinterruption.Message{
		Metadata: messages.Metadata{
			Version:    "0",
			Account:    defaultAccountID,
			DetailType: "EC2 Spot Instance Interruption Warning",
			ID:         string(uuid.NewUUID()),
			Region:     fake.DefaultRegion,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", fake.DefaultRegion, involvedInstanceID),
			},
			Source: ec2Source,
			Time:   time.Now(),
		},
		Detail: spotinterruption.Detail{
			InstanceID:     involvedInstanceID,
			InstanceAction: "terminate",
		},
	}
}

func stateChangeMessage(involvedInstanceID, state string) statechange.Message {
	return statechange.Message{
		Metadata: messages.Metadata{
			Version:    "0",
			Account:    defaultAccountID,
			DetailType: "EC2 Instance State-change Notification",
			ID:         string(uuid.NewUUID()),
			Region:     fake.DefaultRegion,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", fake.DefaultRegion, involvedInstanceID),
			},
			Source: ec2Source,
			Time:   time.Now(),
		},
		Detail: statechange.Detail{
			InstanceID: involvedInstanceID,
			State:      state,
		},
	}
}

func scheduledChangeMessage(involvedInstanceID string) scheduledchange.Message {
	return scheduledchange.Message{
		Metadata: messages.Metadata{
			Version:    "0",
			Account:    defaultAccountID,
			DetailType: "AWS Health Event",
			ID:         string(uuid.NewUUID()),
			Region:     fake.DefaultRegion,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", fake.DefaultRegion, involvedInstanceID),
			},
			Source: healthSource,
			Time:   time.Now(),
		},
		Detail: scheduledchange.Detail{
			Service:           "EC2",
			EventTypeCategory: "scheduledChange",
			AffectedEntities: []scheduledchange.AffectedEntity{
				{
					EntityValue: involvedInstanceID,
				},
			},
		},
	}
}

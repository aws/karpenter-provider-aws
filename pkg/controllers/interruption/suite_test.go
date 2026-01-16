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

	"sigs.k8s.io/karpenter/pkg/metrics"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	servicesqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages/scheduledchange"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages/spotinterruption"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages/statechange"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/sqs"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

const (
	defaultAccountID = "000000000000"
	ec2Source        = "aws.ec2"
	healthSource     = "aws.health"
)

var ctx context.Context
var awsEnv *test.Environment
var env *coretest.Environment
var sqsapi *fake.SQSAPI
var sqsProvider *sqs.DefaultProvider
var unavailableOfferingsCache *awscache.UnavailableOfferings
var fakeClock *clock.FakeClock
var controller *interruption.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSInterruption")
}

var _ = BeforeSuite(func() {
	ctx = options.ToContext(ctx, test.Options())
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...), coretest.WithFieldIndexers(test.NodeInstanceIDFieldIndexer(ctx), test.NodeClaimInstanceIDFieldIndexer(ctx)))
	awsEnv = test.NewEnvironment(ctx, env)
	fakeClock = &clock.FakeClock{}
	unavailableOfferingsCache = awscache.NewUnavailableOfferings()
	sqsapi = &fake.SQSAPI{}
	sqsProvider = lo.Must(sqs.NewDefaultProvider(sqsapi, fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/test-cluster", fake.DefaultRegion, fake.DefaultAccount)))
	cloudProvider := cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider, awsEnv.InstanceTypeStore)
	controller = interruption.NewController(env.Client, cloudProvider, fakeClock, events.NewRecorder(&record.FakeRecorder{}), sqsProvider, sqsapi, unavailableOfferingsCache, awsEnv.InstanceStatusProvider)
	interruption.InstanceStatusInterval = 0
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options(test.OptionsFields{InterruptionQueue: lo.ToPtr("test-cluster")}))
	unavailableOfferingsCache.Flush()
	sqsapi.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("InterruptionHandling", func() {
	var node *corev1.Node
	var nodeClaim *karpv1.NodeClaim
	BeforeEach(func() {
		nodeClaim, node = coretest.NodeClaimAndNode(karpv1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					karpv1.NodePoolLabelKey: "default",
				},
			},
			Status: karpv1.NodeClaimStatus{
				ProviderID: fake.RandomProviderID(),
			},
		})
		metrics.NodeClaimsDisruptedTotal.Reset()
	})
	Context("Processing Messages", func() {
		It("should delete the NodeClaim when receiving a spot interruption warning", func() {
			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectSingletonReconciled(ctx, controller)
			ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 1, map[string]string{
				metrics.ReasonLabel: "spot_interrupted",
				"nodepool":          "default",
			})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, nodeClaim)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete the NodeClaim when receiving a scheduled change message", func() {
			ExpectMessagesCreated(scheduledChangeMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectSingletonReconciled(ctx, controller)
			ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 1, map[string]string{
				metrics.ReasonLabel: "scheduled_change",
				"nodepool":          "default",
			})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, nodeClaim)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete the NodeClaim when receiving a state change message", func() {
			var nodeClaims []*karpv1.NodeClaim
			var messages []any
			for _, state := range []string{"terminated", "stopped", "stopping", "shutting-down"} {
				instanceID := fake.InstanceID()
				nc, n := coretest.NodeClaimAndNode(karpv1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							karpv1.NodePoolLabelKey: "default",
						},
					},
					Status: karpv1.NodeClaimStatus{
						ProviderID: fake.ProviderID(instanceID),
					},
				})
				ExpectApplied(ctx, env.Client, nc, n)
				nodeClaims = append(nodeClaims, nc)
				messages = append(messages, stateChangeMessage(instanceID, state))
			}
			ExpectMessagesCreated(messages...)
			ExpectSingletonReconciled(ctx, controller)
			ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 2, map[string]string{
				metrics.ReasonLabel: "instance_terminated",
				"nodepool":          "default",
			})
			ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 2, map[string]string{
				metrics.ReasonLabel: "instance_stopped",
				"nodepool":          "default",
			})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, lo.Map(nodeClaims, func(nc *karpv1.NodeClaim, _ int) client.Object { return nc })...)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(4))
		})
		It("should handle multiple messages that cause nodeClaim deletion", func() {
			var nodeClaims []*karpv1.NodeClaim
			var instanceIDs []string
			for i := 0; i < 100; i++ {
				instanceID := fake.InstanceID()
				nc, n := coretest.NodeClaimAndNode(karpv1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							karpv1.NodePoolLabelKey: "default",
						},
					},
					Status: karpv1.NodeClaimStatus{
						ProviderID: fake.ProviderID(instanceID),
					},
				})
				ExpectApplied(ctx, env.Client, nc, n)
				instanceIDs = append(instanceIDs, instanceID)
				nodeClaims = append(nodeClaims, nc)
			}

			var messages []any
			for _, id := range instanceIDs {
				messages = append(messages, spotInterruptionMessage(id))
			}
			ExpectMessagesCreated(messages...)
			ExpectSingletonReconciled(ctx, controller)
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, lo.Map(nodeClaims, func(nc *karpv1.NodeClaim, _ int) client.Object { return nc })...)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(100))
		})
		It("should delete a message when the message can't be parsed", func() {
			badMessage := &sqstypes.Message{
				Body: aws.String(string(lo.Must(json.Marshal(map[string]string{
					"field1": "value1",
					"field2": "value2",
				})))),
				MessageId: aws.String(string(uuid.NewUUID())),
			}

			ExpectMessagesCreated(badMessage)

			ExpectSingletonReconciled(ctx, controller)
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete a state change message when the state isn't in accepted states", func() {
			ExpectMessagesCreated(stateChangeMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID)), "creating"))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectSingletonReconciled(ctx, controller)
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectExists(ctx, env.Client, nodeClaim)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should mark the ICE cache for the offering when getting a spot interruption warning", func() {
			nodeClaim.Labels = lo.Assign(nodeClaim.Labels, map[string]string{
				corev1.LabelTopologyZone:       "coretest-zone-1a",
				corev1.LabelInstanceTypeStable: "t3.large",
				karpv1.CapacityTypeLabelKey:    karpv1.CapacityTypeSpot,
			})
			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectSingletonReconciled(ctx, controller)
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, nodeClaim)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))

			// Expect a t3.large in coretest-zone-1a to be added to the ICE cache
			Expect(unavailableOfferingsCache.IsUnavailable("t3.large", "coretest-zone-1a", karpv1.CapacityTypeSpot)).To(BeTrue())
		})
		It("should delete the NodeClaim when an instance is unhealthy due to EC2 status checks", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{InterruptionQueue: lo.ToPtr("")}))
			awsEnv.EC2API.DescribeInstanceStatusOutput.Set(&ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId: lo.ToPtr(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))),
						SystemStatus: &ec2types.InstanceStatusSummary{
							Status: ec2types.SummaryStatusImpaired,
							Details: []ec2types.InstanceStatusDetails{
								{
									Status:        ec2types.StatusTypeFailed,
									Name:          ec2types.StatusNameReachability,
									ImpairedSince: lo.ToPtr(awsEnv.Clock.Now()),
								},
							},
						},
						InstanceStatus: &ec2types.InstanceStatusSummary{
							Status: ec2types.SummaryStatusInitializing,
							Details: []ec2types.InstanceStatusDetails{
								{
									Status: ec2types.StatusTypeInitializing,
									Name:   ec2types.StatusNameReachability,
								},
							},
						},
					},
				},
			})
			awsEnv.Clock.Step(time.Hour)
			ExpectApplied(ctx, env.Client, nodeClaim, node)
			ExpectSingletonReconciled(ctx, controller)
			ExpectMetricCounterValue(metrics.NodeClaimsDisruptedTotal, 1, map[string]string{
				metrics.ReasonLabel: "instance_status_failure",
				"nodepool":          "default",
			})
			ExpectNotFound(ctx, env.Client, nodeClaim)
		})
		It("should NOT delete the NodeClaim when an instance is unhealthy due to a Scheduled Event Status or EBS Status", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{InterruptionQueue: lo.ToPtr("")}))
			awsEnv.EC2API.DescribeInstanceStatusOutput.Set(&ec2.DescribeInstanceStatusOutput{
				InstanceStatuses: []ec2types.InstanceStatus{
					{
						InstanceId: lo.ToPtr(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))),
						AttachedEbsStatus: &ec2types.EbsStatusSummary{
							Status: ec2types.SummaryStatusImpaired,
							Details: []ec2types.EbsStatusDetails{
								{
									Status:        ec2types.StatusTypeFailed,
									Name:          ec2types.StatusNameReachability,
									ImpairedSince: lo.ToPtr(awsEnv.Clock.Now()),
								},
							},
						},
						Events: []ec2types.InstanceStatusEvent{
							{
								Code: ec2types.EventCodeInstanceRetirement,
							},
						},
					},
				},
			})
			awsEnv.Clock.Step(time.Hour)
			ExpectApplied(ctx, env.Client, nodeClaim, node)
			ExpectSingletonReconciled(ctx, controller)
			ExpectExists(ctx, env.Client, nodeClaim)
		})
	})
})

var _ = Describe("Error Handling", func() {
	It("should send an error on polling when QueueNotExists", func() {
		sqsapi.ReceiveMessageBehavior.Error.Set(smithyErrWithCode("QueueDoesNotExist"), fake.MaxCalls(0))
		_ = ExpectSingletonReconcileFailed(ctx, controller)
	})
	It("should send an error on polling when AccessDenied", func() {
		sqsapi.ReceiveMessageBehavior.Error.Set(smithyErrWithCode("AccessDenied"), fake.MaxCalls(0))
		_ = ExpectSingletonReconcileFailed(ctx, controller)
	})
	It("should not return an error when deleting a nodeClaim that is already deleted", func() {
		ExpectMessagesCreated(spotInterruptionMessage(fake.InstanceID()))
		ExpectSingletonReconciled(ctx, controller)
	})
})

func ExpectMessagesCreated(messages ...any) {
	raw := lo.Map(messages, func(m any, _ int) *sqstypes.Message {
		return &sqstypes.Message{
			Body:      aws.String(string(lo.Must(json.Marshal(m)))),
			MessageId: aws.String(string(uuid.NewUUID())),
		}
	})
	sqsapi.ReceiveMessageBehavior.Output.Set(
		&servicesqs.ReceiveMessageOutput{
			Messages: lo.FromSlicePtr(raw),
		},
	)
}

func smithyErrWithCode(code string) smithy.APIError {
	return &smithy.GenericAPIError{
		Code:    code,
		Message: "error",
	}
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

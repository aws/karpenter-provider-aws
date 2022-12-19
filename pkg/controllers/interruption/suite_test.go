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

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/sqs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coresettings "github.com/aws/karpenter-core/pkg/apis/config/settings"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awscache "github.com/aws/karpenter/pkg/cache"
	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/controllers/interruption"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/scheduledchange"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/spotinterruption"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/statechange"
	"github.com/aws/karpenter/pkg/errors"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"
)

const (
	defaultAccountID  = "000000000000"
	defaultInstanceID = "i-08c6fdb11e28c8c90"
	defaultRegion     = "us-west-2"
	ec2Source         = "aws.ec2"
	healthSource      = "aws.health"
)

var ctx context.Context
var env *coretest.Environment
var sqsapi *fake.SQSAPI
var sqsProvider *interruption.SQSProvider
var unavailableOfferingsCache *awscache.UnavailableOfferings
var recorder *coretest.EventRecorder
var fakeClock *clock.FakeClock
var controller *interruption.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSInterruption")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, apis.CRDs...)
	fakeClock = &clock.FakeClock{}
	recorder = coretest.NewEventRecorder()
	unavailableOfferingsCache = awscache.NewUnavailableOfferings(cache.New(awscache.UnavailableOfferingsTTL, awscontext.CacheCleanupInterval))
	sqsapi = &fake.SQSAPI{}
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	sqsProvider = interruption.NewSQSProvider(sqsapi)
	controller = interruption.NewController(env.Client, fakeClock, recorder, sqsProvider, unavailableOfferingsCache)
	settingsStore := coretest.SettingsStore{
		coresettings.ContextKey: coretest.Settings(),
		settings.ContextKey: test.Settings(test.SettingOptions{
			InterruptionQueueName: lo.ToPtr("test-cluster"),
		}),
	}
	ctx = settingsStore.InjectSettings(ctx)
})

var _ = AfterEach(func() {
	sqsapi.Reset()
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("AWSInterruption", func() {
	Context("Processing Messages", func() {
		It("should delete the node when receiving a spot interruption warning", func() {
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: "default",
					},
				},
				ProviderID: makeProviderID(defaultInstanceID),
			})
			ExpectMessagesCreated(spotInterruptionMessage(defaultInstanceID))
			ExpectApplied(ctx, env.Client, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, node)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete the node when receiving a scheduled change message", func() {
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: "default",
					},
				},
				ProviderID: makeProviderID(defaultInstanceID),
			})
			ExpectMessagesCreated(scheduledChangeMessage(defaultInstanceID))
			ExpectApplied(ctx, env.Client, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, node)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should delete the node when receiving a state change message", func() {
			var nodes []*v1.Node
			var messages []interface{}
			for _, state := range []string{"terminated", "stopped", "stopping", "shutting-down"} {
				instanceID := makeInstanceID()
				nodes = append(nodes, coretest.Node(coretest.NodeOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1alpha5.ProvisionerNameLabelKey: "default",
						},
					},
					ProviderID: makeProviderID(instanceID),
				}))
				messages = append(messages, stateChangeMessage(instanceID, state))
			}
			ExpectMessagesCreated(messages...)
			ExpectApplied(ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(4))
		})
		It("should handle multiple messages that cause node deletion", func() {
			var nodes []*v1.Node
			var instanceIDs []string
			for i := 0; i < 100; i++ {
				instanceIDs = append(instanceIDs, makeInstanceID())
				nodes = append(nodes, coretest.Node(coretest.NodeOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1alpha5.ProvisionerNameLabelKey: "default",
						},
					},
					ProviderID: makeProviderID(instanceIDs[len(instanceIDs)-1]),
				}))

			}

			var messages []interface{}
			for _, id := range instanceIDs {
				messages = append(messages, spotInterruptionMessage(id))
			}
			ExpectMessagesCreated(messages...)
			ExpectApplied(ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(100))
		})
		It("should not delete a node when not owned by provisioner", func() {
			node := coretest.Node(coretest.NodeOptions{
				ProviderID: makeProviderID(string(uuid.NewUUID())),
			})
			ExpectMessagesCreated(spotInterruptionMessage(node.Spec.ProviderID))
			ExpectApplied(ctx, env.Client, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNodeExists(ctx, env.Client, node.Name)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
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
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: "default",
					},
				},
				ProviderID: makeProviderID(defaultInstanceID),
			})
			ExpectMessagesCreated(stateChangeMessage(defaultInstanceID, "creating"))
			ExpectApplied(ctx, env.Client, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNodeExists(ctx, env.Client, node.Name)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should mark the ICE cache for the offering when getting a spot interruption warning", func() {
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: "default",
						v1.LabelTopologyZone:             "coretest-zone-1a",
						v1.LabelInstanceTypeStable:       "t3.large",
						v1alpha5.LabelCapacityType:       v1alpha1.CapacityTypeSpot,
					},
				},
				ProviderID: makeProviderID(defaultInstanceID),
			})
			ExpectMessagesCreated(spotInterruptionMessage(defaultInstanceID))
			ExpectApplied(ctx, env.Client, node)

			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
			ExpectNotFound(ctx, env.Client, node)
			Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))

			// Expect a t3.large in coretest-zone-1a to be added to the ICE cache
			Expect(unavailableOfferingsCache.IsUnavailable("t3.large", "coretest-zone-1a", v1alpha1.CapacityTypeSpot)).To(BeTrue())
		})
	})
	Context("Error Handling", func() {
		It("should send an error on polling when QueueNotExists", func() {
			sqsapi.ReceiveMessageBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), fake.MaxCalls(0))
			ExpectReconcileFailed(ctx, controller, types.NamespacedName{})
		})
		It("should send an error on polling when AccessDenied", func() {
			sqsapi.ReceiveMessageBehavior.Error.Set(awsErrWithCode(errors.AccessDeniedCode), fake.MaxCalls(0))
			ExpectReconcileFailed(ctx, controller, types.NamespacedName{})
		})
	})
	Context("Configuration", func() {
		It("should not poll SQS if interruption queue is disabled", func() {
			settingsStore := coretest.SettingsStore{
				coresettings.ContextKey: coretest.Settings(),
				settings.ContextKey: test.Settings(test.SettingOptions{
					InterruptionQueueName: lo.ToPtr(""),
				}),
			}
			ctx = settingsStore.InjectSettings(ctx)
			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(0))
		})
		It("should only call the get queue url once if the queue name doesn't change", func() {
			for i := 0; i < 100; i++ {
				ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			}
			Expect(sqsapi.GetQueueURLBehavior.SuccessfulCalls()).To(Equal(1))
		})
		It("should re-request the queue url from SQS if queue name changes", func() {
			for i := 0; i < 10; i++ {
				ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			}
			Expect(sqsapi.GetQueueURLBehavior.SuccessfulCalls()).To(Equal(1))
			settingsStore := coretest.SettingsStore{
				coresettings.ContextKey: coretest.Settings(),
				settings.ContextKey: test.Settings(test.SettingOptions{
					InterruptionQueueName: lo.ToPtr("other-queue-name"),
				}),
			}
			ctx = settingsStore.InjectSettings(ctx)
			ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
			Expect(sqsapi.GetQueueURLBehavior.SuccessfulCalls()).To(Equal(2))
		})
	})
})

func ExpectMessagesCreated(messages ...interface{}) {
	raw := lo.Map(messages, func(m interface{}, _ int) *sqs.Message {
		return &sqs.Message{
			Body:      aws.String(string(lo.Must(json.Marshal(m)))),
			MessageId: aws.String(string(uuid.NewUUID())),
		}
	})
	sqsapi.ReceiveMessageBehavior.Output.Set(
		&sqs.ReceiveMessageOutput{
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
			Region:     defaultRegion,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", defaultRegion, involvedInstanceID),
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
			Region:     defaultRegion,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", defaultRegion, involvedInstanceID),
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
			Region:     defaultRegion,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", defaultRegion, involvedInstanceID),
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

func makeProviderID(instanceID string) string {
	return fmt.Sprintf("aws:///%s/%s", defaultRegion, instanceID)
}

func makeInstanceID() string {
	return fmt.Sprintf("i-%s", randomdata.Alphanumeric(17))
}

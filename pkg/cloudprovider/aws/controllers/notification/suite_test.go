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

package notification_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/sqs"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"

	"github.com/google/uuid"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/infrastructure"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event"
	scheduledchangev0 "github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/scheduledchange/v0"
	spotinterruptionv0 "github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/spotinterruption/v0"
	statechangev0 "github.com/aws/karpenter/pkg/cloudprovider/aws/controllers/notification/event/statechange/v0"
	awsfake "github.com/aws/karpenter/pkg/cloudprovider/aws/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/options"
)

const (
	defaultAccountID  = "000000000000"
	defaultInstanceID = "i-08c6fdb11e28c8c90"
	defaultRegion     = "us-west-2"
	ec2Source         = "aws.ec2"
	healthSource      = "aws.health"
)

var ctx context.Context
var env *test.Environment
var cluster *state.Cluster
var ec2api *awsfake.EC2API
var sqsapi *awsfake.SQSAPI
var eventbridgeapi *awsfake.EventBridgeAPI
var cloudProvider *fake.CloudProvider
var sqsProvider *aws.SQSProvider
var instanceTypeProvider *aws.InstanceTypeProvider
var eventBridgeProvider *aws.EventBridgeProvider
var recorder *awsfake.EventRecorder
var fakeClock *clock.FakeClock
var cfg *test.Config
var controller *notification.Controller
var infraController *infrastructure.Controller
var nodeStateController *state.NodeController
var infraStartChan chan struct{}
var notificationStartChan chan struct{}

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWS Notification")
}

var _ = BeforeEach(func() {
	opts := options.Options{
		AWSIsolatedVPC: true,
	}
	ctx = injection.WithOptions(ctx, opts)
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cfg = test.NewConfig()
		fakeClock = clock.NewFakeClock(time.Now())
		cloudProvider = &fake.CloudProvider{}
		cluster = state.NewCluster(fakeClock, cfg, env.Client, cloudProvider)
		nodeStateController = state.NewNodeController(env.Client, cluster)
		recorder = awsfake.NewEventRecorder()
		metadata := aws.NewMetadata("us-east-1", "000000000000")

		sqsapi = &awsfake.SQSAPI{}
		sqsProvider = aws.NewSQSProvider(ctx, sqsapi, metadata)
		eventbridgeapi = &awsfake.EventBridgeAPI{}
		eventBridgeProvider = aws.NewEventBridgeProvider(eventbridgeapi, metadata, sqsProvider.QueueName())

		infraStartChan = make(chan struct{})
		notificationStartChan = make(chan struct{})

		ec2api = &awsfake.EC2API{}
		subnetProvider := aws.NewSubnetProvider(ec2api)
		instanceTypeProvider = aws.NewInstanceTypeProvider(env.Ctx, mock.Session, cloudprovider.Options{}, ec2api, subnetProvider)
		infraController = infrastructure.NewController(env.Ctx, env.Client, fakeClock, recorder, sqsProvider, eventBridgeProvider, infraStartChan, env.Ctx.Done())
		controller = notification.NewController(env.Ctx, env.Client, fakeClock, recorder, cluster, sqsProvider, instanceTypeProvider, infraController, notificationStartChan)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Processing Messages", func() {
	It("should delete the node when receiving a spot interruption warning", func() {
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			ProviderID: makeProviderID(defaultInstanceID),
		})
		ExpectMessagesCreated(spotInterruptionMessage(defaultInstanceID))
		ExpectApplied(env.Ctx, env.Client, node)
		ExpectReconcileSucceeded(env.Ctx, nodeStateController, client.ObjectKeyFromObject(node))

		Expect(controller.PollSQS(env.Ctx)).To(Succeed())
		ExpectNotFound(env.Ctx, env.Client, node)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
	})
	It("should delete the node when receiving a scheduled change message", func() {
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			ProviderID: makeProviderID(defaultInstanceID),
		})
		ExpectMessagesCreated(scheduledChangeMessage(defaultInstanceID))
		ExpectApplied(env.Ctx, env.Client, node)
		ExpectReconcileSucceeded(env.Ctx, nodeStateController, client.ObjectKeyFromObject(node))

		Expect(controller.PollSQS(env.Ctx)).To(Succeed())
		ExpectNotFound(env.Ctx, env.Client, node)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
	})
	It("should delete the node when receiving a state change message", func() {
		var nodes []*v1.Node
		var messages []*sqs.Message
		for _, state := range []string{"terminated", "stopped", "stopping", "shutting-down"} {
			instanceID := makeInstanceID()
			nodes = append(nodes, test.Node(test.NodeOptions{
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
		ExpectApplied(env.Ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)

		// Wait for the nodes to reconcile with the cluster state
		ExpectReconcileSucceeded(env.Ctx, nodeStateController, lo.Map(nodes, func(n *v1.Node, _ int) client.ObjectKey { return client.ObjectKeyFromObject(n) })...)
		Expect(controller.PollSQS(env.Ctx)).To(Succeed())
		ExpectNotFound(env.Ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(4))
	})
	It("should handle multiple messages that cause node deletion", func() {
		var nodes []*v1.Node
		var instanceIDs []string
		for i := 0; i < 100; i++ {
			instanceIDs = append(instanceIDs, makeInstanceID())
			nodes = append(nodes, test.Node(test.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: "default",
					},
				},
				ProviderID: makeProviderID(instanceIDs[len(instanceIDs)-1]),
			}))

		}

		var messages []*sqs.Message
		for _, id := range instanceIDs {
			messages = append(messages, spotInterruptionMessage(id))
		}
		ExpectMessagesCreated(messages...)
		ExpectApplied(env.Ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)

		// Wait for the nodes to reconcile with the cluster state
		ExpectReconcileSucceeded(env.Ctx, nodeStateController, lo.Map(nodes, func(n *v1.Node, _ int) client.ObjectKey { return client.ObjectKeyFromObject(n) })...)
		Expect(controller.PollSQS(env.Ctx)).To(Succeed())
		ExpectNotFound(env.Ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(100))
	})
	It("should not delete a node when not owned by provisioner", func() {
		node := test.Node(test.NodeOptions{
			ProviderID: makeProviderID(uuid.NewString()),
		})
		ExpectMessagesCreated(spotInterruptionMessage(node.Spec.ProviderID))
		ExpectApplied(env.Ctx, env.Client, node)
		ExpectReconcileSucceeded(env.Ctx, nodeStateController, client.ObjectKeyFromObject(node))

		Expect(controller.PollSQS(env.Ctx)).To(Succeed())
		ExpectNodeExists(env.Ctx, env.Client, node.Name)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
	})
	It("should delete a message when the message can't be parsed", func() {
		badMessage := &sqs.Message{
			Body: awssdk.String(string(lo.Must(json.Marshal(map[string]string{
				"field1": "value1",
				"field2": "value2",
			})))),
			MessageId: awssdk.String(uuid.NewString()),
		}

		ExpectMessagesCreated(badMessage)
		Expect(controller.PollSQS(env.Ctx)).To(Succeed())
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
	})
	It("should delete a state change message when the state isn't in accepted states", func() {
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			ProviderID: makeProviderID(defaultInstanceID),
		})
		ExpectMessagesCreated(stateChangeMessage(defaultInstanceID, "creating"))
		ExpectApplied(env.Ctx, env.Client, node)
		ExpectReconcileSucceeded(env.Ctx, nodeStateController, client.ObjectKeyFromObject(node))

		Expect(controller.PollSQS(env.Ctx)).To(Succeed())
		ExpectNodeExists(env.Ctx, env.Client, node.Name)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
	})
	It("should mark the ICE cache for the offering when getting a spot interruption warning", func() {
		node := test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
					v1.LabelTopologyZone:             "test-zone-1a",
					v1.LabelInstanceTypeStable:       "t3.large",
					v1alpha5.LabelCapacityType:       v1alpha1.CapacityTypeSpot,
				},
			},
			ProviderID: makeProviderID(defaultInstanceID),
		})
		ExpectMessagesCreated(spotInterruptionMessage(defaultInstanceID))
		ExpectApplied(env.Ctx, env.Client, node)
		ExpectReconcileSucceeded(env.Ctx, nodeStateController, client.ObjectKeyFromObject(node))

		Expect(controller.PollSQS(env.Ctx)).To(Succeed())
		ExpectNotFound(env.Ctx, env.Client, node)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))

		// Expect a t3.large in test-zone-1a to not be returned since we should add it to the ICE cache
		instanceTypes, err := instanceTypeProvider.Get(env.Ctx, &v1alpha1.AWS{}, &v1alpha5.KubeletConfiguration{})
		Expect(err).To(Succeed())

		t3Large := lo.Filter(instanceTypes, func(it cloudprovider.InstanceType, _ int) bool {
			return it.Name() == "t3.large"
		})
		Expect(len(t3Large)).To(BeNumerically("==", 1))
		matchingOfferings := lo.Filter(t3Large[0].Offerings(), func(of cloudprovider.Offering, _ int) bool {
			return of.CapacityType == v1alpha1.CapacityTypeSpot && of.Zone == "test-zone-1a"
		})
		Expect(len(matchingOfferings)).To(BeNumerically("==", 1))
		Expect(matchingOfferings[0].Available).To(BeFalse())
	})
})

var _ = Describe("Error Handling", func() {
	It("should send an error on polling when AccessDenied", func() {
		ExpectClosed(infraStartChan)
		sqsapi.ReceiveMessageBehavior.Error.Set(awsErrWithCode(aws.AccessDeniedCode), awsfake.MaxCalls(0))
		Expect(controller.PollSQS(env.Ctx)).ToNot(Succeed())
	})
	It("should trigger an infrastructure reconciliation on an SQS queue when it doesn't exist", func() {
		sqsapi.GetQueueURLBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), awsfake.MaxCalls(0)) // This mocks the queue not existing
		ExpectClosed(infraStartChan)

		// Infrastructure reconciliation loop has completed
		Eventually(func(g Gomega) {
			g.Expect(sqsapi.CreateQueueBehavior.SuccessfulCalls()).To(Equal(1))
			g.Expect(eventbridgeapi.PutRuleBehavior.SuccessfulCalls()).To(Equal(4))
			g.Expect(eventbridgeapi.PutTargetsBehavior.SuccessfulCalls()).To(Equal(4))
			g.Expect(IsClosed(infraController.Ready())).To(BeTrue())
		}).Should(Succeed())

		sqsapi.ReceiveMessageBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist)) // This mocks the queue being deleted manually after infra reconciliation

		// This should fail with an error since the queue doesn't exist
		Expect(controller.PollSQS(env.Ctx)).ToNot(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(sqsapi.CreateQueueBehavior.SuccessfulCalls()).To(Equal(2))
			g.Expect(eventbridgeapi.PutRuleBehavior.SuccessfulCalls()).To(Equal(8))
			g.Expect(eventbridgeapi.PutTargetsBehavior.SuccessfulCalls()).To(Equal(8))
			g.Expect(IsClosed(infraController.Ready())).To(BeTrue())
		}).Should(Succeed())
	})
})

var _ = Describe("Infrastructure Coordination", func() {
	It("should wait for the infrastructure to be ready before polling SQS", func() {
		ExpectClosed(notificationStartChan)
		Expect(IsClosed(infraController.Ready())).To(BeFalse())
		Consistently(func(g Gomega) {
			g.Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(0))
			g.Expect(sqsapi.ReceiveMessageBehavior.FailedCalls()).To(Equal(0))
		}, time.Second*10).Should(Succeed())

		ExpectClosed(infraStartChan)

		Eventually(func(g Gomega) {
			g.Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(BeNumerically(">", 0))
		}, time.Second*10).Should(Succeed())
	})
})

func ExpectMessagesCreated(messages ...*sqs.Message) {
	sqsapi.ReceiveMessageBehavior.Output.Set(
		&sqs.ReceiveMessageOutput{
			Messages: messages,
		},
	)
}

func awsErrWithCode(code string) awserr.Error {
	return awserr.New(code, "", fmt.Errorf(""))
}

func spotInterruptionMessage(involvedInstanceID string) *sqs.Message {
	evt := spotinterruptionv0.AWSEvent{
		AWSMetadata: event.AWSMetadata{
			Version:    "0",
			Account:    defaultAccountID,
			DetailType: "EC2 Spot Instance Interruption Warning",
			ID:         uuid.NewString(),
			Region:     defaultRegion,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", defaultRegion, involvedInstanceID),
			},
			Source: ec2Source,
			Time:   time.Now(),
		},
		Detail: spotinterruptionv0.EC2SpotInstanceInterruptionWarningDetail{
			InstanceID:     involvedInstanceID,
			InstanceAction: "terminate",
		},
	}
	return &sqs.Message{
		Body:      awssdk.String(string(lo.Must(json.Marshal(evt)))),
		MessageId: awssdk.String(uuid.NewString()),
	}
}

func stateChangeMessage(involvedInstanceID, state string) *sqs.Message {
	evt := statechangev0.AWSEvent{
		AWSMetadata: event.AWSMetadata{
			Version:    "0",
			Account:    defaultAccountID,
			DetailType: "EC2 Instance State-change Notification",
			ID:         uuid.NewString(),
			Region:     defaultRegion,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", defaultRegion, involvedInstanceID),
			},
			Source: ec2Source,
			Time:   time.Now(),
		},
		Detail: statechangev0.EC2InstanceStateChangeNotificationDetail{
			InstanceID: involvedInstanceID,
			State:      state,
		},
	}
	return &sqs.Message{
		Body:      awssdk.String(string(lo.Must(json.Marshal(evt)))),
		MessageId: awssdk.String(uuid.NewString()),
	}
}

// TODO: Update the scheduled change message to accurately reflect a real health event
func scheduledChangeMessage(involvedInstanceID string) *sqs.Message {
	evt := scheduledchangev0.AWSEvent{
		AWSMetadata: event.AWSMetadata{
			Version:    "0",
			Account:    defaultAccountID,
			DetailType: "AWS Health Event",
			ID:         uuid.NewString(),
			Region:     defaultRegion,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", defaultRegion, involvedInstanceID),
			},
			Source: healthSource,
			Time:   time.Now(),
		},
		Detail: scheduledchangev0.AWSHealthEventDetail{
			Service:           "EC2",
			EventTypeCategory: "scheduledChange",
			AffectedEntities: []scheduledchangev0.AffectedEntity{
				{
					EntityValue: involvedInstanceID,
				},
			},
		},
	}
	return &sqs.Message{
		Body:      awssdk.String(string(lo.Must(json.Marshal(evt)))),
		MessageId: awssdk.String(uuid.NewString()),
	}
}

func makeProviderID(instanceID string) string {
	return fmt.Sprintf("aws:///%s/%s", defaultRegion, instanceID)
}

var runes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// nolint:gosec
func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = runes[rand.Intn(len(runes))]
	}
	return string(b)
}

func makeInstanceID() string {
	return fmt.Sprintf("i-%s", randStringRunes(17))
}

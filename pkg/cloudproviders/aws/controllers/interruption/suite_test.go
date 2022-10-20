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
	"math/rand"
	"testing"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/apis/v1alpha1"
	awscache "github.com/aws/karpenter/pkg/cloudproviders/aws/cache"
	awscontext "github.com/aws/karpenter/pkg/cloudproviders/aws/context"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption"
	event2 "github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages/scheduledchange"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages/spotinterruption"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/interruption/messages/statechange"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/controllers/providers"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/errors"
	"github.com/aws/karpenter/pkg/cloudproviders/aws/events"
	awsfake "github.com/aws/karpenter/pkg/cloudproviders/aws/fake"
	"github.com/aws/karpenter/pkg/cloudproviders/common/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/operator/injection"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
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
var sqsapi *awsfake.SQSAPI
var eventbridgeapi *awsfake.EventBridgeAPI
var cloudProvider *fake.CloudProvider
var sqsProvider *providers.SQS
var eventBridgeProvider *providers.EventBridge
var unavailableOfferingsCache *awscache.UnavailableOfferings
var recorder *events.Recorder
var fakeClock *clock.FakeClock
var cfg *test.Config
var controller *interruption.Controller
var nodeStateController *state.NodeController

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
		fakeClock = &clock.FakeClock{}
		cloudProvider = &fake.CloudProvider{}
		cluster = state.NewCluster(fakeClock, cfg, env.Client, cloudProvider)
		nodeStateController = state.NewNodeController(env.Client, cluster)
		recorder = events.NewRecorder(awsfake.NewEventRecorder())
		unavailableOfferingsCache = awscache.NewUnavailableOfferings(cache.New(awscache.UnavailableOfferingsTTL, awscontext.CacheCleanupInterval))

		sqsapi = &awsfake.SQSAPI{}
		sqsProvider = providers.NewSQS(ctx, sqsapi)
		eventbridgeapi = &awsfake.EventBridgeAPI{}
		eventBridgeProvider = providers.NewEventBridge(eventbridgeapi, sqsProvider)

		controller = interruption.NewController(env.Client, fakeClock, recorder, cluster, sqsProvider, unavailableOfferingsCache)
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

		ExpectReconcileSucceeded(env.Ctx, controller, types.NamespacedName{})
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

		ExpectReconcileSucceeded(env.Ctx, controller, types.NamespacedName{})
		ExpectNotFound(env.Ctx, env.Client, node)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))
	})
	It("should delete the node when receiving a state change message", func() {
		var nodes []*v1.Node
		var messages []interface{}
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

		ExpectReconcileSucceeded(env.Ctx, controller, types.NamespacedName{})
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

		var messages []interface{}
		for _, id := range instanceIDs {
			messages = append(messages, spotInterruptionMessage(id))
		}
		ExpectMessagesCreated(messages...)
		ExpectApplied(env.Ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)

		// Wait for the nodes to reconcile with the cluster state
		ExpectReconcileSucceeded(env.Ctx, nodeStateController, lo.Map(nodes, func(n *v1.Node, _ int) client.ObjectKey { return client.ObjectKeyFromObject(n) })...)

		ExpectReconcileSucceeded(env.Ctx, controller, types.NamespacedName{})
		ExpectNotFound(env.Ctx, env.Client, lo.Map(nodes, func(n *v1.Node, _ int) client.Object { return n })...)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(100))
	})
	It("should not delete a node when not owned by provisioner", func() {
		node := test.Node(test.NodeOptions{
			ProviderID: makeProviderID(string(uuid.NewUUID())),
		})
		ExpectMessagesCreated(spotInterruptionMessage(node.Spec.ProviderID))
		ExpectApplied(env.Ctx, env.Client, node)
		ExpectReconcileSucceeded(env.Ctx, nodeStateController, client.ObjectKeyFromObject(node))

		ExpectReconcileSucceeded(env.Ctx, controller, types.NamespacedName{})
		ExpectNodeExists(env.Ctx, env.Client, node.Name)
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

		ExpectReconcileSucceeded(env.Ctx, controller, types.NamespacedName{})
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

		ExpectReconcileSucceeded(env.Ctx, controller, types.NamespacedName{})
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

		ExpectReconcileSucceeded(env.Ctx, controller, types.NamespacedName{})
		ExpectNotFound(env.Ctx, env.Client, node)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(1))

		// Expect a t3.large in test-zone-1a to be added to the ICE cache
		Expect(unavailableOfferingsCache.IsUnavailable("t3.large", "test-zone-1a", v1alpha1.CapacityTypeSpot)).To(BeTrue())
	})
})

var _ = Describe("Error Handling", func() {
	It("should send an error on polling when AccessDenied", func() {
		sqsapi.ReceiveMessageBehavior.Error.Set(awsErrWithCode(errors.AccessDeniedCode), awsfake.MaxCalls(0))

		_, err := controller.Reconcile(env.Ctx, reconcile.Request{})
		Expect(err).ToNot(Succeed())
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

func spotInterruptionMessage(involvedInstanceID string) spotinterruption.Event {
	return spotinterruption.Event{
		Metadata: event2.Metadata{
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

func stateChangeMessage(involvedInstanceID, state string) statechange.Event {
	return statechange.Event{
		Metadata: event2.Metadata{
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

// TODO: Update the scheduled change message to accurately reflect a real health event
func scheduledChangeMessage(involvedInstanceID string) scheduledchange.Event {
	return scheduledchange.Event{
		Metadata: event2.Metadata{
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

func NewWrappedMessage(evt event2.Interface) *sqs.Message {
	return &sqs.Message{
		Body:      aws.String(string(lo.Must(json.Marshal(evt)))),
		MessageId: aws.String(string(uuid.NewUUID())),
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

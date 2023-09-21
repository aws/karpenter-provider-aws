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
	"github.com/aws/aws-sdk-go/service/sqs"
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

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/events"
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
var sqsProvider *interruption.SQSProvider
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
	sqsProvider = interruption.NewSQSProvider(sqsapi)
	controller = interruption.NewController(env.Client, fakeClock, events.NewRecorder(&record.FakeRecorder{}), sqsProvider, unavailableOfferingsCache)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
		InterruptionQueueName: lo.ToPtr("test-cluster"),
	}))
	unavailableOfferingsCache.Flush()
	sqsapi.Reset()
	sqsProvider.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Combined/InterruptionHandling", func() {
	var machineNode, nodeClaimNode *v1.Node
	var machine *v1alpha5.Machine
	var nodeClaim *corev1beta1.NodeClaim
	BeforeEach(func() {
		machine, machineNode = coretest.MachineAndNode(v1alpha5.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: "default",
				},
			},
			Status: v1alpha5.MachineStatus{
				ProviderID: fake.RandomProviderID(),
			},
		})
		nodeClaim, nodeClaimNode = coretest.NodeClaimAndNode(corev1beta1.NodeClaim{
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
	It("should delete both the NodeClaim and the Machine when receiving a spot interruption warning", func() {
		ExpectMessagesCreated(
			spotInterruptionMessage(lo.Must(utils.ParseInstanceID(machine.Status.ProviderID))),
			spotInterruptionMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))),
		)
		ExpectApplied(ctx, env.Client, machine, machineNode, nodeClaim, nodeClaimNode)

		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
		Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
		ExpectNotFound(ctx, env.Client, machine, nodeClaim)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(2))
	})
	It("should delete both the NodeClaim and the Machine when receiving a scheduled change message", func() {
		ExpectMessagesCreated(
			scheduledChangeMessage(lo.Must(utils.ParseInstanceID(machine.Status.ProviderID))),
			scheduledChangeMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))),
		)
		ExpectApplied(ctx, env.Client, machine, machineNode, nodeClaim, nodeClaimNode)

		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
		Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
		ExpectNotFound(ctx, env.Client, machine, nodeClaim)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(2))
	})
	It("should delete the both the NodeClaim and the Machine when receiving a state change message", func() {
		var machines []*v1alpha5.Machine
		var nodeClaims []*corev1beta1.NodeClaim
		var messages []interface{}
		for _, state := range []string{"terminated", "stopped", "stopping", "shutting-down"} {
			mInstanceID := fake.InstanceID()
			m, mNode := coretest.MachineAndNode(v1alpha5.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: "default",
					},
				},
				Status: v1alpha5.MachineStatus{
					ProviderID: fake.ProviderID(mInstanceID),
				},
			})
			ncInstanceID := fake.InstanceID()
			nc, ncNode := coretest.NodeClaimAndNode(corev1beta1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						corev1beta1.NodePoolLabelKey: "default",
					},
				},
				Status: corev1beta1.NodeClaimStatus{
					ProviderID: fake.ProviderID(ncInstanceID),
				},
			})
			ExpectApplied(ctx, env.Client, m, mNode, nc, ncNode)
			machines = append(machines, m)
			nodeClaims = append(nodeClaims, nc)
			messages = append(messages, stateChangeMessage(mInstanceID, state), stateChangeMessage(ncInstanceID, state))
		}
		ExpectMessagesCreated(messages...)
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{})
		Expect(sqsapi.ReceiveMessageBehavior.SuccessfulCalls()).To(Equal(1))
		ExpectNotFound(ctx, env.Client, lo.Map(machines, func(m *v1alpha5.Machine, _ int) client.Object { return m })...)
		ExpectNotFound(ctx, env.Client, lo.Map(nodeClaims, func(nc *corev1beta1.NodeClaim, _ int) client.Object { return nc })...)
		Expect(sqsapi.DeleteMessageBehavior.SuccessfulCalls()).To(Equal(8))
	})
})

var _ = Describe("Error Handling", func() {
	It("should send an error on polling when QueueNotExists", func() {
		sqsapi.ReceiveMessageBehavior.Error.Set(awsErrWithCode(sqs.ErrCodeQueueDoesNotExist), fake.MaxCalls(0))
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

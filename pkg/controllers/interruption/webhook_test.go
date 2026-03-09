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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	servicesqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/sqs"
	"github.com/aws/karpenter-provider-aws/pkg/providers/webhook"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
)

var _ = Describe("Webhook Integration", func() {
	var (
		webhookServer         *httptest.Server
		receivedPayloads      []map[string]interface{}
		payloadsMu            sync.Mutex
		controllerWithWebhook *interruption.Controller
		testSQSProvider       *sqs.DefaultProvider
		testSQSAPI            *fake.SQSAPI
	)

	BeforeEach(func() {
		receivedPayloads = []map[string]interface{}{}

		// Create test webhook server
		webhookServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			json.Unmarshal(body, &payload)

			payloadsMu.Lock()
			receivedPayloads = append(receivedPayloads, payload)
			payloadsMu.Unlock()

			w.WriteHeader(http.StatusOK)
		}))

		// Create webhook provider
		webhookProv, err := webhook.NewDefaultProvider(webhookServer.URL, "", "all")
		Expect(err).ToNot(HaveOccurred())

		// Set up test environment
		testSQSAPI = &fake.SQSAPI{}
		testSQSProvider = lo.Must(sqs.NewDefaultProvider(testSQSAPI, fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/test-cluster", fake.DefaultRegion, fake.DefaultAccount)))

		// Create controller with webhook provider
		testCloudProvider := cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
			env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider, awsEnv.InstanceTypeStore)

		controllerWithWebhook = interruption.NewController(
			env.Client,
			testCloudProvider,
			&clock.FakeClock{},
			events.NewRecorder(&record.FakeRecorder{}),
			testSQSProvider,
			servicesqs.NewFromConfig(aws.Config{}),
			awscache.NewUnavailableOfferings(),
			webhookProv,
		)

		// Reset SQS API
		testSQSAPI.Reset()
	})

	AfterEach(func() {
		if webhookServer != nil {
			webhookServer.Close()
		}
	})

	Context("Webhook Notifications", func() {
		It("should send webhook notification for spot interruption", func() {
			nodeClaim, node := coretest.NodeClaimAndNode(karpv1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						karpv1.NodePoolLabelKey:        "default",
						corev1.LabelInstanceTypeStable: "m5.large",
						corev1.LabelTopologyZone:       "us-west-2a",
						karpv1.CapacityTypeLabelKey:    karpv1.CapacityTypeSpot,
					},
				},
				Status: karpv1.NodeClaimStatus{
					ProviderID: fake.RandomProviderID(),
				},
			})

			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectSingletonReconciled(ctx, controllerWithWebhook)

			// Wait for async webhook to complete
			Eventually(func() int {
				payloadsMu.Lock()
				defer payloadsMu.Unlock()
				return len(receivedPayloads)
			}, 5*time.Second).Should(Equal(1))

			payloadsMu.Lock()
			defer payloadsMu.Unlock()

			// Verify webhook payload
			Expect(receivedPayloads).To(HaveLen(1))
			payload := receivedPayloads[0]

			// Check that payload contains expected fields
			Expect(payload["text"]).To(ContainSubstring("Spot Interruption"))

			// Verify blocks structure (Slack format)
			blocks := payload["blocks"].([]interface{})
			Expect(blocks).To(HaveLen(2))

			// Verify first block contains the event message
			firstBlock := blocks[0].(map[string]interface{})
			Expect(firstBlock["type"]).To(Equal("section"))

			// Verify second block contains instance details
			secondBlock := blocks[1].(map[string]interface{})
			fields := secondBlock["fields"].([]interface{})
			Expect(fields).To(HaveLen(7)) // Cluster, NodeClaim, Instance, Type, Zone, NodePool, Capacity
		})

		It("should send webhook notification for scheduled change", func() {
			nodeClaim, node := coretest.NodeClaimAndNode(karpv1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						karpv1.NodePoolLabelKey: "default",
					},
				},
				Status: karpv1.NodeClaimStatus{
					ProviderID: fake.RandomProviderID(),
				},
			})

			ExpectMessagesCreated(scheduledChangeMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectSingletonReconciled(ctx, controllerWithWebhook)

			// Wait for async webhook to complete
			Eventually(func() int {
				payloadsMu.Lock()
				defer payloadsMu.Unlock()
				return len(receivedPayloads)
			}, 5*time.Second).Should(Equal(1))

			payloadsMu.Lock()
			defer payloadsMu.Unlock()

			Expect(receivedPayloads).To(HaveLen(1))
			payload := receivedPayloads[0]
			Expect(payload["text"]).To(ContainSubstring("Scheduled Change"))
		})

		It("should not send webhook when provider is nil", func() {
			// Use the original controller without webhook provider
			nodeClaim, node := coretest.NodeClaimAndNode(karpv1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						karpv1.NodePoolLabelKey: "default",
					},
				},
				Status: karpv1.NodeClaimStatus{
					ProviderID: fake.RandomProviderID(),
				},
			})

			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			// Use original controller without webhook
			ExpectSingletonReconciled(ctx, controller)

			// Verify no webhooks were sent
			Consistently(func() int {
				payloadsMu.Lock()
				defer payloadsMu.Unlock()
				return len(receivedPayloads)
			}, 2*time.Second).Should(Equal(0))
		})

		It("should not block node deletion when webhook fails", func() {
			// Close the webhook server to simulate failure
			webhookServer.Close()

			nodeClaim, node := coretest.NodeClaimAndNode(karpv1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						karpv1.NodePoolLabelKey: "default",
					},
				},
				Status: karpv1.NodeClaimStatus{
					ProviderID: fake.RandomProviderID(),
				},
			})

			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(ctx, env.Client, nodeClaim, node)

			ExpectSingletonReconciled(ctx, controllerWithWebhook)

			// Verify nodeClaim was still deleted despite webhook failure
			ExpectNotFound(ctx, env.Client, nodeClaim)
		})

		It("should filter events based on configuration", func() {
			// Create webhook provider that only listens for spot interruptions
			webhookProv, err := webhook.NewDefaultProvider(webhookServer.URL, "", "spot_interrupted")
			Expect(err).ToNot(HaveOccurred())

			testCloudProvider := cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
				env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider, awsEnv.InstanceTypeStore)

			filteredController := interruption.NewController(
				env.Client,
				testCloudProvider,
				&clock.FakeClock{},
				events.NewRecorder(&record.FakeRecorder{}),
				testSQSProvider,
				servicesqs.NewFromConfig(aws.Config{}),
				awscache.NewUnavailableOfferings(),
				webhookProv,
			)

			// Create two nodeclaims
			spotNodeClaim, spotNode := coretest.NodeClaimAndNode(karpv1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						karpv1.NodePoolLabelKey: "default",
					},
				},
				Status: karpv1.NodeClaimStatus{
					ProviderID: fake.RandomProviderID(),
				},
			})

			scheduledNodeClaim, scheduledNode := coretest.NodeClaimAndNode(karpv1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						karpv1.NodePoolLabelKey: "default",
					},
				},
				Status: karpv1.NodeClaimStatus{
					ProviderID: fake.RandomProviderID(),
				},
			})

			// Create messages for both
			ExpectMessagesCreated(
				spotInterruptionMessage(lo.Must(utils.ParseInstanceID(spotNodeClaim.Status.ProviderID))),
				scheduledChangeMessage(lo.Must(utils.ParseInstanceID(scheduledNodeClaim.Status.ProviderID))),
			)
			ExpectApplied(ctx, env.Client, spotNodeClaim, spotNode, scheduledNodeClaim, scheduledNode)

			ExpectSingletonReconciled(ctx, filteredController)

			// Wait for webhook to complete
			Eventually(func() int {
				payloadsMu.Lock()
				defer payloadsMu.Unlock()
				return len(receivedPayloads)
			}, 5*time.Second).Should(Equal(1))

			// Verify only spot interruption webhook was sent
			payloadsMu.Lock()
			defer payloadsMu.Unlock()
			Expect(receivedPayloads).To(HaveLen(1))
			Expect(receivedPayloads[0]["text"]).To(ContainSubstring("Spot Interruption"))
		})

		It("should include correct instance details in payload", func() {
			// Update options context to include cluster name
			testCtx := options.ToContext(ctx, &options.Options{
				ClusterName: "test-cluster",
			})

			nodeClaim, node := coretest.NodeClaimAndNode(karpv1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						karpv1.NodePoolLabelKey:        "test-pool",
						corev1.LabelInstanceTypeStable: "m5.xlarge",
						corev1.LabelTopologyZone:       "us-west-2b",
						karpv1.CapacityTypeLabelKey:    karpv1.CapacityTypeSpot,
					},
				},
				Status: karpv1.NodeClaimStatus{
					ProviderID: fake.RandomProviderID(),
				},
			})

			instanceID := lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))
			ExpectMessagesCreated(spotInterruptionMessage(instanceID))
			ExpectApplied(testCtx, env.Client, nodeClaim, node)

			ExpectSingletonReconciled(testCtx, controllerWithWebhook)

			// Wait for async webhook to complete
			Eventually(func() int {
				payloadsMu.Lock()
				defer payloadsMu.Unlock()
				return len(receivedPayloads)
			}, 5*time.Second).Should(Equal(1))

			payloadsMu.Lock()
			defer payloadsMu.Unlock()

			payload := receivedPayloads[0]

			// Verify the payload contains instance details in the fields
			blocks := payload["blocks"].([]interface{})
			secondBlock := blocks[1].(map[string]interface{})
			fields := secondBlock["fields"].([]interface{})

			// Convert fields to a map for easier checking
			fieldTexts := make([]string, 0)
			for _, field := range fields {
				fieldMap := field.(map[string]interface{})
				fieldTexts = append(fieldTexts, fieldMap["text"].(string))
			}

			// Check that fields contain expected values
			Expect(fieldTexts).To(ContainElement(ContainSubstring("m5.xlarge")))
			Expect(fieldTexts).To(ContainElement(ContainSubstring("us-west-2b")))
			Expect(fieldTexts).To(ContainElement(ContainSubstring("test-pool")))
			Expect(fieldTexts).To(ContainElement(ContainSubstring(karpv1.CapacityTypeSpot)))
			Expect(fieldTexts).To(ContainElement(ContainSubstring(instanceID)))
		})

		It("should skip webhook when cluster name not configured", func() {
			// Create context without cluster name (nil options)
			emptyCtx := options.ToContext(ctx, &options.Options{
				ClusterName: "",
			})

			nodeClaim, node := coretest.NodeClaimAndNode(karpv1.NodeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						karpv1.NodePoolLabelKey: "default",
					},
				},
				Status: karpv1.NodeClaimStatus{
					ProviderID: fake.RandomProviderID(),
				},
			})

			ExpectMessagesCreated(spotInterruptionMessage(lo.Must(utils.ParseInstanceID(nodeClaim.Status.ProviderID))))
			ExpectApplied(emptyCtx, env.Client, nodeClaim, node)

			ExpectSingletonReconciled(emptyCtx, controllerWithWebhook)

			// Verify no webhook sent (should be skipped due to missing cluster name)
			Consistently(func() int {
				payloadsMu.Lock()
				defer payloadsMu.Unlock()
				return len(receivedPayloads)
			}, 2*time.Second).Should(Equal(0))

			// Verify nodeClaim was still deleted despite webhook being skipped
			ExpectNotFound(emptyCtx, env.Client, nodeClaim)
		})
	})
})

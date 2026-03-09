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

package webhook_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter-provider-aws/pkg/providers/webhook"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Provider")
}

var _ = Describe("Webhook Provider", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("NewDefaultProvider", func() {
		It("should create provider with default Slack template", func() {
			provider, err := webhook.NewDefaultProvider("http://example.com/webhook", "", "all")
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).ToNot(BeNil())
		})

		It("should create provider with custom template", func() {
			customTemplate := `{"message": "{{.EventReason}}"}`
			provider, err := webhook.NewDefaultProvider("http://example.com/webhook", customTemplate, "all")
			Expect(err).ToNot(HaveOccurred())
			Expect(provider).ToNot(BeNil())
		})

		It("should fail with empty webhook URL", func() {
			_, err := webhook.NewDefaultProvider("", "", "all")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("webhook URL cannot be empty"))
		})

		It("should fail with invalid template", func() {
			invalidTemplate := `{{.Missing`
			_, err := webhook.NewDefaultProvider("http://example.com/webhook", invalidTemplate, "all")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse webhook template"))
		})

		It("should fail with unknown event type", func() {
			_, err := webhook.NewDefaultProvider("http://example.com/webhook", "", "unknown_event")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown webhook event type"))
		})

		It("should parse single event type", func() {
			provider, err := webhook.NewDefaultProvider("http://example.com/webhook", "", "spot_interrupted")
			Expect(err).ToNot(HaveOccurred())
			Expect(provider.ShouldNotify(messages.SpotInterruptionKind)).To(BeTrue())
			Expect(provider.ShouldNotify(messages.ScheduledChangeKind)).To(BeFalse())
		})

		It("should parse multiple event types", func() {
			provider, err := webhook.NewDefaultProvider("http://example.com/webhook", "", "spot_interrupted,instance_stopped")
			Expect(err).ToNot(HaveOccurred())
			Expect(provider.ShouldNotify(messages.SpotInterruptionKind)).To(BeTrue())
			Expect(provider.ShouldNotify(messages.InstanceStoppedKind)).To(BeTrue())
			Expect(provider.ShouldNotify(messages.ScheduledChangeKind)).To(BeFalse())
		})

		It("should enable all events with 'all' keyword", func() {
			provider, err := webhook.NewDefaultProvider("http://example.com/webhook", "", "all")
			Expect(err).ToNot(HaveOccurred())
			Expect(provider.ShouldNotify(messages.SpotInterruptionKind)).To(BeTrue())
			Expect(provider.ShouldNotify(messages.ScheduledChangeKind)).To(BeTrue())
			Expect(provider.ShouldNotify(messages.InstanceStoppedKind)).To(BeTrue())
			Expect(provider.ShouldNotify(messages.InstanceTerminatedKind)).To(BeTrue())
			Expect(provider.ShouldNotify(messages.RebalanceRecommendationKind)).To(BeTrue())
		})
	})

	Describe("SendNotification", func() {
		It("should send notification successfully", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal("POST"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

				body, err := io.ReadAll(r.Body)
				Expect(err).ToNot(HaveOccurred())

				var payload map[string]interface{}
				err = json.Unmarshal(body, &payload)
				Expect(err).ToNot(HaveOccurred())

				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider, err := webhook.NewDefaultProvider(server.URL, "", "all")
			Expect(err).ToNot(HaveOccurred())

			payload := &webhook.NotificationPayload{
				Timestamp:     time.Now(),
				ClusterName:   "test-cluster",
				EventType:     "spot_interrupted",
				EventReason:   "Spot Interruption",
				Message:       "Instance will be terminated in 2 minutes",
				NodeClaimName: "test-nodeclaim",
				NodeName:      "test-node",
				InstanceID:    "i-1234567890",
				InstanceType:  "m5.large",
				Zone:          "us-west-2a",
				NodePoolName:  "default",
				CapacityType:  "spot",
			}

			err = provider.SendNotification(ctx, payload)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should render custom template correctly", func() {
			var receivedPayload map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &receivedPayload)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			customTemplate := `{"cluster": "{{.ClusterName}}", "event": "{{.EventType}}"}`
			provider, err := webhook.NewDefaultProvider(server.URL, customTemplate, "all")
			Expect(err).ToNot(HaveOccurred())

			payload := &webhook.NotificationPayload{
				Timestamp:     time.Now(),
				ClusterName:   "test-cluster",
				EventType:     "spot_interrupted",
				EventReason:   "Spot Interruption",
				Message:       "Test message",
				NodeClaimName: "test-nodeclaim",
				InstanceID:    "i-1234567890",
				InstanceType:  "m5.large",
				Zone:          "us-west-2a",
				NodePoolName:  "default",
				CapacityType:  "spot",
			}

			err = provider.SendNotification(ctx, payload)
			Expect(err).ToNot(HaveOccurred())
			Expect(receivedPayload["cluster"]).To(Equal("test-cluster"))
			Expect(receivedPayload["event"]).To(Equal("spot_interrupted"))
		})

		It("should retry on failure and eventually succeed", func() {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := attempts.Add(1)
				if count < 3 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			provider, err := webhook.NewDefaultProvider(server.URL, "", "all")
			Expect(err).ToNot(HaveOccurred())

			payload := &webhook.NotificationPayload{
				Timestamp:     time.Now(),
				ClusterName:   "test-cluster",
				EventType:     "spot_interrupted",
				EventReason:   "Spot Interruption",
				Message:       "Test message",
				NodeClaimName: "test-nodeclaim",
				InstanceID:    "i-1234567890",
				InstanceType:  "m5.large",
				Zone:          "us-west-2a",
				NodePoolName:  "default",
				CapacityType:  "spot",
			}

			err = provider.SendNotification(ctx, payload)
			Expect(err).ToNot(HaveOccurred())
			Expect(attempts.Load()).To(Equal(int32(3)))
		})

		It("should fail after max retries", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			provider, err := webhook.NewDefaultProvider(server.URL, "", "all")
			Expect(err).ToNot(HaveOccurred())

			payload := &webhook.NotificationPayload{
				Timestamp:     time.Now(),
				ClusterName:   "test-cluster",
				EventType:     "spot_interrupted",
				EventReason:   "Spot Interruption",
				Message:       "Test message",
				NodeClaimName: "test-nodeclaim",
				InstanceID:    "i-1234567890",
				InstanceType:  "m5.large",
				Zone:          "us-west-2a",
				NodePoolName:  "default",
				CapacityType:  "spot",
			}

			err = provider.SendNotification(ctx, payload)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("webhook returned non-success status"))
		})

		It("should fail with invalid template rendering", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			// Template that doesn't produce valid JSON
			invalidTemplate := `{{.EventReason}}`
			provider, err := webhook.NewDefaultProvider(server.URL, invalidTemplate, "all")

			// With startup validation, this should fail at provider creation time
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("template does not produce valid JSON"))
			Expect(provider).To(BeNil())
		})
	})

	Describe("ShouldNotify", func() {
		It("should filter events correctly", func() {
			provider, err := webhook.NewDefaultProvider("http://example.com/webhook", "", "spot_interrupted,instance_stopped")
			Expect(err).ToNot(HaveOccurred())

			Expect(provider.ShouldNotify(messages.SpotInterruptionKind)).To(BeTrue())
			Expect(provider.ShouldNotify(messages.InstanceStoppedKind)).To(BeTrue())
			Expect(provider.ShouldNotify(messages.ScheduledChangeKind)).To(BeFalse())
			Expect(provider.ShouldNotify(messages.InstanceTerminatedKind)).To(BeFalse())
			Expect(provider.ShouldNotify(messages.RebalanceRecommendationKind)).To(BeFalse())
		})

		It("should not notify for NoOp kind", func() {
			provider, err := webhook.NewDefaultProvider("http://example.com/webhook", "", "all")
			Expect(err).ToNot(HaveOccurred())

			Expect(provider.ShouldNotify(messages.NoOpKind)).To(BeFalse())
		})
	})
})

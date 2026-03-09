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

package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
)

// Provider is the interface for sending webhook notifications
type Provider interface {
	SendNotification(ctx context.Context, payload *NotificationPayload) error
	ShouldNotify(messageKind messages.Kind) bool
}

// NotificationPayload contains the data for a webhook notification
type NotificationPayload struct {
	Timestamp     time.Time `json:"timestamp"`
	ClusterName   string    `json:"cluster_name"`
	EventType     string    `json:"event_type"`
	EventReason   string    `json:"event_reason"`
	Message       string    `json:"message"`
	NodeClaimName string    `json:"nodeclaim_name"`
	NodeName      string    `json:"node_name,omitempty"`
	InstanceID    string    `json:"instance_id"`
	InstanceType  string    `json:"instance_type"`
	Zone          string    `json:"zone"`
	NodePoolName  string    `json:"nodepool_name"`
	CapacityType  string    `json:"capacity_type"`
}

// DefaultProvider is the default implementation of Provider
type DefaultProvider struct {
	httpClient    *http.Client
	webhookURL    string
	template      *template.Template
	enabledEvents map[messages.Kind]bool
}

const defaultSlackTemplate = `{
  "text": "{{.EventReason}}: {{.Message}}",
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "*{{.EventReason}}*: {{.Message}}"
      }
    },
    {
      "type": "section",
      "fields": [
        {"type": "mrkdwn", "text": "*Cluster:*\n{{.ClusterName}}"},
        {"type": "mrkdwn", "text": "*NodeClaim:*\n{{.NodeClaimName}}"},
        {"type": "mrkdwn", "text": "*Instance:*\n{{.InstanceID}}"},
        {"type": "mrkdwn", "text": "*Type:*\n{{.InstanceType}}"},
        {"type": "mrkdwn", "text": "*Zone:*\n{{.Zone}}"},
        {"type": "mrkdwn", "text": "*NodePool:*\n{{.NodePoolName}}"},
        {"type": "mrkdwn", "text": "*Capacity:*\n{{.CapacityType}}"}
      ]
    }
  ]
}`

// NewDefaultProvider creates a new webhook provider
func NewDefaultProvider(webhookURL, templateStr, eventsList string) (Provider, error) {
	if webhookURL == "" {
		return nil, fmt.Errorf("webhook URL cannot be empty")
	}

	// Validate webhook URL
	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("webhook URL must use http or https scheme, got: %s", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("webhook URL must have a host")
	}

	// Use default Slack template if none provided
	if templateStr == "" {
		templateStr = defaultSlackTemplate
	}

	// Parse the template
	tmpl, err := template.New("webhook").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse webhook template: %w", err)
	}

	// Validate that template produces valid JSON by testing with dummy data
	testPayload := &NotificationPayload{
		Timestamp:     time.Now(),
		ClusterName:   "test",
		EventType:     "test",
		EventReason:   "test",
		Message:       "test",
		NodeClaimName: "test",
		NodeName:      "test",
		InstanceID:    "test",
		InstanceType:  "test",
		Zone:          "test",
		NodePoolName:  "test",
		CapacityType:  "test",
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, testPayload); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}
	var jsonCheck interface{}
	if err := json.Unmarshal(buf.Bytes(), &jsonCheck); err != nil {
		return nil, fmt.Errorf("template does not produce valid JSON: %w", err)
	}

	// Parse enabled events
	enabledEvents := make(map[messages.Kind]bool)
	events := strings.Split(strings.TrimSpace(eventsList), ",")
	for _, event := range events {
		event = strings.TrimSpace(event)
		switch event {
		case "all":
			enabledEvents[messages.SpotInterruptionKind] = true
			enabledEvents[messages.ScheduledChangeKind] = true
			enabledEvents[messages.InstanceStoppedKind] = true
			enabledEvents[messages.InstanceTerminatedKind] = true
			enabledEvents[messages.RebalanceRecommendationKind] = true
		case "spot_interrupted":
			enabledEvents[messages.SpotInterruptionKind] = true
		case "scheduled_change":
			enabledEvents[messages.ScheduledChangeKind] = true
		case "instance_stopped":
			enabledEvents[messages.InstanceStoppedKind] = true
		case "instance_terminated":
			enabledEvents[messages.InstanceTerminatedKind] = true
		case "rebalance_recommendation":
			enabledEvents[messages.RebalanceRecommendationKind] = true
		default:
			if event != "" {
				return nil, fmt.Errorf("unknown webhook event type: %s", event)
			}
		}
	}

	// Ensure at least one event was configured
	if len(enabledEvents) == 0 {
		return nil, fmt.Errorf("no valid webhook events specified, must include at least one of: spot_interrupted, scheduled_change, instance_stopped, instance_terminated, rebalance_recommendation, or all")
	}

	return &DefaultProvider{
		httpClient: &http.Client{
			// 10s timeout per HTTP request allows time for slow webhook endpoints
			// while preventing indefinite hangs. Controller timeout is 30s (3 attempts × 10s).
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				// MaxIdleConns limits total idle connections across all hosts
				// Set to 10 as interrupt controller uses semaphore to limit concurrent webhooks to 100
				MaxIdleConns: 10,
				// MaxIdleConnsPerHost limits reusable connections to single webhook URL
				// Set to 2 to allow connection reuse while preventing resource buildup
				MaxIdleConnsPerHost: 2,
				// IdleConnTimeout closes idle connections after 30s to prevent resource leaks
				IdleConnTimeout: 30 * time.Second,
				// TLSHandshakeTimeout prevents slow TLS negotiations from hanging
				TLSHandshakeTimeout: 5 * time.Second,
				// ExpectContinueTimeout for 100-continue responses
				ExpectContinueTimeout: 1 * time.Second,
				// Reuse connections for better performance
				DisableKeepAlives: false,
			},
		},
		webhookURL:    webhookURL,
		template:      tmpl,
		enabledEvents: enabledEvents,
	}, nil
}

// ShouldNotify returns true if the given message kind should trigger a notification
func (p *DefaultProvider) ShouldNotify(messageKind messages.Kind) bool {
	return p.enabledEvents[messageKind]
}

// SendNotification sends a notification to the configured webhook URL
func (p *DefaultProvider) SendNotification(ctx context.Context, payload *NotificationPayload) error {
	// Render the template
	var buf bytes.Buffer
	if err := p.template.Execute(&buf, payload); err != nil {
		return fmt.Errorf("failed to render webhook template: %w", err)
	}

	// Note: JSON validation is done at startup in NewDefaultProvider

	// Send the webhook with retries
	// Retry 3 times with exponential backoff (1s, 2s, 4s) to handle transient failures
	// Total retry time: 1+2+4 = 7 seconds plus HTTP request times (up to 3 × 10s = 30s)
	// This matches the controller's 30s webhook timeout
	var lastErr error
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; attempt < len(backoff); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff[attempt-1]):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", p.webhookURL, bytes.NewReader(buf.Bytes()))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send webhook (attempt %d/%d): %w", attempt+1, len(backoff), err)
			continue
		}

		// Read and close response body
		// Limit to 1MB to prevent DoS from malicious webhook endpoints returning large responses
		// Typical Slack/Teams responses are <1KB, so 1MB provides ample safety margin
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		// Include read error in error message if body read failed
		bodyStr := string(body)
		if readErr != nil {
			bodyStr = fmt.Sprintf("[failed to read response body: %v]", readErr)
		}

		lastErr = fmt.Errorf("webhook returned non-success status %d (attempt %d/%d): %s",
			resp.StatusCode, attempt+1, len(backoff), bodyStr)
	}

	return lastErr
}

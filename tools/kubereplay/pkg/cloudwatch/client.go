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

package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/parser"
)

// DefaultWindowSize is the time chunk per Logs Insights query.
// Logs Insights caps at 10,000 results per query; splitting the capture
// duration into smaller windows reduces the chance of hitting that cap on
// busy clusters.
const DefaultWindowSize = 15 * time.Minute

// pollInterval is how often GetQueryResults is polled while a query is running.
const pollInterval = 2 * time.Second

// logGroupClass represents the CloudWatch log group storage class.
type logGroupClass string

const (
	logGroupClassStandard         logGroupClass = "STANDARD"
	logGroupClassInfrequentAccess logGroupClass = "INFREQUENT_ACCESS"
	logGroupClassUnknown          logGroupClass = ""
)

// Client queries CloudWatch Logs for EKS audit events.
//
// It auto-detects the log group class on first use:
//   - STANDARD:          uses FilterLogEvents (lower latency, supports filter patterns)
//   - INFREQUENT_ACCESS: uses Logs Insights StartQuery/GetQueryResults
//                        (FilterLogEvents is not supported on INFREQUENT_ACCESS)
//
// Both code paths produce identical results. Users do not need to know or
// configure the log group class — detection is transparent.
type Client struct {
	api      *cloudwatchlogs.Client
	LogGroup string
	// WindowSize controls query chunk size for the Logs Insights path.
	// Only used when the log group is INFREQUENT_ACCESS.
	// Reduce to 5m if you see "hit 10,000 result cap" on busy clusters.
	WindowSize time.Duration
	// logClass is populated on first call to StreamEvents via detectLogGroupClass.
	logClass logGroupClass
}

// FetchOptions specifies the time range to capture.
type FetchOptions struct {
	StartTime time.Time
	EndTime   time.Time
}

// NewClient creates a CloudWatch Logs client for the given EKS cluster.
// The log group follows EKS convention: /aws/eks/<cluster>/cluster.
func NewClient(api *cloudwatchlogs.Client, clusterName string) *Client {
	return &Client{
		api:        api,
		LogGroup:   fmt.Sprintf("/aws/eks/%s/cluster", clusterName),
		WindowSize: DefaultWindowSize,
	}
}

// StreamEvents queries EKS audit logs for workload events (deployments and jobs).
// It auto-detects the log group class and uses the appropriate API:
//   - STANDARD:          FilterLogEvents
//   - INFREQUENT_ACCESS: StartQuery/GetQueryResults (Logs Insights)
func (c *Client) StreamEvents(ctx context.Context, opts FetchOptions) (<-chan *parser.AuditEvent, <-chan error) {
	eventCh := make(chan *parser.AuditEvent, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		// Detect log group class once before streaming.
		if err := c.detectLogGroupClass(ctx); err != nil {
			errCh <- err
			return
		}
		fmt.Printf("  log group class: %s → using %s\n", c.logClass, c.apiName())

		switch c.logClass {
		case logGroupClassStandard:
			c.streamViaFilterLogEvents(ctx, opts, eventCh, errCh)
		default:
			// INFREQUENT_ACCESS or any unknown future class — use Logs Insights.
			c.streamViaLogsInsights(ctx, opts, eventCh, errCh)
		}
	}()

	return eventCh, errCh
}

// apiName returns a human-readable name of the API being used.
func (c *Client) apiName() string {
	if c.logClass == logGroupClassStandard {
		return "FilterLogEvents"
	}
	return "Logs Insights (StartQuery/GetQueryResults)"
}

// detectLogGroupClass queries CloudWatch to determine the log group class.
// Result is cached in c.logClass so detection only happens once per client.
func (c *Client) detectLogGroupClass(ctx context.Context) error {
	if c.logClass != logGroupClassUnknown {
		return nil // already detected
	}

	out, err := c.api.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(c.LogGroup),
		Limit:              aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("DescribeLogGroups %s: %w", c.LogGroup, err)
	}

	for _, lg := range out.LogGroups {
		if aws.ToString(lg.LogGroupName) == c.LogGroup {
			if lg.LogGroupClass == types.LogGroupClassInfrequentAccess {
				c.logClass = logGroupClassInfrequentAccess
			} else {
				c.logClass = logGroupClassStandard
			}
			return nil
		}
	}

	// Log group not found in results — default to STANDARD so FilterLogEvents
	// is tried first; if it fails with an INFREQUENT_ACCESS error the caller
	// will see a clear error message.
	c.logClass = logGroupClassStandard
	return nil
}

// ── STANDARD path: FilterLogEvents ────────────────────────────────────────────

func (c *Client) streamViaFilterLogEvents(ctx context.Context, opts FetchOptions,
	eventCh chan<- *parser.AuditEvent, errCh chan<- error) {

	// Filter for deployments (create, update, patch, delete) and jobs (create, update, patch).
	// deletecollection is not captured — it doesn't include individual resource names.
	filterPattern := `{ ($.objectRef.resource = "deployments" && ($.verb = "create" || $.verb = "update" || $.verb = "patch" || $.verb = "delete")) || ($.objectRef.resource = "jobs" && ($.verb = "create" || $.verb = "update" || $.verb = "patch")) }`

	var nextToken *string
	for {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
		}

		output, err := c.api.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:        aws.String(c.LogGroup),
			StartTime:           aws.Int64(opts.StartTime.UnixMilli()),
			EndTime:             aws.Int64(opts.EndTime.UnixMilli()),
			FilterPattern:       aws.String(filterPattern),
			Limit:               aws.Int32(10000),
			NextToken:           nextToken,
			LogStreamNamePrefix: aws.String("kube-apiserver-audit"),
		})
		if err != nil {
			errCh <- fmt.Errorf("FilterLogEvents: %w", err)
			return
		}

		for _, event := range output.Events {
			if event.Message == nil {
				continue
			}
			var auditEvent parser.AuditEvent
			if err := json.Unmarshal([]byte(*event.Message), &auditEvent); err != nil {
				continue
			}
			select {
			case eventCh <- &auditEvent:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		if output.NextToken == nil {
			return
		}
		nextToken = output.NextToken
	}
}

// ── INFREQUENT_ACCESS path: Logs Insights ─────────────────────────────────────

func (c *Client) streamViaLogsInsights(ctx context.Context, opts FetchOptions,
	eventCh chan<- *parser.AuditEvent, errCh chan<- error) {

	windows := timeWindows(opts.StartTime, opts.EndTime, c.WindowSize)
	for i, w := range windows {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
		}

		fmt.Printf("  [%d/%d] querying %s → %s\n",
			i+1, len(windows),
			w[0].Format("15:04:05"),
			w[1].Format("15:04:05"),
		)

		events, err := c.queryWindowInsights(ctx, w[0], w[1])
		if err != nil {
			errCh <- err
			return
		}

		for _, event := range events {
			select {
			case eventCh <- event:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}
	}
}

// queryWindowInsights runs a single Logs Insights query over [start, end).
// Equivalent to the FilterLogEvents pattern but works on INFREQUENT_ACCESS.
func (c *Client) queryWindowInsights(ctx context.Context, start, end time.Time) ([]*parser.AuditEvent, error) {
	// Semantically equivalent to the FilterLogEvents filterPattern above.
	// deletecollection is not captured — it doesn't include individual resource names.
	query := `fields @timestamp, @message
| filter @logStream like /kube-apiserver-audit/
| filter objectRef.resource in ["deployments", "jobs"]
| filter verb in ["create", "update", "patch", "delete"]
| filter ispresent(objectRef.name)
| sort @timestamp asc
| limit 10000`

	startResp, err := c.api.StartQuery(ctx, &cloudwatchlogs.StartQueryInput{
		LogGroupName: aws.String(c.LogGroup),
		StartTime:    aws.Int64(start.Unix()),
		EndTime:      aws.Int64(end.Unix()),
		QueryString:  aws.String(query),
		Limit:        aws.Int32(10000),
	})
	if err != nil {
		return nil, fmt.Errorf("StartQuery [%s–%s]: %w",
			start.Format(time.RFC3339), end.Format(time.RFC3339), err)
	}

	queryID := startResp.QueryId

	for {
		select {
		case <-ctx.Done():
			_, _ = c.api.StopQuery(context.Background(), &cloudwatchlogs.StopQueryInput{
				QueryId: queryID,
			})
			return nil, ctx.Err()
		default:
		}

		result, err := c.api.GetQueryResults(ctx, &cloudwatchlogs.GetQueryResultsInput{
			QueryId: queryID,
		})
		if err != nil {
			return nil, fmt.Errorf("GetQueryResults: %w", err)
		}

		switch result.Status {
		case types.QueryStatusRunning, types.QueryStatusScheduled:
			time.Sleep(pollInterval)
			continue
		case types.QueryStatusFailed:
			return nil, fmt.Errorf("query %s failed", aws.ToString(queryID))
		case types.QueryStatusCancelled:
			return nil, fmt.Errorf("query %s was cancelled", aws.ToString(queryID))
		case types.QueryStatusTimeout:
			return nil, fmt.Errorf("query %s timed out — reduce --window size", aws.ToString(queryID))
		}

		// Complete.
		if len(result.Results) == 10000 {
			fmt.Printf("  WARNING: hit 10,000 result cap for window %s→%s — "+
				"some events may be missing. Use a smaller --window value.\n",
				start.Format("15:04:05"), end.Format("15:04:05"))
		}

		var events []*parser.AuditEvent
		for _, row := range result.Results {
			msg := rowField(row, "@message")
			if msg == "" {
				continue
			}
			var auditEvent parser.AuditEvent
			if err := json.Unmarshal([]byte(msg), &auditEvent); err != nil {
				continue
			}
			events = append(events, &auditEvent)
		}
		return events, nil
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────

// timeWindows splits [start, end) into chunks of at most windowSize.
func timeWindows(start, end time.Time, windowSize time.Duration) [][2]time.Time {
	var windows [][2]time.Time
	for t := start; t.Before(end); t = t.Add(windowSize) {
		wEnd := t.Add(windowSize)
		if wEnd.After(end) {
			wEnd = end
		}
		windows = append(windows, [2]time.Time{t, wEnd})
	}
	return windows
}

// rowField extracts a named field value from a Logs Insights result row.
func rowField(row []types.ResultField, name string) string {
	for _, f := range row {
		if aws.ToString(f.Field) == name {
			return aws.ToString(f.Value)
		}
	}
	return ""
}

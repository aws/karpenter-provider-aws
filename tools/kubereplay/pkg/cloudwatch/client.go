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

// pollInterval is how often GetQueryResults is polled while a query runs.
const pollInterval = 2 * time.Second

// Client queries CloudWatch Logs using Logs Insights (StartQuery/GetQueryResults).
//
// This replaces the previous FilterLogEvents-based implementation.
// FilterLogEvents only works on STANDARD log groups and returns an error
// on INFREQUENT_ACCESS log groups:
//
//	InvalidOperationException: FilterLogEvents is not supported for
//	log-group-class INFREQUENT_ACCESS
//
// CloudWatch Logs Insights (StartQuery/GetQueryResults) works on both
// STANDARD and INFREQUENT_ACCESS log group classes, making kubereplay
// compatible with all EKS audit log configurations.
type Client struct {
	api        *cloudwatchlogs.Client
	LogGroup   string
	// WindowSize controls how the capture duration is split into Logs Insights
	// queries. Each query returns at most 10,000 results; reduce WindowSize
	// (e.g. to 5m) if you see the "hit 10,000 result cap" warning.
	WindowSize time.Duration
}

// FetchOptions specifies the time range to capture.
type FetchOptions struct {
	StartTime time.Time
	EndTime   time.Time
}

// NewClient creates a CloudWatch Logs client for the given EKS cluster.
// The log group name follows the EKS convention: /aws/eks/<cluster>/cluster.
func NewClient(api *cloudwatchlogs.Client, clusterName string) *Client {
	return &Client{
		api:        api,
		LogGroup:   fmt.Sprintf("/aws/eks/%s/cluster", clusterName),
		WindowSize: DefaultWindowSize,
	}
}

// StreamEvents queries EKS audit logs for workload events (deployments and jobs)
// and emits them over a channel. The capture duration is split into WindowSize
// chunks to stay under the 10,000 results-per-query Logs Insights limit.
func (c *Client) StreamEvents(ctx context.Context, opts FetchOptions) (<-chan *parser.AuditEvent, <-chan error) {
	eventCh := make(chan *parser.AuditEvent, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

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

			events, err := c.queryWindow(ctx, w[0], w[1])
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
	}()

	return eventCh, errCh
}

// queryWindow runs a single Logs Insights query over the [start, end) window
// and returns all matching audit events.
// Logs Insights works on both STANDARD and INFREQUENT_ACCESS log group classes.
// The query is semantically equivalent to the previous FilterLogEvents pattern.
func (c *Client) queryWindow(ctx context.Context, start, end time.Time) ([]*parser.AuditEvent, error) {
	// This query is equivalent to the FilterLogEvents pattern used previously,
	// but expressed in Logs Insights QL which works on INFREQUENT_ACCESS log groups.
	// Note: deletecollection (bulk delete) is not captured — it does not include
	// individual resource names in the audit log entry.
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
			// Best-effort cancel the in-flight query before returning.
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

		// Status is Complete.
		if len(result.Results) == 10000 {
			fmt.Printf("  WARNING: hit 10,000 result cap for window %s→%s — some events may be missing. "+
				"Use a smaller --window value.\n", start.Format("15:04:05"), end.Format("15:04:05"))
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

// rowField extracts the value of a named field from a Logs Insights result row.
func rowField(row []types.ResultField, name string) string {
	for _, f := range row {
		if aws.ToString(f.Field) == name {
			return aws.ToString(f.Value)
		}
	}
	return ""
}

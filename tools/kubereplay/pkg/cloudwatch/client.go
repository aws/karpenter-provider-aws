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

	"github.com/aws/karpenter-provider-aws/tools/kubereplay/pkg/parser"
)

type Client struct {
	api      cloudwatchlogs.FilterLogEventsAPIClient
	LogGroup string
}

type FetchOptions struct {
	StartTime time.Time
	EndTime   time.Time
}

func NewClient(api cloudwatchlogs.FilterLogEventsAPIClient, clusterName string) *Client {
	return &Client{
		api:      api,
		LogGroup: fmt.Sprintf("/aws/eks/%s/cluster", clusterName),
	}
}

func (c *Client) StreamEvents(ctx context.Context, opts FetchOptions) (<-chan *parser.AuditEvent, <-chan error) {
	eventCh := make(chan *parser.AuditEvent, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		var nextToken *string
		for {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			// Filter for deployments (create, update, patch, delete) and jobs (create, update, patch)
			// This captures workload intent rather than individual pods
			// Note: deletecollection (bulk delete) is not captured - it doesn't include individual resource names
			filterPattern := `{ ($.objectRef.resource = "deployments" && ($.verb = "create" || $.verb = "update" || $.verb = "patch" || $.verb = "delete")) || ($.objectRef.resource = "jobs" && ($.verb = "create" || $.verb = "update" || $.verb = "patch")) }`
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
				errCh <- fmt.Errorf("failed to filter log events: %w", err)
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
	}()

	return eventCh, errCh
}

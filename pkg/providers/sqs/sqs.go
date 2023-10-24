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

package sqs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"

	"github.com/aws/karpenter/pkg/operator/options"
)

type Provider struct {
	client sqsiface.SQSAPI

	name string
	url  string
}

func NewProvider(ctx context.Context, client sqsiface.SQSAPI) (*Provider, error) {
	ret, err := client.GetQueueUrlWithContext(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(options.FromContext(ctx).InterruptionQueue),
	})
	if err != nil {
		return nil, fmt.Errorf("fetching queue url, %w", err)
	}
	return &Provider{
		client: client,
		name:   options.FromContext(ctx).InterruptionQueue,
		url:    aws.StringValue(ret.QueueUrl),
	}, nil
}

func (s *Provider) GetSQSMessages(ctx context.Context) ([]*sqs.Message, error) {
	input := &sqs.ReceiveMessageInput{
		MaxNumberOfMessages: aws.Int64(10),
		VisibilityTimeout:   aws.Int64(20), // Seconds
		WaitTimeSeconds:     aws.Int64(20), // Seconds, maximum for long polling
		AttributeNames: []*string{
			aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
		},
		MessageAttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
		QueueUrl: aws.String(s.url),
	}

	result, err := s.client.ReceiveMessageWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("receiving sqs messages, %w", err)
	}

	return result.Messages, nil
}

func (s *Provider) SendMessage(ctx context.Context, body interface{}) (string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshaling the passed body as json, %w", err)
	}
	input := &sqs.SendMessageInput{
		MessageBody: aws.String(string(raw)),
		QueueUrl:    aws.String(s.url),
	}
	result, err := s.client.SendMessageWithContext(ctx, input)
	if err != nil {
		return "", fmt.Errorf("sending messages to sqs queue, %w", err)
	}
	return aws.StringValue(result.MessageId), nil
}

func (s *Provider) DeleteSQSMessage(ctx context.Context, msg *sqs.Message) error {
	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(s.url),
		ReceiptHandle: msg.ReceiptHandle,
	}

	if _, err := s.client.DeleteMessageWithContext(ctx, input); err != nil {
		return fmt.Errorf("deleting messages from sqs queue, %w", err)
	}
	return nil
}

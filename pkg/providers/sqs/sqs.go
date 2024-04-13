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
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

type Provider interface {
	Name() string
	GetSQSMessages(context.Context) ([]*sqs.Message, error)
	SendMessage(context.Context, interface{}) (string, error)
	DeleteSQSMessage(context.Context, *sqs.Message) error
}

type DefaultProvider struct {
	client sqsiface.SQSAPI

	queueURL string
}

func NewDefaultProvider(client sqsiface.SQSAPI, queueURL string) (*DefaultProvider, error) {
	return &DefaultProvider{
		client:   client,
		queueURL: queueURL,
	}, nil
}

func (p *DefaultProvider) Name() string {
	ss := strings.Split(p.queueURL, "/")
	return ss[len(ss)-1]
}

func (p *DefaultProvider) GetSQSMessages(ctx context.Context) ([]*sqs.Message, error) {
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
		QueueUrl: aws.String(p.queueURL),
	}

	result, err := p.client.ReceiveMessageWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("receiving sqs messages, %w", err)
	}

	return result.Messages, nil
}

func (p *DefaultProvider) SendMessage(ctx context.Context, body interface{}) (string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshaling the passed body as json, %w", err)
	}
	input := &sqs.SendMessageInput{
		MessageBody: aws.String(string(raw)),
		QueueUrl:    aws.String(p.queueURL),
	}
	result, err := p.client.SendMessageWithContext(ctx, input)
	if err != nil {
		return "", fmt.Errorf("sending messages to sqs queue, %w", err)
	}
	return aws.StringValue(result.MessageId), nil
}

func (p *DefaultProvider) DeleteSQSMessage(ctx context.Context, msg *sqs.Message) error {
	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	}

	if _, err := p.client.DeleteMessageWithContext(ctx, input); err != nil {
		return fmt.Errorf("deleting messages from sqs queue, %w", err)
	}
	return nil
}

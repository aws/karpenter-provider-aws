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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/samber/lo"
)

type Provider interface {
	Name() string
	GetSQSMessages(context.Context) ([]*sqstypes.Message, error)
	SendMessage(context.Context, interface{}) (string, error)
	DeleteSQSMessage(context.Context, *sqstypes.Message) error
}

type SQSAPI interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// DefaultProvider is a default implementation of the Provider interface
// for interacting with AWS SQS queues.
type DefaultProvider struct {
	client SQSAPI

	queueURL string
}

func NewDefaultProvider(client SQSAPI, queueURL string) (*DefaultProvider, error) {
	return &DefaultProvider{
		client:   client,
		queueURL: queueURL,
	}, nil
}

func (p *DefaultProvider) Name() string {
	ss := strings.Split(p.queueURL, "/")
	return ss[len(ss)-1]
}

func (p *DefaultProvider) GetSQSMessages(ctx context.Context) ([]*sqstypes.Message, error) {
	input := &sqs.ReceiveMessageInput{
		MaxNumberOfMessages: 10,
		VisibilityTimeout:   20, // Seconds
		WaitTimeSeconds:     20, // Seconds, maximum for long polling
		AttributeNames: []sqstypes.QueueAttributeName{
			"SentTimestamp",
		},
		MessageAttributeNames: []string{
			"All",
		},
		QueueUrl: aws.String(p.queueURL),
	}

	result, err := p.client.ReceiveMessage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("receiving sqs messages, %w", err)
	}

	return lo.ToSlicePtr(result.Messages), nil
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
	result, err := p.client.SendMessage(ctx, input)
	if err != nil {
		return "", fmt.Errorf("sending messages to sqs queue, %w", err)
	}
	return aws.ToString(result.MessageId), nil
}

func (p *DefaultProvider) DeleteSQSMessage(ctx context.Context, msg *sqstypes.Message) error {
	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(p.queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	}

	if _, err := p.client.DeleteMessage(ctx, input); err != nil {
		return fmt.Errorf("deleting messages from sqs queue, %w", err)
	}
	return nil
}

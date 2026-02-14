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

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
)

type Provider interface {
	Name() string
	GetSQSMessages(context.Context) ([]*sqstypes.Message, error)
	SendMessage(context.Context, any) (string, error)
	DeleteSQSMessage(context.Context, *sqstypes.Message) error
}

type DefaultProvider struct {
	client sdk.SQSAPI

	queueURL string
}

func NewDefaultProvider(client sdk.SQSAPI, queueURL string) (*DefaultProvider, error) {
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
		MaxNumberOfMessages: int32(10),
		VisibilityTimeout:   int32(20), // Seconds
		WaitTimeSeconds:     int32(20), // Seconds, maximum for long polling
		AttributeNames: []sqstypes.QueueAttributeName{
			sqstypes.QueueAttributeName(sqstypes.MessageSystemAttributeNameSentTimestamp),
		},
		MessageAttributeNames: []string{
			string(sqstypes.QueueAttributeNameAll),
		},
		QueueUrl: aws.String(p.queueURL),
	}

	result, err := p.client.ReceiveMessage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("receiving sqs messages, %w", err)
	}

	return lo.ToSlicePtr(result.Messages), nil
}

func (p *DefaultProvider) SendMessage(ctx context.Context, body any) (string, error) {
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

func NewSQSProvider(ctx context.Context, sqsapi sdk.SQSAPI) (Provider, error) {
	out, err := sqsapi.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: lo.ToPtr(options.FromContext(ctx).InterruptionQueue)})
	if err != nil {
		return nil, err
	}
	return NewDefaultProvider(sqsapi, lo.FromPtr(out.QueueUrl))
}

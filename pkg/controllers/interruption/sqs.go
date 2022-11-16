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

package interruption

import (
	"context"
	"encoding/json"
	"fmt"
	syncatomic "sync/atomic"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/samber/lo"

	"github.com/aws/karpenter-core/pkg/utils/atomic"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	awserrors "github.com/aws/karpenter/pkg/errors"
)

type SQSProvider struct {
	client sqsiface.SQSAPI

	queueURL  atomic.Lazy[string]
	queueName syncatomic.Pointer[string]
}

func NewSQSProvider(client sqsiface.SQSAPI) *SQSProvider {
	provider := &SQSProvider{
		client: client,
	}
	provider.queueURL.Resolve = func(ctx context.Context) (string, error) {
		input := &sqs.GetQueueUrlInput{
			QueueName: aws.String(settings.FromContext(ctx).InterruptionQueueName),
		}
		ret, err := provider.client.GetQueueUrlWithContext(ctx, input)
		if err != nil {
			return "", fmt.Errorf("fetching queue url, %w", err)
		}
		return aws.StringValue(ret.QueueUrl), nil
	}
	return provider
}

func (s *SQSProvider) QueueExists(ctx context.Context) (bool, error) {
	_, err := s.queueURL.TryGet(ctx, atomic.IgnoreCacheOption)
	if err != nil {
		if awserrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *SQSProvider) DiscoverQueueURL(ctx context.Context) (string, error) {
	if settings.FromContext(ctx).InterruptionQueueName != lo.FromPtr(s.queueName.Load()) {
		res, err := s.queueURL.TryGet(ctx, atomic.IgnoreCacheOption)
		if err != nil {
			return res, err
		}
		s.queueName.Store(lo.ToPtr(settings.FromContext(ctx).InterruptionQueueName))
		return res, nil
	}
	return s.queueURL.TryGet(ctx)
}

func (s *SQSProvider) GetSQSMessages(ctx context.Context) ([]*sqs.Message, error) {
	queueURL, err := s.DiscoverQueueURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering queue url, %w", err)
	}

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
		QueueUrl: aws.String(queueURL),
	}

	result, err := s.client.ReceiveMessageWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("receiving sqs messages, %w", err)
	}

	return result.Messages, nil
}

func (s *SQSProvider) SendMessage(ctx context.Context, body interface{}) (string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshaling the passed body as json, %w", err)
	}
	queueURL, err := s.DiscoverQueueURL(ctx)
	if err != nil {
		return "", fmt.Errorf("fetching queue url, %w", err)
	}
	input := &sqs.SendMessageInput{
		MessageBody: aws.String(string(raw)),
		QueueUrl:    aws.String(queueURL),
	}
	result, err := s.client.SendMessage(input)
	if err != nil {
		return "", fmt.Errorf("sending messages to sqs queue, %w", err)
	}
	return aws.StringValue(result.MessageId), nil
}

func (s *SQSProvider) DeleteSQSMessage(ctx context.Context, msg *sqs.Message) error {
	queueURL, err := s.DiscoverQueueURL(ctx)
	if err != nil {
		return fmt.Errorf("failed fetching queue url, %w", err)
	}

	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	}

	_, err = s.client.DeleteMessageWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("deleting messages from sqs queue, %w", err)
	}
	return nil
}

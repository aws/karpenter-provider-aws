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

package aws

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"knative.dev/pkg/logging"
)

type SQSProvider struct {
	createQueueInput    *sqs.CreateQueueInput
	getQueueURLInput    *sqs.GetQueueUrlInput
	receiveMessageInput *sqs.ReceiveMessageInput
	client              sqsiface.SQSAPI
	mutex               *sync.RWMutex
	queueURL            string
}

func NewSQSProvider(client sqsiface.SQSAPI, queueName string) *SQSProvider {
	createQueueInput := &sqs.CreateQueueInput{
		Attributes: map[string]*string{
			sqs.QueueAttributeNameMessageRetentionPeriod: aws.String("300"),
		},
		QueueName: aws.String(queueName),
	}
	getQueueURLInput := &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	}
	receiveMessageInput := &sqs.ReceiveMessageInput{
		MaxNumberOfMessages: aws.Int64(10),
		VisibilityTimeout:   aws.Int64(20), // Seconds
		WaitTimeSeconds:     aws.Int64(20), // Seconds, maximum for long polling
		AttributeNames: []*string{
			aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
		},
		MessageAttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
	}

	return &SQSProvider{
		createQueueInput:    createQueueInput,
		getQueueURLInput:    getQueueURLInput,
		receiveMessageInput: receiveMessageInput,
		client:              client,
		mutex:               &sync.RWMutex{},
	}
}

func (s *SQSProvider) DiscoverQueueURL(ctx context.Context) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	if s.queueURL != "" {
		return s.queueURL, nil
	}
	result, err := s.client.GetQueueUrlWithContext(ctx, s.getQueueURLInput)
	if err != nil {
		return "", fmt.Errorf("failed fetching queue url, %w", err)
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.queueURL = aws.StringValue(result.QueueUrl)
	return aws.StringValue(result.QueueUrl), nil
}

func (s *SQSProvider) GetSQSMessages(ctx context.Context) ([]*sqs.Message, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("sqsClient.getMessages"))

	queueURL, err := s.DiscoverQueueURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed getting sqs messages, %w", err)
	}

	// Copy the input template and add the discovered queue url
	input, err := deepCopy(s.receiveMessageInput)
	if err != nil {
		return nil, fmt.Errorf("error copying input, %w", err)
	}
	input.QueueUrl = aws.String(queueURL)

	result, err := s.client.ReceiveMessageWithContext(ctx, input)
	if err != nil {
		logging.FromContext(ctx).
			With("error", err).
			Error("failed to fetch messages")
		return nil, err
	}

	return result.Messages, nil
}

func (s *SQSProvider) DeleteSQSMessage(ctx context.Context, msg *sqs.Message) error {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("sqsClient.deleteMessage"))

	queueURL, err := s.DiscoverQueueURL(ctx)
	if err != nil {
		return fmt.Errorf("failed getting sqs messages, %w", err)
	}

	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	}

	_, err = s.client.DeleteMessageWithContext(ctx, input)
	if err != nil {
		logging.FromContext(ctx).
			With("error", err).
			Error("failed to delete message")
		return err
	}

	return nil
}

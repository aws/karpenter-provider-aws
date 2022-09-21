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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/samber/lo"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
)

type SQSClient interface {
	CreateQueueWithContext(context.Context, *sqs.CreateQueueInput, ...request.Option) (*sqs.CreateQueueOutput, error)
	GetQueueUrlWithContext(context.Context, *sqs.GetQueueUrlInput, ...request.Option) (*sqs.GetQueueUrlOutput, error)
	SetQueueAttributesWithContext(context.Context, *sqs.SetQueueAttributesInput, ...request.Option) (*sqs.SetQueueAttributesOutput, error)
	ReceiveMessageWithContext(context.Context, *sqs.ReceiveMessageInput, ...request.Option) (*sqs.ReceiveMessageOutput, error)
	DeleteMessageWithContext(context.Context, *sqs.DeleteMessageInput, ...request.Option) (*sqs.DeleteMessageOutput, error)
	DeleteQueueWithContext(context.Context, *sqs.DeleteQueueInput, ...request.Option) (*sqs.DeleteQueueOutput, error)
}

type QueuePolicy struct {
	Version   string                 `json:"Version"`
	ID        string                 `json:"Id"`
	Statement []QueuePolicyStatement `json:"Statement"`
}

type QueuePolicyStatement struct {
	Effect    string    `json:"Effect"`
	Principal Principal `json:"Principal"`
	Action    []string  `json:"Action"`
	Resource  string    `json:"Resource"`
}

type Principal struct {
	Service []string `json:"Service"`
}

type SQSProvider struct {
	client SQSClient

	createQueueInput    *sqs.CreateQueueInput
	getQueueURLInput    *sqs.GetQueueUrlInput
	receiveMessageInput *sqs.ReceiveMessageInput
	mutex               *sync.RWMutex
	queueURL            string
	queueName           string
	metadata            *Metadata
}

func NewProvider(ctx context.Context, client SQSClient, metadata *Metadata) *SQSProvider {
	provider := &SQSProvider{
		client:    client,
		mutex:     &sync.RWMutex{},
		metadata:  metadata,
		queueName: getQueueName(ctx),
	}
	provider.createQueueInput = &sqs.CreateQueueInput{
		Attributes: provider.getQueueAttributes(),
		QueueName:  aws.String(provider.queueName),
		Tags: map[string]*string{
			v1alpha5.DiscoveryLabelKey: aws.String(injection.GetOptions(ctx).ClusterName),
		},
	}
	provider.getQueueURLInput = &sqs.GetQueueUrlInput{
		QueueName: aws.String(provider.queueName),
	}
	provider.receiveMessageInput = &sqs.ReceiveMessageInput{
		MaxNumberOfMessages: aws.Int64(10),
		VisibilityTimeout:   aws.Int64(10), // Seconds
		WaitTimeSeconds:     aws.Int64(10), // Seconds, maximum for long polling
		AttributeNames: []*string{
			aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
		},
		MessageAttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
	}
	return provider
}

func (s *SQSProvider) QueueName() string {
	return s.queueName
}

func (s *SQSProvider) CreateQueue(ctx context.Context) error {
	result, err := s.client.CreateQueueWithContext(ctx, s.createQueueInput)
	if err != nil {
		return fmt.Errorf("failed creating sqs queue, %w", err)
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.queueURL = aws.StringValue(result.QueueUrl)
	return nil
}

func (s *SQSProvider) SetQueueAttributes(ctx context.Context) error {
	queueURL, err := s.DiscoverQueueURL(ctx, false)
	if err != nil {
		return fmt.Errorf("failed fetching queue url, %w", err)
	}

	setQueueAttributesInput := &sqs.SetQueueAttributesInput{
		Attributes: s.getQueueAttributes(),
		QueueUrl:   aws.String(queueURL),
	}
	_, err = s.client.SetQueueAttributesWithContext(ctx, setQueueAttributesInput)
	if err != nil {
		return fmt.Errorf("failed setting queue attributes, %w", err)
	}
	return nil
}

func (s *SQSProvider) DiscoverQueueURL(ctx context.Context, ignoreCache bool) (string, error) {
	s.mutex.RLock()
	queueURL := s.queueURL
	s.mutex.RUnlock()
	if queueURL != "" && !ignoreCache {
		return queueURL, nil
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// We have to check if the queueUrl is set again here in case multiple threads make it past the read-locked section
	if s.queueURL != "" && !ignoreCache {
		return s.queueURL, nil
	}
	result, err := s.client.GetQueueUrlWithContext(ctx, s.getQueueURLInput)
	if err != nil {
		return "", fmt.Errorf("failed fetching queue url, %w", err)
	}
	s.queueURL = aws.StringValue(result.QueueUrl)
	return aws.StringValue(result.QueueUrl), nil
}

func (s *SQSProvider) GetSQSMessages(ctx context.Context) ([]*sqs.Message, error) {
	queueURL, err := s.DiscoverQueueURL(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed fetching queue url, %w", err)
	}

	// Copy the input template and add the discovered queue url
	input, err := functional.DeepCopy(s.receiveMessageInput)
	if err != nil {
		return nil, fmt.Errorf("failed copying input, %w", err)
	}
	input.QueueUrl = aws.String(queueURL)

	result, err := s.client.ReceiveMessageWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed receiving sqs messages, %w", err)
	}

	return result.Messages, nil
}

func (s *SQSProvider) DeleteSQSMessage(ctx context.Context, msg *sqs.Message) error {
	queueURL, err := s.DiscoverQueueURL(ctx, false)
	if err != nil {
		return fmt.Errorf("failed fetching queue url, %w", err)
	}

	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queueURL),
		ReceiptHandle: msg.ReceiptHandle,
	}

	_, err = s.client.DeleteMessageWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed deleting messages from sqs queue, %w", err)
	}
	return nil
}

func (s *SQSProvider) DeleteQueue(ctx context.Context) error {
	queueURL, err := s.DiscoverQueueURL(ctx, false)
	if err != nil {
		return fmt.Errorf("failed fetching queue url, %w", err)
	}

	input := &sqs.DeleteQueueInput{
		QueueUrl: aws.String(queueURL),
	}
	_, err = s.client.DeleteQueueWithContext(ctx, input)
	if err != nil {
		logging.FromContext(ctx).Errorf("Might have got an error here in the queue, %v", err)
		return fmt.Errorf("failed deleting sqs queue, %w", err)
	}
	return nil
}

func (s *SQSProvider) getQueueAttributes() map[string]*string {
	policy := lo.Must(json.Marshal(s.getQueuePolicy()))
	return map[string]*string{
		sqs.QueueAttributeNameMessageRetentionPeriod: aws.String("300"),
		sqs.QueueAttributeNamePolicy:                 aws.String(string(policy)),
	}
}

func (s *SQSProvider) getQueuePolicy() *QueuePolicy {
	return &QueuePolicy{
		Version: "2008-10-17",
		ID:      "EC2NotificationPolicy",
		Statement: []QueuePolicyStatement{
			{
				Effect: "Allow",
				Principal: Principal{
					Service: []string{
						"events.amazonaws.com",
						"sqs.amazonaws.com",
					},
				},
				Action:   []string{"sqs:SendMessage"},
				Resource: s.getQueueARN(),
			},
		},
	}
}

func (s *SQSProvider) getQueueARN() string {
	return fmt.Sprintf("arn:aws:sqs:%s:%s:%s", s.metadata.Region(), s.metadata.AccountID(), s.queueName)
}

func getQueueName(ctx context.Context) string {
	return fmt.Sprintf("Karpenter-%s-Queue", injection.GetOptions(ctx).ClusterName)
}

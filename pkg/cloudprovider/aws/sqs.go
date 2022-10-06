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
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/samber/lo"

	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/utils/cache"
	"github.com/aws/karpenter/pkg/utils/functional"
	"github.com/aws/karpenter/pkg/utils/injection"
)

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
	client sqsiface.SQSAPI

	createQueueInput    *sqs.CreateQueueInput
	getQueueURLInput    *sqs.GetQueueUrlInput
	receiveMessageInput *sqs.ReceiveMessageInput
	mu                  sync.RWMutex
	queueURL            *string
	queueName           string
	metadataProvider    *MetadataProvider
}

func NewSQSProvider(ctx context.Context, client sqsiface.SQSAPI, metadataProvider *MetadataProvider) *SQSProvider {
	provider := &SQSProvider{
		client:           client,
		mu:               sync.RWMutex{},
		metadataProvider: metadataProvider,
		queueName:        getQueueName(ctx),
	}
	provider.createQueueInput = &sqs.CreateQueueInput{
		Attributes: provider.getQueueAttributes(ctx),
		QueueName:  aws.String(provider.queueName),
		Tags: map[string]*string{
			awsv1alpha1.DiscoveryTagKey: aws.String(injection.GetOptions(ctx).ClusterName),
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
		return fmt.Errorf("creating sqs queue, %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queueURL = result.QueueUrl
	return nil
}

func (s *SQSProvider) SetQueueAttributes(ctx context.Context) error {
	queueURL, err := s.DiscoverQueueURL(ctx, false)
	if err != nil {
		return fmt.Errorf("fetching queue url, %w", err)
	}

	setQueueAttributesInput := &sqs.SetQueueAttributesInput{
		Attributes: s.getQueueAttributes(ctx),
		QueueUrl:   aws.String(queueURL),
	}
	_, err = s.client.SetQueueAttributesWithContext(ctx, setQueueAttributesInput)
	if err != nil {
		return fmt.Errorf("setting queue attributes, %w", err)
	}
	return nil
}

func (s *SQSProvider) DiscoverQueueURL(ctx context.Context, ignoreCache bool) (string, error) {
	opts := lo.Ternary(ignoreCache, cache.IgnoreCacheOption, nil)
	return cache.TryGetStringWithFallback(&s.mu, s.queueURL,
		func() (string, error) {
			ret, err := s.client.GetQueueUrlWithContext(ctx, s.getQueueURLInput)
			if err != nil {
				return "", fmt.Errorf("fetching queue url, %w", err)
			}
			return aws.StringValue(ret.QueueUrl), nil
		},
		opts,
	)
}

func (s *SQSProvider) GetSQSMessages(ctx context.Context) ([]*sqs.Message, error) {
	queueURL, err := s.DiscoverQueueURL(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("fetching queue url, %w", err)
	}

	// Copy the input template and add the discovered queue url
	input, err := functional.DeepCopy(s.receiveMessageInput)
	if err != nil {
		return nil, fmt.Errorf("copying input, %w", err)
	}
	input.QueueUrl = aws.String(queueURL)

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
	queueURL, err := s.DiscoverQueueURL(ctx, false)
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
		return fmt.Errorf("deleting messages from sqs queue, %w", err)
	}
	return nil
}

func (s *SQSProvider) DeleteQueue(ctx context.Context) error {
	queueURL, err := s.DiscoverQueueURL(ctx, false)
	if err != nil {
		if IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("fetching queue url, %w", err)
	}

	input := &sqs.DeleteQueueInput{
		QueueUrl: aws.String(queueURL),
	}
	_, err = s.client.DeleteQueueWithContext(ctx, input)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("deleting sqs queue, %w", err)
	}
	return nil
}

func (s *SQSProvider) getQueueAttributes(ctx context.Context) map[string]*string {
	policy := lo.Must(json.Marshal(s.getQueuePolicy(ctx)))
	return map[string]*string{
		sqs.QueueAttributeNameMessageRetentionPeriod: aws.String("300"),
		sqs.QueueAttributeNamePolicy:                 aws.String(string(policy)),
	}
}

func (s *SQSProvider) getQueuePolicy(ctx context.Context) *QueuePolicy {
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
				Resource: s.getQueueARN(ctx),
			},
		},
	}
}

func (s *SQSProvider) getQueueARN(ctx context.Context) string {
	return fmt.Sprintf("arn:%s:sqs:%s:%s:%s",
		s.metadataProvider.Partition(),
		s.metadataProvider.Region(ctx),
		s.metadataProvider.AccountID(ctx),
		s.queueName,
	)
}

func getQueueName(ctx context.Context) string {
	return fmt.Sprintf("Karpenter-%s-Queue", injection.GetOptions(ctx).ClusterName)
}

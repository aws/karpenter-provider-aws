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

package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/samber/lo"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/utils/atomic"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	awserrors "github.com/aws/karpenter/pkg/errors"
)

type queuePolicy struct {
	Version   string                 `json:"Version"`
	ID        string                 `json:"Id"`
	Statement []queuePolicyStatement `json:"Statement"`
}

type queuePolicyStatement struct {
	Effect    string    `json:"Effect"`
	Principal principal `json:"Principal"`
	Action    []string  `json:"Action"`
	Resource  string    `json:"Resource"`
}

type principal struct {
	Service []string `json:"Service"`
}

type SQS struct {
	client sqsiface.SQSAPI

	queueURL atomic.Lazy[string]
	queueARN atomic.Lazy[string]
}

func NewSQS(client sqsiface.SQSAPI) *SQS {
	provider := &SQS{
		client: client,
	}
	provider.queueURL.Resolve = func(ctx context.Context) (string, error) {
		input := &sqs.GetQueueUrlInput{
			QueueName: aws.String(provider.QueueName(ctx)),
		}
		ret, err := provider.client.GetQueueUrlWithContext(ctx, input)
		if err != nil {
			return "", fmt.Errorf("fetching queue url, %w", err)
		}
		return aws.StringValue(ret.QueueUrl), nil
	}
	provider.queueARN.Resolve = func(ctx context.Context) (string, error) {
		queueURL, err := provider.queueURL.TryGet(ctx)
		if err != nil {
			return "", fmt.Errorf("discovering queue url, %w", err)
		}
		input := &sqs.GetQueueAttributesInput{
			AttributeNames: aws.StringSlice([]string{sqs.QueueAttributeNameQueueArn}),
			QueueUrl:       aws.String(queueURL),
		}
		ret, err := provider.client.GetQueueAttributesWithContext(ctx, input)
		if err != nil {
			return "", fmt.Errorf("fetching queue arn, %w", err)
		}
		if arn, ok := ret.Attributes[sqs.QueueAttributeNameQueueArn]; ok {
			return aws.StringValue(arn), nil
		}
		return "", fmt.Errorf("queue arn not found in queue attributes response")
	}
	return provider
}

func (s *SQS) QueueName(ctx context.Context) string {
	return lo.Substring(settings.FromContext(ctx).ClusterName, 0, 80)
}

func (s *SQS) CreateQueue(ctx context.Context) error {
	input := &sqs.CreateQueueInput{
		QueueName: aws.String(s.QueueName(ctx)),
		Tags:      s.getTags(ctx),
	}
	result, err := s.client.CreateQueueWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("creating sqs queue, %w", err)
	}
	s.queueURL.Set(aws.StringValue(result.QueueUrl))
	return nil
}

func (s *SQS) SetQueueAttributes(ctx context.Context, attributeOverrides map[string]*string) error {
	queueURL, err := s.DiscoverQueueURL(ctx)
	if err != nil {
		return fmt.Errorf("fetching queue url, %w", err)
	}
	attributes, err := s.getQueueAttributes(ctx)
	if err != nil {
		return fmt.Errorf("marshaling queue attributes, %w", err)
	}
	if attributeOverrides != nil {
		attributes = lo.Assign(attributes, attributeOverrides)
	}
	setQueueAttributesInput := &sqs.SetQueueAttributesInput{
		Attributes: attributes,
		QueueUrl:   aws.String(queueURL),
	}
	_, err = s.client.SetQueueAttributesWithContext(ctx, setQueueAttributesInput)
	if err != nil {
		return fmt.Errorf("setting queue attributes, %w", err)
	}
	return nil
}

func (s *SQS) QueueExists(ctx context.Context) (bool, error) {
	_, err := s.queueURL.TryGet(ctx, atomic.IgnoreCacheOption)
	if err != nil {
		if awserrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *SQS) DiscoverQueueURL(ctx context.Context) (string, error) {
	return s.queueURL.TryGet(ctx)
}

func (s *SQS) DiscoverQueueARN(ctx context.Context) (string, error) {
	return s.queueARN.TryGet(ctx)
}

func (s *SQS) GetSQSMessages(ctx context.Context) ([]*sqs.Message, error) {
	queueURL, err := s.DiscoverQueueURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching queue url, %w", err)
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

func (s *SQS) SendMessage(ctx context.Context, body interface{}) (string, error) {
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

func (s *SQS) DeleteSQSMessage(ctx context.Context, msg *sqs.Message) error {
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

func (s *SQS) DeleteQueue(ctx context.Context) error {
	queueURL, err := s.DiscoverQueueURL(ctx)
	if err != nil {
		if awserrors.IsNotFound(err) || awserrors.IsAccessDenied(err) {
			return nil
		}
		return fmt.Errorf("fetching queue url, %w", err)
	}

	input := &sqs.DeleteQueueInput{
		QueueUrl: aws.String(queueURL),
	}
	_, err = s.client.DeleteQueueWithContext(ctx, input)
	if err != nil && !awserrors.IsNotFound(err) {
		return fmt.Errorf("deleting sqs queue, %w", err)
	}
	return nil
}

func (s *SQS) getQueueAttributes(ctx context.Context) (map[string]*string, error) {
	raw, err := s.getQueuePolicy(ctx)
	if err != nil {
		return nil, fmt.Errorf("marshaling queue policy, %w", err)
	}
	policy := lo.Must(json.Marshal(raw))
	return map[string]*string{
		sqs.QueueAttributeNameMessageRetentionPeriod: aws.String("300"),
		sqs.QueueAttributeNamePolicy:                 aws.String(string(policy)),
	}, nil
}

func (s *SQS) getQueuePolicy(ctx context.Context) (*queuePolicy, error) {
	queueARN, err := s.DiscoverQueueARN(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieving queue arn for queue policy, %w", err)
	}
	return &queuePolicy{
		Version: "2008-10-17",
		ID:      "EC2NotificationPolicy",
		Statement: []queuePolicyStatement{
			{
				Effect: "Allow",
				Principal: principal{
					Service: []string{
						"events.amazonaws.com",
						"sqs.amazonaws.com",
					},
				},
				Action:   []string{"sqs:SendMessage"},
				Resource: queueARN,
			},
		},
	}, nil
}

func (s *SQS) getTags(ctx context.Context) map[string]*string {
	return lo.Assign(
		lo.MapEntries(settings.FromContext(ctx).Tags, func(k, v string) (string, *string) {
			return k, lo.ToPtr(v)
		}),
		map[string]*string{
			v1alpha5.DiscoveryTagKey: aws.String(settings.FromContext(ctx).ClusterName),
			v1alpha5.ManagedByTagKey: aws.String(settings.FromContext(ctx).ClusterName),
		},
	)
}

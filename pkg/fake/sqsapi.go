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

package fake

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

const (
	dummyQueueURL = "https://sqs.us-west-2.amazonaws.com/000000000000/Karpenter-cluster-Queue"
)

// SQSBehavior must be reset between tests otherwise tests will
// pollute each other.
type SQSBehavior struct {
	GetQueueURLBehavior        MockedFunction[sqs.GetQueueUrlInput, sqs.GetQueueUrlOutput]
	GetQueueAttributesBehavior MockedFunction[sqs.GetQueueAttributesInput, sqs.GetQueueAttributesOutput]
	ReceiveMessageBehavior     MockedFunction[sqs.ReceiveMessageInput, sqs.ReceiveMessageOutput]
	DeleteMessageBehavior      MockedFunction[sqs.DeleteMessageInput, sqs.DeleteMessageOutput]
}

type SQSAPI struct {
	sqsiface.SQSAPI
	SQSBehavior
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (s *SQSAPI) Reset() {
	s.GetQueueURLBehavior.Reset()
	s.GetQueueAttributesBehavior.Reset()
	s.ReceiveMessageBehavior.Reset()
	s.DeleteMessageBehavior.Reset()
}

//nolint:revive,stylecheck
func (s *SQSAPI) GetQueueUrlWithContext(_ context.Context, input *sqs.GetQueueUrlInput, _ ...request.Option) (*sqs.GetQueueUrlOutput, error) {
	return s.GetQueueURLBehavior.WithDefault(&sqs.GetQueueUrlOutput{
		QueueUrl: aws.String(dummyQueueURL),
	}).Invoke(input)
}

func (s *SQSAPI) GetQueueAttributesWithContext(_ context.Context, input *sqs.GetQueueAttributesInput, _ ...request.Option) (*sqs.GetQueueAttributesOutput, error) {
	return s.GetQueueAttributesBehavior.WithDefault(&sqs.GetQueueAttributesOutput{
		Attributes: map[string]*string{
			sqs.QueueAttributeNameQueueArn: aws.String("arn:aws:sqs:us-west-2:000000000000:Karpenter-Queue"),
		},
	}).Invoke(input)
}

func (s *SQSAPI) ReceiveMessageWithContext(_ context.Context, input *sqs.ReceiveMessageInput, _ ...request.Option) (*sqs.ReceiveMessageOutput, error) {
	return s.ReceiveMessageBehavior.Invoke(input)
}

func (s *SQSAPI) DeleteMessageWithContext(_ context.Context, input *sqs.DeleteMessageInput, _ ...request.Option) (*sqs.DeleteMessageOutput, error) {
	return s.DeleteMessageBehavior.Invoke(input)
}

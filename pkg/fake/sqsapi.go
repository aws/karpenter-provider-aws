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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

const (
	dummyQueueURL = "https://sqs.us-west-2.amazonaws.com/000000000000/Karpenter-cluster-Queue"
)

// SQSBehavior must be reset between tests otherwise tests will
// pollute each other.
type SQSBehavior struct {
	GetQueueURLBehavior    MockedFunction[sqs.GetQueueUrlInput, sqs.GetQueueUrlOutput]
	ReceiveMessageBehavior MockedFunction[sqs.ReceiveMessageInput, sqs.ReceiveMessageOutput]
	DeleteMessageBehavior  MockedFunction[sqs.DeleteMessageInput, sqs.DeleteMessageOutput]
}

type SQSAPI struct {
	sdk.SQSAPI
	SQSBehavior
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (s *SQSAPI) Reset() {
	s.GetQueueURLBehavior.Reset()
	s.ReceiveMessageBehavior.Reset()
	s.DeleteMessageBehavior.Reset()
}

//nolint:revive,stylecheck
func (s *SQSAPI) GetQueueUrl(_ context.Context, input *sqs.GetQueueUrlInput, _ ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
	return s.GetQueueURLBehavior.Invoke(input, func(_ *sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
		return &sqs.GetQueueUrlOutput{
			QueueUrl: aws.String(dummyQueueURL),
		}, nil
	})
}

func (s *SQSAPI) ReceiveMessage(_ context.Context, input *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	return s.ReceiveMessageBehavior.Invoke(input, func(_ *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
		return nil, nil
	})
}

func (s *SQSAPI) DeleteMessage(_ context.Context, input *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	return s.DeleteMessageBehavior.Invoke(input, func(_ *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
		return nil, nil
	})
}

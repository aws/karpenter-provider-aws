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
	CreateQueueOutput                 AtomicPtr[sqs.CreateQueueOutput]
	GetQueueURLOutput                 AtomicPtr[sqs.GetQueueUrlOutput]
	ReceiveMessageOutput              AtomicPtr[sqs.ReceiveMessageOutput]
	CalledWithCreateQueueInput        AtomicPtrSlice[sqs.CreateQueueInput]
	CalledWithGetQueueURLInput        AtomicPtrSlice[sqs.GetQueueUrlInput]
	CalledWithSetQueueAttributesInput AtomicPtrSlice[sqs.SetQueueAttributesInput]
	CalledWithReceiveMessageInput     AtomicPtrSlice[sqs.ReceiveMessageInput]
	CalledWithDeleteMessageInput      AtomicPtrSlice[sqs.DeleteMessageInput]
	CalledWithDeleteQueueInput        AtomicPtrSlice[sqs.DeleteQueueInput]
	NextError                         AtomicError
}

type SQSAPI struct {
	sqsiface.SQSAPI
	SQSBehavior
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (s *SQSAPI) Reset() {
	s.CreateQueueOutput.Reset()
	s.GetQueueURLOutput.Reset()
	s.ReceiveMessageOutput.Reset()
	s.CalledWithCreateQueueInput.Reset()
	s.CalledWithGetQueueURLInput.Reset()
	s.CalledWithSetQueueAttributesInput.Reset()
	s.CalledWithReceiveMessageInput.Reset()
	s.CalledWithDeleteMessageInput.Reset()
	s.CalledWithDeleteQueueInput.Reset()
	s.NextError.Reset()
}

func (s *SQSAPI) CreateQueueWithContext(_ context.Context, input *sqs.CreateQueueInput, _ ...request.Option) (*sqs.CreateQueueOutput, error) {
	if !s.NextError.IsNil() {
		defer s.NextError.Reset()
		return nil, s.NextError.Get()
	}
	s.CalledWithCreateQueueInput.Add(input)

	if !s.CreateQueueOutput.IsNil() {
		return s.CreateQueueOutput.Clone(), nil
	}
	return &sqs.CreateQueueOutput{
		QueueUrl: aws.String(dummyQueueURL),
	}, nil
}

//nolint:revive,stylecheck
func (s *SQSAPI) GetQueueUrlWithContext(_ context.Context, input *sqs.GetQueueUrlInput, _ ...request.Option) (*sqs.GetQueueUrlOutput, error) {
	if !s.NextError.IsNil() {
		defer s.NextError.Reset()
		return nil, s.NextError.Get()
	}
	s.CalledWithGetQueueURLInput.Add(input)

	if !s.GetQueueURLOutput.IsNil() {
		return s.GetQueueURLOutput.Clone(), nil
	}
	return &sqs.GetQueueUrlOutput{
		QueueUrl: aws.String(dummyQueueURL),
	}, nil
}

func (s *SQSAPI) SetQueueAttributesWithContext(_ context.Context, input *sqs.SetQueueAttributesInput, _ ...request.Option) (*sqs.SetQueueAttributesOutput, error) {
	if !s.NextError.IsNil() {
		defer s.NextError.Reset()
		return nil, s.NextError.Get()
	}
	s.CalledWithSetQueueAttributesInput.Add(input)

	return &sqs.SetQueueAttributesOutput{}, nil
}

func (s *SQSAPI) ReceiveMessageWithContext(_ context.Context, input *sqs.ReceiveMessageInput, _ ...request.Option) (*sqs.ReceiveMessageOutput, error) {
	if !s.NextError.IsNil() {
		defer s.NextError.Reset()
		return nil, s.NextError.Get()
	}
	s.CalledWithReceiveMessageInput.Add(input)

	if !s.ReceiveMessageOutput.IsNil() {
		return s.ReceiveMessageOutput.Clone(), nil
	}
	return &sqs.ReceiveMessageOutput{
		Messages: []*sqs.Message{},
	}, nil
}

func (s *SQSAPI) DeleteMessageWithContext(_ context.Context, input *sqs.DeleteMessageInput, _ ...request.Option) (*sqs.DeleteMessageOutput, error) {
	if !s.NextError.IsNil() {
		defer s.NextError.Reset()
		return nil, s.NextError.Get()
	}
	s.CalledWithDeleteMessageInput.Add(input)

	return &sqs.DeleteMessageOutput{}, nil
}

func (s *SQSAPI) DeleteQueueWithContext(_ context.Context, input *sqs.DeleteQueueInput, _ ...request.Option) (*sqs.DeleteQueueOutput, error) {
	if !s.NextError.IsNil() {
		defer s.NextError.Reset()
		return nil, s.NextError.Get()
	}
	s.CalledWithDeleteQueueInput.Add(input)

	return &sqs.DeleteQueueOutput{}, nil
}

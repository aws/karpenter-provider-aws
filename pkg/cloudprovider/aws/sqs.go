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
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"knative.dev/pkg/logging"
)

type SQSProvider struct {
	receiveMessageInput *sqs.ReceiveMessageInput
	deleteMessageInput  *sqs.DeleteMessageInput
	client              sqsiface.SQSAPI
}

func NewSQSProvider(client sqsiface.SQSAPI, queueURL string) *SQSProvider {
	receiveMessageInput := &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
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

	deleteMessageInput := &sqs.DeleteMessageInput{
		QueueUrl: aws.String(queueURL),
	}

	return &SQSProvider{
		receiveMessageInput: receiveMessageInput,
		deleteMessageInput:  deleteMessageInput,
		client:              client,
	}
}

func (s *SQSProvider) GetSQSMessages(ctx context.Context) ([]*sqs.Message, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named("sqsClient.getMessages"))

	result, err := s.client.ReceiveMessageWithContext(ctx, s.receiveMessageInput)
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

	input, err := deepCopyDeleteMessage(s.deleteMessageInput)
	if err != nil {
		return fmt.Errorf("error copying delete message input, %w", err)
	}
	input.ReceiptHandle = msg.ReceiptHandle

	_, err = s.client.DeleteMessageWithContext(ctx, input)
	if err != nil {
		logging.FromContext(ctx).
			With("error", err).
			Error("failed to delete message")
		return err
	}

	return nil
}

func deepCopyDeleteMessage(input *sqs.DeleteMessageInput) (*sqs.DeleteMessageInput, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(input); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(&buf)
	var cp sqs.DeleteMessageInput
	if err := dec.Decode(&cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

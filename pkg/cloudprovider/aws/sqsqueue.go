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
	"fmt"
	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

func Validate(q *v1alpha1.QueueSpec) error {
	_, err := arn.Parse(q.ID)
	return err
}

func init() {
	v1alpha1.RegisterQueueValidator(v1alpha1.AWSSQSQueueType, Validate)
}

type SQSQueue struct {
	ARN    string
	Client sqsiface.SQSAPI
}

func (q *SQSQueue) Name() string {
	return q.ARN
}

func NewSQSQueue(id string, client sqsiface.SQSAPI) *SQSQueue {
	return &SQSQueue{
		ARN:    id,
		Client: client,
	}
}

// Length returns the length of the queue
func (q *SQSQueue) Length() (int64, error) {
	queueURL, err := q.getUrl(q.ARN)
	if err != nil {
		return 0, err
	}

	// Get the attributes of the SQS queue (https://docs.aws.amazon.com/sdk-for-go/api/service/sqs/#GetQueueAttributesInput)
	queueAttributes, err := q.Client.GetQueueAttributes(&sqs.GetQueueAttributesInput{
		AttributeNames: []*string{aws.String("ApproximateNumberOfMessages")},
		QueueUrl:       aws.String(queueURL),
	})
	if err != nil {
		return 0, fmt.Errorf("could not pull SQS queueAttributes with input URL: %w", err)
	}

	// Convert string result into a returnable int64 type
	length, err := strconv.ParseInt(*queueAttributes.Attributes["ApproximateNumberOfMessages"], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("could not resolve SQS queueAttributes types, %w", err)
	}

	return length, nil
}

// OldestMessageAge returns the age of the oldest message
func (q *SQSQueue) OldestMessageAge() (int64, error) {
	return 0, nil
}

// Urls are in the form of https://sqs.<region>.amazonaws.com/<account_id>/<queue_name>
// getUrl parses the SQS ARN and queries the client for its respective URL
func (q *SQSQueue) getUrl(sqsArn string) (string, error) {
	arn, err := arn.Parse(sqsArn)
	if err != nil {
		return "", fmt.Errorf("could not parse ARN for SQS, invalid ARN: %w", err)
	}

	queueURLOutput, err := q.Client.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName:              &arn.Resource,
		QueueOwnerAWSAccountId: &arn.AccountID,
	})
	if err != nil {
		return "", fmt.Errorf("could not get SQS queue URL %w", err)
	}
	return *queueURLOutput.QueueUrl, nil
}

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

package tests

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	awssdkgo "github.com/ellistarn/karpenter/pkg/cloudprovider/aws"
	"testing"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

type mockedGetQueue struct {
	sqsiface.SQSAPI
	queueUrlOutput       sqs.GetQueueUrlOutput
	queueAttributeOutput sqs.GetQueueAttributesOutput
	error                error
}

func (m mockedGetQueue) GetQueueUrl(*sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
	return &m.queueUrlOutput, m.error
}

func (m mockedGetQueue) GetQueueAttributes(*sqs.GetQueueAttributesInput) (*sqs.GetQueueAttributesOutput, error) {
	return &m.queueAttributeOutput, m.error
}

func TestGetQueueLength(t *testing.T) {
	sqs := &awssdkgo.SQSQueue{
		Client: mockedGetQueue{
			queueUrlOutput: sqs.GetQueueUrlOutput{
				QueueUrl: aws.String("oopsydaisy"),
			},
			queueAttributeOutput: sqs.GetQueueAttributesOutput{
				Attributes: map[string]*string{
					"ApproximateNumberOfMessages": aws.String("42"),
				},
			},
			error: nil,
		},
		ARN: "arn:aws:iam:us-west-2:112358132134:fibonacci",
	}

	length, err := sqs.Length()
	if err != nil {
		t.Errorf("Length() returned error %s; want nil", err)
	}
	if length != 42 {
		t.Errorf("Length() = %d; want 42", length)
	}
}

func TestFailedGetQueueLength(t *testing.T) {
	sqs := &awssdkgo.SQSQueue{
		Client: mockedGetQueue{
			queueUrlOutput:       sqs.GetQueueUrlOutput{},
			queueAttributeOutput: sqs.GetQueueAttributesOutput{},
			error:                fmt.Errorf("didn't work for whatever reason"),
		},
		ARN: "arn:aws:iam:us-west-2:112358132134:fibonacci",
	}

	if _, err := sqs.Length(); err == nil {
		t.Errorf("Length() did not return error, want error %s", err)
	}
}

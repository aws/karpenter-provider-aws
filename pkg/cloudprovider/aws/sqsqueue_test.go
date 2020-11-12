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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/awslabs/karpenter/pkg/cloudprovider/aws/fake"

	"github.com/aws/aws-sdk-go/service/sqs"
)

func TestGetQueueLength(t *testing.T) {
	sqs := &SQSQueue{
		Client: fake.SQSAPI{
			QueueUrlOutput: sqs.GetQueueUrlOutput{
				QueueUrl: aws.String("oopsydaisy"),
			},
			QueueAttributeOutput: sqs.GetQueueAttributesOutput{
				Attributes: map[string]*string{
					"ApproximateNumberOfMessages": aws.String("42"),
				},
			},
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
	sqs := &SQSQueue{
		Client: fake.SQSAPI{
			QueueUrlOutput:       sqs.GetQueueUrlOutput{},
			QueueAttributeOutput: sqs.GetQueueAttributesOutput{},
			WantErr:              fmt.Errorf("didn't work for whatever reason"),
		},
		ARN: "arn:aws:iam:us-west-2:112358132134:fibonacci",
	}

	if _, err := sqs.Length(); err == nil {
		t.Errorf("Length() did not return error, want error %s", err)
	}
}

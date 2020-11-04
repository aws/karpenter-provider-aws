package fake

import (
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
)

type SQSAPI struct {
	sqsiface.SQSAPI
	QueueUrlOutput       sqs.GetQueueUrlOutput
	QueueAttributeOutput sqs.GetQueueAttributesOutput
	WantErr              error
}

func (m SQSAPI) GetQueueUrl(*sqs.GetQueueUrlInput) (*sqs.GetQueueUrlOutput, error) {
	return &m.QueueUrlOutput, m.WantErr
}

func (m SQSAPI) GetQueueAttributes(*sqs.GetQueueAttributesInput) (*sqs.GetQueueAttributesOutput, error) {
	return &m.QueueAttributeOutput, m.WantErr
}

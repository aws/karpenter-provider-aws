package sqs

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/aws/karpenter/pkg/cloudprovider/aws/metadata"
)

type Client interface {
	CreateQueueWithContext(context.Context, *sqs.CreateQueueInput, ...request.Option) (*sqs.CreateQueueOutput, error)
	GetQueueUrlWithContext(context.Context, *sqs.GetQueueUrlInput, ...request.Option) (*sqs.GetQueueUrlOutput, error)
	SetQueueAttributesWithContext(context.Context, *sqs.SetQueueAttributesInput, ...request.Option) (*sqs.SetQueueAttributesOutput, error)
	ReceiveMessageWithContext(context.Context, *sqs.ReceiveMessageInput, ...request.Option) (*sqs.ReceiveMessageOutput, error)
	DeleteMessageWithContext(context.Context, *sqs.DeleteMessageInput, ...request.Option) (*sqs.DeleteMessageOutput, error)
	DeleteQueueWithContext(context.Context, *sqs.DeleteQueueInput, ...request.Option) (*sqs.DeleteQueueOutput, error)
}

type Interface interface {
}

type Provider struct {
	client Client

	createQueueInput    *sqs.CreateQueueInput
	getQueueURLInput    *sqs.GetQueueUrlInput
	receiveMessageInput *sqs.ReceiveMessageInput
	mutex               *sync.RWMutex
	queueURL            string
	queueName           string
	metadata            *metadata.Info
}

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

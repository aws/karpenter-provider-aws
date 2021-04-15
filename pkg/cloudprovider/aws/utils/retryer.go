package utils

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

// Retryer implements the aws request.Retryer interface
// and adds support for retrying ec2 InvalidInstanceID.NotFound
// which can occur when instances have recently been created
// and are not yet describe-able due to eventual consistency
type Retryer struct {
	request.Retryer
}

// NewRetryer instantiates a Retryer based on aws client.DefaultRetryer w/ added functionality for karpenter
func NewRetryer() *Retryer {
	return &Retryer{
		Retryer: client.DefaultRetryer{
			NumMaxRetries: 3,
		},
	}
}

// ShouldRetry returns true if the request should be retried
func (r Retryer) ShouldRetry(req *request.Request) bool {
	if r.Retryer.ShouldRetry(req) {
		return true
	}
	// Retry DescribeInstances because EC2 is eventually consistent
	if aerr, ok := req.Error.(awserr.Error); ok && aerr.Code() == "InvalidInstanceID.NotFound" {
		return true
	}
	return false
}

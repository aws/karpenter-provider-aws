package utils

import (
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

// Retryer implements the aws request.Retryer interface
// and adds support for retrying ec2 InvalidInstanceID.NotFound
// which can occur when instances have recently been created
// and are not yet describe-able due to eventual consistency
type Retryer struct {
	baseRetryer request.Retryer
}

// NewRetryer instantiates a Retryer based on aws client.DefaultRetryer w/ added functionality for karpenter
func NewRetryer() *Retryer {
	return &Retryer{
		baseRetryer: client.DefaultRetryer{
			NumMaxRetries: 3,
		},
	}
}

// RetryRules returns the delay duration before retrying this request again
func (r Retryer) RetryRules(req *request.Request) time.Duration {
	return r.baseRetryer.RetryRules(req)
}

// ShouldRetry returns true if the request should be retried
func (r Retryer) ShouldRetry(req *request.Request) bool {
	if r.baseRetryer.ShouldRetry(req) {
		return true
	}
	if req.Error == nil {
		return false
	}
	// Retry DescribeInstances because EC2 is eventually consistent
	if aerr, ok := req.Error.(awserr.Error); ok && aerr.Code() == "InvalidInstanceID.NotFound" {
		return true
	}
	return false
}

// MaxRetries returns the number of maximum retries the service will use to make
// an individual API request.
func (r Retryer) MaxRetries() int {
	return r.baseRetryer.MaxRetries()
}

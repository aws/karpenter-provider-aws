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

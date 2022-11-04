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

package errors

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/aws/aws-sdk-go/service/sqs"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	launchTemplateNotFoundCode = "InvalidLaunchTemplateName.NotFoundException"
	AccessDeniedCode           = "AccessDenied"
	AccessDeniedExceptionCode  = "AccessDeniedException"
)

var (
	// This is not an exhaustive list, add to it as needed
	notFoundErrorCodes = sets.NewString(
		"InvalidInstanceID.NotFound",
		launchTemplateNotFoundCode,
		sqs.ErrCodeQueueDoesNotExist,
		(&eventbridge.ResourceNotFoundException{}).Code(),
	)
	// unfulfillableCapacityErrorCodes signify that capacity is temporarily unable to be launched
	unfulfillableCapacityErrorCodes = sets.NewString(
		"InsufficientInstanceCapacity",
		"MaxSpotInstanceCountExceeded",
		"VcpuLimitExceeded",
		"UnfulfillableCapacity",
		"Unsupported",
	)
	accessDeniedErrorCodes = sets.NewString(
		AccessDeniedCode,
		AccessDeniedExceptionCode,
	)
	recentlyDeletedErrorCodes = sets.NewString(
		sqs.ErrCodeQueueDeletedRecently,
	)
)

type InstanceTerminatedError struct {
	Err error
}

func (e InstanceTerminatedError) Error() string {
	return e.Err.Error()
}

func IsInstanceTerminated(err error) bool {
	if err == nil {
		return false
	}
	var itErr InstanceTerminatedError
	return errors.As(err, &itErr)
}

// IsNotFound returns true if the err is an AWS error (even if it's
// wrapped) and is a known to mean "not found" (as opposed to a more
// serious or unexpected error)
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var awsError awserr.Error
	if errors.As(err, &awsError) {
		return notFoundErrorCodes.Has(awsError.Code())
	}
	return false
}

// IsAccessDenied returns true if the error is an AWS error (even if it's
// wrapped) and is known to mean "access denied" (as opposed to a more
// serious or unexpected error)
func IsAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	var awsError awserr.Error
	if errors.As(err, &awsError) {
		return accessDeniedErrorCodes.Has(awsError.Code())
	}
	return false
}

// IsRecentlyDeleted returns true if the error is an AWS error (even if it's
// wrapped) and is known to mean "recently deleted"
func IsRecentlyDeleted(err error) bool {
	if err == nil {
		return false
	}
	var awsError awserr.Error
	if errors.As(err, &awsError) {
		return recentlyDeletedErrorCodes.Has(awsError.Code())
	}
	return false
}

// IsUnfulfillableCapacity returns true if the Fleet err means
// capacity is temporarily unavailable for launching.
// This could be due to account limits, insufficient ec2 capacity, etc.
func IsUnfulfillableCapacity(err *ec2.CreateFleetError) bool {
	return unfulfillableCapacityErrorCodes.Has(*err.ErrorCode)
}

func IsLaunchTemplateNotFound(err error) bool {
	if err == nil {
		return false
	}
	var awsError awserr.Error
	if errors.As(err, &awsError) {
		return awsError.Code() == launchTemplateNotFoundCode
	}
	return false
}

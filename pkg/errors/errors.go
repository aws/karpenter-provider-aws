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

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/smithy-go"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	launchTemplateNameNotFoundCode = "InvalidLaunchTemplateName.NotFoundException"
)

var (
	// This is not an exhaustive list, add to it as needed
	notFoundErrorCodes = sets.New[string](
		"InvalidInstanceID.NotFound",
		launchTemplateNameNotFoundCode,
		"InvalidLaunchTemplateId.NotFound",
		"QueueDoesNotExist",
		"NoSuchEntity",
	)
	alreadyExistsErrorCodes = sets.New[string](
		"EntityAlreadyExists",
	)
	accessDeniedErrorCodes = sets.New[string](
		"AccessDeniedException",
	)
	// unfulfillableCapacityErrorCodes signify that capacity is temporarily unable to be launched
	unfulfillableCapacityErrorCodes = sets.New[string](
		"InsufficientInstanceCapacity",
		"MaxSpotInstanceCountExceeded",
		"VcpuLimitExceeded",
		"UnfulfillableCapacity",
		"Unsupported",
		"InsufficientFreeAddressesInSubnet",
	)
)

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

// IsNotFound returns true if the err is an AWS error (even if it's
// wrapped) and is a known to mean "not found" (as opposed to a more
// serious or unexpected error)
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return notFoundErrorCodes.Has(apiErr.ErrorCode())
	}
	return false
}

func IgnoreNotFound(err error) error {
	if IsNotFound(err) {
		return nil
	}
	return err
}

func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return alreadyExistsErrorCodes.Has(apiErr.ErrorCode())
	}
	return false
}

func IgnoreAlreadyExists(err error) error {
	if IsAlreadyExists(err) {
		return nil
	}
	return err
}

// IsUnfulfillableCapacity returns true if the Fleet err means
// capacity is temporarily unavailable for launching.
// This could be due to account limits, insufficient ec2 capacity, etc.
func IsUnfulfillableCapacity(err ec2types.CreateFleetError) bool {
	return unfulfillableCapacityErrorCodes.Has(*err.ErrorCode)
}

func IsLaunchTemplateNotFound(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == launchTemplateNameNotFoundCode
	}
	return false
}

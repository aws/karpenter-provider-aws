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
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	launchTemplateNameNotFoundCode = "InvalidLaunchTemplateName.NotFoundException"
	DryRunOperationErrorCode       = "DryRunOperation"
	UnauthorizedOperationErrorCode = "UnauthorizedOperation"
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

	reservationCapacityExceededErrorCode = "ReservationCapacityExceeded"

	// unfulfillableCapacityErrorCodes signify that capacity is temporarily unable to be launched
	unfulfillableCapacityErrorCodes = sets.New[string](
		"InsufficientInstanceCapacity",
		"MaxSpotInstanceCountExceeded",
		"VcpuLimitExceeded",
		"UnfulfillableCapacity",
		"Unsupported",
		"InsufficientFreeAddressesInSubnet",
		reservationCapacityExceededErrorCode,
	)
)

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

func IsDryRunError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == DryRunOperationErrorCode
	}
	return false
}

func IgnoreDryRunError(err error) error {
	if IsDryRunError(err) {
		return nil
	}
	return err
}

func IsUnauthorizedOperationError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == UnauthorizedOperationErrorCode
	}
	return false
}

func IgnoreUnauthorizedOperationError(err error) error {
	if IsUnauthorizedOperationError(err) {
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

func IsReservationCapacityExceeded(err ec2types.CreateFleetError) bool {
	return *err.ErrorCode == reservationCapacityExceededErrorCode
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

// ToReasonMessage converts an error message from AWS into a well-known condition reason
// and well-known condition message that can be used for Launch failure classification
// nolint:gocyclo
func ToReasonMessage(err error) (string, string) {
	if strings.Contains(err.Error(), "AuthFailure.ServiceLinkedRoleCreationNotPermitted") {
		return "SpotSLRCreationFailed", "User does not hae sufficient permission to create the Spot ServiceLinkedRole to launch spot instances"
	}
	if strings.Contains(err.Error(), "UnauthorizedOperation") || strings.Contains(err.Error(), "AccessDenied") || strings.Contains(err.Error(), "AuthFailure") {
		if strings.Contains(err.Error(), "with an explicit deny in a permissions boundary") {
			return "Unauthorized", "User is not authorized to perform this operation due to a permission boundary"
		}
		if strings.Contains(err.Error(), "with an explicit deny in a service control policy") {
			return "Unauthorized", "User is not authorized to perform this operation due to a service control policy"
		}
		return "Unauthorized", "User is not authorized to perform this operation because no identity-based policy allows it"
	}
	if strings.Contains(err.Error(), "iamInstanceProfile.name is invalid") {
		return "InstanceProfileNameInvalid", "Instance profile name used from EC2NodeClass status does not exist"
	}
	if strings.Contains(err.Error(), "InvalidLaunchTemplateId.NotFound") {
		return "LaunchTemplateNotFound", "Launch template used for instance launch wasn't found"
	}
	if strings.Contains(err.Error(), "InvalidAMIID.Malformed") {
		return "InvalidAMIID", "AMI used for instance launch is invalid"
	}
	if strings.Contains(err.Error(), "RequestLimitExceeded") {
		return "RequestLimitExceeded", "Request limit exceeded"
	}
	if strings.Contains(err.Error(), "InternalError") {
		return "InternalError", "An internal error has occurred"
	}
	// ICE Errors come last in this list because we should return a generic ICE error if all of the errors that are returned from
	// fleet are ICE errors
	if strings.Contains(err.Error(), "MaxFleetCountExceeded") {
		return "FleetQuotaExceeded", "A fleet launch was requested but this would exceed your fleet request quota"
	}
	if strings.Contains(err.Error(), "PendingVerification") {
		return "AccountPendingVerification", "An instance launch was requested but the request for launching resources in this region is still being verified"
	}
	if strings.Contains(err.Error(), "MaxSpotInstanceCountExceeded") {
		return "SpotQuotaExceeded", "A spot instance launch was requested but this would exceed your spot instance quota"
	}
	if strings.Contains(err.Error(), "VcpuLimitExceeded") {
		return "VCPULimitExceeded", "An instance was requested that would exceed your VCPU quota"
	}
	if strings.Contains(err.Error(), "InsufficientFreeAddressesInSubnet") {
		return "InsufficientFreeAddressesInSubnet", "There are not enough free IP addresses to launch an instance in this subnet"
	}
	return "LaunchFailed", "Instance launch failed"
}

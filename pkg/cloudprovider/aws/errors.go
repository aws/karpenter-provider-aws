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
	"errors"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/karpenter/pkg/utils/functional"
)

var (
	// This is not an exhaustive list, add to it as needed
	notFoundErrorCodes = []string{
		"InvalidInstanceID.NotFound",
		"InvalidLaunchTemplateName.NotFoundException",
	}
)

// InsufficientCapacityErrorCode indicates that EC2 is temporarily lacking capacity for this
// instance type and availability zone combination
const InsufficientCapacityErrorCode = "InsufficientInstanceCapacity"

// isNotFound returns true if the err is an AWS error (even if it's
// wrapped) and is a known to mean "not found" (as opposed to a more
// serious or unexpected error)
func isNotFound(err error) bool {
	var awsError awserr.Error
	if errors.As(err, &awsError) {
		return functional.ContainsString(notFoundErrorCodes, awsError.Code())
	}
	return false
}

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

package fake

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
)

// IAMBehavior must be reset between tests otherwise tests will
// pollute each other.
type IAMBehavior struct {
	ListInstanceProfilesForRoleOutput AtomicPtr[iam.ListInstanceProfilesForRoleOutput]
	NextError                         AtomicError
}

type IAMAPI struct {
	iamiface.IAMAPI
	IAMBehavior
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (e *IAMAPI) Reset() {
	e.ListInstanceProfilesForRoleOutput.Reset()
	e.NextError.Reset()
}

// ListInstanceProfilesForRoleWithContext mocks the call
// nolint: gocyclo
func (e *IAMAPI) ListInstanceProfilesForRoleWithContext(_ context.Context, input *iam.ListInstanceProfilesForRoleInput, _ ...request.Option) (*iam.ListInstanceProfilesForRoleOutput, error) {
	if !e.NextError.IsNil() {
		defer e.NextError.Reset()
		return nil, e.NextError.Get()
	}
	if !e.ListInstanceProfilesForRoleOutput.IsNil() {
		return e.ListInstanceProfilesForRoleOutput.Clone(), nil
	}
	return &iam.ListInstanceProfilesForRoleOutput{
		InstanceProfiles: []*iam.InstanceProfile{
			{
				InstanceProfileName: aws.String("test-role-instance-profile"),
			},
		},
	}, nil
}

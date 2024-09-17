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
	"fmt"
	"sync"
	"time"

	"github.com/aws/smithy-go"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"
)

const ()

// IAMAPIBehavior must be reset between tests otherwise tests will
// pollute each other.
type IAMAPIBehavior struct {
	GetInstanceProfileBehavior            MockedFunction[iam.GetInstanceProfileInput, iam.GetInstanceProfileOutput]
	CreateInstanceProfileBehavior         MockedFunction[iam.CreateInstanceProfileInput, iam.CreateInstanceProfileOutput]
	DeleteInstanceProfileBehavior         MockedFunction[iam.DeleteInstanceProfileInput, iam.DeleteInstanceProfileOutput]
	AddRoleToInstanceProfileBehavior      MockedFunction[iam.AddRoleToInstanceProfileInput, iam.AddRoleToInstanceProfileOutput]
	TagInstanceProfileBehavior            MockedFunction[iam.TagInstanceProfileInput, iam.TagInstanceProfileOutput]
	RemoveRoleFromInstanceProfileBehavior MockedFunction[iam.RemoveRoleFromInstanceProfileInput, iam.RemoveRoleFromInstanceProfileOutput]
	UntagInstanceProfileBehavior          MockedFunction[iam.UntagInstanceProfileInput, iam.UntagInstanceProfileOutput]
}

type IAMAPI struct {
	sync.Mutex

	IAMClient *iam.Client
	IAMAPIBehavior

	InstanceProfiles map[string]*iamtypes.InstanceProfile
}

func NewIAMAPI() *IAMAPI {
	return &IAMAPI{InstanceProfiles: map[string]*iamtypes.InstanceProfile{}}
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func toSliceOfValues[T any](value []T) []T {
	values := make([]T, len(value))
	for i, p := range value {
		values[i] = p
	}
	return values
}

func (s *IAMAPI) Reset() {
	s.GetInstanceProfileBehavior.Reset()
	s.CreateInstanceProfileBehavior.Reset()
	s.DeleteInstanceProfileBehavior.Reset()
	s.AddRoleToInstanceProfileBehavior.Reset()
	s.RemoveRoleFromInstanceProfileBehavior.Reset()
	s.InstanceProfiles = map[string]*iamtypes.InstanceProfile{}
}

func (s *IAMAPI) GetInstanceProfile(_ context.Context, input *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	return s.GetInstanceProfileBehavior.Invoke(input, func(*iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if i, ok := s.InstanceProfiles[aws.ToString(input.InstanceProfileName)]; ok {
			return &iam.GetInstanceProfileOutput{InstanceProfile: i}, nil
		}
		return nil, &smithy.GenericAPIError{
			Code: "NoSuchEntityException",
			Message: fmt.Sprintf("Instance Profile %s cannot be found",
				aws.ToString(input.InstanceProfileName)),
		}
	})
}

func (s *IAMAPI) CreateInstanceProfile(_ context.Context, input *iam.CreateInstanceProfileInput, _ ...func(*iam.Options)) (*iam.CreateInstanceProfileOutput, error) {
	return s.CreateInstanceProfileBehavior.Invoke(input, func(output *iam.CreateInstanceProfileInput) (*iam.CreateInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if _, ok := s.InstanceProfiles[aws.ToString(input.InstanceProfileName)]; ok {
			return nil, &smithy.GenericAPIError{
				Code: "EntityAlreadyExistsException",
				Message: fmt.Sprintf("Instance Profile %s already exists",
					aws.ToString(input.InstanceProfileName)),
			}
		}
		instanceProfile := &iamtypes.InstanceProfile{
			CreateDate:          aws.Time(time.Now()),
			InstanceProfileId:   aws.String(InstanceProfileID()),
			InstanceProfileName: input.InstanceProfileName,
			Path:                input.Path,
			Tags:                input.Tags,
		}
		s.InstanceProfiles[aws.ToString(input.InstanceProfileName)] = instanceProfile
		return &iam.CreateInstanceProfileOutput{InstanceProfile: instanceProfile}, nil
	})
}

func (s *IAMAPI) DeleteInstanceProfile(_ context.Context, input *iam.DeleteInstanceProfileInput, _ ...func(*iam.Options)) (*iam.DeleteInstanceProfileOutput, error) {
	return s.DeleteInstanceProfileBehavior.Invoke(input, func(output *iam.DeleteInstanceProfileInput) (*iam.DeleteInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if i, ok := s.InstanceProfiles[aws.ToString(input.InstanceProfileName)]; ok {
			if len(i.Roles) > 0 {
				return nil, &smithy.GenericAPIError{
					Code: "DeleteConflictException",
					Message: fmt.Sprintf("Instance Profile %s has roles and cannot be deleted",
						aws.ToString(input.InstanceProfileName)),
				}
			}
			delete(s.InstanceProfiles, aws.ToString(input.InstanceProfileName))
			return &iam.DeleteInstanceProfileOutput{}, nil
		}
		return nil, &smithy.GenericAPIError{
			Code: "NoSuchEntityException",
			Message: fmt.Sprintf("Instance Profile %s cannot be found",
				aws.ToString(input.InstanceProfileName)),
		}
	})
}

func (s *IAMAPI) TagInstanceProfile(_ context.Context, input *iam.TagInstanceProfileInput, _ ...func(*iam.Options)) (*iam.TagInstanceProfileOutput, error) {
	return s.TagInstanceProfileBehavior.Invoke(input, func(output *iam.TagInstanceProfileInput) (*iam.TagInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if profile, ok := s.InstanceProfiles[aws.ToString(input.InstanceProfileName)]; ok {
			profile.Tags = lo.UniqBy(append(toSliceOfValues(input.Tags), toSliceOfValues(profile.Tags)...), func(t iamtypes.Tag) string {
				return lo.FromPtr(t.Key)
			})
			return nil, nil
		}
		return nil, &smithy.GenericAPIError{
			Code: "NoSuchEntityException",
			Message: fmt.Sprintf("Instance Profile %s cannot be found",
				aws.ToString(input.InstanceProfileName)),
		}
	})
}

func (s *IAMAPI) AddRoleToInstanceProfile(_ context.Context, input *iam.AddRoleToInstanceProfileInput, _ ...func(*iam.Options)) (*iam.AddRoleToInstanceProfileOutput, error) {
	return s.AddRoleToInstanceProfileBehavior.Invoke(input, func(output *iam.AddRoleToInstanceProfileInput) (*iam.AddRoleToInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if i, ok := s.InstanceProfiles[aws.ToString(input.InstanceProfileName)]; ok {
			if len(i.Roles) > 0 {
				return nil, &smithy.GenericAPIError{
					Code: "LimitExceededException",
					Message: fmt.Sprintf("Instance Profile %s already has a role",
						aws.ToString(input.InstanceProfileName)),
				}
			}
			i.Roles = append(i.Roles, iamtypes.Role{RoleId: aws.String(RoleID()), RoleName: input.RoleName})
			return nil, nil
		}
		return nil, &smithy.GenericAPIError{
			Code: "NoSuchEntityException",
			Message: fmt.Sprintf("Instance Profile %s cannot be found",
				aws.ToString(input.InstanceProfileName)),
		}
	})
}

func (s *IAMAPI) RemoveRoleFromInstanceProfile(_ context.Context, input *iam.RemoveRoleFromInstanceProfileInput, _ ...func(*iam.Options)) (*iam.RemoveRoleFromInstanceProfileOutput, error) {
	return s.RemoveRoleFromInstanceProfileBehavior.Invoke(input, func(output *iam.RemoveRoleFromInstanceProfileInput) (*iam.RemoveRoleFromInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if i, ok := s.InstanceProfiles[aws.ToString(input.InstanceProfileName)]; ok {
			newRoles := lo.Reject(toSliceOfValues(i.Roles), func(r iamtypes.Role, _ int) bool {
				return aws.ToString(r.RoleName) == aws.ToString(input.RoleName)
			})
			if len(i.Roles) == len(newRoles) {
				return nil, &smithy.GenericAPIError{
					Code: "NoSuchEntityException",
					Message: fmt.Sprintf("Instance Profile %s does not have role %s",
						aws.ToString(input.InstanceProfileName), aws.ToString(input.RoleName)),
				}
			}
			i.Roles = newRoles
			return nil, nil
		}
		return nil, &smithy.GenericAPIError{
			Code: "NoSuchEntityException",
			Message: fmt.Sprintf("Instance Profile %s cannot be found",
				aws.ToString(input.InstanceProfileName)),
		}
	})
}

func (s *IAMAPI) UntagInstanceProfile(_ context.Context, input *iam.UntagInstanceProfileInput, _ ...func(*iam.Options)) (*iam.UntagInstanceProfileOutput, error) {
	return s.UntagInstanceProfileBehavior.Invoke(input, func(output *iam.UntagInstanceProfileInput) (*iam.UntagInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if profile, ok := s.InstanceProfiles[aws.ToString(input.InstanceProfileName)]; ok {
			profile.Tags = lo.Reject(toSliceOfValues(profile.Tags), func(t iamtypes.Tag, _ int) bool {
				return lo.Contains(toSliceOfValues(input.TagKeys), lo.FromPtr(t.Key))
			})
			return nil, nil
		}
		return nil, &smithy.GenericAPIError{
			Code: "NoSuchEntityException",
			Message: fmt.Sprintf("Instance Profile %s cannot be found",
				aws.ToString(input.InstanceProfileName)),
		}
	})
}

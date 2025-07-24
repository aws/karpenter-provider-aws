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
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"
	"github.com/samber/lo"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
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
	ListInstanceProfilesBehavior          MockedFunction[iam.ListInstanceProfilesInput, iam.ListInstanceProfilesOutput]
}

type IAMAPI struct {
	sync.Mutex

	sdk.IAMAPI
	IAMAPIBehavior

	InstanceProfiles map[string]*iamtypes.InstanceProfile
}

func NewIAMAPI() *IAMAPI {
	return &IAMAPI{InstanceProfiles: map[string]*iamtypes.InstanceProfile{}}
}

func (s *IAMAPI) Reset() {
	s.GetInstanceProfileBehavior.Reset()
	s.CreateInstanceProfileBehavior.Reset()
	s.DeleteInstanceProfileBehavior.Reset()
	s.AddRoleToInstanceProfileBehavior.Reset()
	s.RemoveRoleFromInstanceProfileBehavior.Reset()
	s.ListInstanceProfilesBehavior.Reset()
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
			Code: "NoSuchEntity",
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
				Code: "EntityAlreadyExists",
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
			Code: "NoSuchEntity",
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
			profile.Tags = lo.UniqBy(append(input.Tags, profile.Tags...), func(t iamtypes.Tag) string {
				return lo.FromPtr(t.Key)
			})
			return nil, nil
		}
		return nil, &smithy.GenericAPIError{
			Code: "NoSuchEntity",
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
			Code: "NoSuchEntity",
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
			newRoles := lo.Reject(i.Roles, func(r iamtypes.Role, _ int) bool {
				return aws.ToString(r.RoleName) == aws.ToString(input.RoleName)
			})
			if len(i.Roles) == len(newRoles) {
				return nil, &smithy.GenericAPIError{
					Code: "NoSuchEntity",
					Message: fmt.Sprintf("Instance Profile %s does not have role %s",
						aws.ToString(input.InstanceProfileName), aws.ToString(input.RoleName)),
				}
			}
			i.Roles = newRoles
			return nil, nil
		}
		return nil, &smithy.GenericAPIError{
			Code: "NoSuchEntity",
			Message: fmt.Sprintf("Instance Profile %s cannot be found",
				aws.ToString(input.InstanceProfileName)),
		}
	})
}

func (s *IAMAPI) ListInstanceProfiles(_ context.Context, input *iam.ListInstanceProfilesInput, _ ...func(*iam.Options)) (*iam.ListInstanceProfilesOutput, error) {
	return s.ListInstanceProfilesBehavior.Invoke(input, func(*iam.ListInstanceProfilesInput) (*iam.ListInstanceProfilesOutput, error) {
		s.Lock()
		defer s.Unlock()

		var profiles []iamtypes.InstanceProfile
		for _, profile := range s.InstanceProfiles {
			if profile.Path != nil && strings.HasPrefix(*profile.Path, *input.PathPrefix) {
				profiles = append(profiles, *profile)
			}
		}
		return &iam.ListInstanceProfilesOutput{
			InstanceProfiles: profiles,
		}, nil
	})
}

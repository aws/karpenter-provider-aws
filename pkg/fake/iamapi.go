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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
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
	RemoveRoleFromInstanceProfileBehavior MockedFunction[iam.RemoveRoleFromInstanceProfileInput, iam.RemoveRoleFromInstanceProfileOutput]
}

type IAMAPI struct {
	sync.Mutex

	iamiface.IAMAPI
	IAMAPIBehavior

	InstanceProfiles map[string]*iam.InstanceProfile
}

func NewIAMAPI() *IAMAPI {
	return &IAMAPI{InstanceProfiles: map[string]*iam.InstanceProfile{}}
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (s *IAMAPI) Reset() {
	s.GetInstanceProfileBehavior.Reset()
	s.CreateInstanceProfileBehavior.Reset()
	s.DeleteInstanceProfileBehavior.Reset()
	s.AddRoleToInstanceProfileBehavior.Reset()
	s.RemoveRoleFromInstanceProfileBehavior.Reset()
	s.InstanceProfiles = map[string]*iam.InstanceProfile{}
}

func (s *IAMAPI) GetInstanceProfileWithContext(_ context.Context, input *iam.GetInstanceProfileInput, _ ...request.Option) (*iam.GetInstanceProfileOutput, error) {
	return s.GetInstanceProfileBehavior.Invoke(input, func(*iam.GetInstanceProfileInput) (*iam.GetInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if i, ok := s.InstanceProfiles[aws.StringValue(input.InstanceProfileName)]; ok {
			return &iam.GetInstanceProfileOutput{InstanceProfile: i}, nil
		}
		return nil, awserr.New(iam.ErrCodeNoSuchEntityException, fmt.Sprintf("Instance Profile %s cannot be found", aws.StringValue(input.InstanceProfileName)), nil)
	})
}

func (s *IAMAPI) CreateInstanceProfileWithContext(_ context.Context, input *iam.CreateInstanceProfileInput, _ ...request.Option) (*iam.CreateInstanceProfileOutput, error) {
	return s.CreateInstanceProfileBehavior.Invoke(input, func(output *iam.CreateInstanceProfileInput) (*iam.CreateInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if _, ok := s.InstanceProfiles[aws.StringValue(input.InstanceProfileName)]; ok {
			return nil, awserr.New(iam.ErrCodeEntityAlreadyExistsException, fmt.Sprintf("Instance Profile %s already exists", aws.StringValue(input.InstanceProfileName)), nil)
		}
		instanceProfile := &iam.InstanceProfile{
			CreateDate:          aws.Time(time.Now()),
			InstanceProfileId:   aws.String(InstanceProfileID()),
			InstanceProfileName: input.InstanceProfileName,
			Path:                input.Path,
			Tags:                input.Tags,
		}
		s.InstanceProfiles[aws.StringValue(input.InstanceProfileName)] = instanceProfile
		return &iam.CreateInstanceProfileOutput{InstanceProfile: instanceProfile}, nil
	})
}

func (s *IAMAPI) DeleteInstanceProfileWithContext(_ context.Context, input *iam.DeleteInstanceProfileInput, _ ...request.Option) (*iam.DeleteInstanceProfileOutput, error) {
	return s.DeleteInstanceProfileBehavior.Invoke(input, func(output *iam.DeleteInstanceProfileInput) (*iam.DeleteInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if i, ok := s.InstanceProfiles[aws.StringValue(input.InstanceProfileName)]; ok {
			if len(i.Roles) > 0 {
				return nil, awserr.New(iam.ErrCodeDeleteConflictException, "Cannot delete entity, must remove roles from instance profile first.", nil)
			}
			delete(s.InstanceProfiles, aws.StringValue(input.InstanceProfileName))
			return &iam.DeleteInstanceProfileOutput{}, nil
		}
		return nil, awserr.New(iam.ErrCodeNoSuchEntityException, fmt.Sprintf("Instance Profile %s cannot be found", aws.StringValue(input.InstanceProfileName)), nil)
	})
}

func (s *IAMAPI) AddRoleToInstanceProfileWithContext(_ context.Context, input *iam.AddRoleToInstanceProfileInput, _ ...request.Option) (*iam.AddRoleToInstanceProfileOutput, error) {
	return s.AddRoleToInstanceProfileBehavior.Invoke(input, func(output *iam.AddRoleToInstanceProfileInput) (*iam.AddRoleToInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if i, ok := s.InstanceProfiles[aws.StringValue(input.InstanceProfileName)]; ok {
			if len(i.Roles) > 0 {
				return nil, awserr.New(iam.ErrCodeLimitExceededException, "Cannot exceed quota for InstanceSessionsPerInstanceProfile: 1", nil)
			}
			i.Roles = append(i.Roles, &iam.Role{RoleId: aws.String(RoleID()), RoleName: input.RoleName})
			return nil, nil
		}
		return nil, awserr.New(iam.ErrCodeNoSuchEntityException, fmt.Sprintf("Instance Profile %s cannot be found", aws.StringValue(input.InstanceProfileName)), nil)
	})
}

func (s *IAMAPI) RemoveRoleFromInstanceProfileWithContext(_ context.Context, input *iam.RemoveRoleFromInstanceProfileInput, _ ...request.Option) (*iam.RemoveRoleFromInstanceProfileOutput, error) {
	return s.RemoveRoleFromInstanceProfileBehavior.Invoke(input, func(output *iam.RemoveRoleFromInstanceProfileInput) (*iam.RemoveRoleFromInstanceProfileOutput, error) {
		s.Lock()
		defer s.Unlock()

		if i, ok := s.InstanceProfiles[aws.StringValue(input.InstanceProfileName)]; ok {
			newRoles := lo.Reject(i.Roles, func(r *iam.Role, _ int) bool {
				return aws.StringValue(r.RoleName) == aws.StringValue(input.RoleName)
			})
			if len(i.Roles) == len(newRoles) {
				return nil, awserr.New(iam.ErrCodeNoSuchEntityException, fmt.Sprintf("The role with name %s cannot be found", aws.StringValue(input.RoleName)), nil)
			}
			i.Roles = newRoles
			return nil, nil
		}
		return nil, awserr.New(iam.ErrCodeNoSuchEntityException, fmt.Sprintf("Instance Profile %s cannot be found", aws.StringValue(input.InstanceProfileName)), nil)
	})
}

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

package instanceprofile

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/awslabs/operatorpkg/serrors"
	gocache "github.com/patrickmn/go-cache"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/operator/options"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
	awserrors "github.com/aws/karpenter-provider-aws/pkg/errors"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
)

type Provider interface {
	Get(context.Context, string) (*iamtypes.InstanceProfile, error)
	Create(context.Context, string, string, map[string]string, string) error
	Delete(context.Context, string) error
	ListClusterProfiles(context.Context) ([]*iamtypes.InstanceProfile, error)
	ListNodeClassProfiles(context.Context, *v1.EC2NodeClass) ([]*iamtypes.InstanceProfile, error)
	IsProtected(string) bool
	SetProtectedState(string, bool)
}

type DefaultProvider struct {
	iamapi            sdk.IAMAPI
	cache             *gocache.Cache // instanceProfileName -> *iamtypes.InstanceProfile
	protectedProfiles *gocache.Cache // Cache to account for eventual consistency delays when garbage collecting
	region            string
}

func NewDefaultProvider(iamapi sdk.IAMAPI, cache *gocache.Cache, region string) *DefaultProvider {
	return &DefaultProvider{
		iamapi:            iamapi,
		cache:             cache,
		protectedProfiles: gocache.New(time.Minute, time.Minute), // TTL should represent worst case "wait period" before deleting a non-active instance profile is acceptable
		region:            region,
	}
}

func (p *DefaultProvider) Get(ctx context.Context, instanceProfileName string) (*iamtypes.InstanceProfile, error) {
	if instanceProfile, ok := p.cache.Get(instanceProfileName); ok {
		return instanceProfile.(*iamtypes.InstanceProfile), nil
	}
	out, err := p.iamapi.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: lo.ToPtr(instanceProfileName),
	})
	if err != nil {
		return nil, err
	}
	p.cache.SetDefault(instanceProfileName, out.InstanceProfile)
	return out.InstanceProfile, nil
}

func (p *DefaultProvider) Create(ctx context.Context, instanceProfileName string, roleName string, tags map[string]string, nodeClassUID string) error {
	instanceProfile, err := p.Get(ctx, instanceProfileName)
	if err != nil {
		if !awserrors.IsNotFound(err) {
			return serrors.Wrap(fmt.Errorf("getting instance profile, %w", err), "instance-profile", instanceProfileName)
		}
		o, err := p.iamapi.CreateInstanceProfile(ctx, &iam.CreateInstanceProfileInput{
			InstanceProfileName: lo.ToPtr(instanceProfileName),
			Tags:                utils.IAMMergeTags(tags),
			Path:                lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", p.region, options.FromContext(ctx).ClusterName, nodeClassUID)),
		})
		if err != nil {
			log.Printf("CALLING CREATE FAILED")
			return serrors.Wrap(fmt.Errorf("creating instance profile, %w", err), "instance-profile", instanceProfileName)
		}
		log.Printf("CALLING CREATE GOOD")

		instanceProfile = o.InstanceProfile
	}
	// Instance profiles can only have a single role assigned to them so this profile either has 1 or 0 roles
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html
	if len(instanceProfile.Roles) == 1 {
		if lo.FromPtr(instanceProfile.Roles[0].RoleName) == roleName {
			return nil
		}
		if _, err = p.iamapi.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: lo.ToPtr(instanceProfileName),
			RoleName:            instanceProfile.Roles[0].RoleName,
		}); err != nil {
			return serrors.Wrap(fmt.Errorf("removing role for instance profile, %w", err), "role", lo.FromPtr(instanceProfile.Roles[0].RoleName), "instance-profile", instanceProfileName)
		}
	}
	// If the role has a path, ignore the path and take the role name only since AddRoleToInstanceProfile
	// does not support paths in the role name.
	roleName = lo.LastOr(strings.Split(roleName, "/"), roleName)
	log.Printf("HERE IS ROLE NAME: %s", roleName)
	if _, err = p.iamapi.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: lo.ToPtr(instanceProfileName),
		RoleName:            lo.ToPtr(roleName),
	}); err != nil {
		return serrors.Wrap(fmt.Errorf("adding role to instance profile, %w", err), "role", roleName, "instance-profile", instanceProfileName)
	}
	instanceProfile.Roles = []iamtypes.Role{{
		RoleName: lo.ToPtr(roleName),
	}}
	p.cache.SetDefault(instanceProfileName, instanceProfile)
	return nil
}

func (p *DefaultProvider) Delete(ctx context.Context, instanceProfileName string) error {
	out, err := p.iamapi.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: lo.ToPtr(instanceProfileName),
	})
	if err != nil {
		return awserrors.IgnoreNotFound(serrors.Wrap(fmt.Errorf("getting instance profile, %w", err), "instance-profile", instanceProfileName))
	}
	// Instance profiles can only have a single role assigned to them so this profile either has 1 or 0 roles
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html
	if len(out.InstanceProfile.Roles) == 1 {
		if _, err = p.iamapi.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: lo.ToPtr(instanceProfileName),
			RoleName:            out.InstanceProfile.Roles[0].RoleName,
		}); err != nil {
			return serrors.Wrap(fmt.Errorf("removing role from instance profile, %w", err), "role", lo.FromPtr(out.InstanceProfile.Roles[0].RoleName), "instance-profile", instanceProfileName)
		}
	}
	if _, err = p.iamapi.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{
		InstanceProfileName: lo.ToPtr(instanceProfileName),
	}); err != nil {
		return awserrors.IgnoreNotFound(serrors.Wrap(fmt.Errorf("deleting instance profile, %w", err), "instance-profile", instanceProfileName))
	}
	p.cache.Delete(instanceProfileName)
	p.protectedProfiles.Delete(instanceProfileName)
	return nil
}

func (p *DefaultProvider) ListClusterProfiles(ctx context.Context) ([]*iamtypes.InstanceProfile, error) {
	out, err := p.iamapi.ListInstanceProfiles(ctx, &iam.ListInstanceProfilesInput{
		PathPrefix: lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/", p.region, options.FromContext(ctx).ClusterName)),
	})
	if err != nil {
		return nil, fmt.Errorf("listing instance profiles, %w", err)
	}

	var profiles []*iamtypes.InstanceProfile
	for i := range out.InstanceProfiles {
		profiles = append(profiles, &out.InstanceProfiles[i])
	}
	return profiles, nil
}

func (p *DefaultProvider) ListNodeClassProfiles(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]*iamtypes.InstanceProfile, error) {
	out, err := p.iamapi.ListInstanceProfiles(ctx, &iam.ListInstanceProfilesInput{
		PathPrefix: lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", p.region, options.FromContext(ctx).ClusterName, string(nodeClass.UID))),
	})
	if err != nil {
		return nil, fmt.Errorf("listing instance profiles, %w", err)
	}

	var profiles []*iamtypes.InstanceProfile
	for i := range out.InstanceProfiles {
		profiles = append(profiles, &out.InstanceProfiles[i])
	}
	return profiles, nil
}

func (p *DefaultProvider) IsProtected(profileName string) bool {
	_, exists := p.protectedProfiles.Get(profileName)
	return exists
}

// For testing purposes
func (p *DefaultProvider) SetProtectedState(profileName string, protected bool) {
	if !protected {
		p.protectedProfiles.Delete(profileName)
	} else {
		p.protectedProfiles.SetDefault(profileName, struct{}{})
	}
}

func (p *DefaultProvider) Reset() {
	p.protectedProfiles.Flush()
}

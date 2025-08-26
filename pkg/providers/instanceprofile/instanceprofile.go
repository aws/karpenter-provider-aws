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
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/awslabs/operatorpkg/serrors"
	cache "github.com/patrickmn/go-cache"
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
	cache             *cache.Cache // instanceProfileName -> *iamtypes.InstanceProfile
	protectedProfiles *cache.Cache // Cache to account for eventual consistency delays when garbage collecting
	region            string

	roleCache roleCache
}

func NewDefaultProvider(iamapi sdk.IAMAPI, cache *cache.Cache, protectedProfiles *cache.Cache, region string) *DefaultProvider {
	return &DefaultProvider{
		iamapi:            iamapi,
		cache:             cache,
		protectedProfiles: protectedProfiles,
		region:            region,

		roleCache: roleCache{Cache: cache},
	}
}

func getProfileCacheKey(profileName string) string {
	return "instance-profile:" + profileName
}

func (p *DefaultProvider) Get(ctx context.Context, instanceProfileName string) (*iamtypes.InstanceProfile, error) {
	profileCacheKey := getProfileCacheKey(instanceProfileName)
	if instanceProfile, ok := p.cache.Get(profileCacheKey); ok {
		return instanceProfile.(*iamtypes.InstanceProfile), nil
	}
	out, err := p.iamapi.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: lo.ToPtr(instanceProfileName),
	})
	if err != nil {
		return nil, err
	}
	p.cache.SetDefault(profileCacheKey, out.InstanceProfile)
	return out.InstanceProfile, nil
}

func (p *DefaultProvider) Create(ctx context.Context, instanceProfileName string, roleName string, tags map[string]string, nodeClassUID string) error {
	// Don't attempt to create an instance profile if the role hasn't been found. This prevents runaway instance profile
	// creation by the NodeClass controller when there's a missing role.
	if err, ok := p.roleCache.HasError(roleName); ok {
		return fmt.Errorf("role not found, %w", err)
	}

	profileCacheKey := getProfileCacheKey(instanceProfileName)
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
			return serrors.Wrap(err, "instance-profile", instanceProfileName)
		}
		instanceProfile = o.InstanceProfile
	}
	if err := p.ensureRole(ctx, instanceProfile, roleName); err != nil {
		return fmt.Errorf("ensuring role attached, %w", err)
	}
	// Add the role to the cached instance profile for detection in the ensureRole check based on the cache entry
	instanceProfile.Roles = []iamtypes.Role{{
		RoleName: lo.ToPtr(roleName),
	}}
	p.cache.SetDefault(profileCacheKey, instanceProfile)
	return nil
}

// ensureRole ensures that the correct role is attached to the provided instance profile. If a non-matching role is
// found already attached, it's removed.
func (p *DefaultProvider) ensureRole(ctx context.Context, instanceProfile *iamtypes.InstanceProfile, roleName string) error {
	// Instance profiles can only have a single role assigned to them so this profile either has 1 or 0 roles
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html
	if len(instanceProfile.Roles) == 1 {
		if lo.FromPtr(instanceProfile.Roles[0].RoleName) == roleName {
			return nil
		}
		if _, err := p.iamapi.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: instanceProfile.InstanceProfileName,
			RoleName:            instanceProfile.Roles[0].RoleName,
		}); err != nil {
			return serrors.Wrap(
				fmt.Errorf("removing role for instance profile, %w", err),
				"role", lo.FromPtr(instanceProfile.Roles[0].RoleName),
				"instance-profile", lo.FromPtr(instanceProfile.InstanceProfileName),
			)
		}
	}

	// If the role has a path, ignore the path and take the role name only since AddRoleToInstanceProfile
	// does not support paths in the role name.
	roleName = lo.LastOr(strings.Split(roleName, "/"), roleName)
	if _, err := p.iamapi.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: instanceProfile.InstanceProfileName,
		RoleName:            lo.ToPtr(roleName),
	}); err != nil {
		if roleErr, ok := ToRoleNotFoundError(err); ok {
			err = roleErr
			p.roleCache.SetError(roleName, err)
		}
		return serrors.Wrap(
			fmt.Errorf("adding role to instance profile, %w", err),
			"role", roleName,
			"instance-profile", lo.FromPtr(instanceProfile.InstanceProfileName),
		)
	}
	return nil
}

func (p *DefaultProvider) Delete(ctx context.Context, instanceProfileName string) error {
	profileCacheKey := getProfileCacheKey(instanceProfileName)
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
	p.cache.Delete(profileCacheKey)
	p.SetProtectedState(instanceProfileName, false)
	return nil
}

func (p *DefaultProvider) ListClusterProfiles(ctx context.Context) ([]*iamtypes.InstanceProfile, error) {
	input := &iam.ListInstanceProfilesInput{
		PathPrefix: lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/", p.region, options.FromContext(ctx).ClusterName)),
	}

	paginator := iam.NewListInstanceProfilesPaginator(p.iamapi, input)

	var profiles []*iamtypes.InstanceProfile
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing instance profiles, %w", err)
		}
		profiles = append(profiles, lo.ToSlicePtr(out.InstanceProfiles)...)
	}
	return profiles, nil
}

func (p *DefaultProvider) ListNodeClassProfiles(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]*iamtypes.InstanceProfile, error) {
	input := &iam.ListInstanceProfilesInput{
		PathPrefix: lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", p.region, options.FromContext(ctx).ClusterName, string(nodeClass.UID))),
	}

	paginator := iam.NewListInstanceProfilesPaginator(p.iamapi, input)

	var profiles []*iamtypes.InstanceProfile
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing instance profiles, %w", err)
		}
		profiles = append(profiles, lo.ToSlicePtr(out.InstanceProfiles)...)
	}
	return profiles, nil
}

func (p *DefaultProvider) IsProtected(profileName string) bool {
	_, exists := p.protectedProfiles.Get(profileName)
	return exists
}

func (p *DefaultProvider) SetProtectedState(profileName string, protected bool) {
	if !protected {
		p.protectedProfiles.Delete(profileName)
	} else {
		p.protectedProfiles.SetDefault(profileName, struct{}{})
	}
}

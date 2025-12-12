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
	"github.com/aws/smithy-go"
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
	Create(context.Context, string, string, map[string]string, string, bool) error
	Delete(context.Context, string) error
	ListClusterProfiles(context.Context) ([]*iamtypes.InstanceProfile, error)
	ListNodeClassProfiles(context.Context, *v1.EC2NodeClass) ([]*iamtypes.InstanceProfile, error)
	IsProtected(string) bool
	SetProtectedState(string, bool)
}

type DefaultProvider struct {
	iamapi                 sdk.IAMAPI
	instanceProfileCache   *cache.Cache // instanceProfileName -> *iamtypes.InstanceProfile
	roleNotFoundErrorCache RoleNotFoundErrorCache
	protectedProfileCache  *cache.Cache // Cache to account for eventual consistency delays when garbage collecting
	region                 string
}

func NewDefaultProvider(
	iamapi sdk.IAMAPI,
	instanceProfileCache *cache.Cache,
	roleCache *cache.Cache,
	protectedProfileCache *cache.Cache,
	region string,
) *DefaultProvider {
	return &DefaultProvider{
		iamapi:                 iamapi,
		instanceProfileCache:   instanceProfileCache,
		roleNotFoundErrorCache: RoleNotFoundErrorCache{Cache: roleCache},
		protectedProfileCache:  protectedProfileCache,
		region:                 region,
	}
}

func GetProfileCacheKey(profileName string) string {
	return "instance-profile:" + profileName
}

func (p *DefaultProvider) Get(ctx context.Context, instanceProfileName string) (*iamtypes.InstanceProfile, error) {
	profileCacheKey := GetProfileCacheKey(instanceProfileName)
	if instanceProfile, ok := p.instanceProfileCache.Get(profileCacheKey); ok {
		return instanceProfile.(*iamtypes.InstanceProfile), nil
	}
	out, err := p.iamapi.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: lo.ToPtr(instanceProfileName),
	})
	if err != nil {
		return nil, err
	}
	p.instanceProfileCache.SetDefault(profileCacheKey, out.InstanceProfile)
	return out.InstanceProfile, nil
}

func (p *DefaultProvider) Create(
	ctx context.Context,
	instanceProfileName string,
	roleName string,
	tags map[string]string,
	nodeClassUID string,
	usePath bool,
) error {
	// Don't attempt to create an instance profile if the role hasn't been found. This prevents runaway instance profile
	// creation by the NodeClass controller when there's a missing role.
	if err, ok := p.roleNotFoundErrorCache.HasError(roleName); ok {
		return fmt.Errorf("role not found, %w", err)
	}

	shouldUpdate := false
	profileCacheKey := GetProfileCacheKey(instanceProfileName)
	instanceProfile, err := p.Get(ctx, instanceProfileName)
	if err != nil {
		if !awserrors.IsNotFound(err) {
			return serrors.Wrap(fmt.Errorf("getting instance profile, %w", err), "instance-profile", instanceProfileName)
		}
		input := &iam.CreateInstanceProfileInput{
			InstanceProfileName: lo.ToPtr(instanceProfileName),
			Tags:                utils.IAMMergeTags(tags),
		}
		if usePath {
			input.Path = lo.ToPtr(fmt.Sprintf("/karpenter/%s/%s/%s/", p.region, options.FromContext(ctx).ClusterName, nodeClassUID))
		}
		o, err := p.iamapi.CreateInstanceProfile(ctx, input)
		if err != nil {
			return serrors.Wrap(err, "instance-profile", instanceProfileName)
		}
		instanceProfile = o.InstanceProfile
		shouldUpdate = true
	}

	updatedRole, err := p.ensureRole(ctx, instanceProfile, roleName)
	if err != nil {
		return fmt.Errorf("ensuring role attached, %w", err)
	}
	// Add the role to the cached instance profile for detection in the ensureRole check based on the cache entry
	instanceProfile.Roles = []iamtypes.Role{{
		RoleName: lo.ToPtr(roleName),
	}}

	if shouldUpdate || updatedRole {
		p.instanceProfileCache.SetDefault(profileCacheKey, instanceProfile)
	}
	return nil
}

// ensureRole ensures that the correct role is attached to the provided instance profile. If a non-matching role is
// found already attached, it's removed.
func (p *DefaultProvider) ensureRole(ctx context.Context, instanceProfile *iamtypes.InstanceProfile, roleName string) (bool, error) {
	// Instance profiles can only have a single role assigned to them so this profile either has 1 or 0 roles
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html
	if len(instanceProfile.Roles) == 1 {
		if lo.FromPtr(instanceProfile.Roles[0].RoleName) == roleName {
			return false, nil
		}
		// Instance profiles can only have a single role assigned to them so this profile either has 1 or 0 roles
		// https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html
		if _, err := p.iamapi.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: instanceProfile.InstanceProfileName,
			RoleName:            instanceProfile.Roles[0].RoleName,
		}); err != nil {
			return false, serrors.Wrap(
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
		err = serrors.Wrap(
			fmt.Errorf("adding role to instance profile, %w", err),
			"role", roleName,
			"instance-profile", lo.FromPtr(instanceProfile.InstanceProfileName),
		)
		if IsRoleNotFoundError(err) {
			p.roleNotFoundErrorCache.SetError(roleName, err)
		}
		return true, err
	}
	return true, nil
}

func (p *DefaultProvider) Delete(ctx context.Context, instanceProfileName string) error {
	profileCacheKey := GetProfileCacheKey(instanceProfileName)
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
	p.instanceProfileCache.Delete(profileCacheKey)
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
	_, exists := p.protectedProfileCache.Get(profileName)
	return exists
}

func (p *DefaultProvider) SetProtectedState(profileName string, protected bool) {
	if !protected {
		p.protectedProfileCache.Delete(profileName)
	} else {
		p.protectedProfileCache.SetDefault(profileName, struct{}{})
	}
}

// IsRoleNotFoundError converts a smithy.APIError returned by AddRoleToInstanceProfile to a RoleNotFoundError.
func IsRoleNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	apiErr, ok := lo.ErrorsAs[smithy.APIError](err)
	if !ok {
		return false
	}
	if apiErr.ErrorCode() != "NoSuchEntity" {
		return false
	}
	// Differentiate between the instance profile not being found, and the role.
	if !strings.Contains(apiErr.ErrorMessage(), "role") {
		return false
	}
	return true
}

// RoleNotFoundErrorCache is a wrapper around a go-cache for handling role not found errors returned by AddRoleToInstanceProfile.
type RoleNotFoundErrorCache struct {
	*cache.Cache
}

// HasError returns the last RoleNotFoundError encountered when attempting to add the given role to an instance profile.
func (rc RoleNotFoundErrorCache) HasError(roleName string) (error, bool) {
	if err, ok := rc.Get(roleName); ok {
		return err.(error), true
	}
	return nil, false
}

func (rc RoleNotFoundErrorCache) SetError(roleName string, err error) {
	rc.SetDefault(roleName, err)
}

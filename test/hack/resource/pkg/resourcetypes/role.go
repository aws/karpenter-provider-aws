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

package resourcetypes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

const (
	nodeRolePrefix                 = "KarpenterNodeRole-"
	testInstanceProfilePrefix      = "KarpenterNodeInstanceProfile-"
	testDriftInstanceProfilePrefix = "KarpenterNodeInstanceProfile-Drift-"

	// maxRoleDeletesPerRun bounds the damage if a live cluster is ever misread as gone
	maxRoleDeletesPerRun = 50
	roleDeleteAttempts   = 3
)

type profileReleaseAPI interface {
	iam.ListInstanceProfilesForRoleAPIClient
	RemoveRoleFromInstanceProfile(ctx context.Context, params *iam.RemoveRoleFromInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.RemoveRoleFromInstanceProfileOutput, error)
	DeleteInstanceProfile(ctx context.Context, params *iam.DeleteInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.DeleteInstanceProfileOutput, error)
}

type roleAPI interface {
	profileReleaseAPI
	iam.ListRolesAPIClient
	iam.ListRolePoliciesAPIClient
	iam.ListAttachedRolePoliciesAPIClient
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
	DeleteRolePolicy(ctx context.Context, params *iam.DeleteRolePolicyInput, optFns ...func(*iam.Options)) (*iam.DeleteRolePolicyOutput, error)
	DetachRolePolicy(ctx context.Context, params *iam.DetachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.DetachRolePolicyOutput, error)
	DeleteRole(ctx context.Context, params *iam.DeleteRoleInput, optFns ...func(*iam.Options)) (*iam.DeleteRoleOutput, error)
}

type Role struct {
	iamClient roleAPI
	live      *LiveClusters
	clusters  map[string]string

	deleteConflictDelay time.Duration
}

func NewRole(iamClient roleAPI, live *LiveClusters) *Role {
	return &Role{iamClient: iamClient, live: live, clusters: map[string]string{}, deleteConflictDelay: 5 * time.Second}
}

func (r *Role) String() string {
	return "IAMRole"
}

func (r *Role) Global() bool {
	return true
}

func (r *Role) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (names []string, err error) {
	if err := r.live.Err(); err != nil {
		return nil, fmt.Errorf("skipping role sweep, %w", err)
	}
	roles, err := r.getAllRoles(ctx)
	if err != nil {
		return names, err
	}
	var errs error
	for _, role := range roles {
		if !strings.HasPrefix(lo.FromPtr(role.RoleName), nodeRolePrefix) || !lo.FromPtr(role.CreateDate).Before(expirationTime) {
			continue
		}
		// ListRoles doesn't return tags
		out, err := r.iamClient.GetRole(ctx, &iam.GetRoleInput{RoleName: role.RoleName})
		if err != nil {
			errs = multierr.Append(errs, ignoreNoSuchEntity(err))
			continue
		}
		clusterName, ok := nodeRoleCluster(lo.FromPtr(role.RoleName), out.Role.Tags)
		if !ok || slices.Contains(excludedClusters, clusterName) || !r.live.Gone(clusterName) {
			continue
		}
		r.clusters[lo.FromPtr(role.RoleName)] = clusterName
		names = append(names, lo.FromPtr(role.RoleName))
	}
	return names, errs
}

func (r *Role) CountAll(ctx context.Context) (count int, err error) {
	roles, err := r.getAllRoles(ctx)
	return len(roles), err
}

// The iam-<cluster> stack owns the node role during a single cluster cleanup
func (r *Role) Get(_ context.Context, _ string) (names []string, err error) {
	return names, nil
}

func (r *Role) Cleanup(ctx context.Context, names []string) ([]string, error) {
	var deleted []string
	var errs error
	attempts := 0
	for _, name := range names {
		if attempts >= maxRoleDeletesPerRun {
			break
		}
		attempted, err := r.deleteRole(ctx, name)
		if attempted {
			attempts++
		}
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("deleting role %s, %w", name, err))
			continue
		}
		deleted = append(deleted, name)
	}
	return deleted, errs
}

// deleteRole reports whether it got past the safety checks and changed or deleted anything
func (r *Role) deleteRole(ctx context.Context, name string) (bool, error) {
	clusterName := r.clusters[name]
	if !r.live.Gone(clusterName) {
		return false, fmt.Errorf("owning cluster %q is not known to be gone", clusterName)
	}
	profiles, err := releasableProfiles(ctx, r.iamClient, name, clusterName)
	if err != nil {
		return false, err
	}
	if err := detachProfiles(ctx, r.iamClient, name, profiles); err != nil {
		return true, err
	}
	inline := iam.NewListRolePoliciesPaginator(r.iamClient, &iam.ListRolePoliciesInput{RoleName: lo.ToPtr(name)})
	for inline.HasMorePages() {
		out, err := inline.NextPage(ctx)
		if err != nil {
			return true, ignoreNoSuchEntity(err)
		}
		for _, policy := range out.PolicyNames {
			if _, err := r.iamClient.DeleteRolePolicy(ctx, &iam.DeleteRolePolicyInput{RoleName: lo.ToPtr(name), PolicyName: lo.ToPtr(policy)}); ignoreNoSuchEntity(err) != nil {
				return true, err
			}
		}
	}
	attached := iam.NewListAttachedRolePoliciesPaginator(r.iamClient, &iam.ListAttachedRolePoliciesInput{RoleName: lo.ToPtr(name)})
	for attached.HasMorePages() {
		out, err := attached.NextPage(ctx)
		if err != nil {
			return true, ignoreNoSuchEntity(err)
		}
		for _, policy := range out.AttachedPolicies {
			if _, err := r.iamClient.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{RoleName: lo.ToPtr(name), PolicyArn: policy.PolicyArn}); ignoreNoSuchEntity(err) != nil {
				return true, err
			}
		}
	}
	for attempt := 1; ; attempt++ {
		_, err := r.iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: lo.ToPtr(name)})
		// IAM is eventually consistent, so the role can still look attached right after it was freed
		var conflict *iamtypes.DeleteConflictException
		if !errors.As(err, &conflict) || attempt == roleDeleteAttempts {
			return true, ignoreNoSuchEntity(err)
		}
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case <-time.After(r.deleteConflictDelay):
		}
	}
}

func (r *Role) getAllRoles(ctx context.Context) (roles []iamtypes.Role, err error) {
	paginator := iam.NewListRolesPaginator(r.iamClient, &iam.ListRolesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return roles, err
		}
		roles = append(roles, out.Roles...)
	}
	return roles, nil
}

// nodeRoleCluster requires the testing/cluster or karpenter.sh/discovery tag to name the cluster in the role name
func nodeRoleCluster(roleName string, tags []iamtypes.Tag) (string, bool) {
	if !strings.HasPrefix(roleName, nodeRolePrefix) {
		return "", false
	}
	for _, key := range []string{karpenterTestingTag, karpenterSecurityGroupTag} {
		tag, found := lo.Find(tags, func(t iamtypes.Tag) bool { return lo.FromPtr(t.Key) == key })
		clusterName := lo.FromPtr(tag.Value)
		if !found || clusterName == "" {
			continue
		}
		if roleName == nodeRolePrefix+clusterName || strings.HasPrefix(roleName, nodeRolePrefix+clusterName+"-") {
			return clusterName, true
		}
	}
	return "", false
}

func karpenterProfileCluster(name string) (string, bool) {
	i := strings.LastIndex(name, "_")
	if i <= 0 || i == len(name)-1 {
		return "", false
	}
	if strings.IndexFunc(name[i+1:], func(r rune) bool { return r < '0' || r > '9' }) != -1 {
		return "", false
	}
	return name[:i], true
}

// Karpenter names profiles <cluster>_<hash>, the e2e suites KarpenterNodeInstanceProfile-[Drift-]<cluster>
func ownedInstanceProfile(name, clusterName string) bool {
	if clusterName == "" {
		return false
	}
	if c, ok := karpenterProfileCluster(name); ok && c == clusterName {
		return true
	}
	return name == testInstanceProfilePrefix+clusterName || name == testDriftInstanceProfilePrefix+clusterName
}

// releasableProfiles lists the instance profiles holding roleName, and refuses if any of them isn't clusterName's
func releasableProfiles(ctx context.Context, api profileReleaseAPI, roleName, clusterName string) ([]iamtypes.InstanceProfile, error) {
	var profiles []iamtypes.InstanceProfile
	paginator := iam.NewListInstanceProfilesForRolePaginator(api, &iam.ListInstanceProfilesForRoleInput{RoleName: lo.ToPtr(roleName)})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, ignoreNoSuchEntity(err)
		}
		profiles = append(profiles, out.InstanceProfiles...)
	}
	if foreign, found := lo.Find(profiles, func(p iamtypes.InstanceProfile) bool {
		return !ownedInstanceProfile(lo.FromPtr(p.InstanceProfileName), clusterName)
	}); found {
		return nil, &SkippedError{Reason: fmt.Sprintf("role %s is held by instance profile %s, which is not owned by cluster %s", roleName, lo.FromPtr(foreign.InstanceProfileName), clusterName)}
	}
	return profiles, nil
}

func detachProfiles(ctx context.Context, api profileReleaseAPI, roleName string, profiles []iamtypes.InstanceProfile) error {
	for _, p := range profiles {
		if _, err := api.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: p.InstanceProfileName,
			RoleName:            lo.ToPtr(roleName),
		}); ignoreNoSuchEntity(err) != nil {
			return err
		}
		if _, err := api.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{
			InstanceProfileName: p.InstanceProfileName,
		}); ignoreNoSuchEntity(err) != nil {
			return err
		}
	}
	return nil
}

func ignoreNoSuchEntity(err error) error {
	var nse *iamtypes.NoSuchEntityException
	if errors.As(err, &nse) {
		return nil
	}
	return err
}

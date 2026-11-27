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
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"
)

// vanished roles and profiles are still listed, but are gone by the time they're read
type fakeRole struct {
	created  time.Time
	tags     map[string]string
	inline   []string
	attached []string
	vanished bool
}

type fakeProfile struct {
	created  time.Time
	tags     map[string]string
	roles    []string
	vanished bool
}

type fakeIAM struct {
	mu       sync.Mutex
	roles    map[string]*fakeRole
	profiles map[string]*fakeProfile
	calls    []string
	// deleteConflicts is how many more times DeleteRole fails with DeleteConflict for a role, like IAM does right after a change
	deleteConflicts map[string]int
}

func newFakeIAM() *fakeIAM {
	return &fakeIAM{roles: map[string]*fakeRole{}, profiles: map[string]*fakeProfile{}, deleteConflicts: map[string]int{}}
}

func (f *fakeIAM) record(format string, args ...any) {
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func noSuchEntity(name string) error {
	return &iamtypes.NoSuchEntityException{Message: lo.ToPtr(name + " not found")}
}

func toIAMTags(tags map[string]string) []iamtypes.Tag {
	var out []iamtypes.Tag
	for _, k := range lo.Keys(tags) {
		out = append(out, iamtypes.Tag{Key: lo.ToPtr(k), Value: lo.ToPtr(tags[k])})
	}
	return out
}

func (f *fakeIAM) profile(name string) iamtypes.InstanceProfile {
	p := f.profiles[name]
	return iamtypes.InstanceProfile{
		InstanceProfileName: lo.ToPtr(name),
		CreateDate:          lo.ToPtr(p.created),
		Roles:               lo.Map(p.roles, func(r string, _ int) iamtypes.Role { return iamtypes.Role{RoleName: lo.ToPtr(r)} }),
	}
}

func (f *fakeIAM) ListRoles(_ context.Context, _ *iam.ListRolesInput, _ ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := &iam.ListRolesOutput{}
	for _, name := range slices.Sorted(maps.Keys(f.roles)) {
		out.Roles = append(out.Roles, iamtypes.Role{RoleName: lo.ToPtr(name), CreateDate: lo.ToPtr(f.roles[name].created)})
	}
	return out, nil
}

func (f *fakeIAM) GetRole(_ context.Context, in *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.roles[lo.FromPtr(in.RoleName)]
	if !ok || r.vanished {
		return nil, noSuchEntity(lo.FromPtr(in.RoleName))
	}
	return &iam.GetRoleOutput{Role: &iamtypes.Role{RoleName: in.RoleName, CreateDate: lo.ToPtr(r.created), Tags: toIAMTags(r.tags)}}, nil
}

func (f *fakeIAM) ListInstanceProfilesForRole(_ context.Context, in *iam.ListInstanceProfilesForRoleInput, _ ...func(*iam.Options)) (*iam.ListInstanceProfilesForRoleOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// The CI role is only granted this on KarpenterNodeRole-* roles
	if !strings.HasPrefix(lo.FromPtr(in.RoleName), nodeRolePrefix) {
		return nil, errors.New("AccessDenied")
	}
	if _, ok := f.roles[lo.FromPtr(in.RoleName)]; !ok {
		return nil, noSuchEntity(lo.FromPtr(in.RoleName))
	}
	out := &iam.ListInstanceProfilesForRoleOutput{}
	for _, name := range slices.Sorted(maps.Keys(f.profiles)) {
		if slices.Contains(f.profiles[name].roles, lo.FromPtr(in.RoleName)) {
			out.InstanceProfiles = append(out.InstanceProfiles, f.profile(name))
		}
	}
	return out, nil
}

func (f *fakeIAM) ListRolePolicies(_ context.Context, in *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.roles[lo.FromPtr(in.RoleName)]
	if !ok {
		return nil, noSuchEntity(lo.FromPtr(in.RoleName))
	}
	return &iam.ListRolePoliciesOutput{PolicyNames: slices.Clone(r.inline)}, nil
}

func (f *fakeIAM) ListAttachedRolePolicies(_ context.Context, in *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.roles[lo.FromPtr(in.RoleName)]
	if !ok {
		return nil, noSuchEntity(lo.FromPtr(in.RoleName))
	}
	return &iam.ListAttachedRolePoliciesOutput{AttachedPolicies: lo.Map(r.attached, func(a string, _ int) iamtypes.AttachedPolicy {
		return iamtypes.AttachedPolicy{PolicyArn: lo.ToPtr(a)}
	})}, nil
}

func (f *fakeIAM) DeleteRolePolicy(_ context.Context, in *iam.DeleteRolePolicyInput, _ ...func(*iam.Options)) (*iam.DeleteRolePolicyOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("DeleteRolePolicy %s %s", lo.FromPtr(in.RoleName), lo.FromPtr(in.PolicyName))
	r := f.roles[lo.FromPtr(in.RoleName)]
	r.inline = lo.Without(r.inline, lo.FromPtr(in.PolicyName))
	return &iam.DeleteRolePolicyOutput{}, nil
}

func (f *fakeIAM) DetachRolePolicy(_ context.Context, in *iam.DetachRolePolicyInput, _ ...func(*iam.Options)) (*iam.DetachRolePolicyOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("DetachRolePolicy %s %s", lo.FromPtr(in.RoleName), lo.FromPtr(in.PolicyArn))
	r := f.roles[lo.FromPtr(in.RoleName)]
	r.attached = lo.Without(r.attached, lo.FromPtr(in.PolicyArn))
	return &iam.DetachRolePolicyOutput{}, nil
}

func (f *fakeIAM) DeleteRole(_ context.Context, in *iam.DeleteRoleInput, _ ...func(*iam.Options)) (*iam.DeleteRoleOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name := lo.FromPtr(in.RoleName)
	f.record("DeleteRole %s", name)
	r, ok := f.roles[name]
	if !ok {
		return nil, noSuchEntity(name)
	}
	if f.deleteConflicts[name] > 0 {
		f.deleteConflicts[name]--
		return nil, &iamtypes.DeleteConflictException{Message: lo.ToPtr("role is still attached")}
	}
	for _, p := range f.profiles {
		if slices.Contains(p.roles, name) {
			return nil, &iamtypes.DeleteConflictException{Message: lo.ToPtr("role is in an instance profile")}
		}
	}
	if len(r.inline) > 0 || len(r.attached) > 0 {
		return nil, &iamtypes.DeleteConflictException{Message: lo.ToPtr("role has policies")}
	}
	delete(f.roles, name)
	return &iam.DeleteRoleOutput{}, nil
}

func (f *fakeIAM) ListInstanceProfiles(_ context.Context, _ *iam.ListInstanceProfilesInput, _ ...func(*iam.Options)) (*iam.ListInstanceProfilesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := &iam.ListInstanceProfilesOutput{}
	for _, name := range slices.Sorted(maps.Keys(f.profiles)) {
		out.InstanceProfiles = append(out.InstanceProfiles, f.profile(name))
	}
	return out, nil
}

func (f *fakeIAM) ListInstanceProfileTags(_ context.Context, in *iam.ListInstanceProfileTagsInput, _ ...func(*iam.Options)) (*iam.ListInstanceProfileTagsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[lo.FromPtr(in.InstanceProfileName)]
	if !ok || p.vanished {
		return nil, noSuchEntity(lo.FromPtr(in.InstanceProfileName))
	}
	return &iam.ListInstanceProfileTagsOutput{Tags: toIAMTags(p.tags)}, nil
}

func (f *fakeIAM) GetInstanceProfile(_ context.Context, in *iam.GetInstanceProfileInput, _ ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.profiles[lo.FromPtr(in.InstanceProfileName)]; !ok {
		return nil, noSuchEntity(lo.FromPtr(in.InstanceProfileName))
	}
	return &iam.GetInstanceProfileOutput{InstanceProfile: lo.ToPtr(f.profile(lo.FromPtr(in.InstanceProfileName)))}, nil
}

func (f *fakeIAM) RemoveRoleFromInstanceProfile(_ context.Context, in *iam.RemoveRoleFromInstanceProfileInput, _ ...func(*iam.Options)) (*iam.RemoveRoleFromInstanceProfileOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("RemoveRoleFromInstanceProfile %s %s", lo.FromPtr(in.InstanceProfileName), lo.FromPtr(in.RoleName))
	p, ok := f.profiles[lo.FromPtr(in.InstanceProfileName)]
	if !ok || !slices.Contains(p.roles, lo.FromPtr(in.RoleName)) {
		return nil, noSuchEntity(lo.FromPtr(in.InstanceProfileName))
	}
	p.roles = lo.Without(p.roles, lo.FromPtr(in.RoleName))
	return &iam.RemoveRoleFromInstanceProfileOutput{}, nil
}

func (f *fakeIAM) DeleteInstanceProfile(_ context.Context, in *iam.DeleteInstanceProfileInput, _ ...func(*iam.Options)) (*iam.DeleteInstanceProfileOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	name := lo.FromPtr(in.InstanceProfileName)
	f.record("DeleteInstanceProfile %s", name)
	p, ok := f.profiles[name]
	if !ok {
		return nil, noSuchEntity(name)
	}
	if len(p.roles) > 0 {
		return nil, &iamtypes.DeleteConflictException{Message: lo.ToPtr("instance profile has roles")}
	}
	delete(f.profiles, name)
	return &iam.DeleteInstanceProfileOutput{}, nil
}

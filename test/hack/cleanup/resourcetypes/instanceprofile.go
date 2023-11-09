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
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type InstanceProfile struct {
	iamClient *iam.Client
}

func NewInstanceProfile(iamClient *iam.Client) *InstanceProfile {
	return &InstanceProfile{iamClient: iamClient}
}

func (ip *InstanceProfile) String() string {
	return "InstanceProfile"
}

func (ip *InstanceProfile) GetExpired(ctx context.Context, expirationTime time.Time) (names []string, err error) {
	out, err := ip.iamClient.ListInstanceProfiles(ctx, &iam.ListInstanceProfilesInput{})
	if err != nil {
		return names, err
	}

	errs := make([]error, len(out.InstanceProfiles))
	for i := range out.InstanceProfiles {
		profiles, err := ip.iamClient.ListInstanceProfileTags(ctx, &iam.ListInstanceProfileTagsInput{
			InstanceProfileName: out.InstanceProfiles[i].InstanceProfileName,
		})
		if err != nil {
			errs[i] = err
			continue
		}

		for _, t := range profiles.Tags {
			// Since we can only get the date of the instance profile (not the exact time the instance profile was created)
			// we add a day to the time that it was created to account for the worst-case of the instance profile being created
			// at 23:59:59 and being marked with a time of 00:00:00 due to only capturing the date and not the time
			if lo.FromPtr(t.Key) == karpenterTestingTag && out.InstanceProfiles[i].CreateDate.Add(time.Hour*24).Before(expirationTime) {
				names = append(names, lo.FromPtr(out.InstanceProfiles[i].InstanceProfileName))
			}
		}
	}

	return names, multierr.Combine(errs...)
}

func (ip *InstanceProfile) Get(ctx context.Context, clusterName string) (names []string, err error) {
	out, err := ip.iamClient.ListInstanceProfiles(ctx, &iam.ListInstanceProfilesInput{})
	if err != nil {
		return names, err
	}

	errs := make([]error, len(out.InstanceProfiles))
	for i := range out.InstanceProfiles {
		profiles, err := ip.iamClient.ListInstanceProfileTags(ctx, &iam.ListInstanceProfileTagsInput{
			InstanceProfileName: out.InstanceProfiles[i].InstanceProfileName,
		})
		if err != nil {
			errs[i] = err
			continue
		}

		for _, t := range profiles.Tags {
			if lo.FromPtr(t.Key) == karpenterTestingTag && lo.FromPtr(t.Value) == clusterName {
				names = append(names, lo.FromPtr(out.InstanceProfiles[i].InstanceProfileName))
			}
		}
	}

	return names, multierr.Combine(errs...)
}

// Cleanup any old instance profiles that were managed by Karpenter or were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (ip *InstanceProfile) Cleanup(ctx context.Context, names []string) ([]string, error) {
	var deleted []string
	var errs error
	for i := range names {
		out, _ := ip.iamClient.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: lo.ToPtr(names[i])})
		if len(out.InstanceProfile.Roles) != 0 {
			_, _ = ip.iamClient.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{
				InstanceProfileName: lo.ToPtr(names[i]),
				RoleName:            out.InstanceProfile.Roles[0].RoleName,
			})
		}
		_, err := ip.iamClient.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{
			InstanceProfileName: lo.ToPtr(names[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return deleted, errs
}

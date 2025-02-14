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
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type LaunchTemplate struct {
	ec2Client *ec2.Client
}

func NewLaunchTemplate(ec2Client *ec2.Client) *LaunchTemplate {
	return &LaunchTemplate{ec2Client: ec2Client}
}

func (lt *LaunchTemplate) String() string {
	return "LaunchTemplates"
}

func (lt *LaunchTemplate) Global() bool {
	return false
}

func (lt *LaunchTemplate) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (names []string, err error) {
	lts, err := lt.getAllLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("tag-key"),
				Values: []string{karpenterLaunchTemplateTag},
			},
		},
	})
	if err != nil {
		return names, err
	}

	for _, launchtemplate := range lts {
		clusterName, found := lo.Find(launchtemplate.Tags, func(tag ec2types.Tag) bool {
			return *tag.Key == k8sClusterTag
		})
		if found && slices.Contains(excludedClusters, lo.FromPtr(clusterName.Value)) {
			continue
		}
		if lo.FromPtr(launchtemplate.CreateTime).Before(expirationTime) {
			names = append(names, lo.FromPtr(launchtemplate.LaunchTemplateName))
		}
	}

	return names, err
}

func (lt *LaunchTemplate) CountAll(ctx context.Context) (count int, err error) {
	lts, err := lt.getAllLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{})
	if err != nil {
		return count, err
	}

	return len(lts), err
}

func (lt *LaunchTemplate) Get(ctx context.Context, clusterName string) (names []string, err error) {
	lts, err := lt.getAllLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("tag:" + karpenterLaunchTemplateTag),
				Values: []string{clusterName},
			},
		},
	})
	if err != nil {
		return names, err
	}

	for _, launchtemplate := range lts {
		names = append(names, lo.FromPtr(launchtemplate.LaunchTemplateName))
	}

	return names, err
}

// Cleanup any old launch templates that were managed by Karpenter or were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (lt *LaunchTemplate) Cleanup(ctx context.Context, names []string) ([]string, error) {
	var deleted []string
	var errs error
	for i := range names {
		_, err := lt.ec2Client.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{
			LaunchTemplateName: lo.ToPtr(names[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		deleted = append(deleted, names[i])
	}
	return deleted, errs
}

func (lt *LaunchTemplate) getAllLaunchTemplates(ctx context.Context, params *ec2.DescribeLaunchTemplatesInput) (lts []ec2types.LaunchTemplate, err error) {
	paginator := ec2.NewDescribeLaunchTemplatesPaginator(lt.ec2Client, params)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return lts, err
		}
		lts = append(lts, page.LaunchTemplates...)
	}

	return lts, nil
}

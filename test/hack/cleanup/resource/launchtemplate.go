package resource

import (
	"context"
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

func (lt *LaunchTemplate) Type() string {
	return "LaunchTemplates"
}

func (lt *LaunchTemplate) GetExpired(ctx context.Context, expirationTime time.Time) (names []string, err error) {
	var nextToken *string
	for {
		out, err := lt.ec2Client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("tag-key"),
					Values: []string{karpenterLaunchTemplateTag},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return names, err
		}

		for _, launchTemplate := range out.LaunchTemplates {
			if lo.FromPtr(launchTemplate.CreateTime).Before(expirationTime) {
				names = append(names, lo.FromPtr(launchTemplate.LaunchTemplateName))
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return names, err
}

func (lt *LaunchTemplate) Get(ctx context.Context, clusterName string) (names []string, err error) {
	var nextToken *string
	for {
		out, err := lt.ec2Client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("tag:" + karpenterLaunchTemplateTag),
					Values: []string{clusterName},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return names, err
		}

		for _, launchTemplate := range out.LaunchTemplates {
			names = append(names, lo.FromPtr(launchTemplate.LaunchTemplateName))
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return names, err
}

// Cleanup any old launch templates that were managed by Karpenter and were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (lt *LaunchTemplate) Cleanup(ctx context.Context, names []string) ([]string, error) {
	var errs error
	deleted := []string{}
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

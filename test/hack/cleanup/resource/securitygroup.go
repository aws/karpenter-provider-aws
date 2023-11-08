package resource

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type SecurityGroup struct {
	ec2Client *ec2.Client
}

func NewSecurityGroup(ec2Client *ec2.Client) *SecurityGroup {
	return &SecurityGroup{ec2Client: ec2Client}
}

func (sg *SecurityGroup) Type() string {
	return "SecurityGroup"
}

func (sg *SecurityGroup) GetExpired(ctx context.Context, expirationTime time.Time) (ids []string, err error) {
	var nextToken *string
	for {
		out, err := sg.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("group-name"),
					Values: []string{"security-group-drift"},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return ids, err
		}

		for _, sgroup := range out.SecurityGroups {
			creationDate, found := lo.Find(sgroup.Tags, func(tag ec2types.Tag) bool {
				return *tag.Key == "creation-date"
			})
			if !found {
				continue
			}
			time, err := time.Parse(time.RFC3339, *creationDate.Value)
			if err != nil {
				continue
			}
			if time.Before(expirationTime) {
				ids = append(ids, lo.FromPtr(sgroup.GroupId))
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return ids, err
}

func (sg *SecurityGroup) Get(ctx context.Context, clusterName string) (ids []string, err error) {
	var nextToken *string

	for {
		out, err := sg.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("tag:" + karpenterSecurityGroupTag),
					Values: []string{clusterName},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return ids, err
		}

		for _, sgroup := range out.SecurityGroups {
			ids = append(ids, lo.FromPtr(sgroup.GroupId))
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return ids, err
}

func (sg *SecurityGroup) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	deleted := []string{}
	var errs error
	for i := range ids {
		_, err := sg.ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(ids[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		deleted = append(deleted, ids[i])
	}

	return deleted, errs
}

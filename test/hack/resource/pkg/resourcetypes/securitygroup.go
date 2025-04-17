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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"golang.org/x/exp/slices"
)

type SecurityGroup struct {
	ec2Client *ec2.Client
}

func NewSecurityGroup(ec2Client *ec2.Client) *SecurityGroup {
	return &SecurityGroup{ec2Client: ec2Client}
}

func (sg *SecurityGroup) String() string {
	return "SecurityGroup"
}

func (sg *SecurityGroup) Global() bool {
	return false
}

func (sg *SecurityGroup) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (ids []string, err error) {
	sgs, err := sg.getAllSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("group-name"),
				Values: []string{"security-group-drift"},
			},
		},
	})
	if err != nil {
		return ids, err
	}

	for _, sgroup := range sgs {
		clusterName, found := lo.Find(sgroup.Tags, func(tag ec2types.Tag) bool {
			return *tag.Key == karpenterTestingTag
		})
		if found && slices.Contains(excludedClusters, lo.FromPtr(clusterName.Value)) {
			continue
		}
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

	return ids, err
}

func (sg *SecurityGroup) CountAll(ctx context.Context) (count int, err error) {
	sgs, err := sg.getAllSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return count, err
	}

	return len(sgs), err
}

func (sg *SecurityGroup) Get(ctx context.Context, clusterName string) (ids []string, err error) {
	sgs, err := sg.getAllSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("tag:" + karpenterSecurityGroupTag),
				Values: []string{clusterName},
			},
		},
	})
	if err != nil {
		return ids, err
	}

	for _, sgroup := range sgs {
		ids = append(ids, lo.FromPtr(sgroup.GroupId))
	}

	return ids, err
}

// Cleanup any old security groups that were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (sg *SecurityGroup) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	var deleted []string
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

func (sg *SecurityGroup) getAllSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput) (sgs []ec2types.SecurityGroup, err error) {
	paginator := ec2.NewDescribeSecurityGroupsPaginator(sg.ec2Client, params)

	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		sgs = append(sgs, out.SecurityGroups...)
	}

	return sgs, nil
}

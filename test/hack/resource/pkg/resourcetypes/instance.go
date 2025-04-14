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

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"golang.org/x/exp/slices"
)

type Instance struct {
	ec2Client *ec2.Client
}

func NewInstance(ec2Client *ec2.Client) *Instance {
	return &Instance{ec2Client: ec2Client}
}

func (i *Instance) String() string {
	return "Instances"
}

func (i *Instance) Global() bool {
	return false
}

func (i *Instance) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (ids []string, err error) {
	instances, err := i.getAllInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("instance-state-name"),
				Values: []string{string(ec2types.InstanceStateNameRunning)},
			},
			{
				Name:   lo.ToPtr("tag-key"),
				Values: []string{karpenterNodePoolTag},
			},
		},
	})
	if err != nil {
		return ids, err
	}

	for _, instance := range instances {
		clusterName, found := lo.Find(instance.Tags, func(tag ec2types.Tag) bool {
			return *tag.Key == karpenterTestingTag
		})
		if found && slices.Contains(excludedClusters, lo.FromPtr(clusterName.Value)) {
			continue
		}
		if lo.FromPtr(instance.LaunchTime).Before(expirationTime) {
			ids = append(ids, lo.FromPtr(instance.InstanceId))
		}
	}

	return ids, err
}

func (i *Instance) CountAll(ctx context.Context) (count int, err error) {
	instances, err := i.getAllInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name: lo.ToPtr("instance-state-name"),
				Values: []string{
					string(ec2types.InstanceStateNameRunning),
					string(ec2types.InstanceStateNamePending),
					string(ec2types.InstanceStateNameShuttingDown),
					string(ec2types.InstanceStateNameStopped),
					string(ec2types.InstanceStateNameStopping),
				},
			},
		},
	})
	if err != nil {
		return count, err
	}

	return len(instances), err
}

func (i *Instance) Get(ctx context.Context, clusterName string) (ids []string, err error) {
	instances, err := i.getAllInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("instance-state-name"),
				Values: []string{string(ec2types.InstanceStateNameRunning)},
			},
			{
				Name:   lo.ToPtr("tag:" + karpenterClusterNameTag),
				Values: []string{clusterName},
			},
		},
	})
	if err != nil {
		return ids, err
	}

	for _, instance := range instances {
		ids = append(ids, lo.FromPtr(instance.InstanceId))
	}

	return ids, err
}

// Cleanup any old instances that were managed by Karpenter or were provisioned as part of testing
func (i *Instance) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	// The maximum number of EC2 instances which can be specified in a single API call
	const maxIDCount = 1000
	chunkedIDs := lo.Chunk(ids, maxIDCount)
	cleaned := make([]string, 0, len(ids))
	errs := make([]error, 0, len(chunkedIDs))
	for _, ids := range chunkedIDs {
		if _, err := i.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: ids,
		}); err != nil {
			errs = append(errs, err)
			continue
		}
		cleaned = append(cleaned, ids...)
	}
	return cleaned, multierr.Combine(errs...)
}

func (i *Instance) getAllInstances(ctx context.Context, params *ec2.DescribeInstancesInput) (instances []ec2types.Instance, err error) {
	paginator := ec2.NewDescribeInstancesPaginator(i.ec2Client, params)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return instances, err
		}

		for _, res := range page.Reservations {
			instances = append(instances, res.Instances...)
		}
	}

	return instances, nil
}

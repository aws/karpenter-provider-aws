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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

type ENI struct {
	ec2Client *ec2.Client
}

func NewENI(ec2Client *ec2.Client) *ENI {
	return &ENI{ec2Client: ec2Client}
}

func (e *ENI) String() string {
	return "ElasticNetworkInterface"
}

func (e *ENI) Global() bool {
	return false
}

func (e *ENI) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (ids []string, err error) {
	enis, err := e.getAllENIs(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("tag-key"),
				Values: []string{k8sClusterTag},
			},
		},
	})
	if err != nil {
		return ids, err
	}

	for _, ni := range enis {
		clusterName, found := lo.Find(ni.TagSet, func(tag ec2types.Tag) bool {
			return *tag.Key == k8sClusterTag
		})
		if found && slices.Contains(excludedClusters, lo.FromPtr(clusterName.Value)) {
			continue
		}
		creationDate, found := lo.Find(ni.TagSet, func(tag ec2types.Tag) bool {
			return *tag.Key == "node.k8s.amazonaws.com/createdAt"
		})
		if !found {
			continue
		}
		creationTime, err := time.Parse(time.RFC3339, *creationDate.Value)
		if err != nil {
			continue
		}
		if ni.Status == ec2types.NetworkInterfaceStatusAvailable && creationTime.Before(expirationTime) {
			ids = append(ids, lo.FromPtr(ni.NetworkInterfaceId))
		}
	}

	return ids, err
}

func (e *ENI) CountAll(ctx context.Context) (count int, err error) {
	enis, err := e.getAllENIs(ctx, &ec2.DescribeNetworkInterfacesInput{})
	if err != nil {
		return 0, err
	}

	return len(enis), err
}

func (e *ENI) Get(ctx context.Context, clusterName string) (ids []string, err error) {
	enis, err := e.getAllENIs(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("tag:" + k8sClusterTag),
				Values: []string{clusterName},
			},
		},
	})
	if err != nil {
		return ids, err
	}

	for _, ni := range enis {
		ids = append(ids, lo.FromPtr(ni.NetworkInterfaceId))
	}
	return ids, err
}

// Cleanup any old ENIs that were managed by Karpenter or were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (e *ENI) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	var deleted []string
	var errs error
	for i := range ids {
		_, err := e.ec2Client.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: aws.String(ids[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		deleted = append(deleted, ids[i])
	}

	return deleted, errs
}

func (e *ENI) getAllENIs(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput) (enis []ec2types.NetworkInterface, err error) {
	paginator := ec2.NewDescribeNetworkInterfacesPaginator(e.ec2Client, params)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return enis, err
		}
		enis = append(enis, page.NetworkInterfaces...)
	}

	return enis, nil
}

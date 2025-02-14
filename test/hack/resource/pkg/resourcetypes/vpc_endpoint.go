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
)

type VPCEndpoint struct {
	ec2Client *ec2.Client
}

func NewVPCEndpoint(ec2Client *ec2.Client) *VPCEndpoint {
	return &VPCEndpoint{ec2Client: ec2Client}
}

func (v *VPCEndpoint) String() string {
	return "VPCEndpoints"
}

func (v *VPCEndpoint) Global() bool {
	return false
}

func (v *VPCEndpoint) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (ids []string, err error) {
	endpoints, err := v.getAllVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("tag-key"),
				Values: []string{karpenterTestingTag},
			},
		},
	})
	if err != nil {
		return ids, err
	}

	for _, endpoint := range endpoints {
		clusterName, found := lo.Find(endpoint.Tags, func(tag ec2types.Tag) bool {
			return *tag.Key == k8sClusterTag
		})
		if found && slices.Contains(excludedClusters, lo.FromPtr(clusterName.Value)) {
			continue
		}
		if endpoint.CreationTimestamp.Before(expirationTime) {
			ids = append(ids, lo.FromPtr(endpoint.VpcEndpointId))
		}
	}

	return ids, err
}

func (v *VPCEndpoint) CountAll(ctx context.Context) (count int, err error) {
	endpoints, err := v.getAllVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{})
	if err != nil {
		return count, err
	}

	return len(endpoints), err
}

func (v *VPCEndpoint) Get(ctx context.Context, clusterName string) (ids []string, err error) {
	endpoints, err := v.getAllVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
		Filters: []ec2types.Filter{
			{
				Name:   lo.ToPtr("tag:" + karpenterTestingTag),
				Values: []string{clusterName},
			},
		},
	})
	if err != nil {
		return ids, err
	}

	for _, endpoint := range endpoints {
		ids = append(ids, lo.FromPtr(endpoint.VpcEndpointId))
	}

	return ids, err
}

// Cleanup any old VPC endpoints that were provisioned as part of testing
func (v *VPCEndpoint) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	if _, err := v.ec2Client.DeleteVpcEndpoints(ctx, &ec2.DeleteVpcEndpointsInput{
		VpcEndpointIds: ids,
	}); err != nil {
		return nil, err
	}
	return ids, nil
}

func (v *VPCEndpoint) getAllVpcEndpoints(ctx context.Context, params *ec2.DescribeVpcEndpointsInput) (endpoints []ec2types.VpcEndpoint, err error) {
	paginator := ec2.NewDescribeVpcEndpointsPaginator(v.ec2Client, params)

	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return endpoints, err
		}
		endpoints = append(endpoints, out.VpcEndpoints...)
	}

	return endpoints, nil
}

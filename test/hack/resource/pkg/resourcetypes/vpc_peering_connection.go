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

type VPCPeeringConnection struct {
	ec2Client *ec2.Client
}

func NewVPCPeeringConnection(ec2Client *ec2.Client) *VPCPeeringConnection {
	return &VPCPeeringConnection{ec2Client: ec2Client}
}

func (v *VPCPeeringConnection) String() string {
	return "VPCPeeringConnection"
}

func (v *VPCPeeringConnection) Global() bool {
	return false
}

func (v *VPCPeeringConnection) GetExpired(ctx context.Context, expirationTime time.Time, excludedClusters []string) (ids []string, err error) {
	connections, err := v.getAllVpcPeeringConnections(ctx, &ec2.DescribeVpcPeeringConnectionsInput{
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

	for _, connection := range connections {
		clusterName, found := lo.Find(connection.Tags, func(tag ec2types.Tag) bool {
			return *tag.Key == k8sClusterTag
		})
		if found && slices.Contains(excludedClusters, lo.FromPtr(clusterName.Value)) {
			continue
		}
		if connection.ExpirationTime == nil { // Connection is non-pending
			continue
		}
		if connection.ExpirationTime.Before(expirationTime) {
			ids = append(ids, lo.FromPtr(connection.VpcPeeringConnectionId))
		}
	}

	return ids, err
}

func (v *VPCPeeringConnection) CountAll(ctx context.Context) (count int, err error) {
	connections, err := v.getAllVpcPeeringConnections(ctx, &ec2.DescribeVpcPeeringConnectionsInput{})
	if err != nil {
		return count, err
	}

	return len(connections), err
}

func (v *VPCPeeringConnection) Get(ctx context.Context, clusterName string) (ids []string, err error) {
	connections, err := v.getAllVpcPeeringConnections(ctx, &ec2.DescribeVpcPeeringConnectionsInput{
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

	for _, connection := range connections {
		ids = append(ids, lo.FromPtr(connection.VpcPeeringConnectionId))
	}

	return ids, err
}

// Cleanup any old VPC peering connections that were provisioned as part of testing
func (v *VPCPeeringConnection) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	for _, id := range ids {
		if _, err := v.ec2Client.DeleteVpcPeeringConnection(ctx, &ec2.DeleteVpcPeeringConnectionInput{
			VpcPeeringConnectionId: lo.ToPtr(id),
		}); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func (v *VPCPeeringConnection) getAllVpcPeeringConnections(ctx context.Context, params *ec2.DescribeVpcPeeringConnectionsInput) (connections []ec2types.VpcPeeringConnection, err error) {
	paginator := ec2.NewDescribeVpcPeeringConnectionsPaginator(v.ec2Client, params)

	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		connections = append(connections, out.VpcPeeringConnections...)
	}

	return connections, nil
}

package resource

import (
	"context"
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

func (v *VPCEndpoint) Type() string {
	return "VPCEndpoints"
}

func (v *VPCEndpoint) Get(ctx context.Context, clusterName string) (ids []string, err error) {
	var nextToken *string
	for {
		out, err := v.ec2Client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("tag:" + karpenterTestingTag),
					Values: []string{clusterName},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return ids, err
		}
		for _, endpoint := range out.VpcEndpoints {
			ids = append(ids, lo.FromPtr(endpoint.VpcEndpointId))
		}
		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return ids, err
}

func (v *VPCEndpoint) GetExpired(ctx context.Context, expirationTime time.Time) (ids []string, err error) {
	var nextToken *string
	for {
		out, err := v.ec2Client.DescribeVpcEndpoints(ctx, &ec2.DescribeVpcEndpointsInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("tag-key"),
					Values: []string{karpenterTestingTag},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return ids, err
		}
		for _, endpoint := range out.VpcEndpoints {
			if endpoint.CreationTimestamp.Before(expirationTime) {
				ids = append(ids, lo.FromPtr(endpoint.VpcEndpointId))
			}
		}
		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return ids, err
}

func (v *VPCEndpoint) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	if _, err := v.ec2Client.DeleteVpcEndpoints(ctx, &ec2.DeleteVpcEndpointsInput{
		VpcEndpointIds: ids,
	}); err != nil {
		return nil, err
	}
	return ids, nil
}

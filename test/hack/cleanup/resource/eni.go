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

type ENI struct {
	ec2Client *ec2.Client
}

func NewENI(ec2Client *ec2.Client) *ENI {
	return &ENI{ec2Client: ec2Client}
}

func (e *ENI) Type() string {
	return "ElasticNetworkInterface"
}

func (e *ENI) GetExpired(ctx context.Context, expirationTime time.Time) (ids []string, err error) {
	var nextToken *string
	for {
		out, err := e.ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("tag-key"),
					Values: []string{k8sClusterTag},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return ids, err
		}

		for _, ni := range out.NetworkInterfaces {
			creationDate, found := lo.Find(ni.TagSet, func(tag ec2types.Tag) bool {
				return *tag.Key == "node.k8s.amazonaws.com/createdAt"
			})
			if !found {
				continue
			}
			time, err := time.Parse(time.RFC3339, *creationDate.Value)
			if err != nil {
				continue
			}
			if ni.Status == ec2types.NetworkInterfaceStatusAvailable && time.Before(expirationTime) {
				ids = append(ids, lo.FromPtr(ni.NetworkInterfaceId))
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return ids, err
}

func (e *ENI) Get(ctx context.Context, clusterName string) (ids []string, err error) {
	var nextToken *string
	for {
		out, err := e.ec2Client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("tag:" + k8sClusterTag),
					Values: []string{clusterName},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return ids, err
		}

		for _, ni := range out.NetworkInterfaces {
			ids = append(ids, lo.FromPtr(ni.NetworkInterfaceId))
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return ids, err
}

func (e *ENI) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	deleted := []string{}
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

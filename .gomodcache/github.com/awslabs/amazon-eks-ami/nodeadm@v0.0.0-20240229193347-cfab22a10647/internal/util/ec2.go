package util

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EniInfo struct {
	EniCount        int32
	PodsPerEniCount int32
}

type EC2API interface {
	DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)
}

type EC2Client struct {
	Client *ec2.Client
}

func (c *EC2Client) DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	return c.Client.DescribeInstanceTypes(ctx, params, optFns...)
}

func GetEniInfoForInstanceType(ec2API EC2API, instanceType string) (EniInfo, error) {
	describeResp, err := ec2API.DescribeInstanceTypes(context.Background(), &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []types.InstanceType{types.InstanceType(instanceType)},
	})

	if err != nil {
		return EniInfo{}, fmt.Errorf("error describing instance type %s: %w", instanceType, err)
	}

	if len(describeResp.InstanceTypes) > 0 {
		instanceTypeInfo := describeResp.InstanceTypes[0]
		return EniInfo{
			EniCount:        *instanceTypeInfo.NetworkInfo.MaximumNetworkInterfaces,
			PodsPerEniCount: *instanceTypeInfo.NetworkInfo.Ipv4AddressesPerInterface,
		}, nil
	}
	return EniInfo{}, fmt.Errorf("no instance found for type: %s", instanceType)
}

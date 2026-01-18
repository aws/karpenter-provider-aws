package util

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type MockEC2Client struct {
	mock.Mock
}

func (m *MockEC2Client) DescribeInstanceTypes(ctx context.Context, params *ec2.DescribeInstanceTypesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*ec2.DescribeInstanceTypesOutput), args.Error(1)
}

func TestGetEniInfoForInstanceType(t *testing.T) {
	tests := []struct {
		instanceType   string
		expectedResult EniInfo
		mockResponse   ec2.DescribeInstanceTypesOutput
		mockError      error
		expectedError  error
	}{
		{
			instanceType: "t3.medium",
			expectedResult: EniInfo{
				EniCount:        int32(3),
				PodsPerEniCount: int32(6),
			},
			mockResponse: ec2.DescribeInstanceTypesOutput{
				InstanceTypes: []types.InstanceTypeInfo{
					{
						InstanceType: "t3.medium",
						NetworkInfo: &types.NetworkInfo{
							MaximumNetworkInterfaces:  aws.Int32(3),
							Ipv4AddressesPerInterface: aws.Int32(6),
						},
					},
				},
			},
			mockError:     nil,
			expectedError: nil,
		},
		{
			instanceType:   "t3.medium",
			expectedResult: EniInfo{},
			mockResponse: ec2.DescribeInstanceTypesOutput{
				InstanceTypes: []types.InstanceTypeInfo{},
			},
			mockError:     nil,
			expectedError: fmt.Errorf("no instance found for type: t3.medium"),
		},
		{
			instanceType:   "mock-type.large",
			expectedResult: EniInfo{},
			mockResponse: ec2.DescribeInstanceTypesOutput{
				InstanceTypes: []types.InstanceTypeInfo{},
			},
			mockError:     fmt.Errorf("invalid instance type"),
			expectedError: fmt.Errorf("error describing instance type mock-type.large: %w", errors.New("invalid instance type")),
		},
	}

	for _, test := range tests {
		mockEC2 := &MockEC2Client{}
		mockEC2.On("DescribeInstanceTypes", mock.Anything, mock.AnythingOfType("*ec2.DescribeInstanceTypesInput")).Return(&test.mockResponse, test.mockError)

		result, err := GetEniInfoForInstanceType(mockEC2, test.instanceType)
		assert.Equal(t, test.expectedError, err)
		assert.Equal(t, test.expectedResult, result)
	}

}

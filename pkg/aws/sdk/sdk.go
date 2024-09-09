package sdk

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type EC2API interface {
	// EC2 Methods
	DescribeImages(context.Context, *ec2.DescribeImagesInput, ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	DescribeImagesPages(context.Context, *ec2.DescribeImagesInput, func(*ec2.DescribeImagesOutput, bool) bool) error
	DescribeLaunchTemplates(context.Context, *ec2.DescribeLaunchTemplatesInput, ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error)
	DescribeLaunchTemplatesPages(context.Context, *ec2.DescribeLaunchTemplatesInput, func(*ec2.DescribeLaunchTemplatesOutput, bool) bool) error
	DescribeSubnets(context.Context, *ec2.DescribeSubnetsInput, ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeSecurityGroups(context.Context, *ec2.DescribeSecurityGroupsInput, ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeInstanceTypes(context.Context, *ec2.DescribeInstanceTypesInput, ...func(*ec2.Options)) (*ec2.DescribeInstanceTypesOutput, error)
	DescribeInstanceTypesPages(context.Context, *ec2.DescribeInstanceTypesInput, func(*ec2.DescribeInstanceTypesOutput, bool) bool) error
	DescribeInstanceTypeOfferings(context.Context, *ec2.DescribeInstanceTypeOfferingsInput, ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error)
	DescribeInstanceTypeOfferingsPages(context.Context, *ec2.DescribeInstanceTypeOfferingsInput, func(*ec2.DescribeInstanceTypeOfferingsOutput, bool) bool) error
	DescribeAvailabilityZones(context.Context, *ec2.DescribeAvailabilityZonesInput, ...func(*ec2.Options)) (*ec2.DescribeAvailabilityZonesOutput, error)
	DescribeSpotPriceHistory(context.Context, *ec2.DescribeSpotPriceHistoryInput, ...func(*ec2.Options)) (*ec2.DescribeSpotPriceHistoryOutput, error)
	CreateFleet(context.Context, *ec2.CreateFleetInput, ...func(*ec2.Options)) (*ec2.CreateFleetOutput, error)
	TerminateInstances(context.Context, *ec2.TerminateInstancesInput, ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	CreateTags(context.Context, *ec2.CreateTagsInput, ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	DescribeInstancesPages(context.Context, *ec2.DescribeInstancesInput, func(*ec2.DescribeInstancesOutput, bool) bool) error
	CreateLaunchTemplate(context.Context, *ec2.CreateLaunchTemplateInput, ...func(*ec2.Options)) (*ec2.CreateLaunchTemplateOutput, error)
	DeleteLaunchTemplate(context.Context, *ec2.DeleteLaunchTemplateInput, ...func(*ec2.Options)) (*ec2.DeleteLaunchTemplateOutput, error)
	ModifyLaunchTemplate(context.Context, *ec2.ModifyLaunchTemplateInput, ...func(*ec2.Options)) (*ec2.ModifyLaunchTemplateOutput, error)
}

type SQSAPI interface {
	// SQS Methods
	SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	GetQueueUrl(context.Context, *sqs.GetQueueUrlInput, ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)
	ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	CreateQueue(context.Context, *sqs.CreateQueueInput, ...func(*sqs.Options)) (*sqs.CreateQueueOutput, error)
	DeleteQueue(context.Context, *sqs.DeleteQueueInput, ...func(*sqs.Options)) (*sqs.DeleteQueueOutput, error)
}
type IAMAPI interface {
	// IAM Methods
	Reset()
	GetInstanceProfile(context.Context, *iam.GetInstanceProfileInput, ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error)
	CreateInstanceProfile(context.Context, *iam.CreateInstanceProfileInput, ...func(*iam.Options)) (*iam.CreateInstanceProfileOutput, error)
	DeleteInstanceProfile(context.Context, *iam.DeleteInstanceProfileInput, ...func(*iam.Options)) (*iam.DeleteInstanceProfileOutput, error)
	AddRoleToInstanceProfile(context.Context, *iam.AddRoleToInstanceProfileInput, ...func(*iam.Options)) (*iam.AddRoleToInstanceProfileOutput, error)
	TagInstanceProfile(context.Context, *iam.TagInstanceProfileInput, ...func(*iam.Options)) (*iam.TagInstanceProfileOutput, error)
	RemoveRoleFromInstanceProfile(context.Context, *iam.RemoveRoleFromInstanceProfileInput, ...func(*iam.Options)) (*iam.RemoveRoleFromInstanceProfileOutput, error)
	UntagInstanceProfile(context.Context, *iam.UntagInstanceProfileInput, ...func(*iam.Options)) (*iam.UntagInstanceProfileOutput, error)
}

type EKSAPI interface {
	// EKS Methods
	DescribeCluster(context.Context, *eks.DescribeClusterInput, ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	Reset()
}

type PricingAPI interface {
	// Pricing Methods
	GetProductsPages(context.Context, *pricing.GetProductsInput, func(*pricing.GetProductsOutput, bool) bool, ...func(*pricing.Options)) error
}

type SSMAPI interface {
	// SSM Methods
	GetParametersByPathPages(context.Context, *ssm.GetParametersByPathInput, func(*ssm.GetParametersByPathOutput, bool) bool) error
}

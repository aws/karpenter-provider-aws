package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type AWSClient struct {
	EC2Client *ec2.Client
	SQSClient *sqs.Client
	IAMClient *iam.Client
	EKSClient *eks.Client
	PricingClient *pricing.Client
	SSMClient *ssm.Client
}

func NewAWSClient(ctx context.Context) *AWSClient {
	cfg := LoadDefaultConfig(ctx)
	return &AWSClient{
		EC2Client: ec2.NewFromConfig(cfg),
		SQSClient: sqs.NewFromConfig(cfg),
		IAMClient: iam.NewFromConfig(cfg),
		EKSClient: eks.NewFromConfig(cfg),
		PricingClient: pricing.NewFromConfig(cfg),
		SSMClient: ssm.NewFromConfig(cfg),
	}
}

func (c *AWSClient) DescribeInstances(ctx context.Context) (*ec2.DescribeInstancesResponse, error) {
	return c.EC2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
}

func (c *AWSClient) CreateFleet(ctx context.Context, input *ec2.CreateFleetInput) (*ec2.CreateFleetResponse, error) {
	return c.EC2Client.CreateFleet(ctx, input)
}

func (c *AWSClient) TerminateInstances(ctx context.Context, instanceIds []string) (*ec2.TerminateInstancesResponse, error) {
	return c.EC2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: instanceIds})
}

func (c *AWSClient) DescribeImages(ctx context.Context, imageIds []string) (*ec2.DescribeImagesResponse, error) {
	return c.EC2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{ImageIds: imageIds})
}

func (c *AWSClient) DescribeLaunchTemplates(ctx context.Context) (*ec2.DescribeLaunchTemplatesResponse, error) {
	return c.EC2Client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{})
}

func (c *AWSClient) DescribeSubnets(ctx context.Context) (*ec2.DescribeSubnetsResponse, error) {
	return c.EC2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
}

func (c *AWSClient) DescribeSecurityGroups(ctx context.Context) (*ec2.DescribeSecurityGroupsResponse, error) {
	return c.EC2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
}

func (c *AWSClient) DescribeInstanceTypes(ctx context.Context) (*ec2.DescribeInstanceTypesResponse, error) {
	return c.EC2Client.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
}

func (c *AWSClient) DescribeInstanceTypeOfferings(ctx context.Context) (*ec2.DescribeInstanceTypeOfferingsResponse, error) {
	return c.EC2Client.DescribeInstanceTypeOfferings(ctx, &ec2.DescribeInstanceTypeOfferingsInput{})
}

func (c *AWSClient) DescribeAvailabilityZones(ctx context.Context) (*ec2.DescribeAvailabilityZonesResponse, error) {
	return c.EC2Client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})
}

func (c *AWSClient) DescribeSpotPriceHistory(ctx context.Context, input *ec2.DescribeSpotPriceHistoryInput) (*ec2.DescribeSpotPriceHistoryResponse, error) {
	return c.EC2Client.DescribeSpotPriceHistory(ctx, input)
}

func (c *AWSClient) CreateTags(ctx context.Context, input *ec2.CreateTagsInput) (*ec2.CreateTagsResponse, error) {
	return c.EC2Client.CreateTags(ctx, input)
}

func (c *AWSClient) DescribeInstancesPages(ctx context.Context, input *ec2.DescribeInstancesInput, fn func(*ec2.DescribeInstancesResponse, bool) bool) error {
	return c.EC2Client.DescribeInstancesPages(ctx, input, fn)
}

func (c *AWSClient) CreateLaunchTemplate(ctx context.Context, input *ec2.CreateLaunchTemplateInput) (*ec2.CreateLaunchTemplateResponse, error) {
	return c.EC2Client.CreateLaunchTemplate(ctx, input)
}

func (c *AWSClient) ModifyLaunchTemplate(ctx context.Context, input *ec2.ModifyLaunchTemplateInput) (*ec2.ModifyLaunchTemplateResponse, error) {
	return c.EC2Client.ModifyLaunchTemplate(ctx, input)
}

func (c *AWSClient) DescribeLaunchTemplates(ctx context.Context, input *ec2.DescribeLaunchTemplatesInput) (*ec2.DescribeLaunchTemplatesResponse, error) {
	return c.EC2Client.DescribeLaunchTemplates(ctx, input)
}

func (c *AWSClient) DeleteLaunchTemplate(ctx context.Context, input *ec2.DeleteLaunchTemplateInput) (*ec2.DeleteLaunchTemplateResponse, error) {
	return c.EC2Client.DeleteLaunchTemplate(ctx, input)
}

func (c *AWSClient) GetQueueUrl(ctx context.Context, queueName string) (*sqs.GetQueueUrlResponse, error) {
	return c.SQSClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: &queueName})
}

func (c *AWSClient) SendMessage(ctx context.Context, input *sqs.SendMessageInput) (*sqs.SendMessageResponse, error) {
	return c.SQSClient.SendMessage(ctx, input)
}

func (c *AWSClient) ReceiveMessage(ctx context.Context, input *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageResponse, error) {
	return c.SQSClient.ReceiveMessage(ctx, input)
}

func (c *AWSClient) DeleteMessage(ctx context.Context, input *sqs.DeleteMessageInput) (*sqs.DeleteMessageResponse, error) {
	return c.SQSClient.DeleteMessage(ctx, input)
}

func (c *AWSClient) CreateQueue(ctx context.Context, input *sqs.CreateQueueInput) (*sqs.CreateQueueResponse, error) {
	return c.SQSClient.CreateQueue(ctx, input)
}

func (c *AWSClient) DeleteQueue(ctx context.Context, input *sqs.DeleteQueueInput) (*sqs.DeleteQueueResponse, error) {
	return c.SQSClient.DeleteQueue(ctx, input)
}

func (c *AWSClient) GetUser(ctx context.Context) (*iam.GetUserResponse, error) {
	return c.IAMClient.GetUser(ctx, &iam.GetUserInput{})
}

func (c *AWSClient) GetInstanceProfile(ctx context.Context, instanceProfileName string) (*iam.GetInstanceProfileResponse, error) {
	return c.IAMClient.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: &instanceProfileName})
}

func (c *AWSClient) CreateInstanceProfile(ctx context.Context, instanceProfileName string) (*iam.CreateInstanceProfileResponse, error) {
	return c.IAMClient.CreateInstanceProfile(ctx, &iam.CreateInstanceProfileInput{InstanceProfileName: &instanceProfileName})
}

func (c *AWSClient) AddRoleToInstanceProfile(ctx context.Context, instanceProfileName, roleName string) (*iam.AddRoleToInstanceProfileResponse, error) {
	return c.IAMClient.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{InstanceProfileName: &instanceProfileName, RoleName: &roleName})
}

func (c *AWSClient) DeleteInstanceProfile(ctx context.Context, instanceProfileName string) (*iam.DeleteInstanceProfileResponse, error) {
	return c.IAMClient.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{InstanceProfileName: &instanceProfileName})
}

func (c *AWSClient) TagInstanceProfile(ctx context.Context, instanceProfileName string, tags []iam.Tag) (*iam.TagInstanceProfileResponse, error) {
	return c.IAMClient.TagInstanceProfile(ctx, &iam.TagInstanceProfileInput{InstanceProfileName: &instanceProfileName, Tags: tags})
}

func (c *AWSClient) UntagInstanceProfile(ctx context.Context, instanceProfileName string, tagKeys []string) (*iam.UntagInstanceProfileResponse, error) {
	return c.IAMClient.UntagInstanceProfile(ctx, &iam.UntagInstanceProfileInput{InstanceProfileName: &instanceProfileName, TagKeys: tagKeys})
}

func (c *AWSClient) RemoveRoleFromInstanceProfile(ctx context.Context, instanceProfileName, roleName string) (*iam.RemoveRoleFromInstanceProfileResponse, error) {
	return c.IAMClient.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{InstanceProfileName: &instanceProfileName, RoleName: &roleName})
}

func (c *AWSClient) DescribeCluster(ctx context.Context, clusterName string) (*eks.DescribeClusterResponse, error) {
	return c.EKSClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &clusterName})
}

func (c *AWSClient) GetProductsPages(ctx context.Context, input *pricing.GetProductsInput, fn func(*pricing.GetProductsResponse, bool) bool) error {
	return c.PricingClient.GetProductsPages(ctx, input, fn)
}

func (c *AWSClient) GetParametersByPathPages(ctx context.Context, input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathResponse, bool) bool) error {
	return c.SSMClient.GetParametersByPathPages(ctx, input, fn)
}


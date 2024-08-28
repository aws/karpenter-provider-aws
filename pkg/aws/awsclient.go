package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type EC2Client struct {
	EC2Client *ec2.Client
}

type SQSClient struct {
	SQSClient *sqs.Client
}

type IAMClient struct {
	IAMClient *iam.Client
}

type EKSClient struct {
	EKSClient *eks.Client
}

type PricingClient struct {
	PricingClient *pricing.Client
}

type SSMClient struct {
	SSMClient *ssm.Client
}

func NewEC2Client(ctx context.Context) *ec2.Client {
	cfg := LoadDefaultConfig(ctx)
	return ec2.NewFromConfig(cfg)
}

func NewSQSClient(ctx context.Context) *sqs.Client {
	cfg := LoadDefaultConfig(ctx)
	return sqs.NewFromConfig(cfg)
}

func NewIAMClient(ctx context.Context) *iam.Client {
	cfg := LoadDefaultConfig(ctx)
	return iam.NewFromConfig(cfg)
}

func NewEKSClient(ctx context.Context) *eks.Client {
	cfg := LoadDefaultConfig(ctx)
	return eks.NewFromConfig(cfg)
}

func NewPricingClient(ctx context.Context) *pricing.Client {
	cfg := LoadDefaultConfig(ctx)
	return pricing.NewFromConfig(cfg)
}

func NewSSMClient(ctx context.Context) *ssm.Client {
	cfg := LoadDefaultConfig(ctx)
	return ssm.NewFromConfig(cfg)
}

func DescribeInstances(ctx context.Context, client *ec2.Client) (*ec2.DescribeInstancesOutput, error) {
	return client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
}

func CreateFleet(ctx context.Context, client *ec2.Client) (*ec2.CreateFleetResponse, error) {
	return client.CreateFleet(ctx, input)
}

func TerminateInstances(ctx context.Context, client *ec2.Client, instanceIds []string) (*ec2.TerminateInstancesOutput, error) {
	return client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: instanceIds})
}

func DescribeImages(ctx context.Context, client *ec2.Client, imageIds []string) (*ec2.DescribeImagesOutput, error) {
	return client.DescribeImages(ctx, &ec2.DescribeImagesInput{ImageIds: imageIds})
}

func DescribeLaunchTemplates(ctx context.Context, client *ec2.Client) (*ec2.DescribeLaunchTemplatesOutput, error) {
	return client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{})
}

func DescribeSubnets(ctx context.Context, client *ec2.Client) (*ec2.DescribeSubnetsOutput, error) {
	return client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
}

func DescribeSecurityGroups(ctx context.Context, client *ec2.Client) (*ec2.DescribeSecurityGroupsOutput, error) {
	return client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
}

func DescribeInstanceTypes(ctx context.Context, client *ec2.Client) (*ec2.DescribeInstanceTypesOutput, error) {
	return client.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{})
}

func DescribeInstanceTypeOfferings(ctx context.Context, client *ec2.Client) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
	return client.DescribeInstanceTypeOfferings(ctx, &ec2.DescribeInstanceTypeOfferingsInput{})
}

func DescribeAvailabilityZones(ctx context.Context, client *ec2.Client) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return client.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})
}

func DescribeSpotPriceHistory(ctx context.Context, client *ec2.Client, input *ec2.DescribeSpotPriceHistoryInput) (*ec2.DescribeSpotPriceHistoryOutput, error) {
	return client.DescribeSpotPriceHistory(ctx, input)
}

func CreateTags(ctx context.Context, client *ec2.Client, input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return client.CreateTags(ctx, input)
}

func DescribeInstancesPages(ctx context.Context, client *ec2.Client, input *ec2.DescribeInstancesInput, fn func(*ec2.DescribeInstancesOutput, bool) bool) error {
	return client.DescribeInstancesPages(ctx, input, fn)
}

func CreateLaunchTemplate(ctx context.Context, client *ec2.Client, input *ec2.CreateLaunchTemplateInput) (*ec2.CreateLaunchTemplateOutput, error) {
	return client.CreateLaunchTemplate(ctx, input)
}

func ModifyLaunchTemplate(ctx context.Context, client *ec2.Client, input *ec2.ModifyLaunchTemplateInput) (*ec2.ModifyLaunchTemplateOutput, error) {
	return client.ModifyLaunchTemplate(ctx, input)
}

func DescribeLaunchTemplates(ctx context.Context, client *ec2.Client, input *ec2.DescribeLaunchTemplatesInput) (*ec2.DescribeLaunchTemplatesOutput, error) {
	return client.DescribeLaunchTemplates(ctx, input)
}

func DeleteLaunchTemplate(ctx context.Context, client *ec2.Client, input *ec2.DeleteLaunchTemplateInput) (*ec2.DeleteLaunchTemplateOutput, error) {
	return client.DeleteLaunchTemplate(ctx, input)
}

func GetQueueUrl(ctx context.Context, client *sqs.Client, queueName string) (*sqs.GetQueueUrlOutput, error) {
	return client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: &queueName})
}

func SendMessage(ctx context.Context, client *sqs.Client, input *sqs.SendMessageInput) (*sqs.SendMessageOutput, error) {
	return client.SendMessage(ctx, input)
}

func ReceiveMessage(ctx context.Context, client *sqs.Client, input *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
	return client.ReceiveMessage(ctx, input)
}

func DeleteMessage(ctx context.Context, client *sqs.Client, input *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
	return client.DeleteMessage(ctx, input)
}

func CreateQueue(ctx context.Context, client *sqs.Client, input *sqs.CreateQueueInput) (*sqs.CreateQueueOutput, error) {
	return client.CreateQueue(ctx, input)
}

func DeleteQueue(ctx context.Context, client *sqs.Client, input *sqs.DeleteQueueInput) (*sqs.DeleteQueueOutput, error) {
	return client.DeleteQueue(ctx, input)
}

func GetUser(ctx context.Context, client *iam.Client) (*iam.GetUserOutput, error) {
	return client.GetUser(ctx, &iam.GetUserInput{})
}

func GetInstanceProfile(ctx context.Context, client *iam.Client, instanceProfileName string) (*iam.GetInstanceProfileOutput, error) {
	return client.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: &instanceProfileName})
}

func CreateInstanceProfile(ctx context.Context, client *iam.Client, instanceProfileName string) (*iam.CreateInstanceProfileOutput, error) {
	return client.CreateInstanceProfile(ctx, &iam.CreateInstanceProfileInput{InstanceProfileName: &instanceProfileName})
}

func AddRoleToInstanceProfile(ctx context.Context, client *iam.Client, instanceProfileName, roleName string) (*iam.AddRoleToInstanceProfileOutput, error) {
	return client.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{InstanceProfileName: &instanceProfileName, RoleName: &roleName})
}

func DeleteInstanceProfile(ctx context.Context, client *iam.Client, instanceProfileName string) (*iam.DeleteInstanceProfileOutput, error) {
	return client.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{InstanceProfileName: &instanceProfileName})
}

func TagInstanceProfile(ctx context.Context, client *iam.Client, instanceProfileName string, tags []iam.Tag) (*iam.TagInstanceProfileOutput, error) {
	return client.TagInstanceProfile(ctx, &iam.TagInstanceProfileInput{InstanceProfileName: &instanceProfileName, Tags: tags})
}

func UntagInstanceProfile(ctx context.Context, client *iam.Client, instanceProfileName string, tagKeys []string) (*iam.UntagInstanceProfileOutput, error) {
	return client.UntagInstanceProfile(ctx, &iam.UntagInstanceProfileInput{InstanceProfileName: &instanceProfileName, TagKeys: tagKeys})
}

func RemoveRoleFromInstanceProfile(ctx context.Context, client *iam.Client, instanceProfileName, roleName string) (*iam.RemoveRoleFromInstanceProfileOutput, error) {
	return client.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{InstanceProfileName: &instanceProfileName, RoleName: &roleName})
}

func DescribeCluster(ctx context.Context, client *eks.Client, clusterName string) (*eks.DescribeClusterOutput, error) {
	return client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: &clusterName})
}

func GetProductsPages(ctx context.Context, client *pricing.Client, input *pricing.GetProductsInput, fn func(*pricing.GetProductsOutput, bool) bool) error {
	return client.GetProductsPages(ctx, input, fn)
}

func GetParametersByPathPages(ctx context.Context, client *ssm.Client, input *ssm.GetParametersByPathInput, fn func(*ssm.GetParametersByPathOutput, bool) bool) error {
	return client.GetParametersByPathPages(ctx, input, fn)
}


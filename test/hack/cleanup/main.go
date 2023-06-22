package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	expirationTTL            = time.Hour * 12
	karpenterMetricNamespace = "testing.karpenter.sh/cleanup"

	karpenterProvisionerNameTag = "karpenter.sh/provisioner-name"
	karpenterLaunchTemplateTag  = "karpenter.k8s.aws/cluster"
	githubRunURLTag             = "github.com/run-url"
)

func main() {
	ctx := context.Background()
	cfg := lo.Must(config.LoadDefaultConfig(ctx))

	logger := lo.Must(zap.NewProduction()).Sugar()

	expirationTime := time.Now().Add(-expirationTTL)

	logger.With("expiration-time", expirationTime.String()).Infof("resolved expiration time for all resources")

	ec2Client := ec2.NewFromConfig(cfg)
	cloudFormationClient := cloudformation.NewFromConfig(cfg)
	cloudWatchClient := cloudwatch.NewFromConfig(cfg)

	// Terminate any old instances that were provisioned by Karpenter as part of testing
	// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
	ids := getOldInstances(ctx, ec2Client, expirationTime)
	logger.With("ids", ids, "count", len(ids)).Infof("discovered test instances to delete")
	if len(ids) > 0 {
		_ = lo.Must(ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: ids,
		}))
	}
	logger.With("ids", ids, "count", len(ids)).Infof("terminated test instances")
	lo.Must0(fireMetric(ctx, cloudWatchClient, "InstancesDeleted", float64(len(ids))))

	// Terminate any old stacks that were provisioned as part of testing
	// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
	names := getOldStacks(ctx, cloudFormationClient, expirationTime)
	logger.With("names", names, "count", len(names)).Infof("discovered test stacks to delete")
	errs := make([]error, len(names))
	for i := range names {
		_, errs[i] = cloudFormationClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: lo.ToPtr(names[i]),
		})
		logger.With("name", names[i]).Infof("deleted test stack")
	}
	lo.Must0(multierr.Combine(errs...))
	logger.With("names", names, "count", len(names)).Infof("terminated test stacks")
	lo.Must0(fireMetric(ctx, cloudWatchClient, "StacksDeleted", float64(len(names))))

	// Terminate any old launch templates that were managed by Karpenter and were provisioned as part of testing
	names = getOldLaunchTemplates(ctx, ec2Client, expirationTime)
	logger.With("names", names, "count", len(names)).Infof("discovered test launch templates to delete")
	errs = make([]error, len(names))
	for i := range names {
		_, errs[i] = ec2Client.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{
			LaunchTemplateName: lo.ToPtr(names[i]),
		})
		logger.With("name", names[i]).Infof("deleted test launch template")
	}
	lo.Must0(multierr.Combine(errs...))
	logger.With("names", names, "count", len(names)).Infof("terminated test launch templates")
	lo.Must0(fireMetric(ctx, cloudWatchClient, "LaunchTemplatesDeleted", float64(len(names))))
}

func fireMetric(ctx context.Context, cloudWatchClient *cloudwatch.Client, name string, value float64) error {
	_, err := cloudWatchClient.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace: lo.ToPtr(karpenterMetricNamespace),
		MetricData: []cloudwatchtypes.MetricDatum{
			{
				MetricName: lo.ToPtr(name),
				Value:      lo.ToPtr(value),
			},
		},
	})
	return err
}

func getOldInstances(ctx context.Context, ec2Client *ec2.Client, expirationTime time.Time) (ids []string) {
	var nextToken *string
	for {
		out := lo.Must(ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("instance-state-name"),
					Values: []string{string(ec2types.InstanceStateNameRunning)},
				},
				{
					Name:   lo.ToPtr("tag-key"),
					Values: []string{karpenterProvisionerNameTag},
				},
			},
			NextToken: nextToken,
		}))

		for _, res := range out.Reservations {
			for _, instance := range res.Instances {
				if _, found := lo.Find(instance.Tags, func(t ec2types.Tag) bool {
					return lo.FromPtr(t.Key) == "kubernetes.io/cluster/KITInfrastructure"
				}); !found && lo.FromPtr(instance.LaunchTime).Before(expirationTime) {
					ids = append(ids, lo.FromPtr(instance.InstanceId))
				}
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return ids
}

func getOldStacks(ctx context.Context, cloudFormationClient *cloudformation.Client, expirationTime time.Time) (names []string) {
	var nextToken *string
	for {
		out := lo.Must(cloudFormationClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
			NextToken: nextToken,
		}))

		for _, stack := range out.Stacks {
			if _, found := lo.Find(stack.Tags, func(t cloudformationtypes.Tag) bool {
				return lo.FromPtr(t.Key) == githubRunURLTag
			}); found && lo.FromPtr(stack.CreationTime).Before(expirationTime) {
				names = append(names, lo.FromPtr(stack.StackName))
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return names
}

func getOldLaunchTemplates(ctx context.Context, ec2Client *ec2.Client, expirationTime time.Time) (names []string) {
	var nextToken *string
	for {
		out := lo.Must(ec2Client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("tag-key"),
					Values: []string{karpenterLaunchTemplateTag},
				},
			},
			NextToken: nextToken,
		}))

		for _, launchTemplate := range out.LaunchTemplates {
			if lo.FromPtr(launchTemplate.CreateTime).Before(expirationTime) {
				names = append(names, lo.FromPtr(launchTemplate.LaunchTemplateName))
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return names
}

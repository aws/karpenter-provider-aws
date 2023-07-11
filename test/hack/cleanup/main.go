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
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"k8s.io/client-go/util/workqueue"
)

const (
	expirationTTL            = time.Hour * 12
	karpenterMetricNamespace = "testing.karpenter.sh/Cleanup"

	karpenterProvisionerNameTag = "karpenter.sh/provisioner-name"
	karpenterLaunchTemplateTag  = "karpenter.k8s.aws/cluster"
	karpenterSecurityGroupTag   = "karpenter.sh/discovery"
	githubRunURLTag             = "github.com/run-url"
)

type CleanableResourceType interface {
	Type() string
	Cleanup(ctx context.Context)
}

type DefaultResource struct {
	cloudWatchClient *cloudwatch.Client
	expirationTime   time.Time
	logger           *zap.SugaredLogger
}

func main() {
	ctx := context.Background()
	cfg := lo.Must(config.LoadDefaultConfig(ctx))

	logger := lo.Must(zap.NewProduction()).Sugar()

	expirationTime := time.Now().Add(-expirationTTL)

	logger.With("expiration-time", expirationTime.String()).Infof("resolved expiration time for all resources")

	ec2Client := ec2.NewFromConfig(cfg)
	cloudFormationClient := cloudformation.NewFromConfig(cfg)
	cloudWatchClient := cloudwatch.NewFromConfig(cfg)
	iamClient := iam.NewFromConfig(cfg)

	defaultResource := &DefaultResource{cloudWatchClient: cloudWatchClient, expirationTime: expirationTime, logger: logger}
	resources := []CleanableResourceType{
		&instance{ec2Client: ec2Client, DefaultResource: defaultResource},
		&securitygroup{ec2Client: ec2Client, DefaultResource: defaultResource},
		&stack{cloudFormationClient: cloudFormationClient, DefaultResource: defaultResource},
		&launchtemplate{ec2Client: ec2Client, DefaultResource: defaultResource},
		&oidc{iamClient: iamClient, DefaultResource: defaultResource},
	}

	workqueue.ParallelizeUntil(ctx, len(resources), len(resources), func(i int) {
		resources[i].Cleanup(ctx)
	})
}

type instance struct {
	*DefaultResource
	ec2Client *ec2.Client
}

func (i *instance) Type() string {
	return "Instances"
}

// Terminate any old instances that were provisioned by Karpenter as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (i *instance) Cleanup(ctx context.Context) {
	ids, err := i.GetExpired(ctx)
	if err != nil {
		i.logger.With("error", err).Error("getting instances")
	}
	i.logger.With("ids", ids, "count", len(ids)).Infof("discovered test instances to delete")
	if len(ids) > 0 {
		if _, err := i.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
			InstanceIds: ids,
		}); err != nil {
			i.logger.With("ids", ids, "count", len(ids)).Errorf("terminating test instances, %v", err)
		} else {
			i.logger.With("ids", ids, "count", len(ids)).Infof("terminated test instances")
			if err = i.fireMetric(ctx, "InstancesDeleted", float64(len(ids))); err != nil {
				i.logger.With("name", "InstancesDeleted").Errorf("firing metric, %v", err)
			}
		}
	}
}

type securitygroup struct {
	*DefaultResource
	ec2Client *ec2.Client
}

func (sg *securitygroup) Type() string {
	return "Security Group"
}

func (sg *securitygroup) Cleanup(ctx context.Context) {
	ids, err := sg.GetExpired(ctx)
	if err != nil {
		sg.logger.With("error", err).Error("getting security groups")
	}
	sg.logger.With("ids", ids, "count", len(ids)).Infof("discovered test security groups to delete")
	deleted := 0

	for i := range ids {
		_, err := sg.ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(ids[i]),
		})
		deleted += sg.GetCleanup("ids", "security group", ids[i], err)
	}
	if err := sg.fireMetric(ctx, "SecurityGroupDeleted", float64(deleted)); err != nil {
		sg.logger.With("name", "InstancesDeleted").Errorf("firing metric, %v", err)
	}
}

type stack struct {
	*DefaultResource
	cloudFormationClient *cloudformation.Client
}

func (s *stack) Type() string {
	return "Cloudformation Stacks"
}

// Terminate any old stacks that were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (s *stack) Cleanup(ctx context.Context) {
	names, err := s.GetExpired(ctx)
	if err != nil {
		s.logger.With("error", err).Error("getting stacks")
	}
	s.logger.With("names", names, "count", len(names)).Infof("discovered test stacks to delete")
	deleted := 0

	for i := range names {
		_, err := s.cloudFormationClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: lo.ToPtr(names[i]),
		})
		deleted += s.GetCleanup("name", "stack", names[i], err)
	}
	if err := s.fireMetric(ctx, "StacksDeleted", float64(deleted)); err != nil {
		s.logger.With("name", "StacksDeleted").Errorf("firing metric, %v", err)
	}
}

type launchtemplate struct {
	*DefaultResource
	ec2Client *ec2.Client
}

func (lt *launchtemplate) Type() string {
	return "Launch Templates"
}

// Terminate any old launch templates that were managed by Karpenter and were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (lt *launchtemplate) Cleanup(ctx context.Context) {
	names, err := lt.GetExpired(ctx)
	if err != nil {
		lt.logger.With("error", err).Error("getting launch templates")
	}
	lt.logger.With("names", names, "count", len(names)).Infof("discovered test launch templates to delete")
	deleted := 0

	for i := range names {
		_, err := lt.ec2Client.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{
			LaunchTemplateName: lo.ToPtr(names[i]),
		})
		deleted += lt.GetCleanup("name", "launch template", names[i], err)
	}
	if err := lt.fireMetric(ctx, "LaunchTemplatesDeleted", float64(deleted)); err != nil {
		lt.logger.With("name", "LaunchTemplatesDeleted").Errorf("firing metric, %v", err)
	}
}

type oidc struct {
	*DefaultResource
	iamClient *iam.Client
}

func (o *oidc) Type() string {
	return "OpenID Connect Provider"
}

// Terminate any old OIDC providers that were are remaining as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (o *oidc) Cleanup(ctx context.Context) {
	arns, err := o.GetExpired(ctx)
	if err != nil {
		o.logger.With("error", err).Error("getting  OICD provider")
	}
	deleted := 0
	for i := range arns {
		_, err := o.iamClient.DeleteOpenIDConnectProvider(ctx, &iam.DeleteOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: lo.ToPtr(arns[i]),
		})
		deleted += o.GetCleanup("arns", "oidc", arns[i], err)
	}
	if err := o.fireMetric(ctx, "OIDCDeleted", float64(deleted)); err != nil {
		o.logger.With("name", "OIDCDeleted").Errorf("firing metric, %v", err)
	}
}

func (d *DefaultResource) GetCleanup(providerType string, providerName string, provider string, err error) int {
	if err != nil {
		d.logger.With(providerType, provider).Errorf("deleting test cluster %s, %v", providerName, err)
		return 0
	}
	d.logger.With("arn", provider).Infof("deleted test cluster %s", providerName)
	return 1
}

func (d *DefaultResource) fireMetric(ctx context.Context, name string, value float64) error {
	_, err := d.cloudWatchClient.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
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

func (i *instance) GetExpired(ctx context.Context) (ids []string, err error) {
	var nextToken *string
	for {
		out, err := i.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
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
		})
		if err != nil {
			return ids, err
		}

		for _, res := range out.Reservations {
			for _, instance := range res.Instances {
				if _, found := lo.Find(instance.Tags, func(t ec2types.Tag) bool {
					return lo.FromPtr(t.Key) == "kubernetes.io/cluster/KITInfrastructure"
				}); !found && lo.FromPtr(instance.LaunchTime).Before(i.expirationTime) {
					ids = append(ids, lo.FromPtr(instance.InstanceId))
				}
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return ids, err
}

func (sg *securitygroup) GetExpired(ctx context.Context) (ids []string, err error) {
	var nextToken *string
	for {
		out, err := sg.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("group-name"),
					Values: []string{"security-group-drift"},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return ids, err
		}

		for _, sgroup := range out.SecurityGroups {
			creationDate, found := lo.Find(sgroup.Tags, func(tag ec2types.Tag) bool {
				return *tag.Key == "creation-date"
			})
			if !found {
				continue
			}
			time, err := time.Parse(time.RFC3339, *creationDate.Value)
			if err != nil {
				continue
			}
			if time.Before(sg.expirationTime) {
				ids = append(ids, lo.FromPtr(sgroup.GroupId))
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return ids, err
}

func (s *stack) GetExpired(ctx context.Context) (names []string, err error) {
	var nextToken *string
	for {
		out, err := s.cloudFormationClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
			NextToken: nextToken,
		})
		if err != nil {
			return names, err
		}

		stacks := lo.Reject(out.Stacks, func(s cloudformationtypes.Stack, _ int) bool {
			return s.StackStatus == cloudformationtypes.StackStatusDeleteComplete ||
				s.StackStatus == cloudformationtypes.StackStatusDeleteInProgress
		})
		for _, stack := range stacks {
			if _, found := lo.Find(stack.Tags, func(t cloudformationtypes.Tag) bool {
				return lo.FromPtr(t.Key) == githubRunURLTag
			}); found && lo.FromPtr(stack.CreationTime).Before(s.expirationTime) {
				names = append(names, lo.FromPtr(stack.StackName))
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return names, err
}

func (lt *launchtemplate) GetExpired(ctx context.Context) (names []string, err error) {
	var nextToken *string
	for {
		out, err := lt.ec2Client.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{
			Filters: []ec2types.Filter{
				{
					Name:   lo.ToPtr("tag-key"),
					Values: []string{karpenterLaunchTemplateTag},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return names, err
		}

		for _, launchTemplate := range out.LaunchTemplates {
			if lo.FromPtr(launchTemplate.CreateTime).Before(lt.expirationTime) {
				names = append(names, lo.FromPtr(launchTemplate.LaunchTemplateName))
			}
		}

		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return names, err
}

func (o *oidc) GetExpired(ctx context.Context) (names []string, err error) {
	out, err := o.iamClient.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return names, err
	}

	errs := make([]error, len(out.OpenIDConnectProviderList))
	for i := range out.OpenIDConnectProviderList {
		oicd, err := o.iamClient.GetOpenIDConnectProvider(ctx, &iam.GetOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: out.OpenIDConnectProviderList[i].Arn,
		})
		if err != nil {
			errs[i] = err
		}

		for _, t := range oicd.Tags {
			if lo.FromPtr(t.Key) == githubRunURLTag && oicd.CreateDate.Before(o.expirationTime) {
				names = append(names, lo.FromPtr(out.OpenIDConnectProviderList[i].Arn))
			}
		}
	}

	return names, multierr.Combine(errs...)
}

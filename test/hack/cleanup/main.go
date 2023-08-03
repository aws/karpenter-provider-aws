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
	"fmt"
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
	karpenterMetricNamespace = "testing.karpenter.sh/cleanup"

	karpenterProvisionerNameTag = "karpenter.sh/provisioner-name"
	karpenterLaunchTemplateTag  = "karpenter.k8s.aws/cluster"
	karpenterSecurityGroupTag   = "karpenter.sh/discovery"
	githubRunURLTag             = "github.com/run-url"
)

type CleanableResourceType interface {
	Type() string
	Get(context.Context, time.Time) ([]string, error)
	Cleanup(context.Context, []string) ([]string, error)
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

	resources := []CleanableResourceType{
		&instance{ec2Client: ec2Client},
		&securitygroup{ec2Client: ec2Client},
		&stack{cloudFormationClient: cloudFormationClient},
		&launchtemplate{ec2Client: ec2Client},
		&oidc{iamClient: iamClient},
	}
	workqueue.ParallelizeUntil(ctx, len(resources), len(resources), func(i int) {
		ids, err := resources[i].Get(ctx, expirationTime)
		if err != nil {
			logger.With("type", resources[i].Type()).Errorf("%v", err)
		}
		logger.With("type", resources[i].Type(), "ids", ids, "count", len(ids)).Infof("discovered resources")
		if len(ids) > 0 {
			cleaned, err := resources[i].Cleanup(ctx, ids)
			if err != nil {
				logger.With("type", resources[i].Type()).Errorf("%v", err)
			}
			if err = fireMetric(ctx, cloudWatchClient, fmt.Sprintf("%sDeleted", resources[i].Type()), float64(len(cleaned))); err != nil {
				logger.With("type", resources[i].Type()).Errorf("%v", err)
			}
			logger.With("type", resources[i].Type(), "ids", cleaned, "count", len(cleaned)).Infof("deleted resources")
		}
	})
}

type instance struct {
	ec2Client *ec2.Client
}

func (i *instance) Type() string {
	return "Instances"
}

func (i *instance) Get(ctx context.Context, expirationTime time.Time) (ids []string, err error) {
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
	return ids, err
}

// Terminate any old instances that were provisioned by Karpenter as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (i *instance) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	if _, err := i.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: ids,
	}); err != nil {
		return nil, err
	}
	return ids, nil
}

type securitygroup struct {
	ec2Client *ec2.Client
}

func (sg *securitygroup) Type() string {
	return "SecurityGroup"
}

func (sg *securitygroup) Get(ctx context.Context, expirationTime time.Time) (ids []string, err error) {
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
			if time.Before(expirationTime) {
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

func (sg *securitygroup) Cleanup(ctx context.Context, ids []string) ([]string, error) {
	deleted := []string{}
	var errs error
	for i := range ids {
		_, err := sg.ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(ids[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
		}
		deleted = append(deleted, ids[i])
	}

	return deleted, errs
}

type stack struct {
	cloudFormationClient *cloudformation.Client
}

func (s *stack) Type() string {
	return "CloudformationStacks"
}

func (s *stack) Get(ctx context.Context, expirationTime time.Time) (names []string, err error) {
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
			}); found && lo.FromPtr(stack.CreationTime).Before(expirationTime) {
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

// Terminate any old stacks that were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (s *stack) Cleanup(ctx context.Context, names []string) ([]string, error) {
	var errs error
	deleted := []string{}
	for i := range names {
		_, err := s.cloudFormationClient.DeleteStack(ctx, &cloudformation.DeleteStackInput{
			StackName: lo.ToPtr(names[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
		}
		deleted = append(deleted, names[i])
	}
	return deleted, errs
}

type launchtemplate struct {
	ec2Client *ec2.Client
}

func (lt *launchtemplate) Type() string {
	return "LaunchTemplates"
}

func (lt *launchtemplate) Get(ctx context.Context, expirationTime time.Time) (names []string, err error) {
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
			if lo.FromPtr(launchTemplate.CreateTime).Before(expirationTime) {
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

// Terminate any old launch templates that were managed by Karpenter and were provisioned as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (lt *launchtemplate) Cleanup(ctx context.Context, names []string) ([]string, error) {
	var errs error
	deleted := []string{}
	for i := range names {
		_, err := lt.ec2Client.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{
			LaunchTemplateName: lo.ToPtr(names[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
		}
		deleted = append(deleted, names[i])
	}
	return deleted, errs
}

type oidc struct {
	iamClient *iam.Client
}

func (o *oidc) Type() string {
	return "OpenIDConnectProvider"
}

func (o *oidc) Get(ctx context.Context, expirationTime time.Time) (names []string, err error) {
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
			if lo.FromPtr(t.Key) == githubRunURLTag && oicd.CreateDate.Before(expirationTime) {
				names = append(names, lo.FromPtr(out.OpenIDConnectProviderList[i].Arn))
			}
		}
	}

	return names, multierr.Combine(errs...)
}

// Terminate any old OIDC providers that were are remaining as part of testing
// We execute these in serial since we will most likely get rate limited if we try to delete these too aggressively
func (o *oidc) Cleanup(ctx context.Context, arns []string) ([]string, error) {
	var errs error
	deleted := []string{}
	for i := range arns {
		_, err := o.iamClient.DeleteOpenIDConnectProvider(ctx, &iam.DeleteOpenIDConnectProviderInput{
			OpenIDConnectProviderArn: lo.ToPtr(arns[i]),
		})
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	return deleted, errs
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

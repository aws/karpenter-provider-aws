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

package resourcetypes

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/samber/lo"
	"go.uber.org/multierr"
)

// Metric names. CloudformationStacks, ElasticNetworkInterface, InstanceProfile and OpenIDConnectProvider were
// already in the resourceCount table and keep their meaning.
const (
	MetricStacks                 = "CloudformationStacks"
	MetricStacksDeleteFailed     = "CloudformationStacksDeleteFailed"
	MetricStacksReviewInProgress = "CloudformationStacksReviewInProgress"
	MetricENIs                   = "ElasticNetworkInterface"
	MetricAvailableBranchENIs    = "AvailableBranchElasticNetworkInterfaces"
	MetricEKSClusters            = "EKSClusters"
	MetricVPCs                   = "VPCs"
	MetricCapacityReservations   = "ActiveCapacityReservations"
	MetricRoles                  = "IAMRole"
	MetricInstanceProfiles       = "InstanceProfile"
	MetricOIDCProviders          = "OpenIDConnectProvider"
)

const (
	maxResultsPerEC2Page = int32(1000)
	maxResultsPerEKSPage = int32(100)
)

var (
	errTooManyPages = errors.New("stopped listing after the page limit")
	// A var so that tests can lower it
	maxPagesPerCount = 50
)

// Sample is one count
type Sample struct {
	Name   string
	Value  float64
	Global bool
}

type capacityEC2API interface {
	ec2.DescribeVpcsAPIClient
	ec2.DescribeNetworkInterfacesAPIClient
	ec2.DescribeCapacityReservationsAPIClient
}

type capacityIAMAPI interface {
	GetAccountSummary(ctx context.Context, params *iam.GetAccountSummaryInput, optFns ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error)
	ListOpenIDConnectProviders(ctx context.Context, params *iam.ListOpenIDConnectProvidersInput, optFns ...func(*iam.Options)) (*iam.ListOpenIDConnectProvidersOutput, error)
}

// Capacity counts what the account runs out of when test resources leak. Each count lists one resource type once,
// with the largest page size.
type Capacity struct {
	ec2Client            capacityEC2API
	cloudFormationClient cloudformation.DescribeStacksAPIClient
	eksClient            eks.ListClustersAPIClient
	iamClient            capacityIAMAPI
}

func NewCapacity(ec2Client capacityEC2API, cloudFormationClient cloudformation.DescribeStacksAPIClient, eksClient eks.ListClustersAPIClient,
	iamClient capacityIAMAPI) *Capacity {
	return &Capacity{ec2Client: ec2Client, cloudFormationClient: cloudFormationClient, eksClient: eksClient, iamClient: iamClient}
}

// Regional returns what it could count, and an error for each count that failed
func (c *Capacity) Regional(ctx context.Context) ([]Sample, error) {
	return collect(ctx, false, c.stacks, c.enis, c.eksClusters, c.vpcs, c.capacityReservations)
}

// Global counts account-wide resources, so it only needs to run in one region
func (c *Capacity) Global(ctx context.Context) ([]Sample, error) {
	return collect(ctx, true, c.iamSummary, c.oidcProviders)
}

func collect(ctx context.Context, global bool, counts ...func(context.Context) ([]Sample, error)) ([]Sample, error) {
	var samples []Sample
	var errs error
	for _, count := range counts {
		s, err := count(ctx)
		samples = append(samples, lo.Map(s, func(s Sample, _ int) Sample { s.Global = global; return s })...)
		errs = multierr.Append(errs, err)
	}
	return samples, errs
}

func (c *Capacity) stacks(ctx context.Context) ([]Sample, error) {
	var stacks []cloudformationtypes.Stack
	err := listPages(ctx, cloudformation.NewDescribeStacksPaginator(c.cloudFormationClient, &cloudformation.DescribeStacksInput{}),
		func(out *cloudformation.DescribeStacksOutput) { stacks = append(stacks, out.Stacks...) })
	if err != nil {
		return nil, fmt.Errorf("counting stacks, %w", err)
	}
	byStatus := lo.CountValuesBy(stacks, func(s cloudformationtypes.Stack) cloudformationtypes.StackStatus { return s.StackStatus })
	return []Sample{
		{Name: MetricStacks, Value: float64(len(stacks))},
		{Name: MetricStacksDeleteFailed, Value: float64(byStatus[cloudformationtypes.StackStatusDeleteFailed])},
		{Name: MetricStacksReviewInProgress, Value: float64(byStatus[cloudformationtypes.StackStatusReviewInProgress])},
	}, nil
}

// Available branch ENIs are almost always leaked by a deleted cluster, and they block its VPC and stack deletion
func (c *Capacity) enis(ctx context.Context) ([]Sample, error) {
	var total, branch int
	err := listPages(ctx, ec2.NewDescribeNetworkInterfacesPaginator(c.ec2Client, &ec2.DescribeNetworkInterfacesInput{MaxResults: lo.ToPtr(maxResultsPerEC2Page)}),
		func(out *ec2.DescribeNetworkInterfacesOutput) {
			total += len(out.NetworkInterfaces)
			branch += lo.CountBy(out.NetworkInterfaces, func(ni ec2types.NetworkInterface) bool {
				_, ok := branchENICluster(ni)
				return ok && ni.Status == ec2types.NetworkInterfaceStatusAvailable
			})
		})
	if err != nil {
		return nil, fmt.Errorf("counting network interfaces, %w", err)
	}
	return []Sample{{Name: MetricENIs, Value: float64(total)}, {Name: MetricAvailableBranchENIs, Value: float64(branch)}}, nil
}

func (c *Capacity) eksClusters(ctx context.Context) ([]Sample, error) {
	count := 0
	err := listPages(ctx, eks.NewListClustersPaginator(c.eksClient, &eks.ListClustersInput{MaxResults: lo.ToPtr(maxResultsPerEKSPage)}),
		func(out *eks.ListClustersOutput) { count += len(out.Clusters) })
	if err != nil {
		return nil, fmt.Errorf("counting eks clusters, %w", err)
	}
	return []Sample{{Name: MetricEKSClusters, Value: float64(count)}}, nil
}

func (c *Capacity) vpcs(ctx context.Context) ([]Sample, error) {
	count := 0
	err := listPages(ctx, ec2.NewDescribeVpcsPaginator(c.ec2Client, &ec2.DescribeVpcsInput{MaxResults: lo.ToPtr(maxResultsPerEC2Page)}),
		func(out *ec2.DescribeVpcsOutput) { count += len(out.Vpcs) })
	if err != nil {
		return nil, fmt.Errorf("counting vpcs, %w", err)
	}
	return []Sample{{Name: MetricVPCs, Value: float64(count)}}, nil
}

func (c *Capacity) capacityReservations(ctx context.Context) ([]Sample, error) {
	count := 0
	err := listPages(ctx, ec2.NewDescribeCapacityReservationsPaginator(c.ec2Client, &ec2.DescribeCapacityReservationsInput{
		MaxResults: lo.ToPtr(maxResultsPerEC2Page),
		Filters:    []ec2types.Filter{{Name: lo.ToPtr("state"), Values: []string{string(ec2types.CapacityReservationStateActive)}}},
	}), func(out *ec2.DescribeCapacityReservationsOutput) { count += len(out.CapacityReservations) })
	if err != nil {
		return nil, fmt.Errorf("counting capacity reservations, %w", err)
	}
	return []Sample{{Name: MetricCapacityReservations, Value: float64(count)}}, nil
}

// GetAccountSummary returns the role and instance profile counts in one call, instead of listing them
func (c *Capacity) iamSummary(ctx context.Context) ([]Sample, error) {
	out, err := c.iamClient.GetAccountSummary(ctx, &iam.GetAccountSummaryInput{})
	if err != nil {
		return nil, fmt.Errorf("getting the iam account summary, %w", err)
	}
	var samples []Sample
	var errs error
	// The SDK's SummaryKeyType enum doesn't list these keys, but GetAccountSummary returns them
	for _, s := range []struct{ name, key string }{{name: MetricRoles, key: "Roles"}, {name: MetricInstanceProfiles, key: "InstanceProfiles"}} {
		count, ok := out.SummaryMap[s.key]
		if !ok {
			errs = multierr.Append(errs, fmt.Errorf("iam account summary has no %s", s.key))
			continue
		}
		samples = append(samples, Sample{Name: s.name, Value: float64(count)})
	}
	return samples, errs
}

func (c *Capacity) oidcProviders(ctx context.Context) ([]Sample, error) {
	// ListOpenIDConnectProviders isn't paginated
	out, err := c.iamClient.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
	if err != nil {
		return nil, fmt.Errorf("counting oidc providers, %w", err)
	}
	return []Sample{{Name: MetricOIDCProviders, Value: float64(len(out.OpenIDConnectProviderList))}}, nil
}

type pager[O, T any] interface {
	HasMorePages() bool
	NextPage(ctx context.Context, optFns ...func(*T)) (*O, error)
}

// listPages stops at maxPagesPerCount so that a runaway leak can't turn the hourly count into a throttling source
func listPages[O, T any](ctx context.Context, p pager[O, T], each func(*O)) error {
	for pages := 0; p.HasMorePages(); pages++ {
		if pages == maxPagesPerCount {
			return errTooManyPages
		}
		out, err := p.NextPage(ctx)
		if err != nil {
			return err
		}
		each(out)
	}
	return nil
}

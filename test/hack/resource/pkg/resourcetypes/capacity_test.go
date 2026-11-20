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
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/samber/lo"
)

// page returns one page of items and the token of the next one, like the AWS list APIs
func page[T any](items []T, token *string, size int) ([]T, *string) {
	start := 0
	if token != nil {
		start = lo.Must(strconv.Atoi(*token))
	}
	end := min(start+size, len(items))
	if end == len(items) {
		return items[start:end], nil
	}
	return items[start:end], lo.ToPtr(strconv.Itoa(end))
}

type capacityFakes struct {
	vpcs     []ec2types.Vpc
	enis     []ec2types.NetworkInterface
	crs      []ec2types.CapacityReservation
	stacks   []cloudformationtypes.Stack
	clusters []string
	summary  map[string]int32
	oidc     int

	pageSize int
	errs     map[string]error
	calls    []string
}

func (f *capacityFakes) call(name string) error {
	f.calls = append(f.calls, name)
	return f.errs[strings.Fields(name)[0]]
}

func (f *capacityFakes) DescribeVpcs(_ context.Context, in *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	if err := f.call(fmt.Sprintf("DescribeVpcs max=%d", lo.FromPtr(in.MaxResults))); err != nil {
		return nil, err
	}
	items, next := page(f.vpcs, in.NextToken, f.pageSize)
	return &ec2.DescribeVpcsOutput{Vpcs: items, NextToken: next}, nil
}

func (f *capacityFakes) DescribeNetworkInterfaces(_ context.Context, in *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	if err := f.call(fmt.Sprintf("DescribeNetworkInterfaces max=%d", lo.FromPtr(in.MaxResults))); err != nil {
		return nil, err
	}
	items, next := page(f.enis, in.NextToken, f.pageSize)
	return &ec2.DescribeNetworkInterfacesOutput{NetworkInterfaces: items, NextToken: next}, nil
}

func (f *capacityFakes) DescribeCapacityReservations(_ context.Context, in *ec2.DescribeCapacityReservationsInput, _ ...func(*ec2.Options)) (*ec2.DescribeCapacityReservationsOutput, error) {
	if err := f.call(fmt.Sprintf("DescribeCapacityReservations max=%d", lo.FromPtr(in.MaxResults))); err != nil {
		return nil, err
	}
	matching := lo.Filter(f.crs, func(cr ec2types.CapacityReservation, _ int) bool {
		return lo.EveryBy(in.Filters, func(filter ec2types.Filter) bool {
			if lo.FromPtr(filter.Name) != "state" {
				panic(fmt.Sprintf("unsupported filter %s", lo.FromPtr(filter.Name)))
			}
			return slices.Contains(filter.Values, string(cr.State))
		})
	})
	items, next := page(matching, in.NextToken, f.pageSize)
	return &ec2.DescribeCapacityReservationsOutput{CapacityReservations: items, NextToken: next}, nil
}

func (f *capacityFakes) DescribeStacks(_ context.Context, in *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	if err := f.call("DescribeStacks"); err != nil {
		return nil, err
	}
	items, next := page(f.stacks, in.NextToken, f.pageSize)
	return &cloudformation.DescribeStacksOutput{Stacks: items, NextToken: next}, nil
}

func (f *capacityFakes) ListClusters(_ context.Context, in *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	if err := f.call(fmt.Sprintf("ListClusters max=%d", lo.FromPtr(in.MaxResults))); err != nil {
		return nil, err
	}
	items, next := page(f.clusters, in.NextToken, f.pageSize)
	return &eks.ListClustersOutput{Clusters: items, NextToken: next}, nil
}

func (f *capacityFakes) GetAccountSummary(_ context.Context, _ *iam.GetAccountSummaryInput, _ ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error) {
	if err := f.call("GetAccountSummary"); err != nil {
		return nil, err
	}
	return &iam.GetAccountSummaryOutput{SummaryMap: f.summary}, nil
}

func (f *capacityFakes) ListOpenIDConnectProviders(_ context.Context, _ *iam.ListOpenIDConnectProvidersInput, _ ...func(*iam.Options)) (*iam.ListOpenIDConnectProvidersOutput, error) {
	if err := f.call("ListOpenIDConnectProviders"); err != nil {
		return nil, err
	}
	return &iam.ListOpenIDConnectProvidersOutput{OpenIDConnectProviderList: make([]iamtypes.OpenIDConnectProviderListEntry, f.oidc)}, nil
}

func newCapacityFakes() *capacityFakes {
	stack := func(name string, status cloudformationtypes.StackStatus) cloudformationtypes.Stack {
		return cloudformationtypes.Stack{StackName: lo.ToPtr(name), StackStatus: status}
	}
	return &capacityFakes{
		vpcs: []ec2types.Vpc{{VpcId: lo.ToPtr("vpc-1")}, {VpcId: lo.ToPtr("vpc-2")}, {VpcId: lo.ToPtr("vpc-3")}},
		enis: []ec2types.NetworkInterface{
			{NetworkInterfaceId: lo.ToPtr("eni-in-use"), Status: ec2types.NetworkInterfaceStatusInUse},
			{NetworkInterfaceId: lo.ToPtr("eni-available"), Status: ec2types.NetworkInterfaceStatusAvailable},
			branchENI("eni-branch-1", "vpc-1", "gone", ec2types.NetworkInterfaceStatusAvailable),
			branchENI("eni-branch-2", "vpc-1", "gone", ec2types.NetworkInterfaceStatusAvailable),
			branchENI("eni-branch-attached", "vpc-2", "live", ec2types.NetworkInterfaceStatusInUse),
		},
		crs: []ec2types.CapacityReservation{
			{CapacityReservationId: lo.ToPtr("cr-1"), State: ec2types.CapacityReservationStateActive},
			{CapacityReservationId: lo.ToPtr("cr-2"), State: ec2types.CapacityReservationStateActive},
			{CapacityReservationId: lo.ToPtr("cr-3"), State: ec2types.CapacityReservationStateCancelled},
		},
		stacks: []cloudformationtypes.Stack{
			stack("eksctl-a-cluster", cloudformationtypes.StackStatusCreateComplete),
			stack("eksctl-b-cluster", cloudformationtypes.StackStatusDeleteFailed),
			stack("iam-b", cloudformationtypes.StackStatusDeleteFailed),
			stack("iam-c", cloudformationtypes.StackStatusReviewInProgress),
			stack("iam-a", cloudformationtypes.StackStatusUpdateComplete),
		},
		clusters: []string{"a", "b", "c", "d", "e"},
		// Only some of the keys GetAccountSummary returns
		summary:  map[string]int32{"Roles": 358, "RolesQuota": 2000, "InstanceProfiles": 133, "Policies": 749},
		oidc:     47,
		pageSize: 2,
	}
}

func newTestCapacity(f *capacityFakes) *Capacity {
	return NewCapacity(f, f, f, f)
}

// expectSamples checks that samples are exactly want, each reported once, and all global or all regional
func expectSamples(t *testing.T, samples []Sample, global bool, want map[string]float64) {
	t.Helper()
	got := map[string]float64{}
	for _, s := range samples {
		if _, dup := got[s.Name]; dup {
			t.Errorf("sample %s reported twice", s.Name)
		}
		if s.Global != global {
			t.Errorf("sample %s Global = %v, want %v", s.Name, s.Global, global)
		}
		got[s.Name] = s.Value
	}
	if !maps.Equal(got, want) {
		t.Errorf("samples = %v, want %v", got, want)
	}
}

func TestCapacityRegional(t *testing.T) {
	f := newCapacityFakes()
	samples, err := newTestCapacity(f).Regional(context.Background())
	if err != nil {
		t.Fatalf("Regional() error = %v", err)
	}
	expectSamples(t, samples, false, map[string]float64{
		MetricStacks:                 5,
		MetricStacksDeleteFailed:     2,
		MetricStacksReviewInProgress: 1,
		MetricENIs:                   5,
		MetricAvailableBranchENIs:    2,
		MetricEKSClusters:            5,
		MetricVPCs:                   3,
		MetricCapacityReservations:   2,
	})
}

func TestCapacityRegionalListsEachTypeOnceWithLargestPages(t *testing.T) {
	f := newCapacityFakes()
	f.pageSize = 1000
	if _, err := newTestCapacity(f).Regional(context.Background()); err != nil {
		t.Fatalf("Regional() error = %v", err)
	}
	want := []string{
		"DescribeStacks",
		"DescribeNetworkInterfaces max=1000",
		"ListClusters max=100",
		"DescribeVpcs max=1000",
		"DescribeCapacityReservations max=1000",
	}
	if !slices.Equal(f.calls, want) {
		t.Errorf("calls = %v, want %v", f.calls, want)
	}
}

func TestCapacityGlobal(t *testing.T) {
	f := newCapacityFakes()
	samples, err := newTestCapacity(f).Global(context.Background())
	if err != nil {
		t.Fatalf("Global() error = %v", err)
	}
	expectSamples(t, samples, true, map[string]float64{MetricRoles: 358, MetricInstanceProfiles: 133, MetricOIDCProviders: 47})
	if !slices.Equal(f.calls, []string{"GetAccountSummary", "ListOpenIDConnectProviders"}) {
		t.Errorf("calls = %v", f.calls)
	}
}

func TestCapacityKeepsOtherCountsWhenOneFails(t *testing.T) {
	f := newCapacityFakes()
	f.errs = map[string]error{"DescribeStacks": errors.New("Throttling")}
	samples, err := newTestCapacity(f).Regional(context.Background())
	if err == nil || !strings.Contains(err.Error(), "counting stacks") {
		t.Fatalf("Regional() error = %v, want the stack error", err)
	}
	// A failed listing reports nothing, not a zero that looks like the leak stopped
	expectSamples(t, samples, false, map[string]float64{
		MetricENIs:                 5,
		MetricAvailableBranchENIs:  2,
		MetricEKSClusters:          5,
		MetricVPCs:                 3,
		MetricCapacityReservations: 2,
	})
}

func TestCapacityGlobalReportsMissingSummaryKeys(t *testing.T) {
	f := newCapacityFakes()
	delete(f.summary, "Roles")
	samples, err := newTestCapacity(f).Global(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Roles") {
		t.Fatalf("Global() error = %v, want the missing Roles key", err)
	}
	expectSamples(t, samples, true, map[string]float64{MetricInstanceProfiles: 133, MetricOIDCProviders: 47})
}

func TestCapacityStopsAtPageLimit(t *testing.T) {
	defer func(n int) { maxPagesPerCount = n }(maxPagesPerCount)
	maxPagesPerCount = 2
	f := newCapacityFakes()
	f.pageSize = 1
	samples, err := newTestCapacity(f).Regional(context.Background())
	if !errors.Is(err, errTooManyPages) {
		t.Fatalf("Regional() error = %v, want %v", err, errTooManyPages)
	}
	// Only capacity reservations fit in 2 pages of 1
	expectSamples(t, samples, false, map[string]float64{MetricCapacityReservations: 2})
	if n := lo.CountBy(f.calls, func(c string) bool { return strings.HasPrefix(c, "ListClusters") }); n != 2 {
		t.Errorf("ListClusters called %d times, want 2", n)
	}
}

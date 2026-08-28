//go:build test_performance

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

package instancetype

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	kubeletcel "github.com/aws/karpenter-provider-aws/pkg/cel"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
	"github.com/aws/karpenter-provider-aws/pkg/providers/subnet"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"
)

const (
	benchRegion             = "us-east-1"
	benchPlacementGroupName = "bench-partition-pg"
	// benchPartitionCount is the partition count the fake partition placement group advertises
	benchPartitionCount int32 = 7
	// benchReservationCount is the number of capacity reservations the reservation NodeClass has
	benchReservationCount = 25
)

var benchZones = []string{"test-zone-1a", "test-zone-1b", "test-zone-1c"}

type benchFixture struct {
	ctx       context.Context
	provider  *DefaultProvider
	nodeClass *v1.EC2NodeClass
	info      ec2types.InstanceTypeInfo
	zones     []string
	amiFamily amifamily.AMIFamily
	// typeNames is the sorted list of resolved instance-type names, used to build capacity
	// reservations that are guaranteed to match real instance types (and deterministic run-to-run).
	typeNames []string
}

func newBenchFixture(b *testing.B) *benchFixture {
	b.Helper()
	ctx := log.IntoContext(context.Background(), logr.Discard())
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, benchOptions())

	instanceTypes := fake.MakeInstances()
	if len(instanceTypes) == 0 {
		b.Fatal("fake.MakeInstances returned no instance types; the benchmark fixture's pricing-data sourcing changed")
	}
	// fake.MakeInstanceOfferings emits a single zone which under-scales offerings
	offerings := make([]ec2types.InstanceTypeOffering, 0, len(instanceTypes)*len(benchZones))
	for _, it := range instanceTypes {
		for _, zone := range benchZones {
			offerings = append(offerings, ec2types.InstanceTypeOffering{
				InstanceType: it.InstanceType,
				Location:     lo.ToPtr(zone),
			})
		}
	}

	ec2api := fake.NewEC2API()
	ec2api.DescribeInstanceTypesOutput.Set(&ec2.DescribeInstanceTypesOutput{InstanceTypes: instanceTypes})
	ec2api.DescribeInstanceTypeOfferingsOutput.Set(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: offerings})
	ec2api.DescribePlacementGroupsOutput.Set(&ec2.DescribePlacementGroupsOutput{
		PlacementGroups: []ec2types.PlacementGroup{{
			GroupId:        lo.ToPtr("pg-bench0123456789abc"),
			GroupName:      lo.ToPtr(benchPlacementGroupName),
			Strategy:       ec2types.PlacementStrategyPartition,
			PartitionCount: lo.ToPtr(benchPartitionCount),
			State:          ec2types.PlacementGroupStateAvailable,
		}},
	})

	clk := clock.NewFakeClock(time.Now())
	newCache := func() *cache.Cache { return cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval) }
	celEnv := lo.Must(kubeletcel.NewEnvironment())
	provider := NewDefaultProvider(
		newCache(), // instanceTypesCache
		newCache(), // offeringCache
		cache.New(awscache.DiscoveredCapacityCacheTTL, awscache.DefaultCleanupInterval),
		ec2api,
		subnet.NewDefaultProvider(ec2api, newCache(), newCache()),
		pricing.NewDefaultProvider(&fake.PricingAPI{}, ec2api, benchRegion, false),
		capacityreservation.NewProvider(ec2api, clk, newCache(), cache.New(24*time.Hour, awscache.DefaultCleanupInterval)),
		placementgroup.NewProvider(ec2api, newCache(), cache.New(awscache.PlacementGroupAvailabilityTTL, awscache.DefaultCleanupInterval)),
		awscache.NewUnavailableOfferings(),
		NewDefaultResolver(benchRegion, celEnv),
		arczonalshift.NewProvider(fake.NewARCZonalShiftAPI(), clk, ""),
		nil,
		celEnv,
	)

	if err := provider.UpdateInstanceTypes(ctx); err != nil {
		b.Fatalf("UpdateInstanceTypes: %v", err)
	}
	if err := provider.UpdateInstanceTypeOfferings(ctx); err != nil {
		b.Fatalf("UpdateInstanceTypeOfferings: %v", err)
	}
	if len(provider.instanceTypesInfo) == 0 {
		b.Fatal("no instance types resolved after UpdateInstanceTypes; the benchmark fixture's instance-type sourcing changed")
	}

	nodeClass := benchNodeClass()

	// Sorted type names give deterministic capacity-reservation fixtures that are guaranteed to
	// match real instance types.
	typeNames := make([]string, 0, len(provider.instanceTypesInfo))
	for it := range provider.instanceTypesInfo {
		typeNames = append(typeNames, string(it))
	}
	sort.Strings(typeNames)

	// Pick a fixed instance type - deterministic across runs
	info := provider.instanceTypesInfo[ec2types.InstanceType(typeNames[0])]
	f := &benchFixture{
		ctx:       ctx,
		provider:  provider,
		nodeClass: nodeClass,
		info:      info,
		zones:     provider.instanceTypesOfferings[info.InstanceType].UnsortedList(),
		amiFamily: amifamily.GetAMIFamily(nodeClass.AMIFamily(), &amifamily.Options{}),
		typeNames: typeNames,
	}
	return f
}

// benchVariant is a named NodeClass shape that the List / InjectOfferings / cacheKey benchmarks run
// against, so each shape's offering-injection workload is measured and gated independently.
type benchVariant struct {
	name      string
	nodeClass *v1.EC2NodeClass
}

// variants returns the NodeClass shapes to benchmark, ordered cheapest-first.
func (f *benchFixture) variants() []benchVariant {
	return []benchVariant{
		{name: "minimal", nodeClass: benchNodeClass()},
		{name: "cap-reservations", nodeClass: f.capReservationNodeClass()},
		{name: "partition-pg", nodeClass: partitionPGNodeClass()},
	}
}

// benchOptions mirrors pkg/test.Options() defaults without importing pkg/test (which imports this
// package and would create an import cycle from a white-box test).
func benchOptions() *options.Options {
	return &options.Options{
		ClusterName:             "test-cluster",
		ClusterEndpoint:         "https://test-cluster",
		VMMemoryOverheadPercent: 0.075,
		AMIRefreshInterval:      time.Minute,
		SubnetRefreshInterval:   time.Minute,
	}
}

func benchNodeClass() *v1.EC2NodeClass {
	return &v1.EC2NodeClass{
		ObjectMeta: metav1.ObjectMeta{Name: "bench-nodeclass"},
		Spec: v1.EC2NodeClassSpec{
			AMIFamily: lo.ToPtr(v1.AMIFamilyAL2023),
		},
		Status: v1.EC2NodeClassStatus{
			AMIs: []v1.AMI{
				{ID: "ami-test1", Requirements: []corev1.NodeSelectorRequirement{
					{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"amd64"}},
				}},
				{ID: "ami-test4", Requirements: []corev1.NodeSelectorRequirement{
					{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"arm64"}},
				}},
			},
			Subnets: []v1.Subnet{
				{ID: "subnet-test1", Zone: "test-zone-1a", ZoneID: "tstz1-1a"},
				{ID: "subnet-test2", Zone: "test-zone-1b", ZoneID: "tstz1-1b"},
				{ID: "subnet-test3", Zone: "test-zone-1c", ZoneID: "tstz1-1c"},
			},
		},
	}
}

// capReservationNodeClass is the minimal NodeClass plus a set of capacity reservations whose
// instance types are drawn from the resolved instance types (so each matches a type and produces a reserved
// offering).
func (f *benchFixture) capReservationNodeClass() *v1.EC2NodeClass {
	nc := benchNodeClass()
	n := benchReservationCount
	if len(f.typeNames) < n {
		n = len(f.typeNames)
	}
	reservations := make([]v1.CapacityReservation, 0, n)
	for i := 0; i < n; i++ {
		reservations = append(reservations, v1.CapacityReservation{
			AvailabilityZone:      benchZones[0],
			ID:                    fmt.Sprintf("cr-bench%013x", i),
			InstanceMatchCriteria: "open",
			InstanceType:          f.typeNames[i],
			OwnerID:               "123456789012",
			ReservationType:       v1.CapacityReservationTypeDefault,
			State:                 v1.CapacityReservationStateActive,
		})
	}
	nc.Status.CapacityReservations = reservations
	return nc
}

// partitionPGNodeClass is the minimal NodeClass selecting a partition placement group (resolved by
// the fake EC2 API in newBenchFixture).
func partitionPGNodeClass() *v1.EC2NodeClass {
	nc := benchNodeClass()
	nc.Spec.PlacementGroupSelector = &v1.PlacementGroupSelector{Name: lo.ToPtr(benchPlacementGroupName)}
	return nc
}

func BenchmarkInstanceTypeList_WarmCache(b *testing.B) {
	f := newBenchFixture(b)
	for _, v := range f.variants() {
		b.Run(v.name, func(b *testing.B) {
			// Warm the instance-types cache and the offering cache for this variant.
			if _, err := f.provider.List(f.ctx, v.nodeClass); err != nil {
				b.Fatalf("warmup List: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				its, err := f.provider.List(f.ctx, v.nodeClass)
				if err != nil {
					b.Fatal(err)
				}
				_ = its
			}
		})
	}
}

func BenchmarkInstanceTypeList_ColdCache(b *testing.B) {
	f := newBenchFixture(b)
	if _, err := f.provider.List(f.ctx, f.nodeClass); err != nil {
		b.Fatalf("warmup List: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f.provider.instanceTypesCache.Flush()
		b.StartTimer()
		its, err := f.provider.List(f.ctx, f.nodeClass)
		if err != nil {
			b.Fatal(err)
		}
		_ = its
	}
}

func BenchmarkInjectOfferings(b *testing.B) {
	f := newBenchFixture(b)
	for _, v := range f.variants() {
		b.Run(v.name, func(b *testing.B) {
			if _, err := f.provider.List(f.ctx, v.nodeClass); err != nil {
				b.Fatalf("warmup List: %v", err)
			}
			item, ok := f.provider.instanceTypesCache.Get(f.provider.cacheKey(v.nodeClass))
			if !ok {
				b.Fatal("expected warm instance-types cache")
			}
			resolved := item.([]*cloudprovider.InstanceType)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out := f.provider.offeringProvider.InjectOfferings(f.ctx, resolved, f.provider.instanceTypesInfo, v.nodeClass, f.provider.allZones)
				_ = out
			}
		})
	}
}

func BenchmarkComputeRequirements(b *testing.B) {
	f := newBenchFixture(b)
	zoneInfo := f.nodeClass.ZoneInfo()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqs := computeRequirements(f.info, benchRegion, f.zones, zoneInfo, f.amiFamily, nil)
		_ = reqs
	}
}

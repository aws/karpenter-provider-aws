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

package offering

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	arczonalshiftProvider "github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"
)

var benchZones = []string{"test-zone-1a", "test-zone-1b", "test-zone-1c"}

type benchPricingProvider struct{}

func (benchPricingProvider) LivenessProbe(*http.Request) error           { return nil }
func (benchPricingProvider) InstanceTypes() []ec2types.InstanceType      { return nil }
func (benchPricingProvider) UpdateOnDemandPricing(context.Context) error { return nil }
func (benchPricingProvider) UpdateSpotPricing(context.Context) error     { return nil }
func (benchPricingProvider) OnDemandPrice(ec2types.InstanceType) (float64, bool) {
	return 1.5, true
}
func (benchPricingProvider) SpotPrice(ec2types.InstanceType, string) (float64, bool) {
	return 0.5, true
}

func benchNodeClass() *v1.EC2NodeClass {
	nc := &v1.EC2NodeClass{
		ObjectMeta: metav1.ObjectMeta{Name: "bench"},
	}
	nc.Spec.AMIFamily = ptr(v1.AMIFamilyAL2023)
	for i, zone := range benchZones {
		nc.Status.Subnets = append(nc.Status.Subnets, v1.Subnet{
			ID:     fmt.Sprintf("subnet-%d", i),
			Zone:   zone,
			ZoneID: fmt.Sprintf("tstz1-1%d", i),
		})
	}
	return nc
}

func ptr[T any](v T) *T { return &v }

func benchInstanceTypes(n int) ([]*cloudprovider.InstanceType, map[ec2types.InstanceType]ec2types.InstanceTypeInfo) {
	its := make([]*cloudprovider.InstanceType, 0, n)
	info := map[ec2types.InstanceType]ec2types.InstanceTypeInfo{}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("bench.%dxlarge", i)
		its = append(its, &cloudprovider.InstanceType{
			Name: name,
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, name),
				scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, benchZones...),
				scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, karpv1.CapacityTypeOnDemand),
			),
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
			},
		})
		info[ec2types.InstanceType(name)] = ec2types.InstanceTypeInfo{
			InstanceType: ec2types.InstanceType(name),
			NetworkInfo: &ec2types.NetworkInfo{
				EnaSupport:   ec2types.EnaSupportRequired,
				NetworkCards: []ec2types.NetworkCardInfo{{NetworkCardIndex: ptr(int32(0)), MaximumNetworkInterfaces: ptr(int32(8))}},
			},
		}
	}
	return its, info
}

func benchProvider() *DefaultProvider {
	return NewDefaultProvider(
		benchPricingProvider{},
		nil, // capacity reservations: the ReservedCapacity gate is off in this benchmark
		nil, // placement groups: the NodeClass has no PlacementGroupSelector, so this is never called
		awscache.NewUnavailableOfferings(),
		cache.New(awscache.InstanceTypesZonesAndOfferingsTTL, awscache.DefaultCleanupInterval),
		&arczonalshiftProvider.DefaultNoopProvider{},
		nil, // kube client: the NodeOverlay gate is off in this benchmark
	)
}

func benchContext() context.Context {
	return coreoptions.ToContext(context.Background(), &coreoptions.Options{
		FeatureGates: coreoptions.FeatureGates{},
	})
}

func BenchmarkInjectOfferingsWarm(b *testing.B) {
	for _, n := range []int{250, 800} {
		b.Run(fmt.Sprintf("instanceTypes=%d", n), func(b *testing.B) {
			ctx := benchContext()
			p := benchProvider()
			nc := benchNodeClass()
			its, info := benchInstanceTypes(n)
			allZones := sets.New(benchZones...)
			p.InjectOfferings(ctx, its, info, nc, allZones) // warm the offering cache

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p.InjectOfferings(ctx, its, info, nc, allZones)
			}
		})
	}
}

func BenchmarkInjectOfferingsCold(b *testing.B) {
	for _, n := range []int{250, 800} {
		b.Run(fmt.Sprintf("instanceTypes=%d", n), func(b *testing.B) {
			ctx := benchContext()
			nc := benchNodeClass()
			its, info := benchInstanceTypes(n)
			allZones := sets.New(benchZones...)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				p := benchProvider()
				b.StartTimer()
				p.InjectOfferings(ctx, its, info, nc, allZones)
			}
		})
	}
}

func BenchmarkCacheKey(b *testing.B) {
	nc := benchNodeClass()
	its, _ := benchInstanceTypes(800)
	kb := newCacheKeyBuilder(nc, nc.ZoneInfo(), sets.New[string](), nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, it := range its {
			zoneValues := it.Requirements.Get(corev1.LabelTopologyZone).Values()
			capacityTypeValues := it.Requirements.Get(karpv1.CapacityTypeLabelKey).Values()
			_ = kb.cacheKeyFromInstanceType(it, zoneValues, capacityTypeValues)
		}
	}
}

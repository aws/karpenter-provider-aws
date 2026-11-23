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

package offering_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype/offering"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
)

func TestOffering(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Offering")
}

const (
	zoneA = "test-zone-1a"
	zoneB = "test-zone-1b"
	zoneC = "test-zone-1c"

	zoneIDA = "tstz1-1a"
	zoneIDB = "tstz1-1b"
	zoneIDC = "tstz1-1c"
)

type staticPricingProvider struct{}

func (staticPricingProvider) LivenessProbe(*http.Request) error           { return nil }
func (staticPricingProvider) InstanceTypes() []ec2types.InstanceType      { return nil }
func (staticPricingProvider) UpdateOnDemandPricing(context.Context) error { return nil }
func (staticPricingProvider) UpdateSpotPricing(context.Context) error     { return nil }
func (staticPricingProvider) OnDemandPrice(ec2types.InstanceType) (float64, bool) {
	return 1.5, true
}
func (staticPricingProvider) SpotPrice(ec2types.InstanceType, string) (float64, bool) {
	return 0.5, true
}

type mutableZonalShiftProvider struct {
	shifted sets.Set[string]
}

func (p *mutableZonalShiftProvider) UpdateZonalShifts(context.Context) error { return nil }
func (p *mutableZonalShiftProvider) IsZonalShifted(_ context.Context, zoneID string) bool {
	return p.shifted.Has(zoneID)
}
func (p *mutableZonalShiftProvider) ShiftedZones() sets.Set[string] { return p.shifted }

type mutablePlacementGroupProvider struct {
	pg *placementgroup.PlacementGroup
}

func (p *mutablePlacementGroupProvider) Get(context.Context, placementgroup.NodeClass) (*placementgroup.PlacementGroup, error) {
	return p.pg, nil
}

type nodeClassOption func(*v1.EC2NodeClass)

func newNodeClass(opts ...nodeClassOption) *v1.EC2NodeClass {
	nc := &v1.EC2NodeClass{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	nc.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyAL2023)
	nc.Status.Subnets = []v1.Subnet{
		{ID: "subnet-a", Zone: zoneA, ZoneID: zoneIDA},
		{ID: "subnet-b", Zone: zoneB, ZoneID: zoneIDB},
		{ID: "subnet-c", Zone: zoneC, ZoneID: zoneIDC},
	}
	for _, opt := range opts {
		opt(nc)
	}
	return nc
}

type instanceTypeOption func(*cloudprovider.InstanceType, *ec2types.InstanceTypeInfo)

func newInstanceType(name string, opts ...instanceTypeOption) (*cloudprovider.InstanceType, map[ec2types.InstanceType]ec2types.InstanceTypeInfo) {
	return newInstanceTypeIn(name, []string{zoneA, zoneB, zoneC}, []string{karpv1.CapacityTypeOnDemand}, opts...)
}

func newInstanceTypeIn(name string, zones, capacityTypes []string, opts ...instanceTypeOption) (*cloudprovider.InstanceType, map[ec2types.InstanceType]ec2types.InstanceTypeInfo) {
	it := &cloudprovider.InstanceType{
		Name: name,
		Requirements: scheduling.NewRequirements(
			scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, name),
			scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, zones...),
			scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, capacityTypes...),
		),
	}
	info := ec2types.InstanceTypeInfo{
		InstanceType: ec2types.InstanceType(name),
		Hypervisor:   "nitro",
		NetworkInfo: &ec2types.NetworkInfo{
			EnaSupport:                ec2types.EnaSupportRequired,
			Ipv4AddressesPerInterface: lo.ToPtr(int32(30)),
			NetworkCards:              []ec2types.NetworkCardInfo{{NetworkCardIndex: lo.ToPtr(int32(0)), MaximumNetworkInterfaces: lo.ToPtr(int32(8))}},
		},
		NitroEnclavesSupport: ec2types.NitroEnclavesSupportSupported,
		ProcessorInfo: &ec2types.ProcessorInfo{
			SupportedFeatures: []ec2types.SupportedAdditionalProcessorFeature{ec2types.SupportedAdditionalProcessorFeatureNestedVirtualization},
		},
		PlacementGroupInfo: &ec2types.PlacementGroupInfo{
			SupportedStrategies: []ec2types.PlacementGroupStrategy{ec2types.PlacementGroupStrategyCluster},
		},
	}
	for _, opt := range opts {
		opt(it, &info)
	}
	return it, map[ec2types.InstanceType]ec2types.InstanceTypeInfo{ec2types.InstanceType(name): info}
}

type resolveState struct {
	nodeClass    *v1.EC2NodeClass
	instanceType *cloudprovider.InstanceType
	info         map[ec2types.InstanceType]ec2types.InstanceTypeInfo
	shiftedZones sets.Set[string]
	pg           *placementgroup.PlacementGroup
	iceZones     []string
}

func offeringFor(its []*cloudprovider.InstanceType, zone, capacityType string) *cloudprovider.Offering {
	for _, o := range its[0].Offerings {
		if o.Requirements.Get(corev1.LabelTopologyZone).Has(zone) &&
			o.Requirements.Get(karpv1.CapacityTypeLabelKey).Has(capacityType) {
			return o
		}
	}
	return nil
}

var _ = Describe("Offering cache invalidation", func() {
	var ctx context.Context
	var offeringCache *cache.Cache
	var unavailableOfferings *awscache.UnavailableOfferings
	var allZones sets.Set[string]
	var zonalShift *mutableZonalShiftProvider
	var placementGroups *mutablePlacementGroupProvider
	var provider *offering.DefaultProvider

	BeforeEach(func() {
		ctx = coreoptions.ToContext(context.Background(), &coreoptions.Options{
			FeatureGates: coreoptions.FeatureGates{},
		})
		offeringCache = cache.New(awscache.InstanceTypesZonesAndOfferingsTTL, awscache.DefaultCleanupInterval)
		unavailableOfferings = awscache.NewUnavailableOfferings()
		allZones = sets.New(zoneA, zoneB, zoneC)
		zonalShift = &mutableZonalShiftProvider{shifted: sets.New[string]()}
		placementGroups = &mutablePlacementGroupProvider{}
		provider = offering.NewDefaultProvider(
			staticPricingProvider{},
			nil,
			placementGroups,
			unavailableOfferings,
			offeringCache,
			zonalShift,
			nil,
		)
	})

	resolve := func(s resolveState) []*cloudprovider.InstanceType {
		zonalShift.shifted = lo.Ternary(s.shiftedZones != nil, s.shiftedZones, sets.New[string]())
		placementGroups.pg = s.pg
		if s.pg != nil {
			s.nodeClass.Spec.PlacementGroupSelector = &v1.PlacementGroupSelector{Name: lo.ToPtr(s.pg.Name)}
		}
		for _, zone := range s.iceZones {
			unavailableOfferings.MarkUnavailable(ctx, ec2types.InstanceType(s.instanceType.Name), zone, karpv1.CapacityTypeOnDemand, map[string]string{"reason": "test"})
		}
		return provider.InjectOfferings(ctx, []*cloudprovider.InstanceType{s.instanceType}, s.info, s.nodeClass, allZones)
	}

	// The first resolve always misses (no recorded seqNum), so resolve twice to get a reusable entry.
	warm := func(s resolveState) {
		resolve(s)
		resolve(s)
	}

	DescribeTable("should not reuse a cached entry when a cached offering's input changes",
		func(firstFn, secondFn func() resolveState, assert func([]*cloudprovider.InstanceType)) {
			warm(firstFn())
			Expect(offeringCache.ItemCount()).To(Equal(1))
			assert(resolve(secondFn()))
		},
		Entry("instance type name",
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info, iceZones: []string{zoneA}}
			},
			func() resolveState {
				it, info := newInstanceType("m5.xlarge")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeOnDemand).Available).To(BeTrue())
			},
		),
		Entry("zone requirement values",
			func() resolveState {
				it, info := newInstanceTypeIn("m5.large", []string{zoneA}, []string{karpv1.CapacityTypeOnDemand})
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneC, karpv1.CapacityTypeOnDemand).Available).To(BeTrue())
			},
		),
		Entry("capacity type requirement values",
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceTypeIn("m5.large", []string{zoneA, zoneB, zoneC}, []string{karpv1.CapacityTypeOnDemand, karpv1.CapacityTypeSpot})
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeSpot)).ToNot(BeNil())
			},
		),
		Entry("NodeClass network interfaces",
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("m5.large")
				nc := newNodeClass(func(nc *v1.EC2NodeClass) {
					nc.Spec.NetworkInterfaces = []*v1.NetworkInterface{{NetworkCardIndex: 9, DeviceIndex: 0}}
				})
				return resolveState{nodeClass: nc, instanceType: it, info: info}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeOnDemand).Compatible).To(BeFalse())
			},
		),
		Entry("NodeClass subnets",
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("m5.large")
				nc := newNodeClass(func(nc *v1.EC2NodeClass) {
					nc.Status.Subnets = nc.Status.Subnets[:2] // drop the zoneC subnet
				})
				return resolveState{nodeClass: nc, instanceType: it, info: info}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneC, karpv1.CapacityTypeOnDemand).Requirements.Has(v1.LabelTopologyZoneID)).To(BeFalse())
			},
		),
		Entry("zonally shifted zones",
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info, shiftedZones: sets.New(zoneIDC)}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneC, karpv1.CapacityTypeOnDemand).Available).To(BeFalse())
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeOnDemand).Available).To(BeTrue())
			},
		),
		Entry("NodeClass connection tracking",
			func() resolveState {
				it, info := newInstanceType("m5.large", func(_ *cloudprovider.InstanceType, info *ec2types.InstanceTypeInfo) {
					info.Hypervisor = "xen" // connection tracking requires a nitro hypervisor
				})
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("m5.large", func(_ *cloudprovider.InstanceType, info *ec2types.InstanceTypeInfo) {
					info.Hypervisor = "xen"
				})
				nc := newNodeClass(func(nc *v1.EC2NodeClass) {
					nc.Spec.ConnectionTracking = &v1.ConnectionTracking{TCPEstablishedTimeout: lo.ToPtr(int32(60))}
				})
				return resolveState{nodeClass: nc, instanceType: it, info: info}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeOnDemand).Compatible).To(BeFalse())
			},
		),
		Entry("NodeClass nitro enclaves",
			func() resolveState {
				it, info := newInstanceType("m5.large", func(_ *cloudprovider.InstanceType, info *ec2types.InstanceTypeInfo) {
					info.NitroEnclavesSupport = ec2types.NitroEnclavesSupportUnsupported
				})
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("m5.large", func(_ *cloudprovider.InstanceType, info *ec2types.InstanceTypeInfo) {
					info.NitroEnclavesSupport = ec2types.NitroEnclavesSupportUnsupported
				})
				nc := newNodeClass(func(nc *v1.EC2NodeClass) {
					nc.Spec.EnclaveOptions = &v1.EnclaveOptions{Enabled: true}
				})
				return resolveState{nodeClass: nc, instanceType: it, info: info}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeOnDemand).Compatible).To(BeFalse())
			},
		),
		Entry("NodeClass nested virtualization",
			func() resolveState {
				it, info := newInstanceType("m5.large", func(_ *cloudprovider.InstanceType, info *ec2types.InstanceTypeInfo) {
					info.ProcessorInfo = &ec2types.ProcessorInfo{}
				})
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("m5.large", func(_ *cloudprovider.InstanceType, info *ec2types.InstanceTypeInfo) {
					info.ProcessorInfo = &ec2types.ProcessorInfo{}
				})
				nc := newNodeClass(func(nc *v1.EC2NodeClass) {
					nc.Spec.CPUOptions = &v1.CPUOptions{NestedVirtualization: lo.ToPtr("enabled")}
				})
				return resolveState{nodeClass: nc, instanceType: it, info: info}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeOnDemand).Compatible).To(BeFalse())
			},
		),
		Entry("placement group",
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{
					nodeClass:    newNodeClass(),
					instanceType: it,
					info:         info,
					pg:           &placementgroup.PlacementGroup{ID: "pg-1", Name: "pg", Strategy: placementgroup.StrategySpread},
				}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeOnDemand).Compatible).To(BeFalse())
			},
		),
		Entry("NodeClass AMI family",
			func() resolveState {
				it, info := newInstanceType("a1.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("a1.large")
				nc := newNodeClass(func(nc *v1.EC2NodeClass) {
					nc.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyBottlerocket)
				})
				return resolveState{nodeClass: nc, instanceType: it, info: info}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeOnDemand).Compatible).To(BeTrue())
			},
		),
		// ICE is guarded by the seqNum, not the key.
		Entry("unavailable offerings (ICE) sequence number",
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info}
			},
			func() resolveState {
				it, info := newInstanceType("m5.large")
				return resolveState{nodeClass: newNodeClass(), instanceType: it, info: info, iceZones: []string{zoneB}}
			},
			func(its []*cloudprovider.InstanceType) {
				Expect(offeringFor(its, zoneB, karpv1.CapacityTypeOnDemand).Available).To(BeFalse())
				Expect(offeringFor(its, zoneA, karpv1.CapacityTypeOnDemand).Available).To(BeTrue())
			},
		),
	)

	It("should reuse the cached entry when nothing changes", func() {
		it, info := newInstanceType("m5.large")
		resolve(resolveState{nodeClass: newNodeClass(), instanceType: it, info: info})
		first := resolve(resolveState{nodeClass: newNodeClass(), instanceType: it, info: info})
		second := resolve(resolveState{nodeClass: newNodeClass(), instanceType: it, info: info})
		Expect(offeringCache.ItemCount()).To(Equal(1))
		Expect(second[0].Offerings).To(HaveLen(len(first[0].Offerings)))
		Expect(second[0].Offerings[0]).To(BeIdenticalTo(first[0].Offerings[0]))
	})

	It("should reuse the cached entry across calls regardless of requirement value iteration order", func() {
		it, info := newInstanceType("m5.large")
		nc := newNodeClass()
		for i := 0; i < 50; i++ {
			resolve(resolveState{nodeClass: nc, instanceType: it, info: info})
		}
		Expect(offeringCache.ItemCount()).To(Equal(1))
	})

	It("should not share a cache entry between distinct zone sets with a common concatenation", func() {
		// "a"+"bc" vs "ab"+"c"
		ncA := newNodeClass(func(nc *v1.EC2NodeClass) {
			nc.Status.Subnets = []v1.Subnet{{ID: "subnet-1", Zone: "a", ZoneID: "z-1"}, {ID: "subnet-2", Zone: "bc", ZoneID: "z-2"}}
		})
		ncB := newNodeClass(func(nc *v1.EC2NodeClass) {
			nc.Status.Subnets = []v1.Subnet{{ID: "subnet-1", Zone: "ab", ZoneID: "z-1"}, {ID: "subnet-2", Zone: "c", ZoneID: "z-2"}}
		})
		itA, infoA := newInstanceTypeIn("m5.large", []string{"a", "bc"}, []string{karpv1.CapacityTypeOnDemand})
		itB, infoB := newInstanceTypeIn("m5.large", []string{"ab", "c"}, []string{karpv1.CapacityTypeOnDemand})
		allZones = sets.New("a", "ab", "bc", "c")

		resolve(resolveState{nodeClass: ncA, instanceType: itA, info: infoA})
		resolve(resolveState{nodeClass: ncB, instanceType: itB, info: infoB})
		Expect(offeringCache.ItemCount()).To(Equal(2))
	})

	It("should build a canonical key when called concurrently for the same instance types", func() {
		its := make([]*cloudprovider.InstanceType, 0, 20)
		info := map[ec2types.InstanceType]ec2types.InstanceTypeInfo{}
		for i := range 20 {
			it, itInfo := newInstanceType(fmt.Sprintf("m5.%dxlarge", i))
			its = append(its, it)
			for name, v := range itInfo {
				info[name] = v
			}
		}
		nc := newNodeClass()

		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 25 {
					Expect(provider.InjectOfferings(ctx, its, info, nc, allZones)).To(HaveLen(len(its)))
				}
			}()
		}
		wg.Wait()
		Expect(offeringCache.ItemCount()).To(Equal(len(its)))
	})
})

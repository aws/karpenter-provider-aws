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
	"strconv"
	"sync"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/patrickmn/go-cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/karpenter/pkg/apis/v1alpha1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/scheduling"

	arczonalshiftProvider "github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/providers/capacityreservation"
	"github.com/aws/karpenter-provider-aws/pkg/providers/placementgroup"
	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

type Provider interface {
	InjectOfferings(context.Context, []*cloudprovider.InstanceType, *v1.EC2NodeClass, []string) []*cloudprovider.InstanceType
}

// OfferingResolver is called during InjectOfferings to append additional offerings
// to each instance type. Resolvers are called in registration order, each receiving
// the offerings produced by the previous step.
type OfferingResolver interface {
	ResolveOfferings(ctx context.Context, it *cloudprovider.InstanceType, offerings cloudprovider.Offerings, instanceTypeInfo ec2types.InstanceTypeInfo, nodeClass NodeClass, allZones sets.Set[string], shiftedZones sets.Set[string], pg *placementgroup.PlacementGroup) cloudprovider.Offerings
}

type NodeClass interface {
	client.Object
	CapacityReservations() []v1.CapacityReservation
	ZoneInfo() []v1.ZoneInfo
	NetworkInterfaces() []*v1.NetworkInterface
	AMIFamily() string
	PlacementGroupSelector() *v1.PlacementGroupSelector
	ConnectionTracking() *v1.ConnectionTracking
	CPUOptions() *v1.CPUOptions
}

type DefaultProvider struct {
	pricingProvider                pricing.Provider
	capacityReservationProvider    capacityreservation.Provider
	zonalshiftProvider             arczonalshiftProvider.Provider
	placementGroupProvider         placementgroup.Provider
	unavailableOfferings           *awscache.UnavailableOfferings
	lastUnavailableOfferingsSeqNum sync.Map // instance type -> seqNum
	cache                          *cache.Cache
	kubeClient                     client.Client
	overlayPrices                  map[string]float64
	overlayPricesMu                sync.RWMutex
	overlayPricesExpiry            time.Time
	resolvers                      []OfferingResolver
}

func NewDefaultProvider(
	pricingProvider pricing.Provider,
	capacityReservationProvider capacityreservation.Provider,
	placementGroupProvider placementgroup.Provider,
	unavailableOfferingsCache *awscache.UnavailableOfferings,
	offeringCache *cache.Cache,
	zonalshiftProvider arczonalshiftProvider.Provider,
	kubeClient client.Client,
	additionalResolvers ...OfferingResolver,
) *DefaultProvider {
	p := &DefaultProvider{
		pricingProvider:             pricingProvider,
		capacityReservationProvider: capacityReservationProvider,
		placementGroupProvider:      placementGroupProvider,
		unavailableOfferings:        unavailableOfferingsCache,
		cache:                       offeringCache,
		zonalshiftProvider:          zonalshiftProvider,
		kubeClient:                  kubeClient,
	}
	// Register built-in resolvers
	p.resolvers = []OfferingResolver{
		&BaseResolver{
			PricingProvider:                pricingProvider,
			UnavailableOfferings:           unavailableOfferingsCache,
			LastUnavailableOfferingsSeqNum: &p.lastUnavailableOfferingsSeqNum,
			Cache:                          offeringCache,
			ZonalshiftProvider:             zonalshiftProvider,
			GetOverlayPrice:                p.GetOverlayPrice,
		},
		&ReservedCapacityResolver{
			PricingProvider:             pricingProvider,
			CapacityReservationProvider: capacityReservationProvider,
			ZonalshiftProvider:          zonalshiftProvider,
		},
		&PlacementGroupResolver{},
	}
	p.resolvers = append(p.resolvers, additionalResolvers...)
	return p
}

// RegisterResolver adds an OfferingResolver to the provider's pipeline.
// Resolvers are called in registration order during InjectOfferings.
func (p *DefaultProvider) RegisterResolver(r OfferingResolver) {
	p.resolvers = append(p.resolvers, r)
}

func (p *DefaultProvider) InjectOfferings(
	ctx context.Context,
	instanceTypes []*cloudprovider.InstanceType,
	instanceTypeInfo map[ec2types.InstanceType]ec2types.InstanceTypeInfo,
	nodeClass NodeClass,
	allZones sets.Set[string],
) []*cloudprovider.InstanceType {
	// Resolve the placement group once and pass it through to avoid repeated type assertions and lookups
	var pg *placementgroup.PlacementGroup
	if nodeClass.PlacementGroupSelector() != nil {
		pg, _ = p.placementGroupProvider.Get(ctx, nodeClass)
	}
	var its []*cloudprovider.InstanceType
	shiftedZones := p.zonalshiftProvider.ShiftedZones()

	for _, it := range instanceTypes {
		info := instanceTypeInfo[ec2types.InstanceType(it.Name)]
		// Run offering resolvers in order (base → reserved → PG → extensions)
		var offerings cloudprovider.Offerings
		for _, resolver := range p.resolvers {
			offerings = resolver.ResolveOfferings(ctx, it, offerings, info, nodeClass, allZones, shiftedZones, pg)
		}
		// NOTE: By making this copy one level deep, we can modify the offerings without mutating the results from previous
		// GetInstanceTypes calls. This should still be done with caution - it is currently done here in the provider, and
		// once in the instance provider (filterReservedInstanceTypes)

		// Inject placement group requirements into instance type requirements so the scheduler
		// can discover partition topology domains for TopologySpreadConstraints.
		reqs := it.Requirements
		if pg != nil {
			reqs = scheduling.NewRequirements(it.Requirements.Values()...)
			reqs.Add(scheduling.NewRequirement(v1.LabelPlacementGroupID, corev1.NodeSelectorOpIn, pg.ID))
			if pg.Strategy == placementgroup.StrategyPartition && pg.PartitionCount > 0 {
				partitions := make([]string, pg.PartitionCount)
				for i := int32(0); i < pg.PartitionCount; i++ {
					partitions[i] = fmt.Sprintf("%d", i+1)
				}
				reqs.Add(scheduling.NewRequirement(v1.LabelPlacementGroupPartition, corev1.NodeSelectorOpIn, partitions...))
			}
		}
		its = append(its, &cloudprovider.InstanceType{
			Name:         it.Name,
			Requirements: reqs,
			Offerings:    offerings,
			Capacity:     it.Capacity,
			Overhead:     it.Overhead,
		})
	}
	return its
}

// GetOverlayPrice returns the overlay price for the given instance type if one is defined.
//
//nolint:gocyclo
func (p *DefaultProvider) GetOverlayPrice(ctx context.Context, instanceTypeName string) (float64, bool) {
	if !options.FromContext(ctx).FeatureGates.NodeOverlay || p.kubeClient == nil {
		return 0, false
	}
	p.overlayPricesMu.RLock()
	if time.Now().Before(p.overlayPricesExpiry) {
		defer p.overlayPricesMu.RUnlock()
		price, found := p.overlayPrices[instanceTypeName]
		return price, found
	}
	p.overlayPricesMu.RUnlock()

	p.overlayPricesMu.Lock()
	defer p.overlayPricesMu.Unlock()
	if time.Now().Before(p.overlayPricesExpiry) {
		price, found := p.overlayPrices[instanceTypeName]
		return price, found
	}

	overlayList := &v1alpha1.NodeOverlayList{}
	if err := p.kubeClient.List(ctx, overlayList); err != nil {
		log.FromContext(ctx).Error(err, "failed to list NodeOverlays for overlay pricing")
		return 0, false
	}
	prices := map[string]float64{}
	for i := range overlayList.Items {
		overlay := &overlayList.Items[i]
		if !overlay.StatusConditions().IsTrue(v1alpha1.ConditionTypeValidationSucceeded) {
			continue
		}
		if overlay.Spec.Price == nil {
			continue
		}
		// make sure overlays with only an Instance Type and price are considered
		if len(overlay.Spec.Requirements) != 1 || overlay.Spec.Requirements[0].Key != corev1.LabelInstanceTypeStable || overlay.Spec.Requirements[0].Operator != corev1.NodeSelectorOpIn {
			continue
		}
		overlayPrice, err := strconv.ParseFloat(*overlay.Spec.Price, 64)
		if err != nil {
			continue
		}
		for _, val := range overlay.Spec.Requirements[0].Values {
			prices[val] = overlayPrice
		}
	}
	p.overlayPrices = prices
	p.overlayPricesExpiry = time.Now().Add(awscache.OverlayPricedTypesTTL)
	price, found := prices[instanceTypeName]
	return price, found
}

func (p *DefaultProvider) ResetOverlayPrices() {
	p.overlayPricesMu.Lock()
	defer p.overlayPricesMu.Unlock()
	p.overlayPrices = nil
	p.overlayPricesExpiry = time.Time{}
}

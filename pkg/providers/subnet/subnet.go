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

package subnet

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/serrors"

	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

type Provider interface {
	LivenessProbe(*http.Request) error
	List(context.Context, *v1.EC2NodeClass) ([]ec2types.Subnet, error)
	ZonalSubnetsForLaunch(context.Context, *v1.EC2NodeClass, []*cloudprovider.InstanceType, string) (map[string]*Subnet, error)
	UpdateInflightIPs(*ec2.CreateFleetInput, *ec2.CreateFleetOutput, []*cloudprovider.InstanceType, []*Subnet, string)
}

type DefaultProvider struct {
	sync.Mutex
	ec2api                        sdk.EC2API
	cache                         *cache.Cache
	availableIPAddressCache       *cache.Cache
	associatePublicIPAddressCache *cache.Cache
	cm                            *pretty.ChangeMonitor
	inflightIPs                   map[string]int32
}

type Subnet struct {
	ID                      string
	Zone                    string
	ZoneID                  string
	AvailableIPAddressCount int32
}

func NewDefaultProvider(ec2api sdk.EC2API, cache *cache.Cache, availableIPAddressCache *cache.Cache, associatePublicIPAddressCache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		// TODO: Remove cache when we utilize the resolved subnets from the EC2NodeClass.status
		// Subnets are sorted on AvailableIpAddressCount, descending order
		cache:                         cache,
		availableIPAddressCache:       availableIPAddressCache,
		associatePublicIPAddressCache: associatePublicIPAddressCache,
		// inflightIPs is used to track IPs from known launched instances
		inflightIPs: map[string]int32{},
	}
}

func (p *DefaultProvider) List(ctx context.Context, nodeClass *v1.EC2NodeClass) ([]ec2types.Subnet, error) {
	p.Lock()
	defer p.Unlock()
	filterSets := getFilterSets(nodeClass.Spec.SubnetSelectorTerms)
	if len(filterSets) == 0 {
		return []ec2types.Subnet{}, nil
	}
	hash := utils.GetNodeClassHash(nodeClass)
	if subnets, ok := p.cache.Get(hash); ok {
		// Ensure what's returned from this function is a shallow-copy of the slice (not a deep-copy of the data itself)
		// so that modifications to the ordering of the data don't affect the original
		return append([]ec2types.Subnet{}, subnets.([]ec2types.Subnet)...), nil
	}
	// Ensure that all the subnets that are returned here are unique
	subnets := map[string]ec2types.Subnet{}
	for _, filters := range filterSets {
		log.FromContext(ctx).V(1).Info("sending subnet filters to AWS", "filters", filters)
		paginator := ec2.NewDescribeSubnetsPaginator(p.ec2api, &ec2.DescribeSubnetsInput{
			Filters:    filters,
			MaxResults: lo.ToPtr(int32(500)),
		})
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, serrors.Wrap(fmt.Errorf("describing subnets with filters, %w", err), "filters", pretty.Concise(filters))
			}
			for i := range output.Subnets {
				subnets[lo.FromPtr(output.Subnets[i].SubnetId)] = output.Subnets[i]
				p.availableIPAddressCache.SetDefault(lo.FromPtr(output.Subnets[i].SubnetId), lo.FromPtr(output.Subnets[i].AvailableIpAddressCount))
				p.associatePublicIPAddressCache.SetDefault(lo.FromPtr(output.Subnets[i].SubnetId), lo.FromPtr(output.Subnets[i].MapPublicIpOnLaunch))
				// subnets can be leaked here, if a subnets is never called received from ec2
				// we are accepting it for now, as this will be an insignificant amount of memory
				delete(p.inflightIPs, lo.FromPtr(output.Subnets[i].SubnetId)) // remove any previously tracked IP addresses since we just refreshed from EC2
			}
		}
	}
	p.cache.SetDefault(hash, lo.Values(subnets))
	if p.cm.HasChanged(fmt.Sprintf("subnets/%s", nodeClass.Name), lo.Keys(subnets)) {
		log.FromContext(ctx).
			WithValues("subnets", lo.Map(lo.Values(subnets), func(s ec2types.Subnet, _ int) v1.Subnet {
				return v1.Subnet{
					ID:     lo.FromPtr(s.SubnetId),
					Zone:   lo.FromPtr(s.AvailabilityZone),
					ZoneID: lo.FromPtr(s.AvailabilityZoneId),
				}
			})).V(1).Info("discovered subnets")
	}
	return lo.Values(subnets), nil
}

// ZonalSubnetsForLaunch returns a mapping of zone to the subnet with the most available IP addresses and deducts the passed ips from the available count
func (p *DefaultProvider) ZonalSubnetsForLaunch(ctx context.Context, nodeClass *v1.EC2NodeClass, instanceTypes []*cloudprovider.InstanceType, capacityType string) (map[string]*Subnet, error) {
	if len(nodeClass.Status.Subnets) == 0 {
		return nil, fmt.Errorf("no subnets matched selector %v", nodeClass.Spec.SubnetSelectorTerms)
	}

	p.Lock()
	defer p.Unlock()

	zonalSubnets := map[string]*Subnet{}
	availableIPAddressCount := map[string]int32{}
	for _, subnet := range nodeClass.Status.Subnets {
		if subnetAvailableIP, ok := p.availableIPAddressCache.Get(subnet.ID); ok {
			availableIPAddressCount[subnet.ID] = subnetAvailableIP.(int32)
		}
	}

	for _, subnet := range nodeClass.Status.Subnets {
		if v, ok := zonalSubnets[subnet.Zone]; ok {
			currentZonalSubnetIPAddressCount := v.AvailableIPAddressCount
			newZonalSubnetIPAddressCount := availableIPAddressCount[subnet.ID]
			if ips, ok := p.inflightIPs[v.ID]; ok {
				currentZonalSubnetIPAddressCount = ips
			}
			if ips, ok := p.inflightIPs[subnet.ID]; ok {
				newZonalSubnetIPAddressCount = ips
			}

			if currentZonalSubnetIPAddressCount >= newZonalSubnetIPAddressCount {
				continue
			}
		}
		zonalSubnets[subnet.Zone] = &Subnet{ID: subnet.ID, Zone: subnet.Zone, ZoneID: subnet.ZoneID, AvailableIPAddressCount: availableIPAddressCount[subnet.ID]}
	}

	for _, subnet := range zonalSubnets {
		predictedIPsUsed := p.minPods(instanceTypes, scheduling.NewRequirements(
			scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, capacityType),
			scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, subnet.Zone),
		))
		prevIPs := subnet.AvailableIPAddressCount
		if trackedIPs, ok := p.inflightIPs[subnet.ID]; ok {
			prevIPs = trackedIPs
		}
		p.inflightIPs[subnet.ID] = prevIPs - predictedIPsUsed
	}
	return zonalSubnets, nil
}

// UpdateInflightIPs is used to refresh the in-memory IP usage by adding back unused IPs after a CreateFleet response is returned
func (p *DefaultProvider) UpdateInflightIPs(createFleetInput *ec2.CreateFleetInput, createFleetOutput *ec2.CreateFleetOutput, instanceTypes []*cloudprovider.InstanceType,
	subnets []*Subnet, capacityType string) {
	p.Lock()
	defer p.Unlock()

	// Process the CreateFleetInput to pull out all the requested subnetIDs
	fleetInputSubnets := lo.Compact(lo.Uniq(lo.FlatMap(createFleetInput.LaunchTemplateConfigs, func(req ec2types.FleetLaunchTemplateConfigRequest, _ int) []string {
		return lo.Map(req.Overrides, func(override ec2types.FleetLaunchTemplateOverridesRequest, _ int) string {
			return lo.FromPtr(override.SubnetId)
		})
	})))

	// Process the CreateFleetOutput to pull out all the fulfilled subnetIDs
	var fleetOutputSubnets []string
	if createFleetOutput != nil {
		fleetOutputSubnets = lo.Compact(lo.Uniq(lo.Map(createFleetOutput.Instances, func(fleetInstance ec2types.CreateFleetInstance, _ int) string {
			if fleetInstance.LaunchTemplateAndOverrides == nil || fleetInstance.LaunchTemplateAndOverrides.Overrides == nil {
				return ""
			}
			return lo.FromPtr(fleetInstance.LaunchTemplateAndOverrides.Overrides.SubnetId)
		})))
	}

	// Find the subnets that were included in the input but not chosen by Fleet, so we need to add the inflight IPs back to them
	subnetIDsToAddBackIPs, _ := lo.Difference(fleetInputSubnets, fleetOutputSubnets)

	// Aggregate all the cached subnets ip address count
	cachedAvailableIPAddressMap := lo.MapEntries(p.availableIPAddressCache.Items(), func(k string, v cache.Item) (string, int32) {
		return k, v.Object.(int32)
	})

	// Update the inflight IP tracking of subnets stored in the cache that have not be synchronized since the initial
	// deduction of IP addresses before the instance launch
	for cachedSubnetID, cachedIPAddressCount := range cachedAvailableIPAddressMap {
		if !lo.Contains(subnetIDsToAddBackIPs, cachedSubnetID) {
			continue
		}
		originalSubnet, ok := lo.Find(subnets, func(subnet *Subnet) bool {
			return subnet.ID == cachedSubnetID
		})
		if !ok {
			continue
		}
		// If the cached subnet IP address count hasn't changed from the original subnet used to
		// launch the instance, then we need to update the tracked IPs
		if originalSubnet.AvailableIPAddressCount == cachedIPAddressCount {
			// other IPs deducted were opportunistic and need to be readded since Fleet didn't pick those subnets to launch into
			if ips, ok := p.inflightIPs[originalSubnet.ID]; ok {
				minPods := p.minPods(instanceTypes, scheduling.NewRequirements(
					scheduling.NewRequirement(karpv1.CapacityTypeLabelKey, corev1.NodeSelectorOpIn, capacityType),
					scheduling.NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, originalSubnet.Zone),
				))
				p.inflightIPs[originalSubnet.ID] = ips + minPods
			}
		}
	}
}

func (p *DefaultProvider) LivenessProbe(_ *http.Request) error {
	p.Lock()
	//nolint: staticcheck
	p.Unlock()
	return nil
}

func (p *DefaultProvider) minPods(instanceTypes []*cloudprovider.InstanceType, reqs scheduling.Requirements) int32 {
	// filter for instance types available in the zone and capacity type being requested
	filteredInstanceTypes := lo.Filter(instanceTypes, func(it *cloudprovider.InstanceType, _ int) bool {
		return it.Offerings.Available().HasCompatible(reqs)
	})
	if len(filteredInstanceTypes) == 0 {
		return 0
	}
	// Get minimum pods to use when selecting a subnet and deducting what will be launched
	pods, _ := lo.MinBy(filteredInstanceTypes, func(i *cloudprovider.InstanceType, j *cloudprovider.InstanceType) bool {
		return i.Capacity.Pods().Cmp(*j.Capacity.Pods()) < 0
	}).Capacity.Pods().AsInt64()
	//nolint:gosec
	return int32(pods)
}

func getFilterSets(terms []v1.SubnetSelectorTerm) (res [][]ec2types.Filter) {
	for _, term := range terms {
		var filters []ec2types.Filter

		switch {
		case term.ID != "":
			filters = append(filters, ec2types.Filter{
				Name:   aws.String("subnet-id"),
				Values: []string{term.ID},
			})

		case term.CidrBlock != "":
			filters = append(filters, ec2types.Filter{
				Name:   aws.String("cidr-block"),
				Values: []string{term.CidrBlock},
			})
		default:
			for k, v := range term.Tags {
				if v == "*" {
					filters = append(filters, ec2types.Filter{
						Name:   aws.String("tag-key"),
						Values: []string{k},
					})
				} else {
					filters = append(filters, ec2types.Filter{
						Name:   aws.String(fmt.Sprintf("tag:%s", k)),
						Values: []string{v},
					})
				}
			}
		}
		if len(filters) > 0 {
			res = append(res, filters)
		}
	}

	return res
}

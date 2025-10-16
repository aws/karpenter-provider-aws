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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/utils"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

type Provider interface {
	LivenessProbe(*http.Request) error
	List(context.Context, *v1beta1.EC2NodeClass) ([]*ec2.Subnet, error)
	AssociatePublicIPAddressValue(*v1beta1.EC2NodeClass) *bool
	ZonalSubnetsForLaunch(context.Context, *v1beta1.EC2NodeClass, []*cloudprovider.InstanceType, string) (map[string]*Subnet, error)
	UpdateInflightIPs(*ec2.CreateFleetInput, *ec2.CreateFleetOutput, []*cloudprovider.InstanceType, []*Subnet, string)
}

type DefaultProvider struct {
	sync.Mutex
	ec2api                        ec2iface.EC2API
	cache                         *cache.Cache
	availableIPAddressCache       *cache.Cache
	associatePublicIPAddressCache *cache.Cache
	cm                            *pretty.ChangeMonitor
	inflightIPs                   map[string]int64
}

type Subnet struct {
	ID                      string
	Zone                    string
	ZoneID                  string
	AvailableIPAddressCount int64
}

func NewDefaultProvider(ec2api ec2iface.EC2API, cache *cache.Cache, availableIPAddressCache *cache.Cache, associatePublicIPAddressCache *cache.Cache) *DefaultProvider {
	return &DefaultProvider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		// TODO: Remove cache when we utilize the resolved subnets from the EC2NodeClass.status
		// Subnets are sorted on AvailableIpAddressCount, descending order
		cache:                         cache,
		availableIPAddressCache:       availableIPAddressCache,
		associatePublicIPAddressCache: associatePublicIPAddressCache,
		// inflightIPs is used to track IPs from known launched instances
		inflightIPs: map[string]int64{},
	}
}

func (p *DefaultProvider) List(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) ([]*ec2.Subnet, error) {
	p.Lock()
	defer p.Unlock()
	filterSets := getFilterSets(nodeClass.Spec.SubnetSelectorTerms)
	if len(filterSets) == 0 {
		return []*ec2.Subnet{}, nil
	}
	hash := utils.GetNodeClassHash(nodeClass)
	if subnets, ok := p.cache.Get(hash); ok {
		// Ensure what's returned from this function is a shallow-copy of the slice (not a deep-copy of the data itself)
		// so that modifications to the ordering of the data don't affect the original
		return append([]*ec2.Subnet{}, subnets.([]*ec2.Subnet)...), nil
	}

	// Ensure that all the subnets that are returned here are unique
	subnets := map[string]*ec2.Subnet{}
	for _, filters := range filterSets {
		output, err := p.ec2api.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{Filters: filters})
		if err != nil {
			return nil, fmt.Errorf("describing subnets %s, %w", pretty.Concise(filters), err)
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
	p.cache.SetDefault(hash, lo.Values(subnets))
	if p.cm.HasChanged(fmt.Sprintf("subnets/%s", nodeClass.Name), lo.Keys(subnets)) {
		log.FromContext(ctx).
			WithValues("subnets", lo.Map(lo.Values(subnets), func(s *ec2.Subnet, _ int) v1beta1.Subnet {
				return v1beta1.Subnet{
					ID:     lo.FromPtr(s.SubnetId),
					Zone:   lo.FromPtr(s.AvailabilityZone),
					ZoneID: lo.FromPtr(s.AvailabilityZoneId),
				}
			})).V(1).Info("discovered subnets")
	}
	return lo.Values(subnets), nil
}

// associatePublicIPAddressValue validates whether we know the association value for all subnets AND
// that all subnets don't have associatePublicIP set. If both of these are true, we set the value explicitly to false
// For more detail see: https://github.com/aws/karpenter-provider-aws/pull/3814
func (p *DefaultProvider) AssociatePublicIPAddressValue(nodeClass *v1beta1.EC2NodeClass) *bool {
	for _, subnet := range nodeClass.Status.Subnets {
		subnetAssociatePublicIP, ok := p.associatePublicIPAddressCache.Get(subnet.ID)
		if !ok || subnetAssociatePublicIP.(bool) {
			return nil
		}
	}
	return lo.ToPtr(false)
}

// ZonalSubnetsForLaunch returns a mapping of zone to the subnet with the most available IP addresses and deducts the passed ips from the available count
func (p *DefaultProvider) ZonalSubnetsForLaunch(ctx context.Context, nodeClass *v1beta1.EC2NodeClass, instanceTypes []*cloudprovider.InstanceType, capacityType string) (map[string]*Subnet, error) {
	if len(nodeClass.Status.Subnets) == 0 {
		return nil, fmt.Errorf("no subnets matched selector %v", nodeClass.Spec.SubnetSelectorTerms)
	}

	p.Lock()
	defer p.Unlock()

	zonalSubnets := map[string]*Subnet{}
	availableIPAddressCount := map[string]int64{}
	for _, subnet := range nodeClass.Status.Subnets {
		if subnetAvailableIP, ok := p.availableIPAddressCache.Get(subnet.ID); ok {
			availableIPAddressCount[subnet.ID] = subnetAvailableIP.(int64)
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
			scheduling.NewRequirement(corev1beta1.CapacityTypeLabelKey, v1.NodeSelectorOpIn, capacityType),
			scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, subnet.Zone),
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
	fleetInputSubnets := lo.Compact(lo.Uniq(lo.FlatMap(createFleetInput.LaunchTemplateConfigs, func(req *ec2.FleetLaunchTemplateConfigRequest, _ int) []string {
		return lo.Map(req.Overrides, func(override *ec2.FleetLaunchTemplateOverridesRequest, _ int) string {
			if override == nil {
				return ""
			}
			return lo.FromPtr(override.SubnetId)
		})
	})))

	// Process the CreateFleetOutput to pull out all the fulfilled subnetIDs
	var fleetOutputSubnets []string
	if createFleetOutput != nil {
		fleetOutputSubnets = lo.Compact(lo.Uniq(lo.Map(createFleetOutput.Instances, func(fleetInstance *ec2.CreateFleetInstance, _ int) string {
			if fleetInstance == nil || fleetInstance.LaunchTemplateAndOverrides == nil || fleetInstance.LaunchTemplateAndOverrides.Overrides == nil {
				return ""
			}
			return lo.FromPtr(fleetInstance.LaunchTemplateAndOverrides.Overrides.SubnetId)
		})))
	}

	// Find the subnets that were included in the input but not chosen by Fleet, so we need to add the inflight IPs back to them
	subnetIDsToAddBackIPs, _ := lo.Difference(fleetInputSubnets, fleetOutputSubnets)

	// Aggregate all the cached subnets ip address count
	cachedAvailableIPAddressMap := lo.MapEntries(p.availableIPAddressCache.Items(), func(k string, v cache.Item) (string, int64) {
		return k, v.Object.(int64)
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
					scheduling.NewRequirement(corev1beta1.CapacityTypeLabelKey, v1.NodeSelectorOpIn, capacityType),
					scheduling.NewRequirement(v1.LabelTopologyZone, v1.NodeSelectorOpIn, originalSubnet.Zone),
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

func (p *DefaultProvider) minPods(instanceTypes []*cloudprovider.InstanceType, reqs scheduling.Requirements) int64 {
	// filter for instance types available in the zone and capacity type being requested
	filteredInstanceTypes := lo.Filter(instanceTypes, func(it *cloudprovider.InstanceType, _ int) bool {
		offering, ok := it.Offerings.Get(reqs)
		if !ok {
			return false
		}
		return offering.Available
	})
	if len(filteredInstanceTypes) == 0 {
		return 0
	}
	// Get minimum pods to use when selecting a subnet and deducting what will be launched
	pods, _ := lo.MinBy(filteredInstanceTypes, func(i *cloudprovider.InstanceType, j *cloudprovider.InstanceType) bool {
		return i.Capacity.Pods().Cmp(*j.Capacity.Pods()) < 0
	}).Capacity.Pods().AsInt64()
	return pods
}

func getFilterSets(terms []v1beta1.SubnetSelectorTerm) (res [][]*ec2.Filter) {
	idFilter := &ec2.Filter{Name: aws.String("subnet-id")}
	for _, term := range terms {
		switch {
		case term.ID != "":
			idFilter.Values = append(idFilter.Values, aws.String(term.ID))
		default:
			var filters []*ec2.Filter
			for k, v := range term.Tags {
				if v == "*" {
					filters = append(filters, &ec2.Filter{
						Name:   aws.String("tag-key"),
						Values: []*string{aws.String(k)},
					})
				} else {
					filters = append(filters, &ec2.Filter{
						Name:   aws.String(fmt.Sprintf("tag:%s", k)),
						Values: []*string{aws.String(v)},
					})
				}
			}
			res = append(res, filters)
		}
	}
	if len(idFilter.Values) > 0 {
		res = append(res, []*ec2.Filter{idFilter})
	}
	return res
}

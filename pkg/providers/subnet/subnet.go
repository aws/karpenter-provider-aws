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
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	"knative.dev/pkg/logging"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/utils/pretty"
)

type Provider struct {
	sync.RWMutex
	ec2api      ec2iface.EC2API
	cache       *cache.Cache
	cm          *pretty.ChangeMonitor
	inflightIPs map[string]int64
}

func NewProvider(ec2api ec2iface.EC2API, cache *cache.Cache) *Provider {
	return &Provider{
		ec2api: ec2api,
		cm:     pretty.NewChangeMonitor(),
		// TODO: Remove cache when we utilize the resolved subnets from the EC2NodeClass.status
		// Subnets are sorted on AvailableIpAddressCount, descending order
		cache: cache,
		// inflightIPs is used to track IPs from known launched instances
		inflightIPs: map[string]int64{},
	}
}

func (p *Provider) List(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) ([]*ec2.Subnet, error) {
	p.Lock()
	defer p.Unlock()
	filterSets := getFilterSets(nodeClass.Spec.SubnetSelectorTerms)
	if len(filterSets) == 0 {
		return []*ec2.Subnet{}, nil
	}
	hash, err := hashstructure.Hash(filterSets, hashstructure.FormatV2, &hashstructure.HashOptions{SlicesAsSets: true})
	if err != nil {
		return nil, err
	}
	if subnets, ok := p.cache.Get(fmt.Sprint(hash)); ok {
		return subnets.([]*ec2.Subnet), nil
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
			delete(p.inflightIPs, lo.FromPtr(output.Subnets[i].SubnetId)) // remove any previously tracked IP addresses since we just refreshed from EC2
		}
	}
	p.cache.SetDefault(fmt.Sprint(hash), lo.Values(subnets))
	if p.cm.HasChanged(fmt.Sprintf("subnets/%s", nodeClass.Name), subnets) {
		logging.FromContext(ctx).
			With("subnets", lo.Map(lo.Values(subnets), func(s *ec2.Subnet, _ int) string {
				return fmt.Sprintf("%s (%s)", aws.StringValue(s.SubnetId), aws.StringValue(s.AvailabilityZone))
			})).
			Debugf("discovered subnets")
	}
	return lo.Values(subnets), nil
}

// CheckAnyPublicIPAssociations returns a bool indicating whether all referenced subnets assign public IPv4 addresses to EC2 instances created therein
func (p *Provider) CheckAnyPublicIPAssociations(ctx context.Context, nodeClass *v1beta1.EC2NodeClass) (bool, error) {
	subnets, err := p.List(ctx, nodeClass)
	if err != nil {
		return false, err
	}
	_, ok := lo.Find(subnets, func(s *ec2.Subnet) bool {
		return aws.BoolValue(s.MapPublicIpOnLaunch)
	})
	return ok, nil
}

// ZonalSubnetsForLaunch returns a mapping of zone to the subnet with the most available IP addresses and deducts the passed ips from the available count
func (p *Provider) ZonalSubnetsForLaunch(ctx context.Context, nodeClass *v1beta1.EC2NodeClass, instanceTypes []*cloudprovider.InstanceType, capacityType string) (map[string]*ec2.Subnet, error) {
	subnets, err := p.List(ctx, nodeClass)
	if err != nil {
		return nil, err
	}
	if len(subnets) == 0 {
		return nil, fmt.Errorf("no subnets matched selector %v", nodeClass.Spec.SubnetSelectorTerms)
	}
	p.Lock()
	defer p.Unlock()
	// sort subnets in ascending order of available IP addresses and populate map with most available subnet per AZ
	zonalSubnets := map[string]*ec2.Subnet{}
	sort.Slice(subnets, func(i, j int) bool {
		iIPs := aws.Int64Value(subnets[i].AvailableIpAddressCount)
		jIPs := aws.Int64Value(subnets[j].AvailableIpAddressCount)
		// override ip count from ec2.Subnet if we've tracked launches
		if ips, ok := p.inflightIPs[*subnets[i].SubnetId]; ok {
			iIPs = ips
		}
		if ips, ok := p.inflightIPs[*subnets[j].SubnetId]; ok {
			jIPs = ips
		}
		return iIPs < jIPs
	})
	for _, subnet := range subnets {
		zonalSubnets[*subnet.AvailabilityZone] = subnet
	}
	for _, subnet := range zonalSubnets {
		predictedIPsUsed := p.minPods(instanceTypes, *subnet.AvailabilityZone, capacityType)
		prevIPs := *subnet.AvailableIpAddressCount
		if trackedIPs, ok := p.inflightIPs[*subnet.SubnetId]; ok {
			prevIPs = trackedIPs
		}
		p.inflightIPs[*subnet.SubnetId] = prevIPs - predictedIPsUsed
	}
	return zonalSubnets, nil
}

// UpdateInflightIPs is used to refresh the in-memory IP usage by adding back unused IPs after a CreateFleet response is returned
func (p *Provider) UpdateInflightIPs(createFleetInput *ec2.CreateFleetInput, createFleetOutput *ec2.CreateFleetOutput, instanceTypes []*cloudprovider.InstanceType,
	subnets []*ec2.Subnet, capacityType string) {
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

	// Aggregate all the cached subnets
	cachedSubnets := lo.UniqBy(lo.Flatten(lo.MapToSlice(p.cache.Items(), func(_ string, item cache.Item) []*ec2.Subnet {
		return item.Object.([]*ec2.Subnet)
	})), func(subnet *ec2.Subnet) string { return *subnet.SubnetId })

	// Update the inflight IP tracking of subnets stored in the cache that have not be synchronized since the initial
	// deduction of IP addresses before the instance launch
	for _, cachedSubnet := range cachedSubnets {
		if !lo.Contains(subnetIDsToAddBackIPs, *cachedSubnet.SubnetId) {
			continue
		}
		originalSubnet, ok := lo.Find(subnets, func(subnet *ec2.Subnet) bool {
			return *subnet.SubnetId == *cachedSubnet.SubnetId
		})
		if !ok {
			continue
		}
		// If the cached subnet IP address count hasn't changed from the original subnet used to
		// launch the instance, then we need to update the tracked IPs
		if *originalSubnet.AvailableIpAddressCount == *cachedSubnet.AvailableIpAddressCount {
			// other IPs deducted were opportunistic and need to be readded since Fleet didn't pick those subnets to launch into
			if ips, ok := p.inflightIPs[*originalSubnet.SubnetId]; ok {
				minPods := p.minPods(instanceTypes, *originalSubnet.AvailabilityZone, capacityType)
				p.inflightIPs[*originalSubnet.SubnetId] = ips + minPods
			}
		}
	}
}

func (p *Provider) LivenessProbe(_ *http.Request) error {
	p.Lock()
	//nolint: staticcheck
	p.Unlock()
	return nil
}

func (p *Provider) minPods(instanceTypes []*cloudprovider.InstanceType, zone string, capacityType string) int64 {
	// filter for instance types available in the zone and capacity type being requested
	filteredInstanceTypes := lo.Filter(instanceTypes, func(it *cloudprovider.InstanceType, _ int) bool {
		offering, ok := it.Offerings.Get(capacityType, zone)
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

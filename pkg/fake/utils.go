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

package fake

import (
	"fmt"
	"path"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter-provider-aws/pkg/providers/pricing"
)

func InstanceID() string {
	return fmt.Sprintf("i-%s", randomdata.Alphanumeric(17))
}

func RandomProviderID() string {
	return ProviderID(InstanceID())
}

func ProviderID(id string) string {
	return fmt.Sprintf("aws:///%s/%s", DefaultRegion, id)
}

func ImageID() string {
	return fmt.Sprintf("ami-%s", strings.ToLower(randomdata.Alphanumeric(17)))
}
func SecurityGroupID() string {
	return fmt.Sprintf("sg-%s", randomdata.Alphanumeric(17))
}

func SubnetID() string {
	return fmt.Sprintf("subnet-%s", randomdata.Alphanumeric(17))
}

func InstanceProfileID() string {
	return fmt.Sprintf("instanceprofile-%s", randomdata.Alphanumeric(17))
}

func RoleID() string {
	return fmt.Sprintf("role-%s", randomdata.Alphanumeric(17))
}

func LaunchTemplateName() string {
	return fmt.Sprintf("karpenter.k8s.aws/%s", randomdata.Alphanumeric(17))
}

func LaunchTemplateID() string {
	return fmt.Sprint(randomdata.Alphanumeric(17))
}

func PrivateDNSName() string {
	return fmt.Sprintf("ip-192-168-%d-%d.%s.compute.internal", randomdata.Number(0, 256), randomdata.Number(0, 256), DefaultRegion)
}

// SubnetsFromFleetRequest returns a unique slice of subnetIDs passed as overrides from a CreateFleetInput
func SubnetsFromFleetRequest(createFleetInput *ec2.CreateFleetInput) []string {
	return lo.Uniq(lo.Flatten(lo.Map(createFleetInput.LaunchTemplateConfigs, func(ltReq ec2types.FleetLaunchTemplateConfigRequest, _ int) []string {
		var subnets []string
		for _, override := range ltReq.Overrides {
			if override.SubnetId != nil {
				subnets = append(subnets, *override.SubnetId)
			}
		}
		return subnets
	})))
}

// FilterDescribeSecurtyGroups filters the passed in security groups based on the filters passed in.
// Filters are chained with a logical "AND"
func FilterDescribeSecurtyGroups(sgs []ec2types.SecurityGroup, filters []ec2types.Filter) []ec2types.SecurityGroup {
	return lo.Filter(sgs, func(group ec2types.SecurityGroup, _ int) bool {
		return Filter(filters, *group.GroupId, *group.GroupName, "", "", "", group.Tags)
	})
}

// FilterDescribeSubnets filters the passed in subnets based on the filters passed in.
// Filters are chained with a logical "AND"
func FilterDescribeSubnets(subnets []ec2types.Subnet, filters []ec2types.Filter) []ec2types.Subnet {
	return lo.Filter(subnets, func(subnet ec2types.Subnet, _ int) bool {
		cidrBlock := ""
		if subnet.CidrBlock != nil {
			cidrBlock = *subnet.CidrBlock
		}
		return Filter(filters, aws.ToString(subnet.SubnetId), "", "", "", cidrBlock, subnet.Tags)
	})
}

func FilterDescribeCapacityReservations(crs []ec2types.CapacityReservation, ids []string, filters []ec2types.Filter) []ec2types.CapacityReservation {
	idSet := sets.New[string](ids...)
	return lo.Filter(crs, func(cr ec2types.CapacityReservation, _ int) bool {
		if len(ids) != 0 && !idSet.Has(*cr.CapacityReservationId) {
			return false
		}
		return FilterCapacityReservation(filters, *cr.CapacityReservationId, "", *cr.OwnerId, string(cr.State), string(cr.InstanceMatchCriteria), cr.Tags)
	})
}

func FilterDescribeImages(images []ec2types.Image, filters []ec2types.Filter) []ec2types.Image {
	return lo.Filter(images, func(image ec2types.Image, _ int) bool {
		return Filter(filters, *image.ImageId, *image.Name, "", string(image.State), "", image.Tags)
	})
}

//nolint:gocyclo
func Filter(filters []ec2types.Filter, id, name, owner, state, cidrBlock string, tags []ec2types.Tag) bool {
	return lo.EveryBy(filters, func(filter ec2types.Filter) bool {
		switch filterName := aws.ToString(filter.Name); {
		case filterName == "state":
			for _, val := range filter.Values {
				if state == val {
					return true
				}
			}
		case filterName == "subnet-id" || filterName == "group-id" || filterName == "image-id":
			for _, val := range filter.Values {
				if id == val {
					return true
				}
			}
		case filterName == "group-name" || filterName == "name":
			for _, val := range filter.Values {
				if name == val {
					return true
				}
			}
		case filterName == "owner-id":
			for _, val := range filter.Values {
				if owner == val {
					return true
				}
			}
		case filterName == "cidr-block":
			for _, pattern := range filter.Values {
				if match, _ := path.Match(pattern, cidrBlock); match {
					return true
				}
			}
		case strings.HasPrefix(filterName, "tag"):
			if matchTags(tags, filter) {
				return true
			}
		default:
			panic(fmt.Sprintf("Unsupported mock filter %v", filter))
		}
		return false
	})
}

func FilterCapacityReservation(filters []ec2types.Filter, id, name, owner, state, instanceMatchCriteria string, tags []ec2types.Tag) bool {
	return lo.EveryBy(filters, func(filter ec2types.Filter) bool {
		if aws.ToString(filter.Name) == "instance-match-criteria" {
			return lo.Contains(filter.Values, instanceMatchCriteria)
		}
		return Filter([]ec2types.Filter{filter}, id, name, owner, state, tags)
	})
}

// matchTags is a predicate that matches a slice of tags with a tag:<key> or tag-keys filter
// nolint: gocyclo
func matchTags(tags []ec2types.Tag, filter ec2types.Filter) bool {
	if strings.HasPrefix(*filter.Name, "tag:") {
		_, tagKey, _ := strings.Cut(*filter.Name, ":")
		for _, val := range filter.Values {
			for _, tag := range tags {
				if tagKey == *tag.Key && (val == "*" || val == *tag.Value) {
					return true
				}
			}
		}
	} else if strings.HasPrefix(*filter.Name, "tag-key") {
		for _, v := range filter.Values {
			if v == "*" {
				return true
			}
			for _, t := range tags {
				if lo.FromPtr(t.Key) == v {
					return true
				}
			}
		}
	}
	return false
}

func MakeInstances() []ec2types.InstanceTypeInfo {
	var instanceTypes []ec2types.InstanceTypeInfo
	// Use keys from the static pricing data so that we guarantee pricing for the data
	// Create uniform instance data so all of them schedule for a given pod
	for _, it := range pricing.NewDefaultProvider(nil, nil, "us-east-1", true).InstanceTypes() {
		instanceTypes = append(instanceTypes, ec2types.InstanceTypeInfo{
			InstanceType: it,
			ProcessorInfo: &ec2types.ProcessorInfo{
				SupportedArchitectures: []ec2types.ArchitectureType{ec2types.ArchitectureTypeX8664},
			},
			VCpuInfo: &ec2types.VCpuInfo{
				DefaultCores: aws.Int32(1),
				DefaultVCpus: aws.Int32(2),
			},
			MemoryInfo: &ec2types.MemoryInfo{
				SizeInMiB: aws.Int64(8192),
			},
			NetworkInfo: &ec2types.NetworkInfo{
				Ipv4AddressesPerInterface: aws.Int32(10),
				DefaultNetworkCardIndex:   aws.Int32(0),
				NetworkCards: []ec2types.NetworkCardInfo{{
					NetworkCardIndex:         lo.ToPtr(int32(0)),
					MaximumNetworkInterfaces: aws.Int32(3),
				}},
			},
			SupportedUsageClasses: DefaultSupportedUsageClasses,
		})
	}
	return instanceTypes
}

func MakeUniqueInstancesAndFamilies(instances []ec2types.InstanceTypeInfo, numInstanceFamilies int) ([]ec2types.InstanceTypeInfo, sets.Set[string]) {
	var instanceTypes []ec2types.InstanceTypeInfo
	instanceFamilies := sets.Set[string]{}
	for _, it := range instances {
		var found bool
		for instFamily := range instanceFamilies {
			if strings.Split(string(it.InstanceType), ".")[0] == instFamily {
				found = true
				break
			}
		}
		if !found {
			instanceTypes = append(instanceTypes, it)
			instanceFamilies.Insert(strings.Split(string(it.InstanceType), ".")[0])
			if len(instanceFamilies) == numInstanceFamilies {
				break
			}
		}
	}
	return instanceTypes, instanceFamilies
}

func MakeInstanceOfferings(instanceTypes []ec2types.InstanceTypeInfo) []ec2types.InstanceTypeOffering {
	var instanceTypeOfferings []ec2types.InstanceTypeOffering

	// Create uniform instance offering data so all of them schedule for a given pod
	for _, instanceType := range instanceTypes {
		instanceTypeOfferings = append(instanceTypeOfferings, ec2types.InstanceTypeOffering{
			InstanceType: instanceType.InstanceType,
			Location:     aws.String("test-zone-1a"),
		})
	}
	return instanceTypeOfferings
}

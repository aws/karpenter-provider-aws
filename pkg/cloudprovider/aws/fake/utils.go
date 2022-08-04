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
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
)

// SubnetsFromFleetRequest returns a unique slice of subnetIDs passed as overrides from a CreateFleetInput
func SubnetsFromFleetRequest(createFleetInput *ec2.CreateFleetInput) []string {
	return lo.Uniq(lo.Flatten(lo.Map(createFleetInput.LaunchTemplateConfigs, func(ltReq *ec2.FleetLaunchTemplateConfigRequest, _ int) []string {
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
func FilterDescribeSecurtyGroups(sgs []*ec2.SecurityGroup, filters []*ec2.Filter) []*ec2.SecurityGroup {
	return lo.Filter(sgs, func(group *ec2.SecurityGroup, _ int) bool {
		return Filter(filters, *group.GroupId, group.Tags)
	})
}

// FilterDescribeSubnets filters the passed in subnets based on the filters passed in.
// Filters are chained with a logical "AND"
func FilterDescribeSubnets(subnets []*ec2.Subnet, filters []*ec2.Filter) []*ec2.Subnet {
	return lo.Filter(subnets, func(subnet *ec2.Subnet, _ int) bool {
		return Filter(filters, *subnet.SubnetId, subnet.Tags)
	})
}

func Filter(filters []*ec2.Filter, id string, tags []*ec2.Tag) bool {
	return lo.EveryBy(filters, func(filter *ec2.Filter) bool {
		switch filterName := aws.StringValue(filter.Name); {
		case filterName == "subnet-id" || filterName == "group-id":
			for _, val := range filter.Values {
				if id == aws.StringValue(val) {
					return true
				}
			}
		case strings.HasPrefix(filterName, "tag"):
			if matchTags(tags, filter) {
				return true
			}
		default:
			panic("Unsupported mock filter")
		}
		return false
	})
}

// matchTags is a predicate that matches a slice of tags with a tag:<key> or tag-keys filter
// nolint: gocyclo
func matchTags(tags []*ec2.Tag, filter *ec2.Filter) bool {
	if strings.HasPrefix(*filter.Name, "tag:") {
		tagKey := strings.Split(*filter.Name, ":")[1]
		for _, val := range filter.Values {
			for _, tag := range tags {
				if (tagKey == "*" || tagKey == *tag.Key) && (*val == "*" || *val == *tag.Value) {
					return true
				}
			}
		}
	} else if strings.HasPrefix(*filter.Name, "tag-key") {
		for _, v := range filter.Values {
			if aws.StringValue(v) == "*" {
				return true
			}
			for _, t := range tags {
				if aws.StringValue(t.Key) == aws.StringValue(v) {
					return true
				}
			}
		}
	}
	return false
}

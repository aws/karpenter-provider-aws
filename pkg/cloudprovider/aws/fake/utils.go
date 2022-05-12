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
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
)

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

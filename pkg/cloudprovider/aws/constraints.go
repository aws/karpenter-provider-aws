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

package aws

import (
	"fmt"

	"github.com/awslabs/karpenter/pkg/cloudprovider"
)

const (
	nodeLabelPrefix              = "node.k8s.aws"
	capacityTypeSpot             = "spot"
	capacityTypeOnDemand         = "on-demand"
	defaultLaunchTemplateVersion = "$Default"
)

var (
	CapacityTypeLabel          = fmt.Sprintf("%s/capacity-type", nodeLabelPrefix)
	LaunchTemplateIdLabel      = fmt.Sprintf("%s/launch-template-id", nodeLabelPrefix)
	LaunchTemplateVersionLabel = fmt.Sprintf("%s/launch-template-version", nodeLabelPrefix)
	allowedLabels              = []string{CapacityTypeLabel, LaunchTemplateIdLabel, LaunchTemplateVersionLabel}
)

// Constraints are AWS specific constraints
type Constraints cloudprovider.Constraints

func (c *Constraints) GetCapacityType() string {
	capacityType, ok := c.Labels[CapacityTypeLabel]
	if !ok {
		capacityType = capacityTypeOnDemand
	}
	return capacityType
}

type LaunchTemplate struct {
	Id      *string
	Version *string
}

func (c *Constraints) GetLaunchTemplate() *LaunchTemplate {
	id, ok := c.Labels[LaunchTemplateIdLabel]
	if !ok {
		return nil
	}
	version, ok := c.Labels[LaunchTemplateVersionLabel]
	if !ok {
		version = defaultLaunchTemplateVersion
	}
	return &LaunchTemplate{
		Id:      &id,
		Version: &version,
	}
}

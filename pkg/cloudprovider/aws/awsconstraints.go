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
	nodeLabelPrefix      = "node.k8s.aws"
	capacityTypeSpot     = "spot"
	capacityTypeOnDemand = "on-demand"
)

var (
	capacityTypeLabel = fmt.Sprintf("%s/capacity-type", nodeLabelPrefix)
)

// AWSConstraints are AWS specific constraints
type AWSConstraints struct {
	*cloudprovider.Constraints
}

func NewAWSConstraints(constraints *cloudprovider.Constraints) *AWSConstraints {
	return &AWSConstraints{
		constraints,
	}
}

func (a *AWSConstraints) GetCapacityType() string {
	capacityType, ok := a.Labels[capacityTypeLabel]
	if !ok {
		capacityType = capacityTypeOnDemand
	}
	return capacityType
}

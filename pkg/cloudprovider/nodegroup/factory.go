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

package nodegroup

import (
	"fmt"

	"github.com/ellistarn/karpenter/pkg/apis/autoscaling/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/cloudprovider"
	"github.com/ellistarn/karpenter/pkg/cloudprovider/nodegroup/aws"
	"github.com/ellistarn/karpenter/pkg/utils/log"
)

type Factory struct {
}

type invalidNodeGroup struct{}

var (
	invalidError = fmt.Errorf("invalid nodegroup provider")
)

func (*invalidNodeGroup) SetReplicas(count int32) error { return invalidError }
func (*invalidNodeGroup) GetReplicas() (int32, error)   { return 0, invalidError }

func (f *Factory) For(sng *v1alpha1.ScalableNodeGroup) cloudprovider.NodeGroup {
	var nodegroup cloudprovider.NodeGroup
	switch sng.Spec.Type {
	case v1alpha1.AWSEC2AutoScalingGroup:
		nodegroup = aws.NewAutoScalingGroup(sng.Spec.ID)
	case v1alpha1.AWSEKSNodeGroup:
		nodegroup = aws.NewManagedNodeGroup(sng.Spec.ID)
	default:
		log.InvariantViolated(fmt.Sprintf("Failed to instantiate node group of type %s", sng.Spec.Type))
		nodegroup = &invalidNodeGroup{}
	}
	return nodegroup
}

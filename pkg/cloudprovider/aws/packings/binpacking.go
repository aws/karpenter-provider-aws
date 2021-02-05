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

package packings

import (
	"context"

	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"go.uber.org/zap"
)

type binPacker struct {
	ec2 ec2iface.EC2API
}

// Get returns the packings for the provided pods. Computes an ordered set of viable instance
// types for each packing of pods. Instance variety enables EC2 to make better cost and availability decisions.
func (p *binPacker) Get(ctx context.Context, constraints *cloudprovider.CapacityConstraints) (cloudprovider.Packings, error) {
	zap.S().Infof("Successfully packed %d pods onto %d nodes", len(constraints.Pods), 1)
	nodeID := "1"
	return cloudprovider.Packings{
		nodeID: &cloudprovider.NodePacking{
			InstanceTypeOptions: []string{"m5.large"}, // TODO, prioritize possible instance types
			Pods:                constraints.Pods,
		},
	}, nil
}

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
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/samber/lo"

	sdk "github.com/aws/karpenter-provider-aws/pkg/aws"
)

const ()

// EKSAPIBehavior must be reset between tests otherwise tests will
// pollute each other.
type EKSAPIBehavior struct {
	DescribeClusterBehavior MockedFunction[eks.DescribeClusterInput, eks.DescribeClusterOutput]
}

type EKSAPI struct {
	sdk.EKSAPI
	EKSAPIBehavior
}

func NewEKSAPI() *EKSAPI {
	return &EKSAPI{}
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (s *EKSAPI) Reset() {
	s.DescribeClusterBehavior.Reset()
}

func (s *EKSAPI) DescribeCluster(_ context.Context, input *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	return s.DescribeClusterBehavior.Invoke(input, func(*eks.DescribeClusterInput) (*eks.DescribeClusterOutput, error) {
		return &eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{
				KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
					ServiceIpv4Cidr: lo.ToPtr("10.100.0.0/16"),
				},
				Version: lo.ToPtr("1.29"),
			},
		}, nil
	})
}

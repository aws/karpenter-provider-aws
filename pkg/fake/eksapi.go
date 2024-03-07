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

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/samber/lo"
)

const ()

// EKSAPIBehavior must be reset between tests otherwise tests will
// pollute each other.
type EKSAPIBehavior struct {
	DescribeClusterBehavior MockedFunction[eks.DescribeClusterInput, eks.DescribeClusterOutput]
}

type EKSAPI struct {
	eksiface.EKSAPI
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

func (s *EKSAPI) DescribeClusterWithContext(_ context.Context, input *eks.DescribeClusterInput, _ ...request.Option) (*eks.DescribeClusterOutput, error) {
	return s.DescribeClusterBehavior.Invoke(input, func(*eks.DescribeClusterInput) (*eks.DescribeClusterOutput, error) {
		return &eks.DescribeClusterOutput{
			Cluster: &eks.Cluster{
				KubernetesNetworkConfig: &eks.KubernetesNetworkConfigResponse{
					ServiceIpv4Cidr: lo.ToPtr("10.100.0.0/16"),
				},
			},
		}, nil
	})
}

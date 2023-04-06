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
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
)

const ()

// EKSAPIBehavior must be reset between tests otherwise tests will
// pollute each other.
type EKSAPIBehavior struct {
	DescribeClusterBehaviour MockedFunction[eks.DescribeClusterInput, eks.DescribeClusterOutput]
}

type EKSAPI struct {
	eksiface.EKSAPI
	EKSAPIBehavior
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (s *EKSAPI) Reset() {
	s.DescribeClusterBehaviour.Reset()
}

func (s *EKSAPI) DescribeCluster(input *eks.DescribeClusterInput) (*eks.DescribeClusterOutput, error) {
	return s.DescribeClusterBehaviour.Invoke(input, func(*eks.DescribeClusterInput) (*eks.DescribeClusterOutput, error) {
		return nil, nil
	})
}

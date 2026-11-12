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

package ipv6_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/ipv6"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIPv6(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		ipv6.Env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		ipv6.Env.Stop()
	})
	RunSpecs(t, "IPv6")
}

var _ = BeforeEach(func() {
	ipv6.Env.BeforeEach()
	ipv6.NodeClass = ipv6.Env.DefaultEC2NodeClass()
	ipv6.NodePool = ipv6.Env.DefaultNodePool(ipv6.NodeClass)
	ipv6.NodePool = coretest.ReplaceRequirements(ipv6.NodePool,
		karpv1.NodeSelectorRequirementWithMinValues{
			Key:      v1.LabelInstanceCategory,
			Operator: corev1.NodeSelectorOpExists,
		},
		karpv1.NodeSelectorRequirementWithMinValues{
			Key:      corev1.LabelInstanceTypeStable,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{"c5.large"},
		},
	)
})
var _ = AfterEach(func() { ipv6.Env.Cleanup() })
var _ = AfterEach(func() { ipv6.Env.AfterEach() })

var _ = ipv6.RegisterTests()

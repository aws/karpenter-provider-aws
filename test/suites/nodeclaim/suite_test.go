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

package nodeclaim_test

import (
	"testing"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/nodeclaim"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNodeClaim(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		nodeclaim.Env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		nodeclaim.Env.Stop()
	})
	RunSpecs(t, "NodeClaim")
}

var _ = BeforeEach(func() {
	nodeclaim.Env.BeforeEach()
	nodeclaim.NodeClass = nodeclaim.Env.DefaultEC2NodeClass()
	nodeclaim.NodePool = nodeclaim.Env.DefaultNodePool(nodeclaim.NodeClass)
})
var _ = AfterEach(func() { nodeclaim.Env.Cleanup() })
var _ = AfterEach(func() { nodeclaim.Env.AfterEach() })

var _ = nodeclaim.RegisterTests()

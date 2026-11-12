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

package consolidation_test

import (
	"testing"

	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/consolidation"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConsolidation(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		consolidation.Env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		consolidation.Env.Stop()
	})
	RunSpecs(t, "Consolidation")
}

var _ = BeforeEach(func() {
	consolidation.NodeClass = consolidation.Env.DefaultEC2NodeClass()
	consolidation.Env.BeforeEach()
})
var _ = AfterEach(func() { consolidation.Env.Cleanup() })
var _ = AfterEach(func() { consolidation.Env.AfterEach() })

var _ = consolidation.RegisterTests()

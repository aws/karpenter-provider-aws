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

package integration_test

import (
	"testing"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		integration.Env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		integration.Env.Stop()
	})
	RunSpecs(t, "Integration")
}

var _ = BeforeEach(func() {
	integration.Env.BeforeEach()
	integration.NodeClass = integration.Env.DefaultEC2NodeClass()
	integration.NodePool = integration.Env.DefaultNodePool(integration.NodeClass)
})
var _ = AfterEach(func() { integration.Env.Cleanup() })
var _ = AfterEach(func() { integration.Env.AfterEach() })

var _ = integration.RegisterTests()

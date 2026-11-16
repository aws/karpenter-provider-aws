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

package interruption_test

import (
	"testing"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/interruption"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInterruption(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		interruption.Env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		interruption.Env.Stop()
	})
	RunSpecs(t, "Interruption")
}

var _ = BeforeEach(func() {
	interruption.Env.BeforeEach()
	interruption.NodeClass = interruption.Env.DefaultEC2NodeClass()
	interruption.NodePool = interruption.Env.DefaultNodePool(interruption.NodeClass)
})
var _ = AfterEach(func() { interruption.Env.Cleanup() })
var _ = AfterEach(func() { interruption.Env.AfterEach() })

var _ = interruption.RegisterTests()

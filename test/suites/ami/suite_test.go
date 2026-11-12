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

package ami_test

import (
	"testing"

	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/ami"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAMI(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		ami.Env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		ami.Env.Stop()
	})
	RunSpecs(t, "Ami")
}

var _ = BeforeEach(func() {
	ami.Env.BeforeEach()
	ami.NodeClass = ami.Env.DefaultEC2NodeClass()
	ami.NodePool = ami.Env.DefaultNodePool(ami.NodeClass)
})
var _ = AfterEach(func() { ami.Env.Cleanup() })
var _ = AfterEach(func() { ami.Env.AfterEach() })

var _ = ami.RegisterTests()

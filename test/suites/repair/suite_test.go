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

package repair_test

import (
	"testing"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/repair"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRepair(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		repair.Env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		repair.Env.Stop()
	})
	RunSpecs(t, "Repair")
}

var _ = BeforeEach(func() {
	repair.Env.BeforeEach()
	repair.NodeClass = repair.Env.DefaultEC2NodeClass()
	repair.NodePool = repair.Env.DefaultNodePool(repair.NodeClass)
})
var _ = AfterEach(func() { repair.Env.Cleanup() })
var _ = AfterEach(func() { repair.Env.AfterEach() })

var _ = repair.RegisterTests()

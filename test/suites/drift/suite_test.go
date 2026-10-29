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

package drift_test

import (
	"testing"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/drift"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDrift(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		drift.Env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		drift.Env.Stop()
	})
	RunSpecs(t, "Drift")
}

var _ = BeforeEach(func() {
	drift.Env.BeforeEach()
	drift.NodeClass = drift.Env.DefaultEC2NodeClass()
	drift.NodePool = drift.Env.DefaultNodePool(drift.NodeClass)
})
var _ = AfterEach(func() { drift.Env.Cleanup() })
var _ = AfterEach(func() { drift.Env.AfterEach() })

var _ = drift.RegisterTests()

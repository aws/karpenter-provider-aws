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

package scheduling_besteffort_test

import (
	"testing"

	"sigs.k8s.io/karpenter/pkg/operator/options"

	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/scheduling"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSchedulingBestEffort(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		scheduling.Env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		scheduling.Env.Stop()
	})
	RunSpecs(t, "SchedulingBestEffort")
}

var _ = BeforeEach(func() {
	scheduling.Env.BeforeEach()
	scheduling.NodeClass = scheduling.Env.DefaultEC2NodeClass()
	scheduling.NodePool = scheduling.Env.DefaultNodePool(scheduling.NodeClass)
})
var _ = AfterEach(func() { scheduling.Env.Cleanup() })
var _ = AfterEach(func() { scheduling.Env.AfterEach() })

var _ = scheduling.RegisterTests(options.MinValuesPolicyBestEffort)

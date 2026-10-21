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

package localzone_test

import (
	"testing"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/localzone"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLocalZone(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		localzone.Env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		localzone.Env.Stop()
	})
	RunSpecs(t, "LocalZone")
}

var _ = BeforeEach(func() {
	localzone.Env.BeforeEach()
	localzone.NodeClass = localzone.Env.DefaultEC2NodeClass()
	localzone.NodePool = localzone.Env.DefaultNodePool(localzone.NodeClass)
})
var _ = AfterEach(func() { localzone.Env.Cleanup() })
var _ = AfterEach(func() { localzone.Env.AfterEach() })

var _ = localzone.RegisterTests()

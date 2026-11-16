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

package legacyrepair_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/suites/legacyrepair"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLegacyRepair(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		legacyrepair.Env = aws.NewEnvironment(t)
		// Set once for the suite: each spec's BeforeEach persists the current settings, so AfterEach keeps the override
		legacyrepair.Env.ExpectSettingsOverridden(corev1.EnvVar{Name: "LEGACY_NODE_REPAIR", Value: "true"})
	})
	AfterSuite(func() {
		legacyrepair.Env.ExpectSettingsRemoved(corev1.EnvVar{Name: "LEGACY_NODE_REPAIR"})
		legacyrepair.Env.Stop()
	})
	RunSpecs(t, "LegacyRepair")
}

var _ = BeforeEach(func() {
	legacyrepair.Env.BeforeEach()
	legacyrepair.NodeClass = legacyrepair.Env.DefaultEC2NodeClass()
	legacyrepair.NodePool = legacyrepair.Env.DefaultNodePool(legacyrepair.NodeClass)
})
var _ = AfterEach(func() { legacyrepair.Env.Cleanup() })
var _ = AfterEach(func() { legacyrepair.Env.AfterEach() })

var _ = legacyrepair.RegisterTests()

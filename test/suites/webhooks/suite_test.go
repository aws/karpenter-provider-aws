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

package webhooks_test

import (
	"context"
	"testing"

	v1beta1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	karpv1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	karptest "sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var env *aws.Environment
var nodeClass *v1beta1.EC2NodeClass
var nodePool *karpv1beta1.NodePool

func TestWebhooks(t *testing.T) {
	RegisterFailHandler(Fail)

	ctx = TestContextWithLogger(t)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Webhooks")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Webhooks", func() {
	It("should schedule pods when webhooks are disabled", func() {
		pod := karptest.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
	})
})

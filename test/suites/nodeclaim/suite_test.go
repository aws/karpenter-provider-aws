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

package nodeclaim_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/samber/lo"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *environmentaws.Environment
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool

func TestNodeClaim(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "NodeClaim")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("GarbageCollection", func() {
	It("should succeed to garbage collect an Instance that was deleted without the cluster's knowledge", func() {
		// Disable the interruption queue for the garbage collection coretest
		env.ExpectSettingsOverridden(corev1.EnvVar{Name: "INTERRUPTION_QUEUE", Value: ""})
		pod := coretest.Pod()
		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		_, err := env.EC2API.TerminateInstances(env.Context, &ec2.TerminateInstancesInput{
			InstanceIds: []string{lo.Must(utils.ParseInstanceID(node.Spec.ProviderID))},
		})
		Expect(err).ToNot(HaveOccurred())

		// The garbage collection mechanism should eventually delete this NodeClaim and Node
		env.EventuallyExpectNotFound(node)
	})
})

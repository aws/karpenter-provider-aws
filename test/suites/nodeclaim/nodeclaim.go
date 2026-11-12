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

package nodeclaim

import (
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

var Env *environmentaws.Environment
var NodeClass *v1.EC2NodeClass
var NodePool *karpv1.NodePool

func RegisterTests() bool {
	return Describe("GarbageCollection", func() {
		It("should succeed to garbage collect an Instance that was deleted without the cluster's knowledge", func() {
			// Disable the interruption queue for the garbage collection coretest
			Env.ExpectSettingsOverridden(corev1.EnvVar{Name: "INTERRUPTION_QUEUE", Value: ""})
			pod := coretest.Pod()
			Env.ExpectCreated(NodeClass, NodePool, pod)
			Env.EventuallyExpectHealthy(pod)
			node := Env.ExpectCreatedNodeCount("==", 1)[0]

			_, err := Env.EC2API.TerminateInstances(Env.Context, &ec2.TerminateInstancesInput{
				InstanceIds: []string{lo.Must(utils.ParseInstanceID(node.Spec.ProviderID))},
			})
			Expect(err).ToNot(HaveOccurred())

			// The garbage collection mechanism should eventually delete this NodeClaim and Node
			Env.EventuallyExpectNotFound(node)
		})
	})
}

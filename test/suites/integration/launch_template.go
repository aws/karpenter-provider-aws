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

package integration

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func registerLaunchTemplate() bool {
	return Describe("Launch Template Deletion", func() {
		It("should remove the generated Launch Templates when deleting the NodeClass", func() {
			pod := coretest.Pod()
			Env.ExpectCreated(NodePool, NodeClass, pod)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectDeleted(NodePool, NodeClass)
			Eventually(func(g Gomega) {
				output, _ := Env.EC2API.DescribeLaunchTemplates(Env.Context, &ec2.DescribeLaunchTemplatesInput{
					Filters: []ec2types.Filter{
						{Name: aws.String(fmt.Sprintf("tag:%s", v1.LabelNodeClass)), Values: []string{NodeClass.Name}},
					},
				})
				g.Expect(output.LaunchTemplates).To(HaveLen(0))
			}).WithPolling(5.0).Should(Succeed())
		})
	})
}

func registerNitroEnclaves() bool {
	return Describe("Nitro Enclaves", func() {
		It("should launch instances with Nitro Enclaves enabled", func() {
			NodeClass.Spec.EnclaveOptions = &v1.EnclaveOptions{Enabled: true}
			pod := coretest.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			instance := Env.GetInstance(pod.Spec.NodeName)
			Expect(instance.EnclaveOptions).ToNot(BeNil())
			Expect(instance.EnclaveOptions.Enabled).To(HaveValue(BeTrue()))
		})
	})
}

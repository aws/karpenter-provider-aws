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

package integration_test

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

var _ = Describe("Launch Template Deletion", func() {
	It("should remove the generated Launch Templates when deleting the NodeClass", func() {
		pod := coretest.Pod()
		env.ExpectCreated(nodePool, nodeClass, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectDeleted(nodePool, nodeClass)
		Eventually(func(g Gomega) {
			output, _ := env.EC2API.DescribeLaunchTemplates(env.Context, &ec2.DescribeLaunchTemplatesInput{
				Filters: []ec2types.Filter{
					{Name: aws.String(fmt.Sprintf("tag:%s", v1.LabelNodeClass)), Values: []string{nodeClass.Name}},
				},
			})
			g.Expect(output.LaunchTemplates).To(HaveLen(0))
		}).WithPolling(5.0).Should(Succeed())
	})
})

var _ = Describe("Nitro Enclaves", func() {
	It("should launch instances with Nitro Enclaves enabled", func() {
		nodeClass.Spec.EnclaveOptions = &v1.EnclaveOptions{
			Enabled: aws.Bool(true),
		}
		pod := coretest.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.EnclaveOptions).ToNot(BeNil())
		Expect(instance.EnclaveOptions.Enabled).To(HaveValue(BeTrue()))
	})
})

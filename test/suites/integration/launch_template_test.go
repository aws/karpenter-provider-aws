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

var _ = Describe("Launch Template CPU Options", func() {
	It("should create launch template with CPU options", func() {
		nodeClass.Spec.CPUOptions = &v1.CPUOptions{
			CoreCount:          aws.Int32(2),
			ThreadsPerCore:    aws.Int32(1),
			NestedVirtualization: aws.String("enabled"),
		}
		
		pod := coretest.Pod()
		env.ExpectCreated(nodePool, nodeClass, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		// Verify the launch template was created with CPU options
		Eventually(func(g Gomega) {
			output, err := env.EC2API.DescribeLaunchTemplates(env.Context, &ec2.DescribeLaunchTemplatesInput{
				Filters: []ec2types.Filter{
					{Name: aws.String(fmt.Sprintf("tag:%s", v1.LabelNodeClass)), Values: []string{nodeClass.Name}},
				},
			})
			g.Expect(err).To(BeNil())
			g.Expect(output.LaunchTemplates).To(HaveLen(1))
			
			// Get the launch template data to verify CPU options
			ltVersion := aws.ToString(output.LaunchTemplates[0].LatestVersionNumber)
			ltOutput, err := env.EC2API.DescribeLaunchTemplateVersions(env.Context, &ec2.DescribeLaunchTemplateVersionsInput{
				LaunchTemplateId: output.LaunchTemplates[0].LaunchTemplateId,
				Versions:         []string{ltVersion},
			})
			g.Expect(err).To(BeNil())
			g.Expect(ltOutput.LaunchTemplateVersions).To(HaveLen(1))
			
			ltData := ltOutput.LaunchTemplateVersions[0].LaunchTemplateData
			g.Expect(ltData.CpuOptions).ToNot(BeNil())
			g.Expect(ltData.CpuOptions.CoreCount).To(Equal(aws.Int32(2)))
			g.Expect(ltData.CpuOptions.ThreadsPerCore).To(Equal(aws.Int32(1)))
			// Note: NestedVirtualization may not be supported in all AWS regions/instance types yet
		}).WithPolling(5.0).Should(Succeed())
	})
})

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
	"github.com/aws/aws-sdk-go/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter-core/pkg/utils/resources"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("BlockDeviceMappings", func() {
	It("should use specified block device mappings", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				LaunchTemplate: v1alpha1.LaunchTemplate{
					BlockDeviceMappings: []*v1alpha1.BlockDeviceMapping{
						{
							DeviceName: aws.String("/dev/xvda"),
							EBS: &v1alpha1.BlockDevice{
								VolumeSize:          resources.Quantity("10G"),
								VolumeType:          aws.String("io2"),
								IOPS:                aws.Int64(1000),
								Encrypted:           aws.Bool(true),
								DeleteOnTermination: aws.Bool(true),
							},
						},
					},
				},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(len(instance.BlockDeviceMappings)).To(Equal(1))
		Expect(instance.BlockDeviceMappings[0]).ToNot(BeNil())
		Expect(instance.BlockDeviceMappings[0]).To(HaveField("DeviceName", HaveValue(Equal("/dev/xvda"))))
		Expect(instance.BlockDeviceMappings[0].Ebs).To(HaveField("DeleteOnTermination", HaveValue(BeTrue())))
		volume := env.GetVolume(instance.BlockDeviceMappings[0].Ebs.VolumeId)
		Expect(volume).To(HaveField("Encrypted", HaveValue(BeTrue())))
		Expect(volume).To(HaveField("Size", HaveValue(Equal(int64(9))))) // Convert G -> Gib
		Expect(volume).To(HaveField("Iops", HaveValue(Equal(int64(1000)))))
		Expect(volume).To(HaveField("VolumeType", HaveValue(Equal("io2"))))
	})
})

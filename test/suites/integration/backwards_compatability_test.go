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
	"encoding/base64"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
	environmentaws "github.com/aws/karpenter/test/pkg/environment/aws"
)

var _ = Describe("BackwardsCompatability", func() {
	It("should succeed to launch a node by specifying a provider in the Provisioner", func() {
		provisioner := test.Provisioner(
			test.ProvisionerOptions{
				Provider: &v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					Tags: map[string]string{
						"custom-tag":  "custom-value",
						"custom-tag2": "custom-value2",
					},
				},
			},
		)
		pod := test.Pod()
		env.ExpectCreated(pod, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		nodes := env.Monitor.CreatedNodes()
		Expect(nodes).To(HaveLen(1))
		Expect(env.GetInstance(nodes[0].Name).Tags).To(ContainElements(
			&ec2.Tag{Key: lo.ToPtr("custom-tag"), Value: lo.ToPtr("custom-value")},
			&ec2.Tag{Key: lo.ToPtr("custom-tag2"), Value: lo.ToPtr("custom-value2")},
		))
	})
	Context("MachineHydration", func() {
		var customAMI string
		var instanceInput *ec2.RunInstancesInput

		BeforeEach(func() {
			securityGroups := env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
			subnets := env.GetSubnetNameAndIds(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
			Expect(securityGroups).ToNot(HaveLen(0))
			Expect(subnets).ToNot(HaveLen(0))

			customAMI = env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
			instanceInput = &ec2.RunInstancesInput{
				InstanceType: aws.String("c5.large"),
				IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
					Name: aws.String(settings.FromContext(env.Context).DefaultInstanceProfile),
				},
				SecurityGroupIds: lo.Map(securityGroups, func(s environmentaws.SecurityGroup, _ int) *string {
					return s.GroupIdentifier.GroupId
				}),
				SubnetId: aws.String(subnets[0].ID),
				BlockDeviceMappings: []*ec2.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &ec2.EbsBlockDevice{
							Encrypted:           aws.Bool(true),
							DeleteOnTermination: aws.Bool(true),
							VolumeType:          aws.String(ec2.VolumeTypeGp3),
							VolumeSize:          aws.Int64(20),
						},
					},
				},
				ImageId: aws.String(customAMI), // EKS AL2-based AMI
				TagSpecifications: []*ec2.TagSpecification{
					{
						ResourceType: aws.String(ec2.ResourceTypeInstance),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", settings.FromContext(env.Context).ClusterName)),
								Value: aws.String("owned"),
							},
						},
					},
				},
				MinCount: aws.Int64(1),
				MaxCount: aws.Int64(1),
			}
		})
		It("should succeed to hydrate a Machine for an existing instance launched by Karpenter", func() {
			Skip("machine hydration is not yet enabled")

			provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					AMIFamily:             &v1alpha1.AMIFamilyAL2,
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				},
			})
			provisioner := test.Provisioner(test.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceCategory,
						Operator: v1.NodeSelectorOpExists,
					},
				},
			})
			env.ExpectCreated(provider, provisioner)

			// Update the userData for the instance input with the correct provisionerName
			rawContent, err := os.ReadFile("testdata/al2_manual_launch_userdata_input.sh")
			Expect(err).ToNot(HaveOccurred())
			instanceInput.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), settings.FromContext(env.Context).ClusterName,
				settings.FromContext(env.Context).ClusterEndpoint, env.ExpectCABundle(), provisioner.Name))))

			// Create an instance manually to mock Karpenter launching an instance
			_, err = env.EC2API.RunInstances(instanceInput)
			Expect(err).ToNot(HaveOccurred())

			// Wait for the node to register with the cluster
			env.EventuallyExpectCreatedNodeCount("==", 1)
			machines := env.EventuallyExpectCreatedMachineCount("==", 1)
			machine := machines[0]

			// Expect the machine's fields are properly populated
			Expect(machine.Spec.Requirements).To(Equal(provisioner.Spec.Requirements))
			Expect(machine.Spec.MachineTemplateRef.Name).To(Equal(provider.Name))
		})
		It("should succeed to hydrate a Machine for an existing instance launched by Karpenter with provider", func() {
			Skip("machine hydration is not yet enabled")

			provisioner := test.Provisioner(test.ProvisionerOptions{
				Provider: &v1alpha1.AWS{
					AMIFamily:             &v1alpha1.AMIFamilyAL2,
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				},
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceCategory,
						Operator: v1.NodeSelectorOpExists,
					},
				},
			})
			env.ExpectCreated(provisioner)

			// Update the userData for the instance input with the correct provisionerName
			rawContent, err := os.ReadFile("testdata/al2_manual_launch_userdata_input.sh")
			Expect(err).ToNot(HaveOccurred())
			instanceInput.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), settings.FromContext(env.Context).ClusterName,
				settings.FromContext(env.Context).ClusterEndpoint, env.ExpectCABundle(), provisioner.Name))))

			// Create an instance manually to mock Karpenter launching an instance
			_, err = env.EC2API.RunInstances(instanceInput)
			Expect(err).ToNot(HaveOccurred())

			// Wait for the node to register with the cluster
			env.EventuallyExpectCreatedNodeCount("==", 1)
			machines := env.EventuallyExpectCreatedMachineCount("==", 1)
			machine := machines[0]

			// Expect the machine's fields are properly populated
			Expect(machine.Spec.Requirements).To(Equal(provisioner.Spec.Requirements))
			Expect(machine.Annotations).To(HaveKeyWithValue(v1alpha5.ProviderCompatabilityAnnotationKey, v1alpha5.ProviderAnnotation(provisioner.Spec.Provider)[v1alpha5.ProviderCompatabilityAnnotationKey]))
		})
	})
})

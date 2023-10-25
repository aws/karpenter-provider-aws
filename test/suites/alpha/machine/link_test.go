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

package machine_test

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awserrors "github.com/aws/karpenter/pkg/errors"
	awstest "github.com/aws/karpenter/pkg/test"
	environmentaws "github.com/aws/karpenter/test/pkg/environment/aws"
)

var _ = Describe("MachineLink", func() {
	var customAMI string
	var instanceInput *ec2.RunInstancesInput

	BeforeEach(func() {
		securityGroups := env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		subnets := env.GetSubnetNameAndIds(map[string]string{"karpenter.sh/discovery": env.ClusterName})
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
							Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", env.ClusterName)),
							Value: aws.String("owned"),
						},
					},
				},
			},
			MinCount: aws.Int64(1),
			MaxCount: aws.Int64(1),
		}
	})
	It("should succeed to link a Machine for an existing instance launched by Karpenter", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			AMIFamily:             &v1alpha1.AMIFamilyAL2,
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
		})
		env.ExpectCreated(provisioner, provider)

		// Update the userData for the instance input with the correct provisionerName
		rawContent, err := os.ReadFile("testdata/al2_userdata_input.sh")
		Expect(err).ToNot(HaveOccurred())
		instanceInput.TagSpecifications[0].Tags = append(instanceInput.TagSpecifications[0].Tags, &ec2.Tag{
			Key:   aws.String(v1alpha5.ProvisionerNameLabelKey),
			Value: aws.String(provisioner.Name),
		})
		instanceInput.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle(), provisioner.Name))))

		// Create an instance manually to mock Karpenter launching an instance
		out := env.ExpectRunInstances(instanceInput)
		Expect(out.Instances).To(HaveLen(1))

		// Always ensure that we cleanup the instance
		DeferCleanup(func() {
			_, err := env.EC2API.TerminateInstances(&ec2.TerminateInstancesInput{
				InstanceIds: []*string{out.Instances[0].InstanceId},
			})
			if awserrors.IsNotFound(err) {
				return
			}
			Expect(err).ToNot(HaveOccurred())
		})

		// Wait for the node to register with the cluster
		env.EventuallyExpectCreatedNodeCount("==", 1)

		// Restart Karpenter to start the linking process
		env.EventuallyExpectKarpenterRestarted()

		// Expect that the Machine is created when Karpenter starts up
		machines := env.EventuallyExpectCreatedMachineCount("==", 1)
		machine := machines[0]

		// Expect the machine's fields are properly populated
		Expect(machine.Spec.Requirements).To(Equal(provisioner.Spec.Requirements))
		Expect(machine.Spec.MachineTemplateRef.Name).To(Equal(provider.Name))

		// Expect the instance to have the karpenter.sh/managed-by tag
		Eventually(func(g Gomega) {
			instance := env.GetInstanceByID(aws.StringValue(out.Instances[0].InstanceId))
			tag, ok := lo.Find(instance.Tags, func(t *ec2.Tag) bool {
				return aws.StringValue(t.Key) == v1alpha5.MachineManagedByAnnotationKey
			})
			g.Expect(ok).To(BeTrue())
			g.Expect(aws.StringValue(tag.Value)).To(Equal(env.ClusterName))
		}, time.Minute, time.Second).Should(Succeed())
	})
	It("should succeed to link a Machine for an existing instance launched by Karpenter with provider", func() {
		provisioner := test.Provisioner(test.ProvisionerOptions{
			Provider: &v1alpha1.AWS{
				AMIFamily:             &v1alpha1.AMIFamilyAL2,
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			},
		})
		env.ExpectCreated(provisioner)

		// Update the userData for the instance input with the correct provisionerName
		rawContent, err := os.ReadFile("testdata/al2_userdata_input.sh")
		Expect(err).ToNot(HaveOccurred())
		instanceInput.TagSpecifications[0].Tags = append(instanceInput.TagSpecifications[0].Tags, &ec2.Tag{
			Key:   aws.String(v1alpha5.ProvisionerNameLabelKey),
			Value: aws.String(provisioner.Name),
		})
		instanceInput.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle(), provisioner.Name))))

		// Create an instance manually to mock Karpenter launching an instance
		out := env.ExpectRunInstances(instanceInput)
		Expect(out.Instances).To(HaveLen(1))

		// Always ensure that we cleanup the instance
		DeferCleanup(func() {
			_, err := env.EC2API.TerminateInstances(&ec2.TerminateInstancesInput{
				InstanceIds: []*string{out.Instances[0].InstanceId},
			})
			if awserrors.IsNotFound(err) {
				return
			}
			Expect(err).ToNot(HaveOccurred())
		})

		// Wait for the node to register with the cluster
		env.EventuallyExpectCreatedNodeCount("==", 1)

		// Restart Karpenter to start the linking process
		env.EventuallyExpectKarpenterRestarted()

		// Expect that the Machine is created when Karpenter starts up
		machines := env.EventuallyExpectCreatedMachineCount("==", 1)
		machine := machines[0]

		// Expect the machine's fields are properly populated
		Expect(machine.Spec.Requirements).To(Equal(provisioner.Spec.Requirements))
		Expect(machine.Annotations).To(HaveKeyWithValue(v1alpha5.ProviderCompatabilityAnnotationKey, v1alpha5.ProviderAnnotation(provisioner.Spec.Provider)[v1alpha5.ProviderCompatabilityAnnotationKey]))

		// Expect the instance to have the karpenter.sh/managed-by tag
		Eventually(func(g Gomega) {
			instance := env.GetInstanceByID(aws.StringValue(out.Instances[0].InstanceId))
			tag, ok := lo.Find(instance.Tags, func(t *ec2.Tag) bool {
				return aws.StringValue(t.Key) == v1alpha5.MachineManagedByAnnotationKey
			})
			g.Expect(ok).To(BeTrue())
			g.Expect(aws.StringValue(tag.Value)).To(Equal(env.ClusterName))
		}, time.Minute, time.Second).Should(Succeed())
	})
	It("should succeed to link a Machine for an existing instance re-owned by Karpenter", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			AMIFamily:             &v1alpha1.AMIFamilyAL2,
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
		})
		env.ExpectCreated(provisioner, provider)

		// Update the userData for the instance input with the correct provisionerName
		rawContent, err := os.ReadFile("testdata/al2_userdata_input.sh")
		Expect(err).ToNot(HaveOccurred())

		// No tag specifications since we're mocking an instance not launched by Karpenter
		instanceInput.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle(), provisioner.Name))))

		// Create an instance manually to mock Karpenter launching an instance
		out := env.ExpectRunInstances(instanceInput)
		Expect(out.Instances).To(HaveLen(1))

		// Always ensure that we cleanup the instance
		DeferCleanup(func() {
			_, err := env.EC2API.TerminateInstances(&ec2.TerminateInstancesInput{
				InstanceIds: []*string{out.Instances[0].InstanceId},
			})
			if awserrors.IsNotFound(err) {
				return
			}
			Expect(err).ToNot(HaveOccurred())
		})

		// Wait for the node to register with the cluster
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]

		// Add the provisioner-name label to the node to re-own it
		stored := node.DeepCopy()
		node.Labels[v1alpha5.ProvisionerNameLabelKey] = provisioner.Name
		Expect(env.Client.Patch(env.Context, node, client.MergeFrom(stored))).To(Succeed())

		// Restart Karpenter to start the linking process
		env.EventuallyExpectKarpenterRestarted()

		// Expect that the Machine is created when Karpenter starts up
		machines := env.EventuallyExpectCreatedMachineCount("==", 1)
		machine := machines[0]

		// Expect the machine's fields are properly populated
		Expect(machine.Spec.Requirements).To(Equal(provisioner.Spec.Requirements))
		Expect(machine.Spec.MachineTemplateRef.Name).To(Equal(provider.Name))

		// Expect the instance to have the karpenter.sh/managed-by tag and the karpenter.sh/provisioner-name tag
		Eventually(func(g Gomega) {
			instance := env.GetInstanceByID(aws.StringValue(out.Instances[0].InstanceId))
			tag, ok := lo.Find(instance.Tags, func(t *ec2.Tag) bool {
				return aws.StringValue(t.Key) == v1alpha5.MachineManagedByAnnotationKey
			})
			g.Expect(ok).To(BeTrue())
			g.Expect(aws.StringValue(tag.Value)).To(Equal(env.ClusterName))
			tag, ok = lo.Find(instance.Tags, func(t *ec2.Tag) bool {
				return aws.StringValue(t.Key) == v1alpha5.ProvisionerNameLabelKey
			})
			g.Expect(ok).To(BeTrue())
			g.Expect(aws.StringValue(tag.Value)).To(Equal(provisioner.Name))
		}, time.Minute, time.Second).Should(Succeed())
	})
})

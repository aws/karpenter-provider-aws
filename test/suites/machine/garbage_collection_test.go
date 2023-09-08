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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awserrors "github.com/aws/karpenter/pkg/errors"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils"
	environmentaws "github.com/aws/karpenter/test/pkg/environment/aws"
)

var _ = Describe("NodeClaimGarbageCollection", func() {
	var customAMI string
	var instanceInput *ec2.RunInstancesInput
	var provisioner *v1alpha5.Provisioner

	BeforeEach(func() {
		provisioner = test.Provisioner()
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
						{
							Key:   aws.String(v1alpha5.ProvisionerNameLabelKey),
							Value: aws.String(provisioner.Name),
						},
					},
				},
			},
			MinCount: aws.Int64(1),
			MaxCount: aws.Int64(1),
		}
	})
	It("should succeed to garbage collect a Machine that was launched by a Machine but has no Machine mapping", func() {
		// Update the userData for the instance input with the correct provisionerName
		rawContent, err := os.ReadFile("testdata/al2_userdata_input.sh")
		Expect(err).ToNot(HaveOccurred())
		instanceInput.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), settings.FromContext(env.Context).ClusterName,
			settings.FromContext(env.Context).ClusterEndpoint, env.ExpectCABundle(), provisioner.Name))))

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

		// Update the tags to add the karpenter.sh/managed-by tag
		_, err = env.EC2API.CreateTagsWithContext(env.Context, &ec2.CreateTagsInput{
			Resources: []*string{out.Instances[0].InstanceId},
			Tags: []*ec2.Tag{
				{
					Key:   aws.String(v1alpha5.MachineManagedByAnnotationKey),
					Value: aws.String(settings.FromContext(env.Context).ClusterName),
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		// Eventually expect the node and the instance to be removed (shutting-down)
		env.EventuallyExpectNotFound(node)
		Eventually(func(g Gomega) {
			g.Expect(lo.FromPtr(env.GetInstanceByID(aws.StringValue(out.Instances[0].InstanceId)).State.Name)).To(Equal("shutting-down"))
		}, time.Second*10).Should(Succeed())
	})
	It("should succeed to garbage collect a Machine that was deleted without the cluster's knowledge", func() {
		// Disable the interruption queue for the garbage collection test
		env.ExpectSettingsOverridden(map[string]string{
			"aws.interruptionQueueName": "",
		})

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
		})

		pod := test.Pod()
		env.ExpectCreated(provisioner, provider, pod)
		env.EventuallyExpectHealthy(pod)
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		_, err := env.EC2API.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: aws.StringSlice([]string{lo.Must(utils.ParseInstanceID(node.Spec.ProviderID))}),
		})
		Expect(err).ToNot(HaveOccurred())

		// The garbage collection mechanism should eventually delete this machine and node
		env.EventuallyExpectNotFound(node)
	})
})

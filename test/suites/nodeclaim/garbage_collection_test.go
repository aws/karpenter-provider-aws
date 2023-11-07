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
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	awserrors "github.com/aws/karpenter/pkg/errors"
	"github.com/aws/karpenter/pkg/utils"
	environmentaws "github.com/aws/karpenter/test/pkg/environment/aws"
)

var _ = Describe("GarbageCollection", func() {
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
						{
							Key:   aws.String(corev1beta1.NodePoolLabelKey),
							Value: aws.String(nodePool.Name),
						},
					},
				},
			},
			MinCount: aws.Int64(1),
			MaxCount: aws.Int64(1),
		}
	})
	It("should succeed to garbage collect an Instance that was launched by a NodeClaim but has no Instance mapping", func() {
		// Update the userData for the instance input with the correct NodePool
		rawContent, err := os.ReadFile("testdata/al2_userdata_input.sh")
		Expect(err).ToNot(HaveOccurred())
		instanceInput.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), env.ClusterName,
			env.ClusterEndpoint, env.ExpectCABundle(), nodePool.Name))))

		instanceProfileName := fmt.Sprintf("KarpenterNodeInstanceProfile-%s", env.ClusterName)
		ExpectInstanceProfileCreated(instanceProfileName)
		DeferCleanup(func() {
			ExpectInstanceProfileDeleted(instanceProfileName)
		})
		// Create an instance manually to mock Karpenter launching an instance
		out := env.EventuallyExpectRunInstances(instanceInput)
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
					Key:   aws.String(corev1beta1.ManagedByAnnotationKey),
					Value: aws.String(env.ClusterName),
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
	It("should succeed to garbage collect an Instance that was deleted without the cluster's knowledge", func() {
		// Disable the interruption queue for the garbage collection coretest
		env.ExpectSettingsOverridden(v1.EnvVar{Name: "INTERRUPTION_QUEUE", Value: ""})

		pod := coretest.Pod()
		env.ExpectCreated(nodeClass, nodePool, pod)
		env.EventuallyExpectHealthy(pod)
		node := env.ExpectCreatedNodeCount("==", 1)[0]

		_, err := env.EC2API.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: aws.StringSlice([]string{lo.Must(utils.ParseInstanceID(node.Spec.ProviderID))}),
		})
		Expect(err).ToNot(HaveOccurred())

		// The garbage collection mechanism should eventually delete this NodeClaim and Node
		env.EventuallyExpectNotFound(node)
	})
})

func ExpectInstanceProfileCreated(instanceProfileName string) {
	By("creating an instance profile")
	createInstanceProfile := &iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
		Tags: []*iam.Tag{
			{
				Key:   aws.String(coretest.DiscoveryLabel),
				Value: aws.String(env.ClusterName),
			},
		},
	}
	By("adding the karpenter role to new instance profile")
	_, err := env.IAMAPI.CreateInstanceProfile(createInstanceProfile)
	Expect(awserrors.IgnoreAlreadyExists(err)).ToNot(HaveOccurred())
	addInstanceProfile := &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
		RoleName:            aws.String(fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName)),
	}
	_, err = env.IAMAPI.AddRoleToInstanceProfile(addInstanceProfile)
	Expect(ignoreAlreadyContainsRole(err)).ToNot(HaveOccurred())
}

func ExpectInstanceProfileDeleted(instanceProfileName string) {
	By("deleting an instance profile")
	removeRoleFromInstanceProfile := &iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
		RoleName:            aws.String(fmt.Sprintf("KarpenterNodeRole-%s", env.ClusterName)),
	}
	_, err := env.IAMAPI.RemoveRoleFromInstanceProfile(removeRoleFromInstanceProfile)
	Expect(awserrors.IgnoreNotFound(err)).To(BeNil())

	deleteInstanceProfile := &iam.DeleteInstanceProfileInput{
		InstanceProfileName: aws.String(instanceProfileName),
	}
	_, err = env.IAMAPI.DeleteInstanceProfile(deleteInstanceProfile)
	Expect(awserrors.IgnoreNotFound(err)).ToNot(HaveOccurred())
}

func ignoreAlreadyContainsRole(err error) error {
	if err != nil {
		if strings.Contains(err.Error(), "Cannot exceed quota for InstanceSessionsPerInstanceProfile") {
			return nil
		}
	}
	return err
}

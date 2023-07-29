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
)

var _ = Describe("NetworkInterfaces", func() {
	It("should create a default NetworkInterface if none specified, with no public IP auto assignment", func() {
		subnets := env.GetSubnets(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(subnets)).ToNot(Equal(0))
		allSubnets := lo.Flatten(lo.Values(subnets))
		ExpectAssociatePublicIPAddressToBe(false, allSubnets...)

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				LaunchTemplate:        v1alpha1.LaunchTemplate{},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()
		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).To(HaveLen(1))
		Expect(instance.NetworkInterfaces[0]).ToNot(BeNil())
		Expect(instance.NetworkInterfaces[0].Attachment).To(HaveField("DeviceIndex", HaveValue(Equal(int64(0)))))
		Expect(instance.NetworkInterfaces[0].Attachment).To(HaveField("NetworkCardIndex", HaveValue(Equal(int64(0)))))
		Expect(instance.PublicIpAddress).To(BeNil())
	})
	It("should create a default NetworkInterface if none specified, with public IP auto assignment", func() {
		subnets := env.GetSubnets(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(subnets)).ToNot(Equal(0))
		allSubnets := lo.Flatten(lo.Values(subnets))
		ExpectAssociatePublicIPAddressToBe(true, allSubnets...)

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				LaunchTemplate:        v1alpha1.LaunchTemplate{},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()
		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).To(HaveLen(1))
		Expect(instance.NetworkInterfaces[0]).ToNot(BeNil())
		Expect(instance.NetworkInterfaces[0].Attachment).To(HaveField("DeviceIndex", HaveValue(Equal(int64(0)))))
		Expect(instance.NetworkInterfaces[0].Attachment).To(HaveField("NetworkCardIndex", HaveValue(Equal(int64(0)))))
		Expect(instance.PublicIpAddress).ToNot(BeNil())
	})
	It("should use the specified NetworkInterface", func() {
		subnets := env.GetSubnets(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(subnets)).ToNot(Equal(0))
		allSubnets := lo.Flatten(lo.Values(subnets))
		ExpectAssociatePublicIPAddressToBe(false, allSubnets...)

		desc := "a test network interface"
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				LaunchTemplate: v1alpha1.LaunchTemplate{
					NetworkInterfaces: []*v1alpha1.NetworkInterface{
						{
							AssociatePublicIPAddress: aws.Bool(true),
							Description:              aws.String(desc),
							DeviceIndex:              aws.Int64(0),
						},
					},
				},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      "karpenter.k8s.aws/instance-hypervisor",
					Operator: "In",
					Values:   []string{"nitro"},
				},
			},
		})
		pod := test.Pod()
		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).To(HaveLen(1))
		Expect(instance.NetworkInterfaces[0]).ToNot(BeNil())
		Expect(instance.NetworkInterfaces[0].Attachment).To(HaveField("DeviceIndex", HaveValue(Equal(int64(0)))))
		Expect(instance.NetworkInterfaces[0].Attachment).To(HaveField("NetworkCardIndex", HaveValue(Equal(int64(0)))))
		Expect(instance.NetworkInterfaces[0].Ipv4Prefixes).To(HaveLen(2))
		Expect(instance.NetworkInterfaces[0].Ipv6Prefixes).To(HaveLen(2))
		Expect(instance.NetworkInterfaces[0].Description).To(Equal(desc))
		Expect(instance.PublicIpAddress).ToNot(BeNil())
	})
	It("should create a node with more than one NetworkInterface", func() {
		desc1 := "a test network interface"
		desc2 := "another test network interface"
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				LaunchTemplate: v1alpha1.LaunchTemplate{
					NetworkInterfaces: []*v1alpha1.NetworkInterface{
						{
							Description: aws.String(desc1),
							DeviceIndex: aws.Int64(0),
						},
						{
							Description: aws.String(desc2),
							DeviceIndex: aws.Int64(1),
						},
					},
				},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()
		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).To(HaveLen(2))

		Expect(instance.NetworkInterfaces[0]).ToNot(BeNil())
		Expect(instance.NetworkInterfaces[0].Attachment).To(HaveField("DeviceIndex", HaveValue(Equal(int64(0)))))
		Expect(instance.NetworkInterfaces[0].Attachment).To(HaveField("NetworkCardIndex", HaveValue(Equal(int64(0)))))
		Expect(instance.NetworkInterfaces[0].Description).To(Equal(desc1))
		Expect(instance.NetworkInterfaces[0].InterfaceType).To(Equal("interface"))

		Expect(instance.NetworkInterfaces[1]).ToNot(BeNil())
		Expect(instance.NetworkInterfaces[1].Attachment).To(HaveField("DeviceIndex", HaveValue(Equal(int64(1)))))
		Expect(instance.NetworkInterfaces[1].Attachment).To(HaveField("NetworkCardIndex", HaveValue(Equal(int64(1)))))
		Expect(instance.NetworkInterfaces[1].Description).To(Equal(desc2))
		Expect(instance.NetworkInterfaces[1].InterfaceType).To(Equal("efa"))
	})
})

func ExpectAssociatePublicIPAddressToBe(enabled bool, subnetIDs ...string) {
	for subnetID := range subnetIDs {
		_, err := env.EC2API.ModifySubnetAttribute(&ec2.ModifySubnetAttributeInput{
			MapPublicIpOnLaunch: &ec2.AttributeBooleanValue{
				Value: aws.Bool(enabled),
			},
			SubnetId: aws.String(subnetIDs[subnetID]),
		})
		Expect(err).To(BeNil())
	}
}

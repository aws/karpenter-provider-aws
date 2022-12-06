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

package cloudprovider

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
)

var _ = Describe("Subnets", func() {
	It("should default to the cluster's subnets", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod(
			coretest.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: v1alpha5.ArchitectureAmd64}}))[0]
		ExpectScheduled(ctx, env.Client, pod)
		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		input := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(input.LaunchTemplateConfigs).To(HaveLen(1))

		foundNonGPULT := false
		for _, v := range input.LaunchTemplateConfigs {
			for _, ov := range v.Overrides {
				if *ov.InstanceType == "m5.large" {
					foundNonGPULT = true
					Expect(v.Overrides).To(ContainElements(
						&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test1"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1a")},
						&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test2"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1b")},
						&ec2.FleetLaunchTemplateOverridesRequest{SubnetId: aws.String("subnet-test3"), InstanceType: aws.String("m5.large"), AvailabilityZone: aws.String("test-zone-1c")},
					))
				}
			}
		}
		Expect(foundNonGPULT).To(BeTrue())
	})
	It("should launch instances into subnet with the most available IP addresses", func() {
		fakeEC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
			{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(10),
				Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
			{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(100),
				Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
		}})
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1a"}}))[0]
		ExpectScheduled(ctx, env.Client, pod)
		createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
	})
	It("should launch instances into subnets that are excluded by another provisioner", func() {
		fakeEC2API.DescribeSubnetsOutput.Set(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
			{SubnetId: aws.String("test-subnet-1"), AvailabilityZone: aws.String("test-zone-1a"), AvailableIpAddressCount: aws.Int64(10),
				Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-1")}}},
			{SubnetId: aws.String("test-subnet-2"), AvailabilityZone: aws.String("test-zone-1b"), AvailableIpAddressCount: aws.Int64(100),
				Tags: []*ec2.Tag{{Key: aws.String("Name"), Value: aws.String("test-subnet-2")}}},
		}})
		nodeTemplate.Spec.SubnetSelector = map[string]string{"Name": "test-subnet-1"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		podSubnet1 := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, podSubnet1)
		createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-1"))

		provisioner = test.Provisioner(coretest.ProvisionerOptions{Provider: &v1alpha1.AWS{
			SubnetSelector:        map[string]string{"Name": "test-subnet-2"},
			SecurityGroupSelector: map[string]string{"*": "*"},
		}})
		ExpectApplied(ctx, env.Client, provisioner)
		podSubnet2 := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod(coretest.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner.Name}}))[0]
		ExpectScheduled(ctx, env.Client, podSubnet2)
		createFleetInput = fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("test-subnet-2"))
	})
	It("should discover subnet by ID", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf("subnet-test1"))
	})
	It("should discover subnets by IDs", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1,subnet-test2"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf(
			"subnet-test1",
			"subnet-test2",
		))
	})
	It("should discover subnets by IDs and tags", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1,subnet-test2", "foo": "bar"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf(
			"subnet-test1",
			"subnet-test2",
		))
	})
	It("should discover subnets by IDs intersected with tags", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test2", "foo": "bar"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(fake.SubnetsFromFleetRequest(createFleetInput)).To(ConsistOf(
			"subnet-test2",
		))
	})
})

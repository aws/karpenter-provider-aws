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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/mitchellh/hashstructure/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"

	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
)

var _ = Describe("Security Groups", func() {
	It("should default to the clusters security groups", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		ExpectReconcileSucceeded(ctx, nodeTemplateController, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
		input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
		Expect(aws.StringValueSlice(input.LaunchTemplateData.SecurityGroupIds)).To(ConsistOf(
			"sg-test1",
			"sg-test2",
			"sg-test3",
		))
	})
	It("should discover security groups by tag", func() {
		fakeEC2API.DescribeSecurityGroupsOutput.Set(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{
			{GroupId: aws.String("test-sg-1"), Tags: []*ec2.Tag{{Key: aws.String("kubernetes.io/cluster/test-cluster"), Value: aws.String("test-sg-1")}}},
			{GroupId: aws.String("test-sg-2"), Tags: []*ec2.Tag{{Key: aws.String("kubernetes.io/cluster/test-cluster"), Value: aws.String("test-sg-2")}}},
		}})
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		ExpectReconcileSucceeded(ctx, nodeTemplateController, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
		input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
		Expect(aws.StringValueSlice(input.LaunchTemplateData.SecurityGroupIds)).To(ConsistOf(
			"test-sg-1",
			"test-sg-2",
		))
	})
	It("should discover security groups by ID", func() {
		nodeTemplate.Spec.SecurityGroupSelector = map[string]string{"aws-ids": "sg-test1"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		ExpectReconcileSucceeded(ctx, nodeTemplateController, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
		input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
		Expect(aws.StringValueSlice(input.LaunchTemplateData.SecurityGroupIds)).To(ConsistOf(
			"sg-test1",
		))
	})
	It("should discover security groups by IDs", func() {
		nodeTemplate.Spec.SecurityGroupSelector = map[string]string{"aws-ids": "sg-test1,sg-test2"}
		ExpectApplied(ctx, env.Client, test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{
			APIVersion: nodeTemplate.APIVersion,
			Kind:       nodeTemplate.Kind,
			Name:       nodeTemplate.Name,
		}}), nodeTemplate)
		ExpectReconcileSucceeded(ctx, nodeTemplateController, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
		input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
		Expect(aws.StringValueSlice(input.LaunchTemplateData.SecurityGroupIds)).To(ConsistOf(
			"sg-test1",
			"sg-test2",
		))
	})
	It("should discover security groups by IDs and tags", func() {
		nodeTemplate.Spec.SecurityGroupSelector = map[string]string{"aws-ids": "sg-test1,sg-test2", "foo": "bar"}
		ExpectApplied(ctx, env.Client, test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{
			APIVersion: nodeTemplate.APIVersion,
			Kind:       nodeTemplate.Kind,
			Name:       nodeTemplate.Name,
		}}), nodeTemplate)
		ExpectReconcileSucceeded(ctx, nodeTemplateController, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
		input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
		Expect(aws.StringValueSlice(input.LaunchTemplateData.SecurityGroupIds)).To(ConsistOf(
			"sg-test1",
			"sg-test2",
		))
	})
	It("should discover security groups by IDs intersected with tags", func() {
		nodeTemplate.Spec.SecurityGroupSelector = map[string]string{"aws-ids": "sg-test2", "foo": "bar"}
		ExpectApplied(ctx, env.Client, test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{
			APIVersion: nodeTemplate.APIVersion,
			Kind:       nodeTemplate.Kind,
			Name:       nodeTemplate.Name,
		}}), nodeTemplate)
		ExpectReconcileSucceeded(ctx, nodeTemplateController, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
		Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
		input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
		Expect(aws.StringValueSlice(input.LaunchTemplateData.SecurityGroupIds)).To(ConsistOf(
			"sg-test2",
		))
	})
	It("should make sure AWSNodeTemplate Security Groups are grabed from the cache", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		ExpectReconcileSucceeded(ctx, nodeTemplateController, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		filters := utils.GetSecurityGroupFilters(nodeTemplate)
		hash, _ := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
		subnets, ok := securityGroupCache.Get(fmt.Sprint(hash))
		Expect(ok).To(BeTrue())
		Expect(len(subnets.([]*ec2.SecurityGroup))).To(Equal(3))
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod(
			coretest.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: v1alpha5.ArchitectureAmd64}}))[0]
		ExpectScheduled(ctx, env.Client, pod)
	})
})

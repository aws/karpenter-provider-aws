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
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/onsi/gomega/types"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Subnets", func() {
	It("should use the subnet-id selector", func() {
		subnets := env.GetSubnets(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		Expect(len(subnets)).ToNot(Equal(0))
		shuffledAZs := lo.Shuffle(lo.Keys(subnets))
		firstSubnet := subnets[shuffledAZs[0]][0]

		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				ID: firstSubnet,
			},
		}
		pod := test.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SubnetId", HaveValue(Equal(firstSubnet))))
	})
	It("should use resource based naming as node names", func() {
		subnets := env.GetSubnets(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		Expect(len(subnets)).ToNot(Equal(0))

		allSubnets := lo.Flatten(lo.Values(subnets))

		ExpectResourceBasedNamingEnabled(allSubnets...)
		DeferCleanup(func() {
			ExpectResourceBasedNamingDisabled(allSubnets...)
		})
		pod := test.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		ExceptNodeNameToContainInstanceID(pod.Spec.NodeName)
	})
	It("should use the subnet tag selector with multiple tag values", func() {
		// Get all the subnets for the cluster
		subnets := env.GetSubnetNameAndIds(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		Expect(len(subnets)).To(BeNumerically(">", 1))
		firstSubnet := subnets[0]
		lastSubnet := subnets[len(subnets)-1]

		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				Tags: map[string]string{"Name": firstSubnet.Name},
			},
			{
				Tags: map[string]string{"Name": lastSubnet.Name},
			},
		}
		pod := test.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SubnetId", HaveValue(BeElementOf(firstSubnet.ID, lastSubnet.ID))))
	})

	It("should use a subnet within the AZ requested", func() {
		subnets := env.GetSubnets(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		Expect(len(subnets)).ToNot(Equal(0))
		shuffledAZs := lo.Shuffle(lo.Keys(subnets))

		test.ReplaceRequirements(nodePool, corev1beta1.NodeSelectorRequirementWithFlexibility{
			NodeSelectorRequirement: v1.NodeSelectorRequirement{
				Key:      v1.LabelZoneFailureDomainStable,
				Operator: "In",
				Values:   []string{shuffledAZs[0]},
			}})
		pod := test.Pod()

		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SubnetId", Or(
			lo.Map(subnets[shuffledAZs[0]], func(subnetID string, _ int) types.GomegaMatcher { return HaveValue(Equal(subnetID)) })...,
		)))
	})

	It("should have the NodeClass status for subnets", func() {
		env.ExpectCreated(nodeClass)
		EventuallyExpectSubnets(env, nodeClass)
	})
})

func ExpectResourceBasedNamingEnabled(subnetIDs ...string) {
	for subnetID := range subnetIDs {
		_, err := env.EC2API.ModifySubnetAttribute(&ec2.ModifySubnetAttributeInput{
			EnableResourceNameDnsARecordOnLaunch: &ec2.AttributeBooleanValue{
				Value: lo.ToPtr(true),
			},
			SubnetId: lo.ToPtr(subnetIDs[subnetID]),
		})
		Expect(err).To(BeNil())
		_, err = env.EC2API.ModifySubnetAttribute(&ec2.ModifySubnetAttributeInput{
			PrivateDnsHostnameTypeOnLaunch: lo.ToPtr("resource-name"),
			SubnetId:                       lo.ToPtr(subnetIDs[subnetID]),
		})
		Expect(err).To(BeNil())
	}
}

func ExpectResourceBasedNamingDisabled(subnetIDs ...string) {
	for subnetID := range subnetIDs {
		_, err := env.EC2API.ModifySubnetAttribute(&ec2.ModifySubnetAttributeInput{
			EnableResourceNameDnsARecordOnLaunch: &ec2.AttributeBooleanValue{
				Value: lo.ToPtr(false),
			},
			SubnetId: lo.ToPtr(subnetIDs[subnetID]),
		})
		Expect(err).To(BeNil())
		_, err = env.EC2API.ModifySubnetAttribute(&ec2.ModifySubnetAttributeInput{
			PrivateDnsHostnameTypeOnLaunch: lo.ToPtr("ip-name"),
			SubnetId:                       lo.ToPtr(subnetIDs[subnetID]),
		})
		Expect(err).To(BeNil())
	}
}

func ExceptNodeNameToContainInstanceID(nodeName string) {
	instance := env.GetInstance(nodeName)
	Expect(nodeName).To(Not(Equal(lo.FromPtr(instance.InstanceId))))
	ContainSubstring(nodeName, lo.FromPtr(instance.InstanceId))
}

// SubnetInfo is a simple struct for testing
type SubnetInfo struct {
	Name string
	ID   string
}

func EventuallyExpectSubnets(env *aws.Environment, nodeClass *v1beta1.EC2NodeClass) {
	subnets := env.GetSubnets(map[string]string{"karpenter.sh/discovery": env.ClusterName})
	Expect(subnets).ToNot(HaveLen(0))
	ids := sets.New(lo.Flatten(lo.Values(subnets))...)

	Eventually(func(g Gomega) {
		temp := &v1beta1.EC2NodeClass{}
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClass), temp)).To(Succeed())
		g.Expect(sets.New(lo.Map(temp.Status.Subnets, func(s v1beta1.Subnet, _ int) string {
			return s.ID
		})...).Equal(ids))
	}).WithTimeout(10 * time.Second).Should(Succeed())
}

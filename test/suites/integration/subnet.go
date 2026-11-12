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

package integration

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/awslabs/operatorpkg/status"
	"github.com/onsi/gomega/types"
	"github.com/samber/lo"
	"github.com/samber/lo/mutable"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/test"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/awslabs/operatorpkg/test/expectations" //nolint:staticcheck
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func registerSubnet() bool {
	return Describe("Subnets", func() {
		It("should use the subnet-id selector", func() {
			subnets := Env.DiscoveredSubnets()
			Expect(len(subnets)).ToNot(Equal(0))
			shuffledAZs := lo.Keys(subnets)
			mutable.Shuffle(shuffledAZs)
			firstSubnet := subnets[shuffledAZs[0]][0]

			NodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					ID: firstSubnet,
				},
			}
			pod := test.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SubnetId", HaveValue(Equal(firstSubnet))))
		})
		It("should use resource based naming as node names", Label("needs:dedicated-subnets"), func() {
			subnets := Env.DiscoveredSubnets()
			Expect(len(subnets)).ToNot(Equal(0))

			allSubnets := lo.Flatten(lo.Values(subnets))

			ExpectResourceBasedNamingEnabled(allSubnets...)
			DeferCleanup(func() {
				ExpectResourceBasedNamingDisabled(allSubnets...)
			})
			pod := test.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			ExceptNodeNameToContainInstanceID(pod.Spec.NodeName)
		})
		It("should use the subnet tag selector with multiple tag values", func() {
			// Get all the subnets for the cluster
			subnets := Env.DiscoveredSubnetInfo()
			Expect(len(subnets)).To(BeNumerically(">", 1))
			firstSubnet := subnets[0]
			lastSubnet := subnets[len(subnets)-1]

			NodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"Name": firstSubnet.Name},
				},
				{
					Tags: map[string]string{"Name": lastSubnet.Name},
				},
			}
			pod := test.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SubnetId", HaveValue(BeElementOf(firstSubnet.ID, lastSubnet.ID))))
		})

		It("should use a subnet within the AZ requested", func() {
			subnets := Env.DiscoveredSubnets()
			Expect(len(subnets)).ToNot(Equal(0))
			shuffledAZs := lo.Keys(subnets)
			mutable.Shuffle(shuffledAZs)

			test.ReplaceRequirements(NodePool, karpv1.NodeSelectorRequirementWithMinValues{

				Key:      corev1.LabelZoneFailureDomainStable,
				Operator: "In",
				Values:   []string{shuffledAZs[0]},
			})
			pod := test.Pod()

			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)

			Env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SubnetId", Or(
				lo.Map(subnets[shuffledAZs[0]], func(subnetID string, _ int) types.GomegaMatcher { return HaveValue(Equal(subnetID)) })...,
			)))
		})

		It("should have the NodeClass status for subnets", func() {
			Env.ExpectCreated(NodeClass)
			EventuallyExpectSubnets(Env, NodeClass)
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: v1.ConditionTypeSubnetsReady, Status: metav1.ConditionTrue})
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionTrue})
		})
		It("should have the NodeClass status as not ready since subnets were not resolved", func() {
			NodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"karpenter.sh/discovery": "invalidName"},
				},
			}
			Env.ExpectCreated(NodeClass)
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: v1.ConditionTypeSubnetsReady, Status: metav1.ConditionFalse, Message: "SubnetSelector did not match any Subnets"})
			ExpectStatusConditions(Env, Env.Client, 1*time.Minute, NodeClass, status.Condition{Type: status.ConditionReady, Status: metav1.ConditionFalse, Message: "ValidationSucceeded=False, SubnetsReady=False"})
		})
	})
}

func ExpectResourceBasedNamingEnabled(subnetIDs ...string) {
	for subnetID := range subnetIDs {
		_, err := Env.EC2API.ModifySubnetAttribute(Env.Context, &ec2.ModifySubnetAttributeInput{
			EnableResourceNameDnsARecordOnLaunch: &ec2types.AttributeBooleanValue{
				Value: lo.ToPtr(true),
			},
			SubnetId: lo.ToPtr(subnetIDs[subnetID]),
		})
		Expect(err).To(BeNil())
		_, err = Env.EC2API.ModifySubnetAttribute(Env.Context, &ec2.ModifySubnetAttributeInput{
			PrivateDnsHostnameTypeOnLaunch: "resource-name",
			SubnetId:                       lo.ToPtr(subnetIDs[subnetID]),
		})
		Expect(err).To(BeNil())
	}
}

func ExpectResourceBasedNamingDisabled(subnetIDs ...string) {
	for subnetID := range subnetIDs {
		_, err := Env.EC2API.ModifySubnetAttribute(Env.Context, &ec2.ModifySubnetAttributeInput{
			EnableResourceNameDnsARecordOnLaunch: &ec2types.AttributeBooleanValue{
				Value: lo.ToPtr(false),
			},
			SubnetId: lo.ToPtr(subnetIDs[subnetID]),
		})
		Expect(err).To(BeNil())
		_, err = Env.EC2API.ModifySubnetAttribute(Env.Context, &ec2.ModifySubnetAttributeInput{
			PrivateDnsHostnameTypeOnLaunch: "ip-name",
			SubnetId:                       lo.ToPtr(subnetIDs[subnetID]),
		})
		Expect(err).To(BeNil())
	}
}

func ExceptNodeNameToContainInstanceID(nodeName string) {
	instance := Env.GetInstance(nodeName)
	Expect(nodeName).To(Not(Equal(lo.FromPtr(instance.InstanceId))))
	ContainSubstring(nodeName, lo.FromPtr(instance.InstanceId))
}

// SubnetInfo is a simple struct for testing
type SubnetInfo struct {
	Name string
	ID   string
}

func EventuallyExpectSubnets(env *aws.Environment, nodeClass *v1.EC2NodeClass) {
	subnets := env.DiscoveredSubnets()
	Expect(subnets).ToNot(HaveLen(0))
	ids := sets.New(lo.Flatten(lo.Values(subnets))...)

	Eventually(func(g Gomega) {
		temp := &v1.EC2NodeClass{}
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeClass), temp)).To(Succeed())
		g.Expect(sets.New(lo.Map(temp.Status.Subnets, func(s v1.Subnet, _ int) string {
			return s.ID
		})...).Equal(ids))
	}).WithTimeout(10 * time.Second).Should(Succeed())
}

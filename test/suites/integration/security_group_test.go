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
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("SecurityGroups", func() {
	It("should use the security-group-id selector", func() {
		securityGroups := getSecurityGroups(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(securityGroups)).To(BeNumerically(">", 1))

		ids := strings.Join([]string{*securityGroups[0].GroupId, *securityGroups[1].GroupId}, ",")
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"aws-ids": ids},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SecurityGroups", ConsistOf(&securityGroups[0].GroupIdentifier, &securityGroups[1].GroupIdentifier)))
	})

	It("should use the security group selector with multiple tag values", func() {
		securityGroups := getSecurityGroups(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(securityGroups)).To(BeNumerically(">", 1))
		first := securityGroups[0]
		last := securityGroups[len(securityGroups)-1]

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"Name": fmt.Sprintf("%s,%s",
					aws.StringValue(lo.FindOrElse(first.Tags, &ec2.Tag{}, func(tag *ec2.Tag) bool { return aws.StringValue(tag.Key) == "Name" }).Value),
					aws.StringValue(lo.FindOrElse(last.Tags, &ec2.Tag{}, func(tag *ec2.Tag) bool { return aws.StringValue(tag.Key) == "Name" }).Value),
				)},
				SubnetSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SecurityGroups", ConsistOf(&first.GroupIdentifier, &last.GroupIdentifier)))
	})

	It("should update the AWSNodeTemplateStatus for security groups", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			},
		})

		env.ExpectCreated(provider)
		EventuallyExpectSecurityGroups(provider)
	})
})

type SecurityGroup struct {
	ec2.GroupIdentifier
	Tags []*ec2.Tag
}

// getSecurityGroups returns all getSecurityGroups matching the label selector
func getSecurityGroups(tags map[string]string) []SecurityGroup {
	var filters []*ec2.Filter
	for key, val := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(val)},
		})
	}
	var securityGroups []SecurityGroup
	err := env.EC2API.DescribeSecurityGroupsPages(&ec2.DescribeSecurityGroupsInput{Filters: filters}, func(dso *ec2.DescribeSecurityGroupsOutput, _ bool) bool {
		for _, sg := range dso.SecurityGroups {
			securityGroups = append(securityGroups, SecurityGroup{
				Tags:            sg.Tags,
				GroupIdentifier: ec2.GroupIdentifier{GroupId: sg.GroupId, GroupName: sg.GroupName},
			})
		}
		return true
	})
	Expect(err).To(BeNil())
	return securityGroups
}

func EventuallyExpectSecurityGroups(provider *v1alpha1.AWSNodeTemplate) {
	securityGroup := getSecurityGroups(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
	Expect(len(securityGroup)).ToNot(Equal(0))
	var securityGroupID []string

	for _, secGroup := range securityGroup {
		securityGroupID = append(securityGroupID, *secGroup.GroupId)
	}

	Eventually(func(g Gomega) {
		var ant v1alpha1.AWSNodeTemplate
		if err := env.Client.Get(env, client.ObjectKeyFromObject(provider), &ant); err != nil {
			return
		}

		securityGroupsInStatus := lo.Map(ant.Status.SecurityGroups, func(securitygroup v1alpha1.SecurityGroupStatus, _ int) string {
			return securitygroup.ID
		})

		g.Expect(securityGroupsInStatus).To(Equal(securityGroupID))
	}).WithTimeout(10 * time.Second).Should(Succeed())
}

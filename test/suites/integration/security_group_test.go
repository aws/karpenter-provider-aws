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

	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/test/pkg/environment/aws"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("SecurityGroups", func() {
	It("should use the security-group-id selector", func() {
		securityGroups := env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(securityGroups)).To(BeNumerically(">", 1))

		ids := strings.Join([]string{*securityGroups[0].GroupId, *securityGroups[1].GroupId}, ",")
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"aws-ids": ids},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SecurityGroups", ConsistOf(&securityGroups[0].GroupIdentifier, &securityGroups[1].GroupIdentifier)))
	})

	It("should use the security group selector with multiple tag values", func() {
		securityGroups := env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
		Expect(len(securityGroups)).To(BeNumerically(">", 1))
		first := securityGroups[0]
		last := securityGroups[len(securityGroups)-1]

		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"Name": fmt.Sprintf("%s,%s",
					lo.FromPtr(lo.FindOrElse(first.Tags, &ec2.Tag{}, func(tag *ec2.Tag) bool { return lo.FromPtr(tag.Key) == "Name" }).Value),
					lo.FromPtr(lo.FindOrElse(last.Tags, &ec2.Tag{}, func(tag *ec2.Tag) bool { return lo.FromPtr(tag.Key) == "Name" }).Value),
				)},
				SubnetSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SecurityGroups", ConsistOf(&first.GroupIdentifier, &last.GroupIdentifier)))
	})

	It("should update the AWSNodeTemplateStatus for security groups", func() {
		nodeTemplate := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			},
		})

		env.ExpectCreated(nodeTemplate)
		EventuallyExpectSecurityGroups(env, nodeTemplate)
	})
})

func EventuallyExpectSecurityGroups(env *aws.Environment, nodeTemplate *v1alpha1.AWSNodeTemplate) {
	securityGroups := env.GetSecurityGroups(map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName})
	Expect(securityGroups).ToNot(HaveLen(0))

	ids := sets.New(lo.Map(securityGroups, func(s aws.SecurityGroup, _ int) string {
		return lo.FromPtr(s.GroupId)
	})...)
	Eventually(func(g Gomega) {
		temp := &v1alpha1.AWSNodeTemplate{}
		g.Expect(env.Client.Get(env, client.ObjectKeyFromObject(nodeTemplate), temp)).To(Succeed())
		g.Expect(sets.New(lo.Map(temp.Status.SecurityGroups, func(s v1alpha1.SecurityGroup, _ int) string {
			return s.ID
		})...).Equal(ids))
	}).WithTimeout(10 * time.Second).Should(Succeed())
}

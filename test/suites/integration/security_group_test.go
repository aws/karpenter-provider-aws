package integration_test

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
)

var _ = Describe("Subnets", func() {
	BeforeEach(func() {

	})

	It("should use the security-group-id selector", func() {
		securityGroups := getSecurityGroups(map[string]string{"karpenter.sh/discovery": env.ClusterName})
		Expect(len(securityGroups)).ToNot(Equal(0))

		ids := strings.Join(lo.Map(securityGroups, func(sg ec2.GroupIdentifier, _ int) string  {return *sg.GroupId}), ",")
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: awsv1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"aws-ids": ids},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()

		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		env.ExpectInstance(pod.Spec.NodeName).To(HaveField("SecurityGroups", ContainElement(&securityGroups[0])))
	})
})

// getSecurityGroups returns all getSecurityGroups matching the label selector
func getSecurityGroups(tags map[string]string) []ec2.GroupIdentifier {
	var filters []*ec2.Filter
	for key, val := range tags {
		filters = append(filters, &ec2.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []*string{aws.String(val)},
		})
	}
	var securityGroups []ec2.GroupIdentifier
	err := env.EC2API.DescribeSecurityGroupsPages(&ec2.DescribeSecurityGroupsInput{Filters: filters}, func(dso *ec2.DescribeSecurityGroupsOutput, _ bool) bool {
		for _, sg := range dso.SecurityGroups {
			securityGroups = append(securityGroups, ec2.GroupIdentifier{GroupId: sg.GroupId, GroupName: sg.GroupName})
		}
		return true
	})
	Expect(err).To(BeNil())
	return securityGroups
}

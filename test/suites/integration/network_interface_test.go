package integration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
)

var _ = Describe("NetworkInterfaces", func() {

	DescribeTable(
		"should correctly configure public IP assignment on instances",
		func(privateSubnet bool, assignPublicIp *bool) {
			switch privateSubnet {
			case true:
				nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
					{
						Tags: map[string]string{
							"Name":                   "*Private*",
							"karpenter.sh/discovery": env.ClusterName,
						},
					},
				}
			case false:
				nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
					{
						Tags: map[string]string{
							"Name":                   "*Public*",
							"karpenter.sh/discovery": env.ClusterName,
						},
					},
				}
			}
			nodeClass.Spec.AssignPublicIpAddress = assignPublicIp

			pod := test.Pod()
			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			instance := env.GetInstance(pod.Spec.NodeName)

			if assignPublicIp == nil {
				if privateSubnet == true {
					Expect(instance.PublicIpAddress).To(BeNil())
				}
				if privateSubnet == false {
					Expect(instance.PublicIpAddress).ToNot(BeNil())
				}
			}
			if *assignPublicIp == false {
				Expect(instance.PublicIpAddress).To(BeNil())
			}
			if *assignPublicIp == true {
				Expect(instance.PublicIpAddress).ToNot(BeNil())
			}
		},
		Entry("AssignPublicIpAddress not specified by the user while using a private subnet", true, nil),
		Entry("AssignPublicIpAddress not specified by the user while using a public subnet", false, nil),
		Entry("AssignPublicIpAddress set true by user while using a private subnet", true, lo.ToPtr(true)),
		Entry("AssignPublicIpAddress not false by user while using a private subnet", true, lo.ToPtr(false)),
	)

})

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
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("NetworkInterfaces", func() {

	DescribeTable(
		"should correctly configure public IP assignment on instances",
		func(privateSubnet bool, assignPublicIp *bool) {
			nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{{
				Tags: map[string]string{
					"Name":                   lo.Ternary(privateSubnet, "*Private*", "*Public*"),
					"karpenter.sh/discovery": env.ClusterName,
				},
			}}
			nodeClass.Spec.AssociatePublicIPAddress = assignPublicIp

			pod := test.Pod()
			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			instance := env.GetInstance(pod.Spec.NodeName)

			if assignPublicIp == nil {
				if privateSubnet {
					Expect(instance.PublicIpAddress).To(BeNil())
				} else {
					Expect(instance.PublicIpAddress).ToNot(BeNil())
				}
			} else if *assignPublicIp == false {
				Expect(instance.PublicIpAddress).To(BeNil())
			} else if *assignPublicIp == true {
				Expect(instance.PublicIpAddress).ToNot(BeNil())
			}
		},
		Entry("AssociatePublicIPAddress not specified by the user while using a private subnet", true, nil),
		Entry("AssociatePublicIPAddress set true by user while using a private subnet", true, lo.ToPtr(true)),
		Entry("AssociatePublicIPAddress not false by user while using a private subnet", true, lo.ToPtr(false)),

		Entry("AssociatePublicIPAddress not specified by the user while using a public subnet", false, nil),
		Entry("AssociatePublicIPAddress set true by user while using a public subnet", false, lo.ToPtr(true)),
		Entry("AssociatePublicIPAddress not false by user while using a public subnet", false, lo.ToPtr(false)),
	)

})

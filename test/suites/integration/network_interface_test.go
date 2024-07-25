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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
)

var _ = Describe("NetworkInterfaces", func() {
	DescribeTable(
		"should correctly configure public IP assignment on instances",
		func(associatePublicIPAddress *bool) {
			nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{
				Tags: map[string]string{
					"Name":                   "*Private*",
					"karpenter.sh/discovery": env.ClusterName,
				},
			}}
			nodeClass.Spec.AssociatePublicIPAddress = associatePublicIPAddress

			pod := test.Pod()
			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			instance := env.GetInstance(pod.Spec.NodeName)

			if lo.FromPtr(associatePublicIPAddress) {
				Expect(instance.PublicIpAddress).ToNot(BeNil())
			} else {
				Expect(instance.PublicIpAddress).To(BeNil())
			}
		},
		// Only tests private subnets since nodes w/o a public IP address in a public subnet will be unable to join the cluster
		Entry("AssociatePublicIPAddress not specified by the user while using a private subnet", nil),
		Entry("AssociatePublicIPAddress set true by user while using a private subnet", lo.ToPtr(true)),
	)
})

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
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/samber/lo"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ENAExpress", func() {
	DescribeTable("should apply ENA Express settings to the primary network interface",
		func(enaExpress *v1.ENAExpress) {
			nodeClass.Spec.ENAExpress = enaExpress

			pod := test.Pod()
			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)

			instance := env.GetInstance(pod.Spec.NodeName)
			primaryNI, found := environmentaws.FindNetworkInterface(instance.NetworkInterfaces, 0, 0)
			Expect(found).To(BeTrue())
			Expect(primaryNI.Attachment.EnaSrdSpecification).ToNot(BeNil())
			Expect(aws.ToBool(primaryNI.Attachment.EnaSrdSpecification.EnaSrdEnabled)).To(BeTrue())
			udpSpec := lo.FromPtr(primaryNI.Attachment.EnaSrdSpecification.EnaSrdUdpSpecification)
			Expect(aws.ToBool(udpSpec.EnaSrdUdpEnabled)).To(Equal(lo.FromPtr(enaExpress.UDPEnabled)))
		},
		Entry("TCP only", &v1.ENAExpress{Enabled: true}),
		Entry("TCP and UDP", &v1.ENAExpress{Enabled: true, UDPEnabled: lo.ToPtr(true)}),
	)
})

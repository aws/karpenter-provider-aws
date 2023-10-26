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
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
)

var _ = Describe("NetworkInterfaces", func() {
	BeforeEach(func() {
		// Ensure that nodes schedule to private subnets. If a node without a public IP is assigned to a public subnet,
		// and that subnet does not contain any private endpoints to the cluster, the node will be unable to join the
		// cluster.
		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				Tags: map[string]string{
					"Name":                   "*Private*",
					"karpenter.sh/discovery": env.ClusterName,
				},
			},
		}
	})

	DescribeTable(
		"should correctly create NetworkInterfaces",
		func(interfaces ...*v1beta1.NetworkInterface) {
			nodeClass.Spec.NetworkInterfaces = interfaces
			pod := test.Pod()
			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			instance := env.GetInstance(pod.Spec.NodeName)
			for _, interfaceSpec := range interfaces {
				ni, ok := lo.Find(instance.NetworkInterfaces, func(ni *ec2.InstanceNetworkInterface) bool {
					if ni.Description == nil {
						return false
					}
					return *ni.Description == *interfaceSpec.Description
				})
				Expect(ok).To(BeTrue())
				Expect(ni.Attachment).To(HaveField("DeviceIndex", HaveValue(Equal(*interfaceSpec.DeviceIndex))))
			}

			if len(interfaces) == 1 && interfaces[0].AssociatePublicIPAddress != nil {
				if *interfaces[0].AssociatePublicIPAddress {
					Expect(instance.PublicIpAddress).ToNot(BeNil())
				} else {
					Expect(instance.PublicIpAddress).To(BeNil())
				}
			}
		},
		Entry("when a single interface is specified", &v1beta1.NetworkInterface{
			Description: lo.ToPtr("a test interface"),
			DeviceIndex: lo.ToPtr(int64(0)),
		}),
		Entry("when a single interface is specified with AssociatePublicIPAddress = true", &v1beta1.NetworkInterface{
			AssociatePublicIPAddress: lo.ToPtr(true),
			Description:              lo.ToPtr("a test interface"),
			DeviceIndex:              lo.ToPtr(int64(0)),
		}),
		Entry("when a single interface is specified with AssociatePublicIPAddress = false", &v1beta1.NetworkInterface{
			AssociatePublicIPAddress: lo.ToPtr(false),
			Description:              lo.ToPtr("a test interface"),
			DeviceIndex:              lo.ToPtr(int64(0)),
		}),
		Entry(
			"when multiple interfaces are specified",
			&v1beta1.NetworkInterface{
				Description: lo.ToPtr("a test interface"),
				DeviceIndex: lo.ToPtr(int64(0)),
			},
			&v1beta1.NetworkInterface{
				Description: lo.ToPtr("another test interface"),
				DeviceIndex: lo.ToPtr(int64(1)),
			},
		),
	)
})

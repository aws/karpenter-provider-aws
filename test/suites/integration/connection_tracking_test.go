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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnectionTracking", func() {
	It("should apply connection tracking settings to network interfaces", func() {
		nodeClass.Spec.ConnectionTracking = &v1.ConnectionTracking{
			TCPEstablishedTimeout: &metav1.Duration{Duration: 300 * time.Second},
			UDPStreamTimeout:      &metav1.Duration{Duration: 120 * time.Second},
			UDPTimeout:            &metav1.Duration{Duration: 45 * time.Second},
		}

		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).ToNot(BeEmpty())

		// Verify connection tracking settings on the instance's network interface
		primaryNI := instance.NetworkInterfaces[0]
		Expect(primaryNI.Attachment).ToNot(BeNil())
		Expect(aws.ToInt32(primaryNI.Attachment.DeviceIndex)).To(Equal(int32(0)))

		// Verify connection tracking via DescribeNetworkInterfaces
		niOutput, err := env.EC2API.DescribeNetworkInterfaces(env.Context, &ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []string{aws.ToString(primaryNI.NetworkInterfaceId)},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(niOutput.NetworkInterfaces).To(HaveLen(1))

		ni := niOutput.NetworkInterfaces[0]
		Expect(ni.ConnectionTrackingConfiguration).ToNot(BeNil())
		Expect(aws.ToInt32(ni.ConnectionTrackingConfiguration.TcpEstablishedTimeout)).To(Equal(int32(300)))
		Expect(aws.ToInt32(ni.ConnectionTrackingConfiguration.UdpStreamTimeout)).To(Equal(int32(120)))
		Expect(aws.ToInt32(ni.ConnectionTrackingConfiguration.UdpTimeout)).To(Equal(int32(45)))
	})

	It("should allow partial connection tracking configuration", func() {
		nodeClass.Spec.ConnectionTracking = &v1.ConnectionTracking{
			TCPEstablishedTimeout: &metav1.Duration{Duration: 600 * time.Second},
		}

		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).ToNot(BeEmpty())

		primaryNI := instance.NetworkInterfaces[0]

		// Verify connection tracking via DescribeNetworkInterfaces
		niOutput, err := env.EC2API.DescribeNetworkInterfaces(env.Context, &ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []string{aws.ToString(primaryNI.NetworkInterfaceId)},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(niOutput.NetworkInterfaces).To(HaveLen(1))

		ni := niOutput.NetworkInterfaces[0]
		Expect(ni.ConnectionTrackingConfiguration).ToNot(BeNil())
		Expect(aws.ToInt32(ni.ConnectionTrackingConfiguration.TcpEstablishedTimeout)).To(Equal(int32(600)))
		// When only some fields are set, others should be nil or default
		Expect(ni.ConnectionTrackingConfiguration.UdpStreamTimeout).To(BeNil())
		Expect(ni.ConnectionTrackingConfiguration.UdpTimeout).To(BeNil())
	})

	It("should not set connection tracking when not specified", func() {
		// nodeClass.Spec.ConnectionTracking is nil by default

		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).ToNot(BeEmpty())

		primaryNI := instance.NetworkInterfaces[0]

		// Verify connection tracking is not set via DescribeNetworkInterfaces
		niOutput, err := env.EC2API.DescribeNetworkInterfaces(env.Context, &ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []string{aws.ToString(primaryNI.NetworkInterfaceId)},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(niOutput.NetworkInterfaces).To(HaveLen(1))

		ni := niOutput.NetworkInterfaces[0]
		// Connection tracking configuration should be nil when not specified
		Expect(ni.ConnectionTrackingConfiguration).To(BeNil())
	})
})

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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnectionTracking", func() {
	It("should apply connection tracking settings to network interfaces", func() {
		nodeClass.Spec.ConnectionTracking = &v1.ConnectionTracking{
			TCPEstablishedTimeout: lo.ToPtr(int32(300)),
			UDPStreamTimeout:      lo.ToPtr(int32(120)),
			UDPTimeout:            lo.ToPtr(int32(45)),
		}

		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).ToNot(BeEmpty())

		primaryNI := instance.NetworkInterfaces[0]
		Expect(primaryNI.Attachment).ToNot(BeNil())
		Expect(aws.ToInt32(primaryNI.Attachment.DeviceIndex)).To(Equal(int32(0)))

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

	// This test uses the legacy EFACount path (via vpc.amazonaws.com/efa resource requests),
	// which creates interfaces with InterfaceType "efa" (not "efa-only"). All "efa" interfaces
	// receive connection tracking settings. The efa-only exclusion only applies to explicitly
	// configured networkInterfaces with interfaceType "efa-only".
	It("should apply connection tracking settings to all EFA network interfaces", func() {
		env.ExpectEFADevicePluginCreated()

		nodeClass.Spec.ConnectionTracking = &v1.ConnectionTracking{
			TCPEstablishedTimeout: lo.ToPtr(int32(300)),
			UDPStreamTimeout:      lo.ToPtr(int32(120)),
			UDPTimeout:            lo.ToPtr(int32(45)),
		}
		// Instances launched with multiple network interfaces cannot get a public IP.
		nodeClass.Spec.SubnetSelectorTerms[0].Tags["Name"] = "*Private*"

		nodePool.Spec.Template.Labels = map[string]string{"aws.amazon.com/efa": "true"}
		nodePool.Spec.Template.Spec.Taints = []corev1.Taint{
			{Key: "aws.amazon.com/efa", Effect: corev1.TaintEffectNoSchedule},
		}

		dep := test.Deployment(test.DeploymentOptions{
			Replicas: 1,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "conntrack-efa"}},
				Tolerations: []corev1.Toleration{
					{Key: "aws.amazon.com/efa", Operator: corev1.TolerationOpExists},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{"vpc.amazonaws.com/efa": resource.MustParse("1")},
					Limits:   corev1.ResourceList{"vpc.amazonaws.com/efa": resource.MustParse("1")},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, dep)
		pods := env.EventuallyExpectHealthyPodCount(selector, 1)
		env.ExpectCreatedNodeCount("==", 1)

		instance := env.GetInstance(pods[0].Spec.NodeName)
		Expect(instance.NetworkInterfaces).ToNot(BeEmpty())

		// Only check ENIs that were created by Karpenter (launched with the instance via launch template).
		// Filter to ENIs with Attachment.DeviceIndex specified at launch (DeviceIndex >= 0 and Status "attached").
		// Exclude ENIs added post-launch by the VPC resource controller (e.g., trunk/branch interfaces).
		karpenterNIIDs := lo.FilterMap(instance.NetworkInterfaces, func(ni ec2types.InstanceNetworkInterface, _ int) (string, bool) {
			if ni.Attachment == nil {
				return "", false
			}
			// ENIs at device index 0 (primary) and those specified in the launch template are created at launch
			return aws.ToString(ni.NetworkInterfaceId), aws.ToInt32(ni.Attachment.DeviceIndex) == 0 || aws.ToString(ni.InterfaceType) == "efa"
		})
		Expect(karpenterNIIDs).ToNot(BeEmpty())

		niOutput, err := env.EC2API.DescribeNetworkInterfaces(env.Context, &ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: karpenterNIIDs,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(niOutput.NetworkInterfaces).ToNot(BeEmpty())
		for _, ni := range niOutput.NetworkInterfaces {
			Expect(ni.ConnectionTrackingConfiguration).ToNot(BeNil(), "ENI %s missing connection tracking config", aws.ToString(ni.NetworkInterfaceId))
			Expect(aws.ToInt32(ni.ConnectionTrackingConfiguration.TcpEstablishedTimeout)).To(Equal(int32(300)))
			Expect(aws.ToInt32(ni.ConnectionTrackingConfiguration.UdpStreamTimeout)).To(Equal(int32(120)))
			Expect(aws.ToInt32(ni.ConnectionTrackingConfiguration.UdpTimeout)).To(Equal(int32(45)))
		}
	})

	It("should apply partial connection tracking settings", func() {
		nodeClass.Spec.ConnectionTracking = &v1.ConnectionTracking{
			TCPEstablishedTimeout: lo.ToPtr(int32(86400)),
		}

		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).ToNot(BeEmpty())

		primaryNI := instance.NetworkInterfaces[0]
		niOutput, err := env.EC2API.DescribeNetworkInterfaces(env.Context, &ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []string{aws.ToString(primaryNI.NetworkInterfaceId)},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(niOutput.NetworkInterfaces).To(HaveLen(1))

		ni := niOutput.NetworkInterfaces[0]
		Expect(ni.ConnectionTrackingConfiguration).ToNot(BeNil())
		Expect(aws.ToInt32(ni.ConnectionTrackingConfiguration.TcpEstablishedTimeout)).To(Equal(int32(86400)))
	})

	It("should not set connection tracking when not specified", func() {
		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		instance := env.GetInstance(pod.Spec.NodeName)
		Expect(instance.NetworkInterfaces).ToNot(BeEmpty())

		primaryNI := instance.NetworkInterfaces[0]
		niOutput, err := env.EC2API.DescribeNetworkInterfaces(env.Context, &ec2.DescribeNetworkInterfacesInput{
			NetworkInterfaceIds: []string{aws.ToString(primaryNI.NetworkInterfaceId)},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(niOutput.NetworkInterfaces).To(HaveLen(1))
		// EC2 returns nil for ConnectionTrackingConfiguration when no custom values were set in the launch template.
		Expect(niOutput.NetworkInterfaces[0].ConnectionTrackingConfiguration).To(BeNil())
	})
})

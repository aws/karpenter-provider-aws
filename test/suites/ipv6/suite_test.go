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

package ipv6_test

import (
	"net"
	"testing"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *aws.Environment
var nodeClass *v1.EC2NodeClass
var nodePool *karpv1.NodePool

func TestIPv6(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = aws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "IPv6")
}

var _ = BeforeEach(func() {
	env.BeforeEach()
	nodeClass = env.DefaultEC2NodeClass()
	nodePool = env.DefaultNodePool(nodeClass)
	nodePool = coretest.ReplaceRequirements(nodePool,
		karpv1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      v1.LabelInstanceCategory,
				Operator: corev1.NodeSelectorOpExists,
			},
		},
		karpv1.NodeSelectorRequirementWithMinValues{
			NodeSelectorRequirement: corev1.NodeSelectorRequirement{
				Key:      corev1.LabelInstanceTypeStable,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"c5.large"},
			},
		},
	)
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("IPv6", func() {
	It("should provision an IPv6 node by discovering kube-dns IPv6", func() {
		pod := coretest.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		node := env.GetNode(pod.Spec.NodeName)
		internalIPv6Addrs := lo.Filter(node.Status.Addresses, func(addr corev1.NodeAddress, _ int) bool {
			return addr.Type == corev1.NodeInternalIP && net.ParseIP(addr.Address).To4() == nil
		})
		Expect(internalIPv6Addrs).To(HaveLen(1))
	})
	It("should provision an IPv6 node by discovering kubeletConfig kube-dns IP", func() {
		clusterDNSAddr := env.ExpectIPv6ClusterDNS()
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{ClusterDNS: []string{clusterDNSAddr}}
		pod := coretest.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		node := env.GetNode(pod.Spec.NodeName)
		internalIPv6Addrs := lo.Filter(node.Status.Addresses, func(addr corev1.NodeAddress, _ int) bool {
			return addr.Type == corev1.NodeInternalIP && net.ParseIP(addr.Address).To4() == nil
		})
		Expect(internalIPv6Addrs).To(HaveLen(1))
	})
	It("should provision a static IPv6 prefix with node launch and set IPv6 as primary in the primary network interface", func() {
		clusterDNSAddr := env.ExpectIPv6ClusterDNS()
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{ClusterDNS: []string{clusterDNSAddr}}
		pod := coretest.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		node := env.GetNode(pod.Spec.NodeName)
		instance := env.GetInstanceByID(env.ExpectParsedProviderID(node.Spec.ProviderID))
		Expect(instance.NetworkInterfaces).To(HaveLen(1))
		Expect(instance.NetworkInterfaces[0].Ipv6Addresses).To(HaveLen(1))
		_, hasIPv6Primary := lo.Find(instance.NetworkInterfaces[0].Ipv6Addresses, func(ip types.InstanceIpv6Address) bool {
			return lo.FromPtr(ip.IsPrimaryIpv6)
		})
		Expect(hasIPv6Primary).To(BeTrue())
	})
})

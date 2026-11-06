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

package ipv6

import (
	"net"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var Env *aws.Environment
var NodeClass *v1.EC2NodeClass
var NodePool *karpv1.NodePool

func RegisterTests() bool {
	Describe("IPv6", func() {
		BeforeEach(func() {
			Env.RequireIPv6Cluster()
		})
		It("should provision an IPv6 node by discovering kube-dns IPv6", func() {
			pod := coretest.Pod()
			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)
			node := Env.GetNode(pod.Spec.NodeName)
			internalIPv6Addrs := lo.Filter(node.Status.Addresses, func(addr corev1.NodeAddress, _ int) bool {
				return addr.Type == corev1.NodeInternalIP && net.ParseIP(addr.Address).To4() == nil
			})
			Expect(internalIPv6Addrs).To(HaveLen(1))
		})
		It("should provision an IPv6 node by discovering kubeletConfig kube-dns IP", func() {
			clusterDNSAddr := Env.ExpectIPv6ClusterDNS()
			NodeClass.Spec.Kubelet = test.MustMakeKubeletConfiguration(map[string]interface{}{"clusterDNS": []string{clusterDNSAddr}})
			pod := coretest.Pod()
			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)
			node := Env.GetNode(pod.Spec.NodeName)
			internalIPv6Addrs := lo.Filter(node.Status.Addresses, func(addr corev1.NodeAddress, _ int) bool {
				return addr.Type == corev1.NodeInternalIP && net.ParseIP(addr.Address).To4() == nil
			})
			Expect(internalIPv6Addrs).To(HaveLen(1))
		})
		It("should provision a static IPv6 prefix with node launch and set IPv6 as primary in the primary network interface", func() {
			clusterDNSAddr := Env.ExpectIPv6ClusterDNS()
			NodeClass.Spec.Kubelet = test.MustMakeKubeletConfiguration(map[string]interface{}{"clusterDNS": []string{clusterDNSAddr}})
			pod := coretest.Pod()
			Env.ExpectCreated(pod, NodeClass, NodePool)
			Env.EventuallyExpectHealthy(pod)
			Env.ExpectCreatedNodeCount("==", 1)
			node := Env.GetNode(pod.Spec.NodeName)
			instance := Env.GetInstanceByID(Env.ExpectParsedProviderID(node.Spec.ProviderID))
			Expect(instance.NetworkInterfaces).ToNot(BeEmpty())
			primaryNI, found := aws.FindNetworkInterface(instance.NetworkInterfaces, 0, 0)
			Expect(found).To(BeTrue())
			Expect(primaryNI.Ipv6Addresses).To(HaveLen(1))
			_, hasIPv6Primary := lo.Find(primaryNI.Ipv6Addresses, func(ip types.InstanceIpv6Address) bool {
				return lo.FromPtr(ip.IsPrimaryIpv6)
			})
			Expect(hasIPv6Primary).To(BeTrue())
		})
	})
	return true
}

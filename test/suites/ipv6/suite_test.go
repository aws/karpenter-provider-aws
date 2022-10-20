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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudproviders/aws/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

var env *aws.Environment

func TestIPv6(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		var err error
		env, err = aws.NewEnvironment(t)
		Expect(err).ToNot(HaveOccurred())
	})
	RunSpecs(t, "IPv6")
}

var _ = BeforeEach(func() { env.BeforeEach() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("IPv6", func() {
	It("should provision an IPv6 node by discovering kube-dns IPv6", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AMISelector: map[string]string{"aws-ids": "ami-0f17a47520a746bec"},
			AWS: awsv1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}, Requirements: []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelInstanceTypeStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"t3a.small"},
			},
			{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"on-demand"},
			},
		}})

		pod := test.Pod()
		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		node := env.GetNode(pod.Spec.NodeName)
		internalIPv6Addrs := lo.Filter(node.Status.Addresses, func(addr v1.NodeAddress, _ int) bool {
			return addr.Type == v1.NodeInternalIP && net.ParseIP(addr.Address).To4() == nil
		})
		Expect(internalIPv6Addrs).To(HaveLen(1))
	})
	It("should provision an IPv6 node by discovering kubeletConfig kube-dns IP", func() {
		clusterDNSAddr := env.ExpectIPv6ClusterDNS()
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AMISelector: map[string]string{"aws-ids": "ami-0f17a47520a746bec"},
			AWS: awsv1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			}})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}, Requirements: []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelInstanceTypeStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"t3a.small"},
			},
			{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"on-demand"},
			},
		}, Kubelet: &v1alpha5.KubeletConfiguration{ClusterDNS: []string{clusterDNSAddr}}})

		pod := test.Pod()
		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		node := env.GetNode(pod.Spec.NodeName)
		internalIPv6Addrs := lo.Filter(node.Status.Addresses, func(addr v1.NodeAddress, _ int) bool {
			return addr.Type == v1.NodeInternalIP && net.ParseIP(addr.Address).To4() == nil
		})
		Expect(internalIPv6Addrs).To(HaveLen(1))
	})
})

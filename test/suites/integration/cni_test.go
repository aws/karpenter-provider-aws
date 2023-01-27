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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"

	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("CNITests", func() {
	It("should set max pods to 110 when AWSENILimited when AWS_ENI_LIMITED_POD_DENSITY is false", func() {
		env.ExpectSettingsOverridden(map[string]string{
			"aws.enableENILimitedPodDensity": "false",
		})
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyAL2,
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()
		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		var node corev1.Node
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: pod.Spec.NodeName}, &node)).To(Succeed())
		allocatablePods, _ := node.Status.Allocatable.Pods().AsInt64()
		Expect(allocatablePods).To(Equal(int64(110)))
	})
	It("should set eni-limited maxPods when AWSENILimited when AWS_ENI_LIMITED_POD_DENSITY is true", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				AMIFamily:             &v1alpha1.AMIFamilyAL2,
			},
		})
		provisioner := test.Provisioner(test.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name}})
		pod := test.Pod()
		env.ExpectCreated(pod, provider, provisioner)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		var node corev1.Node
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: pod.Spec.NodeName}, &node)).To(Succeed())
		allocatablePods, _ := node.Status.Allocatable.Pods().AsInt64()
		Expect(allocatablePods).To(Equal(eniLimitedPodsFor(node.Labels["node.kubernetes.io/instance-type"])))
	})
})

func eniLimitedPodsFor(instanceType string) int64 {
	instance, err := env.EC2API.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{
		InstanceTypes: aws.StringSlice([]string{instanceType}),
	})
	Expect(err).ToNot(HaveOccurred())
	networkInfo := *instance.InstanceTypes[0].NetworkInfo
	return *networkInfo.MaximumNetworkInterfaces*(*networkInfo.Ipv4AddressesPerInterface-1) + 2
}

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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudproviders/aws/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("CNITests", func() {
	It("should set max pods to 110 when AWSENILimited when AWS_ENI_LIMITED_POD_DENSITY is false", func() {
		updateEnvironment(corev1.EnvVar{Name: "AWS_ENI_LIMITED_POD_DENSITY", Value: "false"})
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: awsv1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				AMIFamily:             &awsv1alpha1.AMIFamilyAL2,
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
		// At the end of the test, set this to true since an unset/true value is the default and what all other integration tests
		// should rely on.
		updateEnvironment(corev1.EnvVar{Name: "AWS_ENI_LIMITED_POD_DENSITY", Value: "true"})
	})
	It("should set eni-limited maxPods when AWSENILimited when AWS_ENI_LIMITED_POD_DENSITY is true", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: awsv1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
				AMIFamily:             &awsv1alpha1.AMIFamilyAL2,
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

// updateEnvironment sets the provided envVar to all of Karpenter's containers.
// It makes the assumption that Karpenter is deployed as `karpenter/karpenter`. All previously applied
// environment variables are preserved, and this function will return once all Karpenter pods are recycled.
func updateEnvironment(envVar corev1.EnvVar) {
	karpenterDeployment, err := env.KubeClient.AppsV1().Deployments("karpenter").Get(env.Context, "karpenter", v1.GetOptions{})
	Expect(err).ToNot(HaveOccurred())
	originalDeployment := karpenterDeployment.DeepCopy()
	karpenterPodContainers := karpenterDeployment.Spec.Template.Spec.Containers
	for idx := range karpenterPodContainers {
		var updatedVariables = []corev1.EnvVar{envVar}
		for _, currentVariable := range karpenterPodContainers[idx].Env {
			if currentVariable.Name != envVar.Name {
				updatedVariables = append(updatedVariables, currentVariable)
			}
		}
		karpenterPodContainers[idx].Env = updatedVariables
	}
	Expect(env.Client.Patch(env.Context, karpenterDeployment, client.MergeFrom(originalDeployment))).To(Succeed())
	env.EventuallyExpectKarpenterWithEnvVar(envVar)
}

func eniLimitedPodsFor(instanceType string) int64 {
	instance, err := env.EC2API.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{
		InstanceTypes: aws.StringSlice([]string{instanceType}),
	})
	Expect(err).ToNot(HaveOccurred())
	networkInfo := *instance.InstanceTypes[0].NetworkInfo
	return *networkInfo.MaximumNetworkInterfaces*(*networkInfo.Ipv4AddressesPerInterface-1) + 2
}

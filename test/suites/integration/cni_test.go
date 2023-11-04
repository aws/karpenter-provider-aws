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
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/karpenter-core/pkg/test"
)

var _ = Describe("CNITests", func() {
	It("should set max pods to 110 when AWSENILimited when AWS_ENI_LIMITED_POD_DENSITY is false", func() {
		env.ExpectSettingsOverriddenLegacy(map[string]string{"aws.enableENILimitedPodDensity": "false"})
		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		var node corev1.Node
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: pod.Spec.NodeName}, &node)).To(Succeed())
		allocatablePods, _ := node.Status.Allocatable.Pods().AsInt64()
		Expect(allocatablePods).To(Equal(int64(110)))
	})
	It("should set eni-limited maxPods when AWSENILimited when AWS_ENI_LIMITED_POD_DENSITY is true", func() {
		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		var node corev1.Node
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: pod.Spec.NodeName}, &node)).To(Succeed())
		allocatablePods, _ := node.Status.Allocatable.Pods().AsInt64()
		Expect(allocatablePods).To(Equal(eniLimitedPodsFor(node.Labels["node.kubernetes.io/instance-type"])))
	})
	It("should set maxPods when reservedENIs is set", func() {
		env.ExpectSettingsOverridden(corev1.EnvVar{Name: "RESERVED_ENIS", Value: "1"})
		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		var node corev1.Node
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: pod.Spec.NodeName}, &node)).To(Succeed())
		allocatablePods, _ := node.Status.Allocatable.Pods().AsInt64()
		Expect(allocatablePods).To(Equal(reservedENIsFor(node.Labels["node.kubernetes.io/instance-type"])))
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

func reservedENIsFor(instanceType string) int64 {
	instance, err := env.EC2API.DescribeInstanceTypes(&ec2.DescribeInstanceTypesInput{
		InstanceTypes: aws.StringSlice([]string{instanceType}),
	})
	Expect(err).ToNot(HaveOccurred())
	networkInfo := *instance.InstanceTypes[0].NetworkInfo
	reservedENIs := 0
	reservedENIsVar, ok := lo.Find(env.ExpectSettings(), func(v corev1.EnvVar) bool { return v.Name == "RESERVED_ENIS" })
	if ok {
		reservedENIs, err = strconv.Atoi(reservedENIsVar.Value)
		Expect(err).ToNot(HaveOccurred())
	}
	return (*networkInfo.MaximumNetworkInterfaces-int64(reservedENIs))*(*networkInfo.Ipv4AddressesPerInterface-1) + 2
}

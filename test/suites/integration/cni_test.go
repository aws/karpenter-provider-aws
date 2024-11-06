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

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("CNITests", func() {
	It("should set eni-limited maxPods", func() {
		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
		var node corev1.Node
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: pod.Spec.NodeName}, &node)).To(Succeed())
		allocatablePods, _ := node.Status.Allocatable.Pods().AsInt64()
		Expect(allocatablePods).To(Equal(eniLimitedPodsFor(node.Labels["node.kubernetes.io/instance-type"])))
	})
	It("should set max pods to 110 if maxPods is set in kubelet", func() {
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: lo.ToPtr[int32](110)}
		pod := test.Pod()
		env.ExpectCreated(pod, nodeClass, nodePool)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)

		var node corev1.Node
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: pod.Spec.NodeName}, &node)).To(Succeed())
		allocatablePods, _ := node.Status.Allocatable.Pods().AsInt64()
		Expect(allocatablePods).To(Equal(int64(110)))
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
	instance, err := env.EC2API.DescribeInstanceTypes(env.Context, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{ec2types.InstanceType(instanceType)},
	})
	Expect(err).ToNot(HaveOccurred())
	networkInfo := *instance.InstanceTypes[0].NetworkInfo
	return int64(*networkInfo.MaximumNetworkInterfaces*(*networkInfo.Ipv4AddressesPerInterface-1) + 2)
}

func reservedENIsFor(instanceType string) int64 {
	instance, err := env.EC2API.DescribeInstanceTypes(env.Context, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{ec2types.InstanceType(instanceType)},
	})
	Expect(err).ToNot(HaveOccurred())
	networkInfo := *instance.InstanceTypes[0].NetworkInfo
	reservedENIs := 0
	reservedENIsVar, ok := lo.Find(env.ExpectSettings(), func(v corev1.EnvVar) bool { return v.Name == "RESERVED_ENIS" })
	if ok {
		reservedENIs, err = strconv.Atoi(reservedENIsVar.Value)
		Expect(err).ToNot(HaveOccurred())
	}
	return int64((int(*networkInfo.MaximumNetworkInterfaces)-reservedENIs)*(int(*networkInfo.Ipv4AddressesPerInterface-1)) + 2)
}

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

	"time"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/providers/instance"
	"github.com/aws/karpenter/pkg/test"
)

const createdAtTag = "node.k8s.amazonaws.com/createdAt"

var _ = Describe("Tags", func() {
	Context("Static Tags", func() {
		It("should tag all associated resources", func() {
			nodeClass.Spec.Tags["TestTag"] = "TestVal"
			pod := coretest.Pod()

			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			instance := env.GetInstance(pod.Spec.NodeName)
			volumes := env.GetVolumes(lo.Map(instance.BlockDeviceMappings, func(bdm *ec2.InstanceBlockDeviceMapping, _ int) *string {
				return bdm.Ebs.VolumeId
			})...)
			networkInterfaces := env.GetNetworkInterfaces(lo.Map(instance.NetworkInterfaces, func(ni *ec2.InstanceNetworkInterface, _ int) *string {
				return ni.NetworkInterfaceId
			})...)

			Expect(tagMap(instance.Tags)).To(HaveKeyWithValue("TestTag", "TestVal"))
			for _, volume := range volumes {
				Expect(tagMap(volume.Tags)).To(HaveKeyWithValue("TestTag", "TestVal"))
			}
			for _, networkInterface := range networkInterfaces {
				// Any ENI that contains this createdAt tag was created by the VPC CNI DaemonSet
				if _, ok := tagMap(networkInterface.TagSet)[createdAtTag]; !ok {
					Expect(tagMap(networkInterface.TagSet)).To(HaveKeyWithValue("TestTag", "TestVal"))
				}
			}
		})
		It("should tag spot instance requests when creating resources", func() {
			coretest.ReplaceRequirements(nodePool, v1.NodeSelectorRequirement{
				Key:      corev1beta1.CapacityTypeLabelKey,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{corev1beta1.CapacityTypeSpot},
			})
			nodeClass.Spec.Tags = map[string]string{"TestTag": "TestVal"}
			pod := coretest.Pod()

			env.ExpectCreated(pod, nodeClass, nodePool)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			instance := env.GetInstance(pod.Spec.NodeName)
			Expect(instance.SpotInstanceRequestId).ToNot(BeNil())
			spotInstanceRequest := env.GetSpotInstanceRequest(instance.SpotInstanceRequestId)
			Expect(tagMap(spotInstanceRequest.Tags)).To(HaveKeyWithValue("TestTag", "TestVal"))
		})
	})

	Context("Tagging Controller", func() {
		It("should tag with karpenter.sh/nodeclaim and Name tag", func() {
			pod := coretest.Pod()

			env.ExpectCreated(nodePool, nodeClass, pod)
			env.EventuallyExpectCreatedNodeCount("==", 1)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
			nodeName := client.ObjectKeyFromObject(node)

			Eventually(func(g Gomega) {
				node = &v1.Node{}
				g.Expect(env.Client.Get(env.Context, nodeName, node)).To(Succeed())
				g.Expect(node.Annotations).To(HaveKeyWithValue(v1beta1.AnnotationInstanceTagged, "true"))
			}, time.Minute)

			nodeInstance := instance.NewInstance(lo.ToPtr(env.GetInstance(node.Name)))
			Expect(nodeInstance.Tags).To(HaveKeyWithValue("Name", node.Name))
			Expect(nodeInstance.Tags).To(HaveKey("karpenter.sh/nodeclaim"))
		})

		It("shouldn't overwrite custom Name tags", func() {
			nodeClass = test.EC2NodeClass(*nodeClass, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{
				Tags: map[string]string{"Name": "custom-name", "testing/cluster": env.ClusterName},
			}})
			nodePool = coretest.NodePool(*nodePool, corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClassRef: &corev1beta1.NodeClassReference{Name: nodeClass.Name},
						},
					},
				},
			})
			pod := coretest.Pod()

			env.ExpectCreated(nodePool, nodeClass, pod)
			env.EventuallyExpectCreatedNodeCount("==", 1)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
			nodeName := client.ObjectKeyFromObject(node)

			Eventually(func(g Gomega) {
				node = &v1.Node{}
				g.Expect(env.Client.Get(env.Context, nodeName, node)).To(Succeed())
				g.Expect(node.Annotations).To(HaveKeyWithValue(v1beta1.AnnotationInstanceTagged, "true"))
			}, time.Minute)

			nodeInstance := instance.NewInstance(lo.ToPtr(env.GetInstance(node.Name)))
			Expect(nodeInstance.Tags).To(HaveKeyWithValue("Name", "custom-name"))
			Expect(nodeInstance.Tags).To(HaveKey("karpenter.sh/nodeclaim"))
		})
	})
})

func tagMap(tags []*ec2.Tag) map[string]string {
	return lo.SliceToMap(tags, func(tag *ec2.Tag) (string, string) {
		return *tag.Key, *tag.Value
	})
}

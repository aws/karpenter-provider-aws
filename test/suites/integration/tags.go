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

package integration

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/awslabs/operatorpkg/object"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instance"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

const createdAtTag = "node.k8s.amazonaws.com/createdAt"

func registerTags() bool {
	return Describe("Tags", func() {
		Context("Static Tags", func() {
			It("should tag all associated resources", func() {
				NodeClass.Spec.Tags["TestTag"] = "TestVal"
				pod := coretest.Pod()
				Env.ExpectCreated(pod, NodeClass, NodePool)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)
				instance := Env.GetInstance(pod.Spec.NodeName)
				volumes := Env.GetVolumes(lo.Map(instance.BlockDeviceMappings, func(bdm ec2types.InstanceBlockDeviceMapping, _ int) string {
					return lo.FromPtr(bdm.Ebs.VolumeId)
				})...)
				networkInterfaces := Env.GetNetworkInterfaces(lo.Map(instance.NetworkInterfaces, func(ni ec2types.InstanceNetworkInterface, _ int) string {
					return lo.FromPtr(ni.NetworkInterfaceId)
				})...)

				Expect(instance.Tags).To(ContainElement(ec2types.Tag{Key: lo.ToPtr("TestTag"), Value: lo.ToPtr("TestVal")}))
				for _, volume := range volumes {
					Expect(volume.Tags).To(ContainElement(ec2types.Tag{Key: lo.ToPtr("TestTag"), Value: lo.ToPtr("TestVal")}))
				}
				for _, networkInterface := range networkInterfaces {
					// Any ENI that contains this createdAt tag was created by the VPC CNI DaemonSet
					if !lo.ContainsBy(networkInterface.TagSet, func(t ec2types.Tag) bool { return lo.FromPtr(t.Key) == createdAtTag }) {
						Expect(networkInterface.TagSet).To(ContainElement(ec2types.Tag{Key: lo.ToPtr("TestTag"), Value: lo.ToPtr("TestVal")}))
					}
				}
			})
			It("should tag spot instance requests when creating resources", func() {
				coretest.ReplaceRequirements(NodePool, karpv1.NodeSelectorRequirementWithMinValues{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.CapacityTypeSpot},
				})
				NodeClass.Spec.Tags = map[string]string{"TestTag": "TestVal"}
				pod := coretest.Pod()
				Env.ExpectCreated(pod, NodeClass, NodePool)
				Env.EventuallyExpectHealthy(pod)
				Env.ExpectCreatedNodeCount("==", 1)
				instance := Env.GetInstance(pod.Spec.NodeName)
				Expect(instance.SpotInstanceRequestId).ToNot(BeNil())
				spotInstanceRequest := Env.GetSpotInstance(lo.FromPtr(instance.SpotInstanceRequestId))
				Expect(spotInstanceRequest.Tags).To(ContainElement(ec2types.Tag{Key: lo.ToPtr("TestTag"), Value: lo.ToPtr("TestVal")}))
			})
			It("should tag managed instance profiles", func() {
				if Env.PrivateCluster {
					Skip("skipping managed instance profiles test for private cluster")
				}
				NodeClass.Spec.Tags["TestTag"] = "TestVal"
				Env.ExpectCreated(NodeClass)

				Eventually(func(g Gomega) {
					err := Env.Client.Get(Env.Context, client.ObjectKeyFromObject(NodeClass), NodeClass)
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(NodeClass.Status.InstanceProfile).ToNot(BeEmpty())
				}).Should(Succeed())

				profile := Env.EventuallyExpectInstanceProfileExists(NodeClass.Status.InstanceProfile)
				Expect(profile.Tags).To(ContainElements(
					iamtypes.Tag{Key: lo.ToPtr(fmt.Sprintf("kubernetes.io/cluster/%s", Env.ClusterName)), Value: lo.ToPtr("owned")},
					iamtypes.Tag{Key: lo.ToPtr(v1.LabelNodeClass), Value: lo.ToPtr(NodeClass.Name)},
					iamtypes.Tag{Key: lo.ToPtr(v1.EKSClusterNameTagKey), Value: lo.ToPtr(Env.ClusterName)},
				))
			})
		})
		Context("Tagging Controller", func() {
			It("should tag with karpenter.sh/nodeclaim and Name tag", func() {
				pod := coretest.Pod()
				Env.ExpectCreated(NodePool, NodeClass, pod)
				Env.EventuallyExpectCreatedNodeCount("==", 1)
				node := Env.EventuallyExpectInitializedNodeCount("==", 1)[0]
				nodeClaim := Env.ExpectNodeClaimCount("==", 1)[0]
				Eventually(func(g Gomega) {
					g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
					g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationInstanceTagged, "true"))
					g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationClusterNameTaggedCompatability, "true"))
				}, time.Minute).Should(Succeed())

				ctx := options.ToContext(Env.Context, &options.Options{})
				nodeInstance := instance.NewInstance(ctx, Env.GetInstance(node.Name))
				Expect(nodeInstance.Tags).To(HaveKeyWithValue("Name", node.Name))
				Expect(nodeInstance.Tags).To(HaveKeyWithValue("karpenter.sh/nodeclaim", nodeClaim.Name))
				Expect(nodeInstance.Tags).To(HaveKeyWithValue("eks:eks-cluster-name", Env.ClusterName))
			})
			It("shouldn't overwrite custom Name tags", func() {
				NodeClass = test.EC2NodeClass(*NodeClass, v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{
					Tags: map[string]string{"Name": "custom-name", "testing/cluster": Env.ClusterName},
				}})
				if Env.PrivateCluster {
					NodeClass.Spec.Role = ""
					NodeClass.Spec.InstanceProfile = lo.ToPtr(fmt.Sprintf("KarpenterNodeInstanceProfile-%s", Env.ClusterName))
				}
				NodePool = coretest.NodePool(*NodePool, karpv1.NodePool{
					Spec: karpv1.NodePoolSpec{
						Template: karpv1.NodeClaimTemplate{
							Spec: karpv1.NodeClaimTemplateSpec{
								NodeClassRef: &karpv1.NodeClassReference{
									Group: object.GVK(NodeClass).Group,
									Kind:  object.GVK(NodeClass).Kind,
									Name:  NodeClass.Name,
								},
							},
						},
					},
				})
				pod := coretest.Pod()
				Env.ExpectCreated(NodePool, NodeClass, pod)
				Env.EventuallyExpectCreatedNodeCount("==", 1)
				node := Env.EventuallyExpectInitializedNodeCount("==", 1)[0]
				nodeClaim := Env.ExpectNodeClaimCount("==", 1)[0]
				Eventually(func(g Gomega) {
					g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
					g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationInstanceTagged, "true"))
					g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationClusterNameTaggedCompatability, "true"))
				}, time.Minute).Should(Succeed())

				ctx := options.ToContext(Env.Context, &options.Options{})
				nodeInstance := instance.NewInstance(ctx, Env.GetInstance(node.Name))
				Expect(nodeInstance.Tags).To(HaveKeyWithValue("Name", "custom-name"))
				Expect(nodeInstance.Tags).To(HaveKeyWithValue("karpenter.sh/nodeclaim", nodeClaim.Name))
				Expect(nodeInstance.Tags).To(HaveKeyWithValue("eks:eks-cluster-name", Env.ClusterName))
			})
			It("should tag instance with eks:eks-cluster-name tag when the tag doesn't exist", func() {
				pod := coretest.Pod()
				Env.ExpectCreated(NodePool, NodeClass, pod)
				Env.EventuallyExpectCreatedNodeCount("==", 1)
				node := Env.EventuallyExpectInitializedNodeCount("==", 1)[0]
				nodeClaim := Env.ExpectNodeClaimCount("==", 1)[0]
				Eventually(func(g Gomega) {
					g.Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
					g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationInstanceTagged, "true"))
					g.Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.AnnotationClusterNameTaggedCompatability, "true"))
				}, time.Minute).Should(Succeed())

				_, err := Env.EC2API.DeleteTags(Env.Context, &ec2.DeleteTagsInput{
					Resources: []string{Env.ExpectParsedProviderID(node.Spec.ProviderID)},
					Tags:      []ec2types.Tag{{Key: lo.ToPtr(v1.EKSClusterNameTagKey)}},
				})
				Expect(err).ToNot(HaveOccurred())

				By(fmt.Sprintf("Removing the %s annotation to re-trigger tagging", v1.AnnotationClusterNameTaggedCompatability))
				Expect(Env.Client.Get(Env.Context, client.ObjectKeyFromObject(nodeClaim), nodeClaim)).To(Succeed())
				delete(nodeClaim.Annotations, v1.AnnotationClusterNameTaggedCompatability)
				Env.ExpectUpdated(nodeClaim)

				By(fmt.Sprintf("Polling for the %s tag update", v1.EKSClusterNameTagKey))
				Eventually(func(g Gomega) {
					out, err := Env.EC2API.DescribeInstances(Env.Context, &ec2.DescribeInstancesInput{
						InstanceIds: []string{Env.ExpectParsedProviderID(node.Spec.ProviderID)},
					})
					g.Expect(err).ToNot(HaveOccurred())
					g.Expect(out.Reservations).To(HaveLen(1))
					g.Expect(out.Reservations[0].Instances).To(HaveLen(1))
					g.Expect(out.Reservations[0].Instances[0].Tags).To(ContainElement(ec2types.Tag{Key: lo.ToPtr(v1.EKSClusterNameTagKey), Value: lo.ToPtr(Env.ClusterName)}))
				}).Should(Succeed())
			})
		})
	})
}

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

	"fmt"
	"time"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/providers/instance"
	"github.com/aws/karpenter/pkg/test"
)

var _ = Describe("Tags", func() {
	Context("Static Tags", func() {
		It("should tag all associated resources", func() {
			provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					Tags:                  map[string]string{"TestTag": "TestVal"},
				},
			})
			provisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
			pod := coretest.Pod()

			env.ExpectCreated(pod, provider, provisioner)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			instance := env.GetInstance(pod.Spec.NodeName)
			volumeTags := tagMap(env.GetVolume(instance.BlockDeviceMappings[0].Ebs.VolumeId).Tags)
			instanceTags := tagMap(instance.Tags)

			Expect(instanceTags).To(HaveKeyWithValue("TestTag", "TestVal"))
			Expect(volumeTags).To(HaveKeyWithValue("TestTag", "TestVal"))
		})
		It("should tag all associated resources with global tags", func() {
			provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
				AWS: v1alpha1.AWS{
					SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
					SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				},
			})

			env.ExpectSettingsOverridden(map[string]string{
				"aws.tags": `{"TestTag": "TestVal", "example.com/tag": "custom-value"}`,
			})
			provisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: provider.Name}})
			pod := coretest.Pod()

			env.ExpectCreated(pod, provider, provisioner)
			env.EventuallyExpectHealthy(pod)
			env.ExpectCreatedNodeCount("==", 1)
			instance := env.GetInstance(pod.Spec.NodeName)
			volumeTags := tagMap(env.GetVolume(instance.BlockDeviceMappings[0].Ebs.VolumeId).Tags)
			instanceTags := tagMap(instance.Tags)

			Expect(instanceTags).To(HaveKeyWithValue("TestTag", "TestVal"))
			Expect(volumeTags).To(HaveKeyWithValue("TestTag", "TestVal"))
			Expect(instanceTags).To(HaveKeyWithValue("example.com/tag", "custom-value"))
			Expect(volumeTags).To(HaveKeyWithValue("example.com/tag", "custom-value"))
		})
	})

	Context("Tagging Controller", func() {
		var nodeClass *v1beta1.EC2NodeClass
		var nodePool *corev1beta1.NodePool

		BeforeEach(func() {
			nodeClass = test.EC2NodeClass(v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{
				SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{{
					Tags: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				}},
				SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{{
					Tags: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				}},
			}})

			nodePool = coretest.NodePool(corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClass: &corev1beta1.NodeClassReference{
								Name: nodeClass.Name,
							},
						},
					},
					Limits: corev1beta1.Limits{},
				},
			})
		})

		It("should tag with karpenter.k8s.aws/nodeclaim and Name tag", func() {
			Skip("NodeClaim tagging tests disabled until v1beta1")
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
			Skip("NodeClaim tagging tests disabled until v1beta1")
			nodeClass = test.EC2NodeClass(*nodeClass, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{
				Tags: map[string]string{"Name": "custom-name"},
			}})
			nodePool = coretest.NodePool(*nodePool, corev1beta1.NodePool{
				Spec: corev1beta1.NodePoolSpec{
					Template: corev1beta1.NodeClaimTemplate{
						Spec: corev1beta1.NodeClaimSpec{
							NodeClass: &corev1beta1.NodeClassReference{Name: nodeClass.Name},
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

		It("shouldn't tag nodes provisioned by v1alpha5 provisioner", func() {
			nodeTemplate := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
				SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
				SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			}})

			provisioner := coretest.Provisioner(coretest.ProvisionerOptions{
				ProviderRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			})

			pod := coretest.Pod()
			env.ExpectCreated(nodeTemplate, provisioner, pod)
			env.EventuallyExpectCreatedNodeCount("==", 1)
			node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]

			nodeInstance := instance.NewInstance(lo.ToPtr(env.GetInstance(node.Name)))
			Expect(nodeInstance.Tags).To(HaveKeyWithValue("Name", fmt.Sprintf("karpenter.sh/provisioner-name/%s", provisioner.Name)))
			Expect(nodeInstance.Tags).NotTo(HaveKey("karpenter.sh/nodeclaim"))
		})

	})
})

func tagMap(tags []*ec2.Tag) map[string]string {
	return lo.SliceToMap(tags, func(tag *ec2.Tag) (string, string) {
		return *tag.Key, *tag.Value
	})
}

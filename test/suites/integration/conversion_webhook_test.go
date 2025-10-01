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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	karpv1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	karptest "sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var _ = Describe("Conversion Webhooks", func() {
	Context("NodePool", func() {
		It("should not update a metadata generation when the same resource is applied for the v1beta1 APIs", func() {
			// created v1beta1 resource
			storedv1beta1NodePool := &karpv1beta1.NodePool{
				ObjectMeta: karptest.ObjectMeta(),
				Spec: karpv1beta1.NodePoolSpec{
					Limits: karpv1beta1.Limits{
						corev1.ResourceCPU: lo.Must(resource.ParseQuantity("20m")),
					},
					Disruption: karpv1beta1.Disruption{
						ConsolidationPolicy: karpv1beta1.ConsolidationPolicyWhenEmpty,
						ConsolidateAfter:    lo.ToPtr(karpv1beta1.MustParseNillableDuration("1h")),
						ExpireAfter:         karpv1beta1.MustParseNillableDuration("1h"),
					},
					Template: karpv1beta1.NodeClaimTemplate{
						Spec: karpv1beta1.NodeClaimSpec{
							NodeClassRef: &karpv1beta1.NodeClassReference{
								Name: "test-nodeclass",
							},
							Requirements: []karpv1beta1.NodeSelectorRequirementWithMinValues{
								{
									NodeSelectorRequirement: corev1.NodeSelectorRequirement{
										Key:      corev1.LabelOSStable,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{string(corev1.Linux)},
									},
								},
							},
							Kubelet: &karpv1beta1.KubeletConfiguration{
								MaxPods:     lo.ToPtr(int32(110)),
								PodsPerCore: lo.ToPtr(int32(10)),
								SystemReserved: map[string]string{
									string(corev1.ResourceCPU):              "200m",
									string(corev1.ResourceMemory):           "200Mi",
									string(corev1.ResourceEphemeralStorage): "1Gi",
								},
								KubeReserved: map[string]string{
									string(corev1.ResourceCPU):              "200m",
									string(corev1.ResourceMemory):           "200Mi",
									string(corev1.ResourceEphemeralStorage): "1Gi",
								},
								EvictionHard: map[string]string{
									"memory.available":   "5%",
									"nodefs.available":   "5%",
									"nodefs.inodesFree":  "5%",
									"imagefs.available":  "5%",
									"imagefs.inodesFree": "5%",
									"pid.available":      "3%",
								},
								EvictionSoft: map[string]string{
									"memory.available":   "10%",
									"nodefs.available":   "10%",
									"nodefs.inodesFree":  "10%",
									"imagefs.available":  "10%",
									"imagefs.inodesFree": "10%",
									"pid.available":      "6%",
								},
								EvictionSoftGracePeriod: map[string]metav1.Duration{
									"memory.available":   {Duration: time.Minute * 2},
									"nodefs.available":   {Duration: time.Minute * 2},
									"nodefs.inodesFree":  {Duration: time.Minute * 2},
									"imagefs.available":  {Duration: time.Minute * 2},
									"imagefs.inodesFree": {Duration: time.Minute * 2},
									"pid.available":      {Duration: time.Minute * 2},
								},
								EvictionMaxPodGracePeriod:   lo.ToPtr(int32(120)),
								ImageGCHighThresholdPercent: lo.ToPtr(int32(50)),
								ImageGCLowThresholdPercent:  lo.ToPtr(int32(10)),
								CPUCFSQuota:                 lo.ToPtr(false),
							},
						},
					},
				},
			}

			// Use a deepcopy to make sure the nodePool object is not populated with the returned object from the APIServer
			env.ExpectCreated(storedv1beta1NodePool.DeepCopy())
			v1beta1NodePool := env.ExpectExists(storedv1beta1NodePool.DeepCopy()).(*karpv1beta1.NodePool)
			Expect(v1beta1NodePool.Generation).To(BeNumerically("==", 1))

			// Second apply of the same NodePool does not increase the generation
			env.ExpectUpdated(storedv1beta1NodePool.DeepCopy())
			v1beta1NodePool = env.ExpectExists(storedv1beta1NodePool.DeepCopy()).(*karpv1beta1.NodePool)
			Expect(v1beta1NodePool.Generation).To(BeNumerically("==", 1))
			Expect(v1beta1NodePool.Spec.Template.Spec.NodeClassRef.APIVersion).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.NodeClassRef.APIVersion))
			Expect(v1beta1NodePool.Spec.Disruption.ConsolidateAfter.Duration.String()).To(Equal(storedv1beta1NodePool.Spec.Disruption.ConsolidateAfter.String()))
			Expect(v1beta1NodePool.Spec.Disruption.ExpireAfter.Duration.String()).To(Equal(storedv1beta1NodePool.Spec.Disruption.ExpireAfter.String()))
			// Kubelet Validation
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.MaxPods).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.MaxPods))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.PodsPerCore).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.PodsPerCore))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.SystemReserved).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.SystemReserved))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.KubeReserved).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.KubeReserved))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.EvictionHard).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.EvictionHard))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.EvictionSoft).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.EvictionSoft))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.EvictionSoftGracePeriod).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.EvictionSoftGracePeriod))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.EvictionMaxPodGracePeriod).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.EvictionMaxPodGracePeriod))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.ImageGCHighThresholdPercent).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.ImageGCHighThresholdPercent))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.ImageGCLowThresholdPercent).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.ImageGCLowThresholdPercent))
			Expect(v1beta1NodePool.Spec.Template.Spec.Kubelet.CPUCFSQuota).To(Equal(storedv1beta1NodePool.Spec.Template.Spec.Kubelet.CPUCFSQuota))
		})
		It("should not update a metadata generation when the same resource is applied for the v1 APIs", func() {
			nodePool.Spec.Disruption = karpv1.Disruption{
				ConsolidateAfter: karpv1.MustParseNillableDuration("1h"),
			}
			nodePool.Spec.Template.Spec.ExpireAfter = karpv1.MustParseNillableDuration("1h")
			nodePool.Spec.Limits = karpv1.Limits{
				corev1.ResourceCPU: lo.Must(resource.ParseQuantity("20m")),
			}

			// Use a deepcopy to make sure the nodePool object is not populated with the returned object from the APIServer
			env.ExpectCreated(nodePool.DeepCopy())
			v1NodePool := env.ExpectExists(nodePool).(*karpv1.NodePool)
			Expect(v1NodePool.Generation).To(BeNumerically("==", 1))

			// Second apply of the same NodePool does not increase the generation
			env.ExpectUpdated(nodePool.DeepCopy())
			v1NodePool = env.ExpectExists(nodePool).(*karpv1.NodePool)
			Expect(v1NodePool.Generation).To(BeNumerically("==", 1))
			Expect(v1NodePool.Spec.Template.Spec.NodeClassRef.Group).To(Equal(nodePool.Spec.Template.Spec.NodeClassRef.Group))
			Expect(v1NodePool.Spec.Disruption.ConsolidateAfter.Duration.String()).To(Equal(nodePool.Spec.Disruption.ConsolidateAfter.String()))
			Expect(v1NodePool.Spec.Template.Spec.ExpireAfter.Duration.String()).To(Equal(nodePool.Spec.Template.Spec.ExpireAfter.String()))
		})
	})
	Context("EC2NodeClass", func() {
		It("should not update a metadata generation when the same resource is applied for the v1beta1 APIs", func() {
			// created v1beta1 resource
			storedv1beta1nodeclass := test.BetaEC2NodeClass()

			env.ExpectCreated(storedv1beta1nodeclass.DeepCopy())
			v1beta1nodeclass := env.ExpectExists(storedv1beta1nodeclass.DeepCopy()).(*v1beta1.EC2NodeClass)
			Expect(v1beta1nodeclass.Generation).To(BeNumerically("==", 1))

			// Second apply of the same NodeClass does not increase the generation
			env.ExpectUpdated(storedv1beta1nodeclass.DeepCopy())
			v1beta1nodeclass = env.ExpectExists(storedv1beta1nodeclass).(*v1beta1.EC2NodeClass)
			Expect(v1beta1nodeclass.Generation).To(BeNumerically("==", 1))
		})
		It("should not update a metadata generation when the same resource is applied for v1 APIs", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				MaxPods:     lo.ToPtr(int32(110)),
				PodsPerCore: lo.ToPtr(int32(10)),
				SystemReserved: map[string]string{
					string(corev1.ResourceCPU):              "200m",
					string(corev1.ResourceMemory):           "200Mi",
					string(corev1.ResourceEphemeralStorage): "1Gi",
				},
				KubeReserved: map[string]string{
					string(corev1.ResourceCPU):              "200m",
					string(corev1.ResourceMemory):           "200Mi",
					string(corev1.ResourceEphemeralStorage): "1Gi",
				},
				EvictionHard: map[string]string{
					"memory.available":   "5%",
					"nodefs.available":   "5%",
					"nodefs.inodesFree":  "5%",
					"imagefs.available":  "5%",
					"imagefs.inodesFree": "5%",
					"pid.available":      "3%",
				},
				EvictionSoft: map[string]string{
					"memory.available":   "10%",
					"nodefs.available":   "10%",
					"nodefs.inodesFree":  "10%",
					"imagefs.available":  "10%",
					"imagefs.inodesFree": "10%",
					"pid.available":      "6%",
				},
				EvictionSoftGracePeriod: map[string]metav1.Duration{
					"memory.available":   {Duration: time.Minute * 2},
					"nodefs.available":   {Duration: time.Minute * 2},
					"nodefs.inodesFree":  {Duration: time.Minute * 2},
					"imagefs.available":  {Duration: time.Minute * 2},
					"imagefs.inodesFree": {Duration: time.Minute * 2},
					"pid.available":      {Duration: time.Minute * 2},
				},
				EvictionMaxPodGracePeriod:   lo.ToPtr(int32(120)),
				ImageGCHighThresholdPercent: lo.ToPtr(int32(50)),
				ImageGCLowThresholdPercent:  lo.ToPtr(int32(10)),
				CPUCFSQuota:                 lo.ToPtr(false),
			}
			// Use a deepcopy to make sure the nodePool object is not populated with the returned object from the APIServer
			env.ExpectCreated(nodeClass.DeepCopy())
			v1nodeclass := env.ExpectExists(nodeClass.DeepCopy()).(*v1.EC2NodeClass)
			Expect(v1nodeclass.Generation).To(BeNumerically("==", 1))

			// Second apply of the same NodeClass does not increase the generation
			env.ExpectUpdated(nodeClass.DeepCopy())
			v1nodeclass = env.ExpectExists(nodeClass.DeepCopy()).(*v1.EC2NodeClass)
			Expect(v1nodeclass.Generation).To(BeNumerically("==", 1))
			// Kubelet Validation
			Expect(v1nodeclass.Spec.Kubelet.MaxPods).To(Equal(nodeClass.Spec.Kubelet.MaxPods))
			Expect(v1nodeclass.Spec.Kubelet.PodsPerCore).To(Equal(nodeClass.Spec.Kubelet.PodsPerCore))
			Expect(v1nodeclass.Spec.Kubelet.SystemReserved).To(Equal(nodeClass.Spec.Kubelet.SystemReserved))
			Expect(v1nodeclass.Spec.Kubelet.KubeReserved).To(Equal(nodeClass.Spec.Kubelet.KubeReserved))
			Expect(v1nodeclass.Spec.Kubelet.EvictionHard).To(Equal(nodeClass.Spec.Kubelet.EvictionHard))
			Expect(v1nodeclass.Spec.Kubelet.EvictionSoft).To(Equal(nodeClass.Spec.Kubelet.EvictionSoft))
			Expect(v1nodeclass.Spec.Kubelet.EvictionSoftGracePeriod).To(Equal(nodeClass.Spec.Kubelet.EvictionSoftGracePeriod))
			Expect(v1nodeclass.Spec.Kubelet.EvictionMaxPodGracePeriod).To(Equal(nodeClass.Spec.Kubelet.EvictionMaxPodGracePeriod))
			Expect(v1nodeclass.Spec.Kubelet.ImageGCHighThresholdPercent).To(Equal(nodeClass.Spec.Kubelet.ImageGCHighThresholdPercent))
			Expect(v1nodeclass.Spec.Kubelet.ImageGCLowThresholdPercent).To(Equal(nodeClass.Spec.Kubelet.ImageGCLowThresholdPercent))
			Expect(v1nodeclass.Spec.Kubelet.CPUCFSQuota).To(Equal(nodeClass.Spec.Kubelet.CPUCFSQuota))
		})
	})
})

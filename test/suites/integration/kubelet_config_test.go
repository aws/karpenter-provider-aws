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
	"math"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/ptr"

	corev1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("KubeletConfiguration Overrides", func() {
	Context("All kubelet configuration set", func() {
		BeforeEach(func() {
			// MaxPods needs to account for the daemonsets that will run on the nodes
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				MaxPods:     ptr.Int32(110),
				PodsPerCore: ptr.Int32(10),
				SystemReserved: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("200m"),
					v1.ResourceMemory:           resource.MustParse("200Mi"),
					v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
				},
				KubeReserved: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("200m"),
					v1.ResourceMemory:           resource.MustParse("200Mi"),
					v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
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
				EvictionMaxPodGracePeriod:   ptr.Int32(120),
				ImageGCHighThresholdPercent: ptr.Int32(50),
				ImageGCLowThresholdPercent:  ptr.Int32(10),
				CPUCFSQuota:                 ptr.Bool(false),
			}
		})
		DescribeTable("Linux AMIFamilies",
			func(amiFamily *string) {
				nodeClass.Spec.AMIFamily = amiFamily
				pod := test.Pod(test.PodOptions{
					NodeSelector: map[string]string{
						v1.LabelOSStable:   string(v1.Linux),
						v1.LabelArchStable: "amd64",
					},
				})
				env.ExpectCreated(nodeClass, nodePool, pod)
				env.EventuallyExpectHealthy(pod)
				env.ExpectCreatedNodeCount("==", 1)
			},
			Entry("when the AMIFamily is AL2", &v1beta1.AMIFamilyAL2),
			Entry("when the AMIFamily is Ubuntu", &v1beta1.AMIFamilyUbuntu),
			Entry("when the AMIFamily is Bottlerocket", &v1beta1.AMIFamilyBottlerocket),
		)
		DescribeTable("Windows AMIFamilies",
			func(amiFamily *string) {
				env.ExpectWindowsIPAMEnabled()
				DeferCleanup(func() {
					env.ExpectWindowsIPAMDisabled()
				})

				nodeClass.Spec.AMIFamily = amiFamily
				// Need to enable nodepool-level OS-scoping for now since DS evaluation is done off of the nodepool
				// requirements, not off of the instance type options so scheduling can fail if nodepool aren't
				// properly scoped
				// TODO: remove this requirement once VPC RC rolls out m7a.*, r7a.*, c7a.* ENI data (https://github.com/aws/karpenter-provider-aws/issues/4472)
				test.ReplaceRequirements(nodePool,
					v1.NodeSelectorRequirement{
						Key:      v1beta1.LabelInstanceFamily,
						Operator: v1.NodeSelectorOpNotIn,
						Values:   aws.ExcludedInstanceFamilies,
					},
					v1.NodeSelectorRequirement{
						Key:      v1.LabelOSStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{string(v1.Windows)},
					},
				)
				pod := test.Pod(test.PodOptions{
					Image: aws.WindowsDefaultImage,
					NodeSelector: map[string]string{
						v1.LabelOSStable:   string(v1.Windows),
						v1.LabelArchStable: "amd64",
					},
				})
				env.ExpectCreated(nodeClass, nodePool, pod)
				env.EventuallyExpectHealthyWithTimeout(time.Minute*15, pod)
				env.ExpectCreatedNodeCount("==", 1)
			},
			Entry("when the AMIFamily is Windows2019", &v1beta1.AMIFamilyWindows2019),
			Entry("when the AMIFamily is Windows2022", &v1beta1.AMIFamilyWindows2022),
		)
	})
	It("should schedule pods onto separate nodes when maxPods is set", func() {
		// Get the DS pod count and use it to calculate the DS pod overhead
		dsCount := env.GetDaemonSetCount(nodePool)
		nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
			MaxPods: ptr.Int32(1 + int32(dsCount)),
		}

		numPods := 3
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("100m")},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 3)
		env.EventuallyExpectUniqueNodeNames(selector, 3)
	})
	It("should schedule pods onto separate nodes when podsPerCore is set", func() {
		// PodsPerCore needs to account for the daemonsets that will run on the nodes
		// This will have 4 pods available on each node (2 taken by daemonset pods)
		test.ReplaceRequirements(nodePool,
			v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceCPU,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"2"},
			},
		)
		numPods := 4
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("100m")},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

		// Get the DS pod count and use it to calculate the DS pod overhead
		// We calculate podsPerCore to split the test pods and the DS pods between two nodes:
		//   1. If # of DS pods is odd, we will have i.e. ceil((3+2)/2) = 3
		//      Since we restrict node to two cores, we will allow 6 pods. One node will have 3
		//      DS pods and 3 test pods. Other node will have 1 test pod and 3 DS pods
		//   2. If # of DS pods is even, we will have i.e. ceil((4+2)/2) = 3
		//      Since we restrict node to two cores, we will allow 6 pods. Both nodes will have
		//      4 DS pods and 2 test pods.
		dsCount := env.GetDaemonSetCount(nodePool)
		nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
			PodsPerCore: ptr.Int32(int32(math.Ceil(float64(2+dsCount) / 2))),
		}

		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 2)
		env.EventuallyExpectUniqueNodeNames(selector, 2)
	})
	It("should ignore podsPerCore value when Bottlerocket is used", func() {
		nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
		// All pods should schedule to a single node since we are ignoring podsPerCore value
		// This would normally schedule to 3 nodes if not using Bottlerocket
		test.ReplaceRequirements(nodePool,
			v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceCPU,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"2"},
			},
		)

		nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{PodsPerCore: ptr.Int32(1)}
		numPods := 6
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("100m")},
				},
			},
		})
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)

		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectUniqueNodeNames(selector, 1)
	})
})

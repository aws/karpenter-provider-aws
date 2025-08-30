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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("KubeletConfiguration Overrides", func() {
	Context("All kubelet configuration set", func() {
		BeforeEach(func() {
			// MaxPods needs to account for the daemonsets that will run on the nodes
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
		})
		DescribeTable("Linux AMIFamilies",
			func(alias string) {
				if strings.Contains(alias, "al2") && env.K8sMinorVersion() > 32 {
					Skip("AL2 is not supported on versions > 1.32")
				}
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: alias}}
				// TODO (jmdeal@): remove once 22.04 AMIs are supported
				pod := test.Pod(test.PodOptions{
					NodeSelector: map[string]string{
						corev1.LabelOSStable:   string(corev1.Linux),
						corev1.LabelArchStable: "amd64",
					},
				})
				env.ExpectCreated(nodeClass, nodePool, pod)
				env.EventuallyExpectHealthy(pod)
				env.ExpectCreatedNodeCount("==", 1)
			},
			Entry("when the AMIFamily is AL2", "al2@latest"),
			Entry("when the AMIFamily is AL2023", "al2023@latest"),
			Entry("when the AMIFamily is Bottlerocket", "bottlerocket@latest"),
		)
		DescribeTable("Windows AMIFamilies",
			func(term v1.AMISelectorTerm) {
				env.ExpectWindowsIPAMEnabled()
				DeferCleanup(func() {
					env.ExpectWindowsIPAMDisabled()
				})

				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{term}
				// Need to enable nodepool-level OS-scoping for now since DS evaluation is done off of the nodepool
				// requirements, not off of the instance type options so scheduling can fail if nodepool aren't
				// properly scoped
				test.ReplaceRequirements(nodePool,
					karpv1.NodeSelectorRequirementWithMinValues{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      corev1.LabelOSStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{string(corev1.Windows)},
						},
					},
				)
				pod := test.Pod(test.PodOptions{
					Image: aws.WindowsDefaultImage,
					NodeSelector: map[string]string{
						corev1.LabelOSStable:   string(corev1.Windows),
						corev1.LabelArchStable: "amd64",
					},
				})
				env.ExpectCreated(nodeClass, nodePool, pod)
				env.EventuallyExpectHealthyWithTimeout(time.Minute*15, pod)
				env.ExpectCreatedNodeCount("==", 1)
			},
			// Windows tests are can flake due to the instance types that are used in testing.
			// The VPC Resource controller will need to support the instance types that are used.
			// If the instance type is not supported by the controller resource `vpc.amazonaws.com/PrivateIPv4Address` will not register.
			// Issue: https://github.com/aws/karpenter-provider-aws/issues/4472
			// See: https://github.com/aws/amazon-vpc-resource-controller-k8s/blob/master/pkg/aws/vpc/limits.go
			Entry("when the AMIFamily is Windows2019", v1.AMISelectorTerm{Alias: "windows2019@latest"}),
			Entry("when the AMIFamily is Windows2022", v1.AMISelectorTerm{Alias: "windows2022@latest"}),
		)
	})
	It("should schedule pods onto separate nodes when maxPods is set", func() {
		// Get the DS pod count and use it to calculate the DS pod overhead
		dsCount := env.GetDaemonSetCount(nodePool)
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
			MaxPods: lo.ToPtr(1 + int32(dsCount)),
		}

		numPods := 3
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
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
			karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceCPU,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"2"},
				},
			},
		)
		numPods := 4
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
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
		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
			PodsPerCore: lo.ToPtr(int32(math.Ceil(float64(2+dsCount) / 2))),
		}

		env.ExpectCreated(nodeClass, nodePool, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 2)
		env.EventuallyExpectUniqueNodeNames(selector, 2)
	})
	It("should ignore podsPerCore value when Bottlerocket is used", func() {
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
		// All pods should schedule to a single node since we are ignoring podsPerCore value
		// This would normally schedule to 3 nodes if not using Bottlerocket
		test.ReplaceRequirements(nodePool,
			karpv1.NodeSelectorRequirementWithMinValues{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceCPU,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"2"},
				},
			},
		)

		nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{PodsPerCore: lo.ToPtr(int32(1))}
		numPods := 6
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
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

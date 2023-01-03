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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter/pkg/apis/config/settings"
	awstest "github.com/aws/karpenter/pkg/test"

	pscheduling "github.com/aws/karpenter-core/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
)

var _ = Describe("KubeletConfiguration Overrides", func() {
	It("should startup successfully with all kubelet configuration set", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})

		// MaxPods needs to account for the daemonsets that will run on the nodes
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Kubelet: &v1alpha5.KubeletConfiguration{
				ContainerRuntime: ptr.String("containerd"),
				MaxPods:          ptr.Int32(110),
				PodsPerCore:      ptr.Int32(10),
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
				EvictionMaxPodGracePeriod: ptr.Int32(120),
			},
		})

		pod := test.Pod()
		env.ExpectCreated(provisioner, provider, pod)
		env.EventuallyExpectHealthy(pod)
		env.ExpectCreatedNodeCount("==", 1)
	})
	It("should schedule pods onto separate nodes when maxPods is set", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})

		// MaxPods needs to account for the daemonsets that will run on the nodes
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelOSStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{string(v1.Linux)},
				},
			},
		})

		// Get the DS pod count and use it to calculate the DS pod overhead
		dsCount := getDaemonSetPodCount(provisioner)
		provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
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
		env.ExpectCreated(provisioner, provider, dep)

		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 3)
		env.ExpectUniqueNodeNames(selector, 3)
	})
	It("should schedule pods onto separate nodes when podsPerCore is set", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		// PodsPerCore needs to account for the daemonsets that will run on the nodes
		// This will have 4 pods available on each node (2 taken by daemonset pods)
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceCPU,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"2"},
				},
				{
					Key:      v1.LabelOSStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{string(v1.Linux)},
				},
			},
		})
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
		dsCount := getDaemonSetPodCount(provisioner)
		provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
			PodsPerCore: ptr.Int32(int32(math.Ceil(float64(2+dsCount) / 2))),
		}

		env.ExpectCreated(provisioner, provider, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 2)
		env.ExpectUniqueNodeNames(selector, 2)
	})
	It("should ignore podsPerCore value when Bottlerocket is used", func() {
		provider := awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			AMIFamily:             &v1alpha1.AMIFamilyBottlerocket,
		}})
		// All pods should schedule to a single node since we are ignoring podsPerCore value
		// This would normally schedule to 3 nodes if not using Bottlerocket
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Kubelet: &v1alpha5.KubeletConfiguration{
				PodsPerCore: ptr.Int32(1),
			},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha1.LabelInstanceCPU,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"2"},
				},
			},
		})
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

		env.ExpectCreated(provisioner, provider, dep)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.ExpectCreatedNodeCount("==", 1)
		env.ExpectUniqueNodeNames(selector, 1)
	})
})

// Performs the same logic as the scheduler to get the number of daemonset
// pods that we estimate we will need to schedule as overhead to each node
func getDaemonSetPodCount(provisioner *v1alpha5.Provisioner) int {
	daemonSetList := &appsv1.DaemonSetList{}
	Expect(env.Client.List(env.Context, daemonSetList)).To(Succeed())

	count := 0
	for _, daemonSet := range daemonSetList.Items {
		p := &v1.Pod{Spec: daemonSet.Spec.Template.Spec}
		nodeTemplate := pscheduling.NewMachineTemplate(provisioner)
		if err := nodeTemplate.Taints.Tolerates(p); err != nil {
			continue
		}
		if err := nodeTemplate.Requirements.Compatible(scheduling.NewPodRequirements(p)); err != nil {
			continue
		}
		count++
	}
	return count
}

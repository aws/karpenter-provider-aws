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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/ptr"

	"github.com/aws/karpenter/pkg/apis/awsnodetemplate/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	awsv1alpha1 "github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/scheduling"
	"github.com/aws/karpenter/pkg/test"
)

var _ = Describe("KubeletConfiguration Overrides", func() {
	It("should schedule pods onto separate nodes when maxPods is set", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
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

		pods := []*v1.Pod{test.Pod(), test.Pod(), test.Pod()}
		env.ExpectCreated(provisioner, provider)
		for _, pod := range pods {
			env.ExpectCreated(pod)
		}
		env.EventuallyExpectHealthy(pods...)
		env.ExpectCreatedNodeCount("==", 3)

		nodeNames := sets.NewString()
		for _, pod := range pods {
			nodeNames.Insert(pod.Spec.NodeName)
		}
		Expect(len(nodeNames)).To(BeNumerically("==", 3))
	})
	It("should schedule pods onto separate nodes when podsPerCore is set", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
		}})
		// PodsPerCore needs to account for the daemonsets that will run on the nodes
		// This will have 4 pods available on each node (2 taken by daemonset pods)
		provisioner := test.Provisioner(test.ProvisionerOptions{
			ProviderRef: &v1alpha5.ProviderRef{Name: provider.Name},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      awsv1alpha1.LabelInstanceCPU,
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

		pods := []*v1.Pod{test.Pod(), test.Pod(), test.Pod(), test.Pod()}
		env.ExpectCreated(provisioner, provider)
		for _, pod := range pods {
			env.ExpectCreated(pod)
		}
		env.EventuallyExpectHealthy(pods...)
		env.ExpectCreatedNodeCount("==", 2)

		nodeNames := sets.NewString()
		for _, pod := range pods {
			nodeNames.Insert(pod.Spec.NodeName)
		}
		Expect(len(nodeNames)).To(BeNumerically("==", 2))
	})
	It("should ignore podsPerCore value when Bottlerocket is used", func() {
		provider := test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: awsv1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": env.ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": env.ClusterName},
			AMIFamily:             &awsv1alpha1.AMIFamilyBottlerocket,
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
					Key:      awsv1alpha1.LabelInstanceCPU,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"2"},
				},
			},
		})

		pods := []*v1.Pod{test.Pod(), test.Pod(), test.Pod(), test.Pod(), test.Pod(), test.Pod()}
		env.ExpectCreated(provisioner, provider)
		for _, pod := range pods {
			env.ExpectCreated(pod)
		}
		env.EventuallyExpectHealthy(pods...)
		env.ExpectCreatedNodeCount("==", 1)
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
		nodeTemplate := scheduling.NewNodeTemplate(provisioner)
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

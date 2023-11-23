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

package consolidation_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/test/pkg/debug"

	environmentaws "github.com/aws/karpenter/test/pkg/environment/aws"
	"github.com/aws/karpenter/test/pkg/environment/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *environmentaws.Environment

func TestConsolidation(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Consolidation")
}

var nodeClass *v1beta1.EC2NodeClass

var _ = BeforeEach(func() {
	nodeClass = env.DefaultEC2NodeClass()
	env.BeforeEach()
})
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Consolidation", func() {
	It("should consolidate nodes (delete)", Label(debug.NoWatch), Label(debug.NoEvents), func() {
		nodePool := test.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Disruption: corev1beta1.Disruption{
					ConsolidationPolicy: corev1beta1.ConsolidationPolicyWhenUnderutilized,
					// Disable Consolidation until we're ready
					ConsolidateAfter: &corev1beta1.NillableDuration{},
				},
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						Requirements: []v1.NodeSelectorRequirement{
							{
								Key:      corev1beta1.CapacityTypeLabelKey,
								Operator: v1.NodeSelectorOpIn,
								// we don't replace spot nodes, so this forces us to only delete nodes
								Values: []string{corev1beta1.CapacityTypeSpot},
							},
							{
								Key:      v1beta1.LabelInstanceSize,
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{"medium", "large", "xlarge"},
							},
							{
								Key:      v1beta1.LabelInstanceFamily,
								Operator: v1.NodeSelectorOpNotIn,
								// remove some cheap burstable and the odd c1 instance types so we have
								// more control over what gets provisioned
								Values: []string{"t2", "t3", "c1", "t3a", "t4g"},
							},
						},
						NodeClassRef: &corev1beta1.NodeClassReference{Name: nodeClass.Name},
					},
				},
			},
		})

		var numPods int32 = 100
		dep := test.Deployment(test.DeploymentOptions{
			Replicas: numPods,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
				},
			},
		})

		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(nodePool, nodeClass, dep)

		env.EventuallyExpectHealthyPodCount(selector, int(numPods))

		// reduce the number of pods by 60%
		dep.Spec.Replicas = aws.Int32(40)
		env.ExpectUpdated(dep)
		env.EventuallyExpectAvgUtilization(v1.ResourceCPU, "<", 0.5)

		// Enable consolidation as WhenUnderutilized doesn't allow a consolidateAfter value
		nodePool.Spec.Disruption.ConsolidateAfter = nil
		env.ExpectUpdated(nodePool)

		// With consolidation enabled, we now must delete nodes
		env.EventuallyExpectAvgUtilization(v1.ResourceCPU, ">", 0.6)

		env.ExpectDeleted(dep)
	})
	It("should consolidate on-demand nodes (replace)", func() {
		nodePool := test.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Disruption: corev1beta1.Disruption{
					ConsolidationPolicy: corev1beta1.ConsolidationPolicyWhenUnderutilized,
					// Disable Consolidation until we're ready
					ConsolidateAfter: &corev1beta1.NillableDuration{},
				},
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						Requirements: []v1.NodeSelectorRequirement{
							{
								Key:      corev1beta1.CapacityTypeLabelKey,
								Operator: v1.NodeSelectorOpIn,
								// we don't replace spot nodes, so this forces us to only delete nodes
								Values: []string{corev1beta1.CapacityTypeOnDemand},
							},
							{
								Key:      v1beta1.LabelInstanceSize,
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{"large", "2xlarge"},
							},
						},
						NodeClassRef: &corev1beta1.NodeClassReference{Name: nodeClass.Name},
					},
				},
			},
		})

		var numPods int32 = 3
		largeDep := test.Deployment(test.DeploymentOptions{
			Replicas: numPods,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "large-app"},
				},
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       v1.LabelHostname,
						WhenUnsatisfiable: v1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "large-app",
							},
						},
					},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4")},
				},
			},
		})
		smallDep := test.Deployment(test.DeploymentOptions{
			Replicas: numPods,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "small-app"},
				},
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       v1.LabelHostname,
						WhenUnsatisfiable: v1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "small-app",
							},
						},
					},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1.5")},
				},
			},
		})

		selector := labels.SelectorFromSet(largeDep.Spec.Selector.MatchLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(nodePool, nodeClass, largeDep, smallDep)

		env.EventuallyExpectHealthyPodCount(selector, int(numPods))

		// 3 nodes due to the anti-affinity rules
		env.ExpectCreatedNodeCount("==", 3)

		// scaling down the large deployment leaves only small pods on each node
		largeDep.Spec.Replicas = aws.Int32(0)
		env.ExpectUpdated(largeDep)
		env.EventuallyExpectAvgUtilization(v1.ResourceCPU, "<", 0.5)

		nodePool.Spec.Disruption.ConsolidateAfter = nil
		env.ExpectUpdated(nodePool)

		// With consolidation enabled, we now must replace each node in turn to consolidate due to the anti-affinity
		// rules on the smaller deployment.  The 2xl nodes should go to a large
		env.EventuallyExpectAvgUtilization(v1.ResourceCPU, ">", 0.8)

		var nodes v1.NodeList
		Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
		numLargeNodes := 0
		numOtherNodes := 0
		for _, n := range nodes.Items {
			// only count the nodes created by the provisoiner
			if n.Labels[corev1beta1.NodePoolLabelKey] != nodePool.Name {
				continue
			}
			if strings.HasSuffix(n.Labels[v1.LabelInstanceTypeStable], ".large") {
				numLargeNodes++
			} else {
				numOtherNodes++
			}
		}

		// all of the 2xlarge nodes should have been replaced with large instance types
		Expect(numLargeNodes).To(Equal(3))
		// and we should have no other nodes
		Expect(numOtherNodes).To(Equal(0))

		env.ExpectDeleted(largeDep, smallDep)
	})
	It("should consolidate on-demand nodes to spot (replace)", func() {
		nodePool := test.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Disruption: corev1beta1.Disruption{
					ConsolidationPolicy: corev1beta1.ConsolidationPolicyWhenUnderutilized,
					// Disable Consolidation until we're ready
					ConsolidateAfter: &corev1beta1.NillableDuration{},
				},
				Template: corev1beta1.NodeClaimTemplate{
					Spec: corev1beta1.NodeClaimSpec{
						Requirements: []v1.NodeSelectorRequirement{
							{
								Key:      corev1beta1.CapacityTypeLabelKey,
								Operator: v1.NodeSelectorOpIn,
								// we don't replace spot nodes, so this forces us to only delete nodes
								Values: []string{corev1beta1.CapacityTypeOnDemand},
							},
							{
								Key:      v1beta1.LabelInstanceSize,
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{"large"},
							},
						},
						NodeClassRef: &corev1beta1.NodeClassReference{Name: nodeClass.Name},
					},
				},
			},
		})

		var numPods int32 = 2
		smallDep := test.Deployment(test.DeploymentOptions{
			Replicas: numPods,
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "small-app"},
				},
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{
					{
						MaxSkew:           1,
						TopologyKey:       v1.LabelHostname,
						WhenUnsatisfiable: v1.DoNotSchedule,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "small-app",
							},
						},
					},
				},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1.5")},
				},
			},
		})

		selector := labels.SelectorFromSet(smallDep.Spec.Selector.MatchLabels)
		env.ExpectCreatedNodeCount("==", 0)
		env.ExpectCreated(nodePool, nodeClass, smallDep)

		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
		env.ExpectCreatedNodeCount("==", int(numPods))

		// Enable spot capacity type after the on-demand node is provisioned
		// Expect the node to consolidate to a spot instance as it will be a cheaper
		// instance than on-demand
		nodePool.Spec.Disruption.ConsolidateAfter = nil
		test.ReplaceRequirements(nodePool,
			v1.NodeSelectorRequirement{
				Key:      corev1beta1.CapacityTypeLabelKey,
				Operator: v1.NodeSelectorOpExists,
			},
			v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceSize,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"large"},
			},
		)
		env.ExpectUpdated(nodePool)

		// Eventually expect the on-demand nodes to be consolidated into
		// spot nodes after some time
		Eventually(func(g Gomega) {
			var nodes v1.NodeList
			Expect(env.Client.List(env.Context, &nodes)).To(Succeed())
			var spotNodes []*v1.Node
			var otherNodes []*v1.Node
			for i, n := range nodes.Items {
				// only count the nodes created by the nodePool
				if n.Labels[corev1beta1.NodePoolLabelKey] != nodePool.Name {
					continue
				}
				if n.Labels[corev1beta1.CapacityTypeLabelKey] == corev1beta1.CapacityTypeSpot {
					spotNodes = append(spotNodes, &nodes.Items[i])
				} else {
					otherNodes = append(otherNodes, &nodes.Items[i])
				}
			}
			// all the on-demand nodes should have been replaced with spot nodes
			msg := fmt.Sprintf("node names, spot= %v, other = %v", common.NodeNames(spotNodes), common.NodeNames(otherNodes))
			g.Expect(len(spotNodes)).To(BeNumerically("==", numPods), msg)
			// and we should have no other nodes
			g.Expect(len(otherNodes)).To(BeNumerically("==", 0), msg)
		}, time.Minute*10).Should(Succeed())

		env.ExpectDeleted(smallDep)
	})
})

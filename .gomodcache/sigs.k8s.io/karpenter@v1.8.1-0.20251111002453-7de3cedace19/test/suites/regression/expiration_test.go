/*
Copyright The Kubernetes Authors.

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
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	"sigs.k8s.io/karpenter/pkg/test"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Expiration", func() {
	var dep *appsv1.Deployment
	var selector labels.Selector
	var numPods int
	BeforeEach(func() {
		numPods = 1
		dep = test.Deployment(test.DeploymentOptions{
			Replicas: int32(numPods),
			PodOptions: test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "my-app",
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		})
		selector = labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
	})
	It("should expire the node after the expiration is reached", func() {
		// Disable Drift to prevent Karpenter from continually expiring nodes
		// This test initially launches a node with an expiration. After removing the expireAfter value,
		// drift will be induced on all nodes owned by the nodepool. The replacement node for the expired node
		// will not have a time limitation, allowing the test to complete without racing against the next expiration.
		nodePool.Spec.Disruption = v1.Disruption{
			Budgets: []v1.Budget{
				{
					Nodes:   "0",
					Reasons: []v1.DisruptionReason{v1.DisruptionReasonDrifted},
				},
			},
		}
		if env.IsDefaultNodeClassKWOK() {
			nodePool.Spec.Template.Spec.ExpireAfter = v1.MustParseNillableDuration("30s")
		} else {
			nodePool.Spec.Template.Spec.ExpireAfter = v1.MustParseNillableDuration("3m")
		}
		env.ExpectCreated(nodeClass, nodePool, dep)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		// Disable expiration so newly created node won't terminate
		nodePool.Spec.Template.Spec.ExpireAfter = v1.NillableDuration{}
		env.ExpectUpdated(nodePool)
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(selector, numPods)
		env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

		env.EventuallyExpectCreatedNodeCount("==", 1)
		// Set the limit to 0 to make sure we don't continue to create nodeClaims.
		// This is CRITICAL since it prevents leaking node resources into subsequent tests
		nodePool.Spec.Limits = v1.Limits{
			corev1.ResourceCPU: resource.MustParse("0"),
		}
		env.ExpectUpdated(nodePool)

		// After the deletion timestamp is set and all pods are drained
		// the node should be gone
		env.EventuallyExpectNotFound(nodeClaim, node)

		env.EventuallyExpectCreatedNodeClaimCount("==", 1)
		env.EventuallyExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectHealthyPodCount(selector, numPods)
	})
	It("should replace expired node with a single node and schedule all pods", func() {
		// Disable Drift to prevent Karpenter from continually expiring nodes
		// This test initially launches a node with an expiration. After removing the expireAfter value,
		// drift will be induced on all nodes owned by the nodepool. The replacement node for the expired node
		// will not have a time limitation, allowing the test to complete without racing against the next expiration.
		nodePool.Spec.Disruption = v1.Disruption{
			Budgets: []v1.Budget{
				{
					Nodes:   "0",
					Reasons: []v1.DisruptionReason{v1.DisruptionReasonDrifted},
				},
			},
		}
		if env.IsDefaultNodeClassKWOK() {
			nodePool.Spec.Template.Spec.ExpireAfter = v1.MustParseNillableDuration("30s")
		} else {
			nodePool.Spec.Template.Spec.ExpireAfter = v1.MustParseNillableDuration("3m")
		}

		var numPods int32 = 5
		// We should setup a PDB that will only allow a minimum of 1 pod to be pending at a time
		minAvailable := intstr.FromInt32(numPods - 1)
		pdb := test.PodDisruptionBudget(test.PDBOptions{
			Labels: map[string]string{
				"app": "my-app",
			},
			MinAvailable: &minAvailable,
		})
		dep.Spec.Replicas = &numPods
		selector := labels.SelectorFromSet(dep.Spec.Selector.MatchLabels)
		env.ExpectCreated(nodeClass, nodePool, pdb, dep)

		nodeClaim := env.EventuallyExpectCreatedNodeClaimCount("==", 1)[0]
		// Disable expiration so newly created node won't terminate
		nodePool.Spec.Template.Spec.ExpireAfter = v1.NillableDuration{}
		env.ExpectUpdated(nodePool)
		node := env.EventuallyExpectCreatedNodeCount("==", 1)[0]
		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
		env.Monitor.Reset() // Reset the monitor so that we can expect a single node to be spun up after expiration

		env.EventuallyExpectCreatedNodeCount("==", 1)
		// Set the limit to 0 to make sure we don't continue to create nodeClaims.
		// This is CRITICAL since it prevents leaking node resources into subsequent tests
		nodePool.Spec.Limits = v1.Limits{
			corev1.ResourceCPU: resource.MustParse("0"),
		}
		env.ExpectUpdated(nodePool)

		// After the deletion timestamp is set and all pods are drained
		// the node should be gone
		env.EventuallyExpectNotFound(nodeClaim, node)

		env.EventuallyExpectCreatedNodeClaimCount("==", 1)
		env.EventuallyExpectCreatedNodeCount("==", 1)
		env.EventuallyExpectHealthyPodCount(selector, int(numPods))
	})
})

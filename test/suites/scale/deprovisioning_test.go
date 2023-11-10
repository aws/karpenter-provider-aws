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

package scale_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/pkg/ptr"

	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/scheduledchange"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils"
	"github.com/aws/karpenter/test/pkg/debug"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

const (
	deprovisioningTypeKey = "testing/deprovisioning-type"
	consolidationValue    = "consolidation"
	emptinessValue        = "emptiness"
	expirationValue       = "expiration"
	noExpirationValue     = "noExpiration"
	driftValue            = "drift"
	noDriftValue          = "noDrift"
)

const (
	multipleDeprovisionersTestGroup = "multipleDeprovisioners"
	consolidationTestGroup          = "consolidation"
	emptinessTestGroup              = "emptiness"
	expirationTestGroup             = "expiration"
	driftTestGroup                  = "drift"
	interruptionTestGroup           = "interruption"

	defaultTestName = "default"
)

// disableProvisioningLimits represents limits that can be applied to a nodePool if you want a nodePool
// that can deprovision nodes but cannot provision nodes
var disableProvisioningLimits = corev1beta1.Limits{
	v1.ResourceCPU:    resource.MustParse("0"),
	v1.ResourceMemory: resource.MustParse("0Gi"),
}

var _ = Describe("Deprovisioning", Label(debug.NoWatch), Label(debug.NoEvents), func() {
	var nodePool *corev1beta1.NodePool
	var nodeClass *v1beta1.EC2NodeClass
	var deployment *appsv1.Deployment
	var deploymentOptions test.DeploymentOptions
	var selector labels.Selector
	var dsCount int

	BeforeEach(func() {
		env.ExpectSettingsOverridden(v1.EnvVar{Name: "FEATURE_GATES", Value: "Drift=True"})
		nodeClass = env.DefaultEC2NodeClass()
		nodePool = env.DefaultNodePool(nodeClass)
		nodePool.Spec.Limits = nil
		test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirement{
			Key:      v1beta1.LabelInstanceHypervisor,
			Operator: v1.NodeSelectorOpIn,
			Values:   []string{"nitro"},
		})
		deploymentOptions = test.DeploymentOptions{
			PodOptions: test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("10m"),
						v1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		}
		deployment = test.Deployment(deploymentOptions)
		selector = labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)
		dsCount = env.GetDaemonSetCount(nodePool)
	})

	AfterEach(func() {
		env.Cleanup()
	})

	Context("Multiple Deprovisioners", func() {
		It("should run consolidation, emptiness, expiration, and drift simultaneously", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			nodeCountPerNodePool := 10
			replicas := replicasPerNode * nodeCountPerNodePool

			disruptionMethods := []string{
				consolidationValue,
				emptinessValue,
				expirationValue,
				driftValue,
			}
			expectedNodeCount := nodeCountPerNodePool * len(disruptionMethods)

			deploymentMap := map[string]*appsv1.Deployment{}
			// Generate all the deployments for multi-deprovisioning
			for _, v := range disruptionMethods {
				deploymentOptions.PodOptions.NodeSelector = map[string]string{
					deprovisioningTypeKey: v,
				}
				deploymentOptions.PodOptions.Labels = map[string]string{
					deprovisioningTypeKey: v,
				}
				deploymentOptions.PodOptions.Tolerations = []v1.Toleration{
					{
						Key:      deprovisioningTypeKey,
						Operator: v1.TolerationOpEqual,
						Value:    v,
						Effect:   v1.TaintEffectNoSchedule,
					},
				}
				deploymentOptions.Replicas = int32(replicas)
				d := test.Deployment(deploymentOptions)
				deploymentMap[v] = d
			}

			nodePoolMap := map[string]*corev1beta1.NodePool{}
			// Generate all the nodePools for multi-deprovisioning
			for _, v := range disruptionMethods {
				np := test.NodePool()
				np.Spec = *nodePool.Spec.DeepCopy()
				np.Spec.Template.Spec.Taints = []v1.Taint{
					{
						Key:    deprovisioningTypeKey,
						Value:  v,
						Effect: v1.TaintEffectNoSchedule,
					},
				}
				np.Spec.Template.Labels = map[string]string{
					deprovisioningTypeKey: v,
				}
				np.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
				}
				nodePoolMap[v] = test.NodePool(*np)
			}

			By("waiting for the deployment to deploy all of its pods")
			var wg sync.WaitGroup
			for _, d := range deploymentMap {
				wg.Add(1)
				go func(dep *appsv1.Deployment) {
					defer GinkgoRecover()
					defer wg.Done()

					env.ExpectCreated(dep)
					env.EventuallyExpectPendingPodCount(labels.SelectorFromSet(dep.Spec.Selector.MatchLabels), int(lo.FromPtr(dep.Spec.Replicas)))
				}(d)
			}
			wg.Wait()

			// Create a separate nodeClass for drift so that we can change the nodeClass later without it affecting
			// the other nodePools
			driftNodeClass := awstest.EC2NodeClass()
			driftNodeClass.Spec = *nodeClass.Spec.DeepCopy()
			nodePoolMap[driftValue].Spec.Template.Spec.NodeClassRef = &corev1beta1.NodeClassReference{
				Name: driftNodeClass.Name,
			}
			env.MeasureProvisioningDurationFor(func() {
				By("kicking off provisioning by applying the nodePool and nodeClass")
				env.ExpectCreated(driftNodeClass, nodeClass)
				for _, p := range nodePoolMap {
					env.ExpectCreated(p)
				}

				env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)

				// Wait for all pods across all deployments we have created to be in a healthy state
				wg = sync.WaitGroup{}
				for _, d := range deploymentMap {
					wg.Add(1)
					go func(dep *appsv1.Deployment) {
						defer GinkgoRecover()
						defer wg.Done()

						env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(dep.Spec.Selector.MatchLabels), int(lo.FromPtr(dep.Spec.Replicas)))
					}(d)
				}
				wg.Wait()
			}, map[string]string{
				aws.TestCategoryDimension:           multipleDeprovisionersTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			By("scaling down replicas across deployments")
			deploymentMap[consolidationValue].Spec.Replicas = lo.ToPtr[int32](int32(int(float64(replicas) * 0.2)))
			deploymentMap[emptinessValue].Spec.Replicas = lo.ToPtr[int32](0)
			for _, d := range deploymentMap {
				env.ExpectUpdated(d)
			}

			// Create a nodePool for expiration so that expiration can do replacement
			nodePoolMap[noExpirationValue] = test.NodePool()
			nodePoolMap[noExpirationValue].Spec = *nodePoolMap[expirationValue].Spec.DeepCopy()

			// Enable consolidation, emptiness, and expiration
			nodePoolMap[consolidationValue].Spec.Disruption.ConsolidateAfter = nil
			nodePoolMap[emptinessValue].Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenEmpty
			nodePoolMap[emptinessValue].Spec.Disruption.ConsolidateAfter.Duration = ptr.Duration(0)
			nodePoolMap[expirationValue].Spec.Disruption.ExpireAfter.Duration = ptr.Duration(0)
			nodePoolMap[expirationValue].Spec.Limits = disableProvisioningLimits
			// Update the drift NodeClass to start drift on Nodes assigned to this NodeClass
			driftNodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket

			// Create test assertions to ensure during the multiple deprovisioner scale-downs
			type testAssertions struct {
				deletedCount             int
				deletedNodeCountSelector labels.Selector
				nodeCount                int
				nodeCountSelector        labels.Selector
				createdCount             int
			}
			assertionMap := map[string]testAssertions{
				consolidationValue: {
					deletedCount: int(float64(nodeCountPerNodePool) * 0.8),
					nodeCount:    int(float64(nodeCountPerNodePool) * 0.2),
					createdCount: 0,
				},
				emptinessValue: {
					deletedCount: nodeCountPerNodePool,
					nodeCount:    0,
					createdCount: 0,
				},
				expirationValue: {
					deletedCount: nodeCountPerNodePool,
					nodeCount:    nodeCountPerNodePool,
					nodeCountSelector: labels.SelectorFromSet(map[string]string{
						corev1beta1.NodePoolLabelKey: nodePoolMap[noExpirationValue].Name,
					}),
					createdCount: nodeCountPerNodePool,
				},
				driftValue: {
					deletedCount: nodeCountPerNodePool,
					nodeCount:    nodeCountPerNodePool,
					createdCount: nodeCountPerNodePool,
				},
			}
			totalDeletedCount := lo.Reduce(lo.Values(assertionMap), func(agg int, assertion testAssertions, _ int) int {
				return agg + assertion.deletedCount
			}, 0)
			totalCreatedCount := lo.Reduce(lo.Values(assertionMap), func(agg int, assertion testAssertions, _ int) int {
				return agg + assertion.createdCount
			}, 0)
			env.MeasureDeprovisioningDurationFor(func() {
				By("enabling deprovisioning across nodePools")
				for _, p := range nodePoolMap {
					env.ExpectCreatedOrUpdated(p)
				}
				env.ExpectUpdated(driftNodeClass)

				By("waiting for the nodes across all deprovisioners to get deleted")
				wg = sync.WaitGroup{}
				for k, v := range assertionMap {
					wg.Add(1)
					go func(d string, assertions testAssertions) {
						defer GinkgoRecover()
						defer wg.Done()

						// Provide a default selector based on the original nodePool name if one isn't specified
						selector = assertions.deletedNodeCountSelector
						if selector == nil {
							selector = labels.SelectorFromSet(map[string]string{corev1beta1.NodePoolLabelKey: nodePoolMap[d].Name})
						}
						env.EventuallyExpectDeletedNodeCountWithSelector("==", assertions.deletedCount, selector)

						// Provide a default selector based on the original nodePool name if one isn't specified
						selector = assertions.nodeCountSelector
						if selector == nil {
							selector = labels.SelectorFromSet(map[string]string{corev1beta1.NodePoolLabelKey: nodePoolMap[d].Name})
						}
						env.EventuallyExpectNodeCountWithSelector("==", assertions.nodeCount, selector)
						env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deploymentMap[d].Spec.Selector.MatchLabels), int(lo.FromPtr(deploymentMap[d].Spec.Replicas)))

					}(k, v)
				}
				wg.Wait()
			}, map[string]string{
				aws.TestCategoryDimension:           multipleDeprovisionersTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(totalCreatedCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(totalDeletedCount),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})
		}, SpecTimeout(time.Hour))
	})
	Context("Consolidation", func() {
		It("should delete all empty nodes with consolidation", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 200
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureProvisioningDurationFor(func() {
				By("kicking off provisioning by applying the nodePool and nodeClass")
				env.ExpectCreated(nodePool, nodeClass)

				env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           consolidationTestGroup,
				aws.TestNameDimension:               "empty/delete",
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			// Delete deployment to make nodes empty
			env.ExpectDeleted(deployment)
			env.EventuallyExpectHealthyPodCount(selector, 0)

			env.MeasureDeprovisioningDurationFor(func() {
				By("kicking off deprovisioning by setting the consolidation enabled value on the nodePool")
				nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenUnderutilized
				env.ExpectUpdated(nodePool)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", 0)
			}, map[string]string{
				aws.TestCategoryDimension:           consolidationTestGroup,
				aws.TestNameDimension:               "empty/delete",
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(0),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(expectedNodeCount),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})
		}, SpecTimeout(time.Minute*30))
		It("should consolidate nodes to get a higher utilization (multi-consolidation delete)", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 200
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureProvisioningDurationFor(func() {
				By("kicking off provisioning by applying the nodePool and nodeClass")
				env.ExpectCreated(nodePool, nodeClass)

				env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           consolidationTestGroup,
				aws.TestNameDimension:               "delete",
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			replicas = int(float64(replicas) * 0.2)
			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			env.ExpectUpdated(deployment)
			env.EventuallyExpectHealthyPodCount(selector, replicas)

			env.MeasureDeprovisioningDurationFor(func() {
				By("kicking off deprovisioning by setting the consolidation enabled value on the nodePool")
				nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenUnderutilized
				env.ExpectUpdated(nodePool)

				env.EventuallyExpectDeletedNodeCount("==", int(float64(expectedNodeCount)*0.8))
				env.EventuallyExpectNodeCount("==", int(float64(expectedNodeCount)*0.2))
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           consolidationTestGroup,
				aws.TestNameDimension:               "delete",
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(0),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(int(float64(expectedNodeCount) * 0.8)),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})
		}, SpecTimeout(time.Minute*30))
		It("should consolidate nodes to get a higher utilization (single consolidation replace)", func(_ context.Context) {
			replicasPerNode := 1
			expectedNodeCount := 20 // we're currently doing around 1 node/2 mins so this test should run deprovisioning in about 45m
			replicas := replicasPerNode * expectedNodeCount

			// Add in a instance type size requirement that's larger than the smallest that fits the pods.
			test.ReplaceRequirements(nodePool, v1.NodeSelectorRequirement{
				Key:      v1beta1.LabelInstanceSize,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"2xlarge"},
			})

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			// Hostname anti-affinity to require one pod on each node
			deployment.Spec.Template.Spec.Affinity = &v1.Affinity{
				PodAntiAffinity: &v1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
						{
							LabelSelector: deployment.Spec.Selector,
							TopologyKey:   v1.LabelHostname,
						},
					},
				},
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureProvisioningDurationFor(func() {
				By("kicking off provisioning by applying the nodePool and nodeClass")
				env.ExpectCreated(nodePool, nodeClass)

				env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           consolidationTestGroup,
				aws.TestNameDimension:               "replace",
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			env.MeasureDeprovisioningDurationFor(func() {
				By("kicking off deprovisioning by setting the consolidation enabled value on the nodePool")
				// The nodePool defaults to a larger instance type than we need so enabling consolidation and making
				// the requirements wide-open should cause deletes and increase our utilization on the cluster
				nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenUnderutilized
				nodePool.Spec.Template.Spec.Requirements = lo.Reject(nodePool.Spec.Template.Spec.Requirements, func(r v1.NodeSelectorRequirement, _ int) bool {
					return r.Key == v1beta1.LabelInstanceSize
				})
				env.ExpectUpdated(nodePool)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount) // every node should delete due to replacement
				env.EventuallyExpectNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           consolidationTestGroup,
				aws.TestNameDimension:               "replace",
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(expectedNodeCount),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})
		}, SpecTimeout(time.Hour))
	})
	Context("Emptiness", func() {
		It("should deprovision all nodes when empty", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 200
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureProvisioningDurationFor(func() {
				By("kicking off provisioning by applying the nodePool and nodeClass")
				env.ExpectCreated(nodePool, nodeClass)

				env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           emptinessTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			By("waiting for all deployment pods to be deleted")
			// Delete deployment to make nodes empty
			env.ExpectDeleted(deployment)
			env.EventuallyExpectHealthyPodCount(selector, 0)

			env.MeasureDeprovisioningDurationFor(func() {
				By("kicking off deprovisioning emptiness by setting the ttlSecondsAfterEmpty value on the nodePool")
				nodePool.Spec.Disruption.ConsolidationPolicy = corev1beta1.ConsolidationPolicyWhenEmpty
				nodePool.Spec.Disruption.ConsolidateAfter.Duration = ptr.Duration(0)
				env.ExpectCreatedOrUpdated(nodePool)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", 0)
			}, map[string]string{
				aws.TestCategoryDimension:           emptinessTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(0),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(expectedNodeCount),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})
		}, SpecTimeout(time.Minute*30))
	})
	Context("Expiration", func() {
		It("should expire all nodes", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 20 // we're currently doing around 1 node/2 mins so this test should run deprovisioning in about 45m
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureProvisioningDurationFor(func() {
				By("kicking off provisioning by applying the nodePool and nodeClass")
				env.ExpectCreated(nodePool, nodeClass)

				env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           expirationTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			env.MeasureDeprovisioningDurationFor(func() {
				By("kicking off deprovisioning expiration by setting the ttlSecondsUntilExpired value on the nodePool")
				// Change limits so that replacement nodes will use another nodePool.
				nodePool.Spec.Limits = disableProvisioningLimits
				// Enable Expiration
				nodePool.Spec.Disruption.ExpireAfter.Duration = ptr.Duration(0)

				noExpireNodePool := test.NodePool(*nodePool.DeepCopy())
				noExpireNodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
				}
				env.ExpectCreatedOrUpdated(nodePool, noExpireNodePool)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           expirationTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(expectedNodeCount),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})
		}, SpecTimeout(time.Hour))
	})
	Context("Drift", func() {
		It("should drift all nodes", func(_ context.Context) {
			// Before Deprovisioning, we need to Provision the cluster to the state that we need.
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 20 // we're currently doing around 1 node/2 mins so this test should run deprovisioning in about 45m
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureProvisioningDurationFor(func() {
				By("kicking off provisioning by applying the nodePool and nodeClass")
				env.ExpectCreated(nodePool, nodeClass)

				env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           driftTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			env.MeasureDeprovisioningDurationFor(func() {
				By("kicking off deprovisioning drift by changing the nodeClass AMIFamily")
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				env.ExpectCreatedOrUpdated(nodeClass)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           driftTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(expectedNodeCount),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})
		}, SpecTimeout(time.Hour))
	})
	Context("Interruption", func() {
		It("should interrupt all nodes due to scheduledChange", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 200
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			var nodes []*v1.Node
			env.MeasureProvisioningDurationFor(func() {
				By("kicking off provisioning by applying the nodePool and nodeClass")
				env.ExpectCreated(nodePool, nodeClass)
				env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				nodes = env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           interruptionTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			var msgs []interface{}
			for _, node := range nodes {
				instanceID, err := utils.ParseInstanceID(node.Spec.ProviderID)
				Expect(err).ToNot(HaveOccurred())
				msgs = append(msgs, scheduledChangeMessage(env.Region, "000000000000", instanceID))
			}

			env.MeasureDeprovisioningDurationFor(func() {
				By("kicking off deprovisioning by adding scheduledChange messages to the queue")
				env.ExpectMessagesCreated(msgs...)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, map[string]string{
				aws.TestCategoryDimension:           interruptionTestGroup,
				aws.TestNameDimension:               defaultTestName,
				aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
				aws.DeprovisionedNodeCountDimension: strconv.Itoa(expectedNodeCount),
				aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
			})
		}, SpecTimeout(time.Minute*30))
	})
})

func scheduledChangeMessage(region, accountID, involvedInstanceID string) scheduledchange.Message {
	return scheduledchange.Message{
		Metadata: messages.Metadata{
			Version:    "0",
			Account:    accountID,
			DetailType: "AWS Health Event",
			ID:         string(uuid.NewUUID()),
			Region:     region,
			Resources: []string{
				fmt.Sprintf("arn:aws:ec2:%s:instance/%s", region, involvedInstanceID),
			},
			Source: "aws.health",
			Time:   time.Now(),
		},
		Detail: scheduledchange.Detail{
			Service:           "EC2",
			EventTypeCategory: "scheduledChange",
			AffectedEntities: []scheduledchange.AffectedEntity{
				{
					EntityValue: involvedInstanceID,
				},
			},
		},
	}
}

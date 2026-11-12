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

package scale

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/awslabs/operatorpkg/object"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/uuid"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/interruption/messages/scheduledchange"
	awstest "github.com/aws/karpenter-provider-aws/pkg/test"
	"github.com/aws/karpenter-provider-aws/pkg/utils"
	"github.com/aws/karpenter-provider-aws/test/pkg/debug"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	deprovisioningTypeKey = "testing/deprovisioning-type"
	consolidationValue    = "consolidation"
	emptinessValue        = "emptiness"
	expirationValue       = "expiration"
	noExpirationValue     = "noExpiration"
	driftValue            = "drift"
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
var disableProvisioningLimits = karpv1.Limits{
	corev1.ResourceCPU:    resource.MustParse("0"),
	corev1.ResourceMemory: resource.MustParse("0Gi"),
}

//nolint:gocyclo
func registerDeprovisioning() bool {
	Describe("Deprovisioning", Label(debug.NoWatch), Label(debug.NoEvents), func() {
		var nodePool *karpv1.NodePool
		var nodeClass *v1.EC2NodeClass
		var deployment *appsv1.Deployment
		var deploymentOptions test.DeploymentOptions
		var selector labels.Selector
		var dsCount int

		BeforeEach(func() {
			Env.ExpectSettingsOverridden(corev1.EnvVar{Name: "FEATURE_GATES", Value: "Drift=True"})
			nodeClass = Env.DefaultEC2NodeClass()
			nodePool = Env.DefaultNodePool(nodeClass)
			nodePool.Spec.Limits = nil
			test.ReplaceRequirements(nodePool, []karpv1.NodeSelectorRequirementWithMinValues{
				{
					Key:      v1.LabelInstanceHypervisor,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"nitro"},
				},
				// Ensure that all pods can fit on to the provisioned nodes including all daemonsets
				{
					Key:      v1.LabelInstanceSize,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"large"},
				},
			}...)
			nodePool.Spec.Disruption.Budgets = []karpv1.Budget{{
				Nodes: "70%",
			}}
			deploymentOptions = test.DeploymentOptions{
				PodOptions: test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("10m"),
							corev1.ResourceMemory: resource.MustParse("50Mi"),
						},
					},
					TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
				},
			}
			deployment = test.Deployment(deploymentOptions)
			selector = labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)
			dsCount = Env.GetDaemonSetCount(nodePool)
		})

		AfterEach(func() {
			Env.Cleanup()
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
					deploymentOptions.PodOptions.Tolerations = []corev1.Toleration{
						{
							Key:      deprovisioningTypeKey,
							Operator: corev1.TolerationOpEqual,
							Value:    v,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					}
					deploymentOptions.Replicas = int32(replicas)
					d := test.Deployment(deploymentOptions)
					deploymentMap[v] = d
				}

				nodePoolMap := map[string]*karpv1.NodePool{}
				// Generate all the nodePools for multi-deprovisioning
				for _, v := range disruptionMethods {
					np := test.NodePool()
					np.Spec = *nodePool.Spec.DeepCopy()
					np.Spec.Template.Spec.Taints = []corev1.Taint{
						{
							Key:    deprovisioningTypeKey,
							Value:  v,
							Effect: corev1.TaintEffectNoSchedule,
						},
					}
					np.Spec.Template.Labels = map[string]string{
						deprovisioningTypeKey: v,
					}
					nodePoolMap[v] = test.NodePool(*np)
				}

				By("waiting for the deployment to deploy all of its pods")
				var wg sync.WaitGroup
				for _, dep := range deploymentMap {
					wg.Go(func() {
						defer GinkgoRecover()

						Env.ExpectCreated(dep)
						Env.EventuallyExpectPendingPodCount(labels.SelectorFromSet(dep.Spec.Selector.MatchLabels), int(lo.FromPtr(dep.Spec.Replicas)))
					})
				}
				wg.Wait()

				nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
					"maxPods": int32(maxPodDensity), //nolint:gosec
				})
				// Create a separate nodeClass for drift so that we can change the nodeClass later without it affecting
				// the other nodePools
				driftNodeClass := awstest.EC2NodeClass()
				driftNodeClass.Spec = *nodeClass.Spec.DeepCopy()
				nodePoolMap[driftValue].Spec.Template.Spec.NodeClassRef = &karpv1.NodeClassReference{
					Group: object.GVK(driftNodeClass).Group,
					Kind:  object.GVK(driftNodeClass).Kind,
					Name:  driftNodeClass.Name,
				}
				Env.MeasureProvisioningDurationFor(func() {
					By("kicking off provisioning by applying the nodePool and nodeClass")
					Env.ExpectCreated(driftNodeClass, nodeClass)
					for _, p := range nodePoolMap {
						Env.ExpectCreated(p)
					}

					Env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
					Env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)

					// Wait for all pods across all deployments we have created to be in a healthy state
					wg = sync.WaitGroup{}
					for _, dep := range deploymentMap {
						wg.Go(func() {
							defer GinkgoRecover()

							Env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(dep.Spec.Selector.MatchLabels), int(lo.FromPtr(dep.Spec.Replicas)))
						})
					}
					wg.Wait()
				}, map[string]string{
					aws.TestCategoryDimension:           multipleDeprovisionersTestGroup,
					aws.TestNameDimension:               defaultTestName,
					aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
					aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
					aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
				})

				Env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

				By("scaling down replicas across deployments")
				deploymentMap[consolidationValue].Spec.Replicas = lo.ToPtr[int32](int32(int(float64(replicas) * 0.2))) //nolint:gosec
				deploymentMap[emptinessValue].Spec.Replicas = lo.ToPtr[int32](0)
				for _, d := range deploymentMap {
					Env.ExpectUpdated(d)
				}

				// Create a nodePool for expiration so that expiration can do replacement
				nodePoolMap[noExpirationValue] = test.NodePool()
				nodePoolMap[noExpirationValue].Spec = *nodePoolMap[expirationValue].Spec.DeepCopy()

				// Enable consolidation, emptiness, and expiration
				nodePoolMap[consolidationValue].Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
				nodePoolMap[emptinessValue].Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmpty
				nodePoolMap[emptinessValue].Spec.Disruption.ConsolidateAfter.Duration = lo.ToPtr(time.Duration(0))
				nodePoolMap[expirationValue].Spec.Template.Spec.ExpireAfter.Duration = lo.ToPtr(time.Duration(0))
				nodePoolMap[expirationValue].Spec.Limits = disableProvisioningLimits
				// Update the drift NodeClass to start drift on Nodes assigned to this NodeClass
				driftNodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}

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
							karpv1.NodePoolLabelKey: nodePoolMap[noExpirationValue].Name,
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
				Env.MeasureDeprovisioningDurationFor(func() {
					By("enabling deprovisioning across nodePools")
					for _, p := range nodePoolMap {
						p.Spec.Disruption.Budgets = []karpv1.Budget{{
							Nodes: "70%",
						}}
						Env.ExpectCreatedOrUpdated(p)
					}
					Env.ExpectUpdated(driftNodeClass)

					By("waiting for the nodes across all deprovisioners to get deleted")
					wg = sync.WaitGroup{}
					for d, assertions := range assertionMap {
						wg.Go(func() {
							defer GinkgoRecover()

							// Provide a default selector based on the original nodePool name if one isn't specified
							selector = assertions.deletedNodeCountSelector
							if selector == nil {
								selector = labels.SelectorFromSet(map[string]string{karpv1.NodePoolLabelKey: nodePoolMap[d].Name})
							}
							Env.EventuallyExpectDeletedNodeCountWithSelector("==", assertions.deletedCount, selector)

							// Provide a default selector based on the original nodePool name if one isn't specified
							selector = assertions.nodeCountSelector
							if selector == nil {
								selector = labels.SelectorFromSet(map[string]string{karpv1.NodePoolLabelKey: nodePoolMap[d].Name})
							}
							Env.EventuallyExpectNodeCountWithSelector("==", assertions.nodeCount, selector)
							Env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deploymentMap[d].Spec.Selector.MatchLabels), int(lo.FromPtr(deploymentMap[d].Spec.Replicas)))

						})
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
			It("should delete all empty nodes with consolidation", Label("needs:cost-budget"), func(_ context.Context) {
				replicasPerNode := 20
				maxPodDensity := replicasPerNode + dsCount
				expectedNodeCount := 200
				replicas := replicasPerNode * expectedNodeCount

				deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
				nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
					"maxPods": int32(maxPodDensity), //nolint:gosec
				})

				By("waiting for the deployment to deploy all of its pods")
				Env.ExpectCreated(deployment)
				Env.EventuallyExpectPendingPodCount(selector, replicas)

				Env.MeasureProvisioningDurationFor(func() {
					By("kicking off provisioning by applying the nodePool and nodeClass")
					Env.ExpectCreated(nodePool, nodeClass)

					Env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
					Env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
				}, map[string]string{
					aws.TestCategoryDimension:           consolidationTestGroup,
					aws.TestNameDimension:               "empty/delete",
					aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
					aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
					aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
				})

				Env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

				// Delete deployment to make nodes empty
				Env.ExpectDeleted(deployment)
				Env.EventuallyExpectHealthyPodCount(selector, 0)

				Env.MeasureDeprovisioningDurationFor(func() {
					By("kicking off deprovisioning by setting the consolidation enabled value on the nodePool")
					nodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized
					nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
					Env.ExpectUpdated(nodePool)

					Env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectNodeCount("==", 0)
				}, map[string]string{
					aws.TestCategoryDimension:           consolidationTestGroup,
					aws.TestNameDimension:               "empty/delete",
					aws.ProvisionedNodeCountDimension:   strconv.Itoa(0),
					aws.DeprovisionedNodeCountDimension: strconv.Itoa(expectedNodeCount),
					aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
				})
			}, SpecTimeout(time.Minute*30))
			It("should consolidate nodes to get a higher utilization (multi-consolidation delete)", Label("needs:cost-budget"), func(_ context.Context) {
				replicasPerNode := 20
				maxPodDensity := replicasPerNode + dsCount
				expectedNodeCount := 200
				replicas := replicasPerNode * expectedNodeCount

				deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
				nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
					"maxPods": int32(maxPodDensity), //nolint:gosec
				})

				By("waiting for the deployment to deploy all of its pods")
				Env.ExpectCreated(deployment)
				Env.EventuallyExpectPendingPodCount(selector, replicas)

				Env.MeasureProvisioningDurationFor(func() {
					By("kicking off provisioning by applying the nodePool and nodeClass")
					Env.ExpectCreated(nodePool, nodeClass)

					Env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
					Env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
				}, map[string]string{
					aws.TestCategoryDimension:           consolidationTestGroup,
					aws.TestNameDimension:               "delete",
					aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
					aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
					aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
				})

				Env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

				replicas = int(float64(replicas) * 0.2)
				deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas)) //nolint:gosec
				Env.ExpectUpdated(deployment)
				Env.EventuallyExpectHealthyPodCount(selector, replicas)

				Env.MeasureDeprovisioningDurationFor(func() {
					By("kicking off deprovisioning by setting the consolidation enabled value on the nodePool")
					nodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized
					nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
					Env.ExpectUpdated(nodePool)

					Env.EventuallyExpectDeletedNodeCount("==", int(float64(expectedNodeCount)*0.8))
					Env.EventuallyExpectNodeCount("==", int(float64(expectedNodeCount)*0.2))
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
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
				test.ReplaceRequirements(nodePool, karpv1.NodeSelectorRequirementWithMinValues{
					Key:      v1.LabelInstanceSize,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"2xlarge"},
				})

				deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
				// Hostname anti-affinity to require one pod on each node
				deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: deployment.Spec.Selector,
								TopologyKey:   corev1.LabelHostname,
							},
						},
					},
				}

				By("waiting for the deployment to deploy all of its pods")
				Env.ExpectCreated(deployment)
				Env.EventuallyExpectPendingPodCount(selector, replicas)

				Env.MeasureProvisioningDurationFor(func() {
					By("kicking off provisioning by applying the nodePool and nodeClass")
					Env.ExpectCreated(nodePool, nodeClass)

					Env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
					Env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
				}, map[string]string{
					aws.TestCategoryDimension:           consolidationTestGroup,
					aws.TestNameDimension:               "replace",
					aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
					aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
					aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
				})

				Env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

				Env.MeasureDeprovisioningDurationFor(func() {
					By("kicking off deprovisioning by setting the consolidation enabled value on the nodePool")
					// The nodePool defaults to a larger instance type than we need so enabling consolidation and making
					// the requirements wide-open should cause deletes and increase our utilization on the cluster
					nodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmptyOrUnderutilized
					nodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("0s")
					nodePool.Spec.Template.Spec.Requirements = lo.Reject(nodePool.Spec.Template.Spec.Requirements, func(r karpv1.NodeSelectorRequirementWithMinValues, _ int) bool {
						return r.Key == v1.LabelInstanceSize
					})
					Env.ExpectUpdated(nodePool)

					Env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount) // every node should delete due to replacement
					Env.EventuallyExpectNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
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
			It("should deprovision all nodes when empty", Label("needs:cost-budget"), func(_ context.Context) {
				replicasPerNode := 20
				maxPodDensity := replicasPerNode + dsCount
				expectedNodeCount := 200
				replicas := replicasPerNode * expectedNodeCount

				deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
				nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
					"maxPods": int32(maxPodDensity), //nolint:gosec
				})

				By("waiting for the deployment to deploy all of its pods")
				Env.ExpectCreated(deployment)
				Env.EventuallyExpectPendingPodCount(selector, replicas)

				Env.MeasureProvisioningDurationFor(func() {
					By("kicking off provisioning by applying the nodePool and nodeClass")
					Env.ExpectCreated(nodePool, nodeClass)

					Env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
					Env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
				}, map[string]string{
					aws.TestCategoryDimension:           emptinessTestGroup,
					aws.TestNameDimension:               defaultTestName,
					aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
					aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
					aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
				})

				Env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

				By("waiting for all deployment pods to be deleted")
				// Delete deployment to make nodes empty
				Env.ExpectDeleted(deployment)
				Env.EventuallyExpectHealthyPodCount(selector, 0)

				Env.MeasureDeprovisioningDurationFor(func() {
					By("kicking off deprovisioning emptiness by setting the ttlSecondsAfterEmpty value on the nodePool")
					nodePool.Spec.Disruption.ConsolidationPolicy = karpv1.ConsolidationPolicyWhenEmpty
					nodePool.Spec.Disruption.ConsolidateAfter.Duration = lo.ToPtr(time.Duration(0))
					Env.ExpectCreatedOrUpdated(nodePool)

					Env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectNodeCount("==", 0)
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
				nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
					"maxPods": int32(maxPodDensity), //nolint:gosec
				})
				// Enable Expiration
				nodePool.Spec.Template.Spec.ExpireAfter = karpv1.MustParseNillableDuration("5m")

				By("waiting for the deployment to deploy all of its pods")
				Env.ExpectCreated(deployment)
				Env.EventuallyExpectPendingPodCount(selector, replicas)

				Env.MeasureProvisioningDurationFor(func() {
					By("kicking off provisioning by applying the nodePool and nodeClass")
					Env.ExpectCreated(nodePool, nodeClass)

					Env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
					Env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
				}, map[string]string{
					aws.TestCategoryDimension:           expirationTestGroup,
					aws.TestNameDimension:               defaultTestName,
					aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
					aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
					aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
				})

				Env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

				Env.MeasureDeprovisioningDurationFor(func() {
					By("kicking off deprovisioning expiration by setting the ttlSecondsUntilExpired value on the nodePool")
					// Change limits so that replacement nodes will use another nodePool.
					nodePool.Spec.Limits = disableProvisioningLimits

					noExpireNodePool := test.NodePool(*nodePool.DeepCopy())

					// Disable Expiration
					noExpireNodePool.Spec.Disruption.ConsolidateAfter = karpv1.MustParseNillableDuration("Never")
					noExpireNodePool.Spec.Template.Spec.ExpireAfter.Duration = nil

					noExpireNodePool.ObjectMeta = metav1.ObjectMeta{Name: test.RandomName()}
					noExpireNodePool.Spec.Limits = nil
					Env.ExpectCreatedOrUpdated(nodePool, noExpireNodePool)

					Env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
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
				nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
					"maxPods": int32(maxPodDensity), //nolint:gosec
				})

				By("waiting for the deployment to deploy all of its pods")
				Env.ExpectCreated(deployment)
				Env.EventuallyExpectPendingPodCount(selector, replicas)

				Env.MeasureProvisioningDurationFor(func() {
					By("kicking off provisioning by applying the nodePool and nodeClass")
					Env.ExpectCreated(nodePool, nodeClass)

					Env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
					Env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
				}, map[string]string{
					aws.TestCategoryDimension:           driftTestGroup,
					aws.TestNameDimension:               defaultTestName,
					aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
					aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
					aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
				})

				Env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

				Env.MeasureDeprovisioningDurationFor(func() {
					By("kicking off deprovisioning drift by changing the nodeClass AMIFamily")
					nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
					Env.ExpectCreatedOrUpdated(nodeClass)

					Env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
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
			It("should interrupt all nodes due to scheduledChange", Label("needs:cost-budget"), func(_ context.Context) {
				replicasPerNode := 20
				maxPodDensity := replicasPerNode + dsCount
				expectedNodeCount := 200
				replicas := replicasPerNode * expectedNodeCount

				deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
				nodeClass.Spec.Kubelet = awstest.MustMakeKubeletConfiguration(map[string]interface{}{
					"maxPods": int32(maxPodDensity), //nolint:gosec
				})

				By("waiting for the deployment to deploy all of its pods")
				Env.ExpectCreated(deployment)
				Env.EventuallyExpectPendingPodCount(selector, replicas)

				var nodes []*corev1.Node
				Env.MeasureProvisioningDurationFor(func() {
					By("kicking off provisioning by applying the nodePool and nodeClass")
					Env.ExpectCreated(nodePool, nodeClass)
					Env.EventuallyExpectCreatedNodeClaimCount("==", expectedNodeCount)
					Env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
					nodes = Env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
				}, map[string]string{
					aws.TestCategoryDimension:           interruptionTestGroup,
					aws.TestNameDimension:               defaultTestName,
					aws.ProvisionedNodeCountDimension:   strconv.Itoa(expectedNodeCount),
					aws.DeprovisionedNodeCountDimension: strconv.Itoa(0),
					aws.PodDensityDimension:             strconv.Itoa(replicasPerNode),
				})

				Env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

				var msgs []any
				for _, node := range nodes {
					instanceID, err := utils.ParseInstanceID(node.Spec.ProviderID)
					Expect(err).ToNot(HaveOccurred())
					msgs = append(msgs, scheduledChangeMessage(Env.Region, "000000000000", instanceID))
				}

				Env.MeasureDeprovisioningDurationFor(func() {
					By("kicking off deprovisioning by adding scheduledChange messages to the queue")
					Env.ExpectMessagesCreated(msgs...)

					Env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectNodeCount("==", expectedNodeCount)
					Env.EventuallyExpectHealthyPodCount(selector, replicas)
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
	return true
}

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

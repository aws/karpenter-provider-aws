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

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages"
	"github.com/aws/karpenter/pkg/controllers/interruption/messages/scheduledchange"
	awstest "github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils"
	"github.com/aws/karpenter/test/pkg/debug"
	"github.com/aws/karpenter/test/pkg/environment/aws"
)

const (
	deprovisioningTypeKey = v1alpha5.TestingGroup + "/deprovisioning-type"
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

// disableProvisioningLimits represents limits that can be applied to a provisioner if you want a provisioner
// that can deprovision nodes but cannot provision nodes
var disableProvisioningLimits = &v1alpha5.Limits{
	Resources: v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("0"),
		v1.ResourceMemory: resource.MustParse("0Gi"),
	},
}

var _ = Describe("Deprovisioning", Label(debug.NoWatch), Label(debug.NoEvents), func() {
	var provisioner *v1alpha5.Provisioner
	var provisionerOptions test.ProvisionerOptions
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	var deployment *appsv1.Deployment
	var deploymentOptions test.DeploymentOptions
	var selector labels.Selector
	var dsCount int

	BeforeEach(func() {
		env.ExpectSettingsOverridden(map[string]string{
			"featureGates.driftEnabled": "true",
		})
		nodeTemplate = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
		provisionerOptions = test.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{
				Name: nodeTemplate.Name,
			},
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha1.CapacityTypeOnDemand},
				},
				{
					Key:      v1.LabelOSStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{string(v1.Linux)},
				},
				{
					Key:      "karpenter.k8s.aws/instance-hypervisor",
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"nitro"},
				},
			},
			// No limits!!!
			// https://tenor.com/view/chaos-gif-22919457
			Limits: v1.ResourceList{},
		}
		provisioner = test.Provisioner(provisionerOptions)
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
		dsCount = env.GetDaemonSetCount(provisioner)
	})

	AfterEach(func() {
		env.Cleanup()
	})

	Context("Multiple Deprovisioners", func() {
		It("should run consolidation, emptiness, expiration, and drift simultaneously", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			nodeCountPerProvisioner := 10
			replicas := replicasPerNode * nodeCountPerProvisioner

			deprovisioningTypes := []string{
				consolidationValue,
				emptinessValue,
				expirationValue,
				driftValue,
			}
			expectedNodeCount := nodeCountPerProvisioner * len(deprovisioningTypes)

			deploymentMap := map[string]*appsv1.Deployment{}
			// Generate all the deployments for multi-deprovisioning
			for _, v := range deprovisioningTypes {
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

			provisionerMap := map[string]*v1alpha5.Provisioner{}
			// Generate all the provisioners for multi-deprovisioning
			for _, v := range deprovisioningTypes {
				provisionerOptions.Taints = []v1.Taint{
					{
						Key:    deprovisioningTypeKey,
						Value:  v,
						Effect: v1.TaintEffectNoSchedule,
					},
				}
				provisionerOptions.Labels = map[string]string{
					deprovisioningTypeKey: v,
				}
				provisionerOptions.Kubelet = &v1alpha5.KubeletConfiguration{
					MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
				}
				provisionerMap[v] = test.Provisioner(provisionerOptions)
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

			// Create a separate nodeTemplate for drift so that we can change the nodeTemplate later without it affecting
			// the other provisioners
			driftNodeTemplate := nodeTemplate.DeepCopy()
			driftNodeTemplate.Name = test.RandomName()
			provisionerMap[driftValue].Spec.ProviderRef.Name = driftNodeTemplate.Name

			env.MeasureDurationFor(func() {
				By("kicking off provisioning by applying the provisioner and nodeTemplate")
				env.ExpectCreated(driftNodeTemplate, nodeTemplate)
				for _, p := range provisionerMap {
					env.ExpectCreated(p)
				}

				env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
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
			}, aws.ProvisioningEventType, multipleDeprovisionersTestGroup, defaultTestName, aws.GenerateTestDimensions(expectedNodeCount, 0, replicasPerNode))

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			By("scaling down replicas across deployments")
			deploymentMap[consolidationValue].Spec.Replicas = lo.ToPtr[int32](int32(int(float64(replicas) * 0.2)))
			deploymentMap[emptinessValue].Spec.Replicas = lo.ToPtr[int32](0)
			for _, d := range deploymentMap {
				env.ExpectUpdated(d)
			}

			var totalDeletedCount int
			var totalCreatedCount int

			env.MeasureDurationFor(func() {
				By("enabling deprovisioning across provisioners")
				// Create a provisioner for expiration so that expiration can do replacement
				provisionerMap[noExpirationValue] = test.Provisioner()
				provisionerMap[noExpirationValue].Spec = provisionerMap[expirationValue].Spec

				provisionerMap[consolidationValue].Spec.Consolidation = &v1alpha5.Consolidation{Enabled: lo.ToPtr(true)}
				provisionerMap[emptinessValue].Spec.TTLSecondsAfterEmpty = lo.ToPtr[int64](0)
				provisionerMap[expirationValue].Spec.TTLSecondsUntilExpired = lo.ToPtr[int64](0)
				provisionerMap[expirationValue].Spec.Limits = disableProvisioningLimits
				for _, p := range provisionerMap {
					env.ExpectCreatedOrUpdated(p)
				}
				driftNodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				env.ExpectUpdated(driftNodeTemplate)

				By("waiting for the nodes across all deprovisioners to get deleted")
				type testAssertions struct {
					deletedCount             int
					deletedNodeCountSelector labels.Selector
					nodeCount                int
					nodeCountSelector        labels.Selector
					createdCount             int
				}
				assertionMap := map[string]testAssertions{
					consolidationValue: {
						deletedCount: int(float64(nodeCountPerProvisioner) * 0.8),
						nodeCount:    int(float64(nodeCountPerProvisioner) * 0.2),
						createdCount: 0,
					},
					emptinessValue: {
						deletedCount: nodeCountPerProvisioner,
						nodeCount:    0,
						createdCount: 0,
					},
					expirationValue: {
						deletedCount: nodeCountPerProvisioner,
						nodeCount:    nodeCountPerProvisioner,
						nodeCountSelector: labels.SelectorFromSet(map[string]string{
							v1alpha5.ProvisionerNameLabelKey: provisionerMap[noExpirationValue].Name,
						}),
						createdCount: nodeCountPerProvisioner,
					},
					driftValue: {
						deletedCount: nodeCountPerProvisioner,
						nodeCount:    nodeCountPerProvisioner,
						createdCount: nodeCountPerProvisioner,
					},
				}
				totalDeletedCount = lo.Reduce(lo.Values(assertionMap), func(agg int, assertion testAssertions, _ int) int {
					return agg + assertion.deletedCount
				}, 0)
				totalCreatedCount = lo.Reduce(lo.Values(assertionMap), func(agg int, assertion testAssertions, _ int) int {
					return agg + assertion.createdCount
				}, 0)
				wg = sync.WaitGroup{}
				for k, v := range assertionMap {
					wg.Add(1)
					go func(d string, assertions testAssertions) {
						defer GinkgoRecover()
						defer wg.Done()

						env.MeasureDurationFor(func() {
							// Provide a default selector based on the original provisioner name if one isn't specified
							selector = assertions.deletedNodeCountSelector
							if selector == nil {
								selector = labels.SelectorFromSet(map[string]string{v1alpha5.ProvisionerNameLabelKey: provisionerMap[d].Name})
							}
							env.EventuallyExpectDeletedNodeCountWithSelector("==", assertions.deletedCount, selector)

							// Provide a default selector based on the original provisioner name if one isn't specified
							selector = assertions.nodeCountSelector
							if selector == nil {
								selector = labels.SelectorFromSet(map[string]string{v1alpha5.ProvisionerNameLabelKey: provisionerMap[d].Name})
							}
							env.EventuallyExpectNodeCountWithSelector("==", assertions.nodeCount, selector)
							env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deploymentMap[d].Spec.Selector.MatchLabels), int(lo.FromPtr(deploymentMap[d].Spec.Replicas)))
						}, aws.DeprovisioningEventType, multipleDeprovisionersTestGroup, defaultTestName,
							lo.Assign(map[string]string{aws.TestSubEventTypeDimension: d}, aws.GenerateTestDimensions(assertions.createdCount, assertions.deletedCount, replicasPerNode)))
					}(k, v)
				}
				wg.Wait()
			}, aws.DeprovisioningEventType, multipleDeprovisionersTestGroup, defaultTestName, aws.GenerateTestDimensions(totalCreatedCount, totalDeletedCount, replicasPerNode))
		}, SpecTimeout(time.Hour))
	})
	Context("Consolidation", func() {
		It("should delete all empty nodes with consolidation", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 200
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureDurationFor(func() {
				By("kicking off provisioning by applying the provisioner and nodeTemplate")
				env.ExpectCreated(provisioner, nodeTemplate)

				env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.ProvisioningEventType, consolidationTestGroup, "empty/delete", aws.GenerateTestDimensions(expectedNodeCount, 0, replicasPerNode))

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			// Delete deployment to make nodes empty
			env.ExpectDeleted(deployment)
			env.EventuallyExpectHealthyPodCount(selector, 0)

			env.MeasureDurationFor(func() {
				By("kicking off deprovisioning by setting the consolidation enabled value on the provisioner")
				provisioner.Spec.Consolidation = &v1alpha5.Consolidation{Enabled: lo.ToPtr(true)}
				env.ExpectUpdated(provisioner)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", 0)
			}, aws.DeprovisioningEventType, consolidationTestGroup, "empty/delete", aws.GenerateTestDimensions(0, expectedNodeCount, replicasPerNode))
		}, SpecTimeout(time.Minute*30))
		It("should consolidate nodes to get a higher utilization (multi-consolidation delete)", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 200
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureDurationFor(func() {
				By("kicking off provisioning by applying the provisioner and nodeTemplate")
				env.ExpectCreated(provisioner, nodeTemplate)

				env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.ProvisioningEventType, consolidationTestGroup, "delete", aws.GenerateTestDimensions(expectedNodeCount, 0, replicasPerNode))

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			replicas = int(float64(replicas) * 0.2)
			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			env.ExpectUpdated(deployment)
			env.EventuallyExpectHealthyPodCount(selector, replicas)

			env.MeasureDurationFor(func() {
				By("kicking off deprovisioning by setting the consolidation enabled value on the provisioner")
				provisioner.Spec.Consolidation = &v1alpha5.Consolidation{Enabled: lo.ToPtr(true)}
				env.ExpectUpdated(provisioner)

				env.EventuallyExpectDeletedNodeCount("==", int(float64(expectedNodeCount)*0.8))
				env.EventuallyExpectNodeCount("==", int(float64(expectedNodeCount)*0.2))
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.DeprovisioningEventType, consolidationTestGroup, "delete", aws.GenerateTestDimensions(env.Monitor.CreatedNodeCount(), int(float64(expectedNodeCount)*0.8), replicasPerNode))
		}, SpecTimeout(time.Minute*30))
		It("should consolidate nodes to get a higher utilization (single consolidation replace)", func(_ context.Context) {
			replicasPerNode := 1
			expectedNodeCount := 20 // we're currently doing around 1 node/2 mins so this test should run deprovisioning in about 45m
			replicas := replicasPerNode * expectedNodeCount

			// Add in a instance type size requirement that's larger than the smallest that fits the pods.
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements, v1.NodeSelectorRequirement{
				Key:      v1alpha1.LabelInstanceSize,
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

			env.MeasureDurationFor(func() {
				By("kicking off provisioning by applying the provisioner and nodeTemplate")
				env.ExpectCreated(provisioner, nodeTemplate)

				env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.ProvisioningEventType, consolidationTestGroup, "replace", aws.GenerateTestDimensions(expectedNodeCount, 0, replicasPerNode))

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			env.MeasureDurationFor(func() {
				By("kicking off deprovisioning by setting the consolidation enabled value on the provisioner")
				// The provisioner defaults to a larger instance type than we need so enabling consolidation and making
				// the requirements wide-open should cause deletes and increase our utilization on the cluster
				provisioner.Spec.Consolidation = &v1alpha5.Consolidation{Enabled: lo.ToPtr(true)}
				provisioner.Spec.Requirements = lo.Reject(provisioner.Spec.Requirements, func(r v1.NodeSelectorRequirement, _ int) bool {
					return r.Key == v1alpha1.LabelInstanceSize
				})
				env.ExpectUpdated(provisioner)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount) // every node should delete due to replacement
				env.EventuallyExpectNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.DeprovisioningEventType, consolidationTestGroup, "replace", aws.GenerateTestDimensions(env.Monitor.CreatedNodeCount(), expectedNodeCount, replicasPerNode))
		}, SpecTimeout(time.Hour))
	})
	Context("Emptiness", func() {
		It("should deprovision all nodes when empty", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 200
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureDurationFor(func() {
				By("kicking off provisioning by applying the provisioner and nodeTemplate")
				env.ExpectCreated(provisioner, nodeTemplate)

				env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.ProvisioningEventType, emptinessTestGroup, defaultTestName, aws.GenerateTestDimensions(expectedNodeCount, 0, replicasPerNode))

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			By("waiting for all deployment pods to be deleted")
			// Delete deployment to make nodes empty
			env.ExpectDeleted(deployment)
			env.EventuallyExpectHealthyPodCount(selector, 0)

			env.MeasureDurationFor(func() {
				By("kicking off deprovisioning emptiness by setting the ttlSecondsAfterEmpty value on the provisioner")
				provisioner.Spec.TTLSecondsAfterEmpty = lo.ToPtr[int64](0)
				env.ExpectCreatedOrUpdated(provisioner)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", 0)
			}, aws.DeprovisioningEventType, emptinessTestGroup, defaultTestName, aws.GenerateTestDimensions(0, expectedNodeCount, replicasPerNode))
		}, SpecTimeout(time.Minute*30))
	})
	Context("Expiration", func() {
		It("should expire all nodes", func(_ context.Context) {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 20 // we're currently doing around 1 node/2 mins so this test should run deprovisioning in about 45m
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureDurationFor(func() {
				By("kicking off provisioning by applying the provisioner and nodeTemplate")
				env.ExpectCreated(provisioner, nodeTemplate)

				env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.ProvisioningEventType, expirationTestGroup, defaultTestName, aws.GenerateTestDimensions(expectedNodeCount, 0, replicasPerNode))

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			env.MeasureDurationFor(func() {
				By("kicking off deprovisioning expiration by setting the ttlSecondsUntilExpired value on the provisioner")
				// Change Provisioner limits so that replacement nodes will use another provisioner.
				provisioner.Spec.Limits = disableProvisioningLimits
				// Enable Expiration
				provisioner.Spec.TTLSecondsUntilExpired = lo.ToPtr[int64](0)

				noExpireProvisioner := test.Provisioner(provisionerOptions)
				noExpireProvisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
					MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
				}
				env.ExpectCreatedOrUpdated(provisioner, noExpireProvisioner)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.DeprovisioningEventType, expirationTestGroup, defaultTestName, aws.GenerateTestDimensions(expectedNodeCount, expectedNodeCount, replicasPerNode))
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
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			env.MeasureDurationFor(func() {
				By("kicking off provisioning by applying the provisioner and nodeTemplate")
				env.ExpectCreated(provisioner, nodeTemplate)

				env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.ProvisioningEventType, driftTestGroup, defaultTestName, aws.GenerateTestDimensions(expectedNodeCount, 0, replicasPerNode))

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			env.MeasureDurationFor(func() {
				By("kicking off deprovisioning drift by changing the nodeTemplate AMIFamily")
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				env.ExpectCreatedOrUpdated(nodeTemplate)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.DeprovisioningEventType, driftTestGroup, defaultTestName, aws.GenerateTestDimensions(expectedNodeCount, expectedNodeCount, replicasPerNode))
		}, SpecTimeout(time.Hour))
	})
	Context("Interruption", func() {
		It("should interrupt all nodes due to scheduledChange", func(_ context.Context) {
			env.ExpectQueueExists() // Ensure the queue exists before sending messages

			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 200
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			var nodes []*v1.Node
			env.MeasureDurationFor(func() {
				By("kicking off provisioning by applying the provisioner and nodeTemplate")
				env.ExpectCreated(provisioner, nodeTemplate)
				env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
				env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
				nodes = env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.ProvisioningEventType, interruptionTestGroup, defaultTestName, aws.GenerateTestDimensions(expectedNodeCount, 0, replicasPerNode))

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			var msgs []interface{}
			for _, node := range nodes {
				instanceID, err := utils.ParseInstanceID(node.Spec.ProviderID)
				Expect(err).ToNot(HaveOccurred())
				msgs = append(msgs, scheduledChangeMessage(env.Region, "000000000000", instanceID))
			}

			env.MeasureDurationFor(func() {
				By("kicking off deprovisioning by adding scheduledChange messages to the queue")
				env.ExpectMessagesCreated(msgs...)

				env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
				env.EventuallyExpectNodeCount("==", expectedNodeCount)
				env.EventuallyExpectHealthyPodCount(selector, replicas)
			}, aws.DeprovisioningEventType, interruptionTestGroup, defaultTestName, aws.GenerateTestDimensions(expectedNodeCount, expectedNodeCount, replicasPerNode))
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

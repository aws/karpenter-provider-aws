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
)

const deprovisioningTypeKey = v1alpha5.TestingGroup + "/deprovisioning-type"
const (
	consolidationValue = "consolidation"
	emptinessValue     = "emptiness"
	expirationValue    = "expiration"
	driftValue         = "drift"
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
		// Expect the Prometheus client to be up
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
					Key:      v1alpha1.LabelInstanceSize,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"4xlarge"},
				},
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
						v1.ResourceCPU:    resource.MustParse("100m"),
						v1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
				TerminationGracePeriodSeconds: lo.ToPtr[int64](0),
			},
		}
		deployment = test.Deployment(deploymentOptions)
		// Zonal topology spread to avoid exhausting IPs in each subnet
		// TODO @joinnis: Use prefix delegation to avoid IP exhaustion issues with private AZs and ipv4
		deployment.Spec.Template.Spec.TopologySpreadConstraints = []v1.TopologySpreadConstraint{
			{
				LabelSelector:     deployment.Spec.Selector,
				TopologyKey:       v1.LabelTopologyZone,
				MaxSkew:           1,
				WhenUnsatisfiable: v1.DoNotSchedule,
			},
		}
		selector = labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)
		dsCount = env.GetDaemonSetCount(provisioner)
	})

	AfterEach(func() {
		env.Cleanup()
	})

	Context("Multiple Deprovisioners", func() {
		It("should run consolidation, emptiness, expiration, and drift simultaneously", func() {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			nodeCountPerProvisioner := 12 // A multiple of 3 and 4 so that it spreads evenly with 3 or 4 AZs
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
				d.Spec.Template.Spec.TopologySpreadConstraints = []v1.TopologySpreadConstraint{
					{
						LabelSelector:     d.Spec.Selector,
						TopologyKey:       v1.LabelTopologyZone,
						MaxSkew:           1,
						WhenUnsatisfiable: v1.DoNotSchedule,
					},
				}
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

			By("kicking off provisioning by applying the provisioner and nodeTemplate")
			// Create a separate nodeTemplate for drift so that we can change the nodeTemplate later without it affecting
			// the other provisioners
			driftNodeTemplate := nodeTemplate.DeepCopy()
			driftNodeTemplate.Name = test.RandomName()
			provisionerMap[driftValue].Spec.ProviderRef.Name = driftNodeTemplate.Name

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

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			By("scaling down replicas across deployments")
			deploymentMap[consolidationValue].Spec.Replicas = lo.ToPtr[int32](int32(int(float64(replicas) * 0.2)))
			deploymentMap[emptinessValue].Spec.Replicas = lo.ToPtr[int32](0)
			for _, d := range deploymentMap {
				env.ExpectUpdated(d)
			}

			By("enabling deprovisioning across provisioners")
			provisionerMap[consolidationValue].Spec.Consolidation = &v1alpha5.Consolidation{Enabled: lo.ToPtr(true)}
			provisionerMap[emptinessValue].Spec.TTLSecondsAfterEmpty = lo.ToPtr[int64](0)
			provisionerMap[expirationValue].Spec.TTLSecondsUntilExpired = lo.ToPtr[int64](0)
			provisionerMap[expirationValue].Spec.Limits = disableProvisioningLimits
			provisionerMap[driftValue].Spec.Limits = disableProvisioningLimits
			for _, p := range provisionerMap {
				env.ExpectUpdated(p)
			}
			driftNodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
			env.ExpectUpdated(driftNodeTemplate)

			By("waiting for the nodes across all deprovisoiners to get deleted")
			wg = sync.WaitGroup{}
			wg.Add(4)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				env.EventuallyExpectDeletedNodeCountWithSelector("==", int(float64(expectedNodeCount)*0.8), labels.SelectorFromSet(map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisionerMap[consolidationValue].Name,
				}))
				env.EventuallyExpectNodeCountWithSelector("==", int(float64(expectedNodeCount)*0.2), labels.SelectorFromSet(map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisionerMap[consolidationValue].Name,
				}))
				env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deploymentMap[consolidationValue].Labels), int(lo.FromPtr(deploymentMap[consolidationValue].Spec.Replicas)))
			}()
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				env.EventuallyExpectDeletedNodeCountWithSelector("==", expectedNodeCount, labels.SelectorFromSet(map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisionerMap[emptinessValue].Name,
				}))
				env.EventuallyExpectNodeCountWithSelector("==", 0, labels.SelectorFromSet(map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisionerMap[emptinessValue].Name,
				}))
				env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deploymentMap[emptinessValue].Labels), int(lo.FromPtr(deploymentMap[emptinessValue].Spec.Replicas)))
			}()
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				env.EventuallyExpectDeletedNodeCountWithSelector("==", expectedNodeCount, labels.SelectorFromSet(map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisionerMap[expirationValue].Name,
				}))
				env.EventuallyExpectNodeCountWithSelector("==", expectedNodeCount, labels.SelectorFromSet(map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisionerMap[expirationValue].Name,
				}))
				env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deploymentMap[expirationValue].Labels), int(lo.FromPtr(deploymentMap[expirationValue].Spec.Replicas)))
			}()
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				env.EventuallyExpectDeletedNodeCountWithSelector("==", expectedNodeCount, labels.SelectorFromSet(map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisionerMap[driftValue].Name,
				}))
				env.EventuallyExpectNodeCountWithSelector("==", expectedNodeCount, labels.SelectorFromSet(map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisionerMap[driftValue].Name,
				}))
				env.EventuallyExpectHealthyPodCount(labels.SelectorFromSet(deploymentMap[driftValue].Labels), int(lo.FromPtr(deploymentMap[driftValue].Spec.Replicas)))
			}()
			wg.Wait()
		})
	})
	Context("Consolidation", func() {
		It("should consolidate nodes to get a higher utilization (multi-consolidation delete)", func() {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 204 // A multiple of 3 and 4 so that it spreads evenly with 3 or 4 AZs
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			By("kicking off provisioning by applying the provisioner and nodeTemplate")
			env.ExpectCreated(provisioner, nodeTemplate)

			env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
			env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			replicas = int(float64(replicas) * 0.2)
			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			env.ExpectUpdated(deployment)

			By("kicking off deprovisioning by adding enabling consolidation")
			provisioner.Spec.Consolidation = &v1alpha5.Consolidation{Enabled: lo.ToPtr(true)}
			env.ExpectUpdated(provisioner)

			env.EventuallyExpectDeletedNodeCount("==", int(float64(expectedNodeCount)*0.8))
			env.EventuallyExpectNodeCount("==", int(float64(expectedNodeCount)*0.2))
			env.EventuallyExpectHealthyPodCount(selector, replicas)
		})
		It("should consolidate nodes to get a higher utilization (multi-consolidation replace)", func() {
			replicasPerNode := 1
			expectedNodeCount := 30
			replicas := replicasPerNode * expectedNodeCount

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

			By("kicking off provisioning by applying the provisioner and nodeTemplate")
			env.ExpectCreated(provisioner, nodeTemplate)

			env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
			env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			By("kicking off deprovisioning by enabling consolidation")
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
		})
	})
	Context("Emptiness", func() {
		It("should deprovision all nodes when empty", func() {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 204 // A multiple of 3 and 4 so that it spreads evenly with 3 or 4 AZs
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			By("kicking off provisioning by applying the provisioner and nodeTemplate")
			env.ExpectCreated(provisioner, nodeTemplate)

			env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
			env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			By("waiting for all deployment pods to be deleted")
			// Fully scale down all pods to make nodes empty
			deployment.Spec.Replicas = lo.ToPtr[int32](0)
			env.ExpectDeleted(deployment)
			env.EventuallyExpectHealthyPodCount(selector, 0)

			By("kicking off deprovisioning by adding ttlSecondsAfterEmpty")
			provisioner.Spec.TTLSecondsAfterEmpty = lo.ToPtr[int64](0)
			env.ExpectCreatedOrUpdated(provisioner)

			env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectNodeCount("==", 0)
			env.EventuallyExpectHealthyPodCount(selector, replicas)
		})
	})
	Context("Expiration", func() {
		It("should expire all nodes", func() {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 24 // A multiple of 3 and 4 so that it spreads evenly with 3 or 4 AZs
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			By("kicking off provisioning by applying the provisioner and nodeTemplate")
			env.ExpectCreated(provisioner, nodeTemplate)

			env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
			env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			By("kicking off deprovisioning by adding expiration and another provisioner")
			// Change Provisioner limits so that replacement nodes will use another provisioner.
			provisioner.Spec.Limits = disableProvisioningLimits
			// Enable Expiration
			provisioner.Spec.TTLSecondsUntilExpired = lo.ToPtr[int64](0)

			noExpireProvisioner := test.Provisioner(test.ProvisionerOptions{
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceSize,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"4xlarge"},
					},
					{
						Key:      v1alpha5.LabelCapacityType,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{v1alpha1.CapacityTypeOnDemand},
					},
				},
				ProviderRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			})
			env.ExpectCreatedOrUpdated(provisioner, noExpireProvisioner)

			env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)
		})
	})
	Context("Drift", func() {
		It("should drift all nodes", func() {
			// Before Deprovisioning, we need to Provision the cluster to the state that we need.
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 24 // A multiple of 3 and 4 so that it spreads evenly with 3 or 4 AZs
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			By("kicking off provisioning by applying the provisioner and nodeTemplate")
			env.ExpectCreated(provisioner, nodeTemplate)

			env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
			env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			By("kicking off drift by updating the AMIFamily")
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
			env.ExpectCreatedOrUpdated(provisioner)

			env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)
		})
	})
	Context("Interruption", func() {
		It("should interrupt all nodes due to scheduledChange", func() {
			replicasPerNode := 20
			maxPodDensity := replicasPerNode + dsCount
			expectedNodeCount := 204 // A multiple of 3 and 4 so that it spreads evenly with 3 or 4 AZs
			replicas := replicasPerNode * expectedNodeCount

			deployment.Spec.Replicas = lo.ToPtr[int32](int32(replicas))
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				MaxPods: lo.ToPtr[int32](int32(maxPodDensity)),
			}

			By("waiting for the deployment to deploy all of its pods")
			env.ExpectCreated(deployment)
			env.EventuallyExpectPendingPodCount(selector, replicas)

			By("kicking off provisioning by applying the provisioner and nodeTemplate")
			env.ExpectCreated(provisioner, nodeTemplate)

			env.EventuallyExpectCreatedMachineCount("==", expectedNodeCount)
			env.EventuallyExpectCreatedNodeCount("==", expectedNodeCount)
			nodes := env.EventuallyExpectInitializedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)

			env.Monitor.Reset() // Reset the monitor so that we now track the nodes starting at this point in time

			var msgs []scheduledchange.Message
			for _, node := range nodes {
				instanceID, err := utils.ParseInstanceID(node.Spec.ProviderID)
				Expect(err).ToNot(HaveOccurred())
				msgs = append(msgs, scheduledChangeMessage(env.Region, "000000000000", instanceID))
			}

			By("kicking off deprovisioning by creating scheduledChange messages against the SQS queue")
			env.ExpectMessagesCreated(msgs)

			env.EventuallyExpectDeletedNodeCount("==", expectedNodeCount)
			env.EventuallyExpectNodeCount("==", expectedNodeCount)
			env.EventuallyExpectHealthyPodCount(selector, replicas)
		})
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

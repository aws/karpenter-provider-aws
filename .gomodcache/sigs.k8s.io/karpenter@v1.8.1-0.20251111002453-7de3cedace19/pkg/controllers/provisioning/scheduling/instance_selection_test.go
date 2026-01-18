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

package scheduling_test

import (
	"fmt"
	"math/rand"

	"github.com/mitchellh/hashstructure/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	scheduler "sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

var _ = Describe("Instance Type Selection", func() {
	var nodePool *v1.NodePool
	var minPrice float64
	var instanceTypeMap map[string]*cloudprovider.InstanceType
	nodePrice := func(n *corev1.Node) float64 {
		return instanceTypeMap[n.Labels[corev1.LabelInstanceTypeStable]].Offerings.Compatible(scheduler.NewLabelRequirements(n.Labels)).Cheapest().Price
	}

	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Template: v1.NodeClaimTemplate{
					Spec: v1.NodeClaimTemplateSpec{
						Requirements: []v1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      v1.CapacityTypeLabelKey,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{v1.CapacityTypeSpot, v1.CapacityTypeOnDemand},
								},
							},
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      corev1.LabelArchStable,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{v1.ArchitectureArm64, v1.ArchitectureAmd64},
								},
							},
						},
					},
				},
			},
		})
		cloudProvider.InstanceTypes = fake.InstanceTypesAssorted()
		instanceTypeMap = getInstanceTypeMap(cloudProvider.InstanceTypes)
		minPrice = getMinPrice(cloudProvider.InstanceTypes)
		// add some randomness to instance type ordering to ensure we sort everywhere we need to
		rand.Shuffle(len(cloudProvider.InstanceTypes), func(i, j int) {
			cloudProvider.InstanceTypes[i], cloudProvider.InstanceTypes[j] = cloudProvider.InstanceTypes[j], cloudProvider.InstanceTypes[i]
		})
	})

	// This set of tests ensure that we schedule on the cheapest valid instance type while also ensuring that all of the
	// instance types passed to the cloud provider are also valid per nodePool and node selector requirements.  In some
	// ways they repeat some other tests, but the testing regarding checking against all possible instance types
	// passed to the cloud provider is unique.
	It("should schedule on one of the cheapest instances", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
	})
	It("should schedule on one of the cheapest instances (pod arch = amd64)", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      corev1.LabelArchStable,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{v1.ArchitectureAmd64},
			}}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		// ensure that the entire list of instance types match the label
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelArchStable, v1.ArchitectureAmd64)
	})
	It("should schedule on one of the cheapest instances (pod arch = arm64)", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      corev1.LabelArchStable,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{v1.ArchitectureArm64},
			}}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelArchStable, v1.ArchitectureArm64)
	})
	It("should schedule on one of the cheapest instances (prov arch = amd64)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.ArchitectureAmd64},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelArchStable, v1.ArchitectureAmd64)
	})
	It("should schedule on one of the cheapest instances (prov arch = arm64)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.ArchitectureArm64},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelArchStable, v1.ArchitectureArm64)
	})
	It("should schedule on one of the cheapest instances (prov os = windows)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelOSStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{string(corev1.Windows)},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelOSStable, string(corev1.Windows))
	})
	It("should schedule on one of the cheapest instances (pod os = windows)", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      corev1.LabelOSStable,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{string(corev1.Windows)},
			}}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelOSStable, string(corev1.Windows))
	})
	It("should schedule on one of the cheapest instances (prov os = windows)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelOSStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{string(corev1.Windows)},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelOSStable, string(corev1.Windows))
	})
	It("should schedule on one of the cheapest instances (pod os = linux)", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      corev1.LabelOSStable,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{string(corev1.Linux)},
			}}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelOSStable, string(corev1.Linux))
	})
	It("should schedule on one of the cheapest instances (pod os = linux)", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      corev1.LabelOSStable,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{string(corev1.Linux)},
			}}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelOSStable, string(corev1.Linux))
	})
	It("should schedule on one of the cheapest instances (prov zone = test-zone-2)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelTopologyZone,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"test-zone-2"},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelTopologyZone, "test-zone-2")
	})
	It("should schedule on one of the cheapest instances (pod zone = test-zone-2)", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      corev1.LabelTopologyZone,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"test-zone-2"},
			}}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelTopologyZone, "test-zone-2")
	})
	It("should schedule on one of the cheapest instances (prov ct = spot)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.CapacityTypeSpot},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), v1.CapacityTypeLabelKey, v1.CapacityTypeSpot)
	})
	It("should schedule on one of the cheapest instances (pod ct = spot)", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      v1.CapacityTypeLabelKey,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{v1.CapacityTypeSpot},
			}}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), v1.CapacityTypeLabelKey, v1.CapacityTypeSpot)
	})
	It("should schedule on one of the cheapest instances (prov ct = ondemand, prov zone = test-zone-1)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.CapacityTypeOnDemand},
				},
			},
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelTopologyZone,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"test-zone-1"},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(supportedInstanceTypes(cloudProvider.CreateCalls[0]), v1.CapacityTypeOnDemand, "test-zone-1")
	})
	It("should schedule on one of the cheapest instances (pod ct = spot, pod zone = test-zone-1)", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      v1.CapacityTypeLabelKey,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{v1.CapacityTypeSpot},
			},
				{
					Key:      corev1.LabelTopologyZone,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"test-zone-1"},
				},
			}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(supportedInstanceTypes(cloudProvider.CreateCalls[0]), v1.CapacityTypeSpot, "test-zone-1")
	})
	It("should schedule on one of the cheapest instances (prov ct = spot, pod zone = test-zone-2)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.CapacityTypeSpot},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{{
				Key:      corev1.LabelTopologyZone,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{"test-zone-2"},
			}}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(supportedInstanceTypes(cloudProvider.CreateCalls[0]), v1.CapacityTypeSpot, "test-zone-2")
	})
	It("should schedule on one of the cheapest instances (prov ct = ondemand/test-zone-1/arm64/windows)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.ArchitectureArm64},
				},
			},
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelOSStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{string(corev1.Windows)},
				},
			},
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.CapacityTypeOnDemand},
				},
			},
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelTopologyZone,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"test-zone-1"},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(supportedInstanceTypes(cloudProvider.CreateCalls[0]), v1.CapacityTypeOnDemand, "test-zone-1")
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelOSStable, string(corev1.Windows))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelArchStable, "arm64")
	})
	It("should schedule on one of the cheapest instances (prov = spot/test-zone-2, pod = amd64/linux)", func() {
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.ArchitectureAmd64},
				},
			},
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelOSStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{string(corev1.Linux)},
				},
			},
		}
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{
				{
					Key:      v1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.CapacityTypeSpot},
				},
				{
					Key:      corev1.LabelTopologyZone,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"test-zone-2"},
				},
			}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(supportedInstanceTypes(cloudProvider.CreateCalls[0]), v1.CapacityTypeSpot, "test-zone-2")
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelOSStable, string(corev1.Linux))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelArchStable, "amd64")
	})
	It("should schedule on one of the cheapest instances (pod ct = spot/test-zone-2/amd64/linux)", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{
				{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.ArchitectureAmd64},
				},
				{
					Key:      corev1.LabelOSStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{string(corev1.Linux)},
				},
				{
					Key:      v1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.CapacityTypeSpot},
				},
				{
					Key:      corev1.LabelTopologyZone,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"test-zone-2"},
				},
			}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(supportedInstanceTypes(cloudProvider.CreateCalls[0]), v1.CapacityTypeSpot, "test-zone-2")
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelOSStable, string(corev1.Linux))
		ExpectInstancesWithLabel(supportedInstanceTypes(cloudProvider.CreateCalls[0]), corev1.LabelArchStable, "amd64")
	})
	It("should not schedule if no instance type matches selector (pod arch = arm)", func() {
		// remove all Arm instance types
		cloudProvider.InstanceTypes = filterInstanceTypes(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType) bool {
			return i.Requirements.Get(corev1.LabelArchStable).Has(v1.ArchitectureAmd64)
		})

		Expect(len(cloudProvider.InstanceTypes)).To(BeNumerically(">", 0))
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{
				{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.ArchitectureArm64},
				},
			}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
	})
	It("should not schedule if no instance type matches selector (pod arch = arm zone=test-zone-2)", func() {
		// remove all Arm instance types in zone-2
		cloudProvider.InstanceTypes = filterInstanceTypes(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType) bool {
			for _, off := range i.Offerings {
				if off.Requirements.Get(corev1.LabelTopologyZone).Any() == "test-zone-2" {
					return i.Requirements.Get(corev1.LabelArchStable).Has(v1.ArchitectureAmd64)
				}
			}
			return true
		})
		Expect(len(cloudProvider.InstanceTypes)).To(BeNumerically(">", 0))
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{
				{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.ArchitectureArm64},
				},
				{
					Key:      corev1.LabelTopologyZone,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"test-zone-2"},
				},
			}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
	})
	It("should not schedule if no instance type matches selector (prov arch = arm / pod zone=test-zone-2)", func() {
		// remove all Arm instance types in zone-2
		cloudProvider.InstanceTypes = filterInstanceTypes(cloudProvider.InstanceTypes, func(i *cloudprovider.InstanceType) bool {
			for _, off := range i.Offerings {
				if off.Requirements.Get(corev1.LabelTopologyZone).Any() == "test-zone-2" {
					return i.Requirements.Get(corev1.LabelArchStable).Has(v1.ArchitectureAmd64)
				}
			}
			return true
		})

		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      corev1.LabelArchStable,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{v1.ArchitectureArm64},
				},
			},
		}
		Expect(len(cloudProvider.InstanceTypes)).To(BeNumerically(">", 0))
		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{
				{
					Key:      corev1.LabelTopologyZone,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"test-zone-2"},
				},
			}})
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
		Expect(cloudProvider.CreateCalls).To(HaveLen(0))
	})
	It("should schedule on an instance with enough resources", func() {
		// this is a pretty thorough exercise of scheduling, so we also check an invariant that scheduling doesn't
		// modify the instance type's Overhead() or Resources() maps so they can return the same map every time instead
		// of re-alllocating a new one per call
		resourceHashes := map[string]uint64{}
		overheadHashes := map[string]uint64{}
		for _, it := range cloudProvider.InstanceTypes {
			var err error
			resourceHashes[it.Name], err = hashstructure.Hash(it.Capacity, hashstructure.FormatV2, nil)
			Expect(err).To(BeNil())
			overheadHashes[it.Name], err = hashstructure.Hash(it.Overhead.Total(), hashstructure.FormatV2, nil)
			Expect(err).To(BeNil())
		}
		ExpectApplied(ctx, env.Client, nodePool)
		// these values are constructed so that three of these pods can always fit on at least one of our instance types
		for _, cpu := range []float64{0.1, 1.0, 2, 2.5, 4, 8, 16} {
			for _, mem := range []float64{0.1, 1.0, 2, 4, 8, 16, 32} {
				cluster.Reset()
				cloudProvider.CreateCalls = nil
				opts := test.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%0.1f", cpu)),
						corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%0.1fGi", mem)),
					}}}
				pods := []*corev1.Pod{
					test.UnschedulablePod(opts), test.UnschedulablePod(opts), test.UnschedulablePod(opts),
				}
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
				nodeNames := sets.NewString()
				for _, p := range pods {
					node := ExpectScheduled(ctx, env.Client, p)
					nodeNames.Insert(node.Name)
				}
				// should fit on one node
				Expect(nodeNames).To(HaveLen(1))
				totalPodResources := resources.RequestsForPods(pods...)
				for _, it := range supportedInstanceTypes(cloudProvider.CreateCalls[0]) {
					totalReserved := resources.Merge(totalPodResources, it.Overhead.Total())
					// the total pod resources in CPU and memory + instance overhead should always be less than the
					// resources available on every viable instance has
					Expect(totalReserved.Cpu().Cmp(it.Capacity[corev1.ResourceCPU])).To(Equal(-1))
					Expect(totalReserved.Memory().Cmp(it.Capacity[corev1.ResourceMemory])).To(Equal(-1))
				}
			}
		}
		for _, it := range cloudProvider.InstanceTypes {
			resourceHash, err := hashstructure.Hash(it.Capacity, hashstructure.FormatV2, nil)
			Expect(err).To(BeNil())
			overheadHash, err := hashstructure.Hash(it.Overhead.Total(), hashstructure.FormatV2, nil)
			Expect(err).To(BeNil())
			Expect(resourceHash).To(Equal(resourceHashes[it.Name]), fmt.Sprintf("expected %s Resources() to not be modified by scheduling", it.Name))
			Expect(overheadHash).To(Equal(overheadHashes[it.Name]), fmt.Sprintf("expected %s Overhead() to not be modified by scheduling", it.Name))
		}
	})
	It("should schedule on cheaper on-demand instance even when spot price ordering would place other instance types first", func() {
		cloudProvider.InstanceTypes = []*cloudprovider.InstanceType{
			fake.NewInstanceType(fake.InstanceTypeOptions{
				Name:             "test-instance1",
				Architecture:     "amd64",
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Offerings: []*cloudprovider.Offering{
					{
						Available:    true,
						Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        1.0,
					},
					{
						Available:    true,
						Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        0.2,
					},
				},
			}),
			fake.NewInstanceType(fake.InstanceTypeOptions{
				Name:             "test-instance2",
				Architecture:     "amd64",
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Offerings: []*cloudprovider.Offering{
					{
						Available:    true,
						Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeOnDemand, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        1.3,
					},
					{
						Available:    true,
						Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1a"}),
						Price:        0.1,
					},
				},
			}),
		}
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{
				NodeSelectorRequirement: corev1.NodeSelectorRequirement{
					Key:      v1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"on-demand"},
				},
			},
		}

		ExpectApplied(ctx, env.Client, nodePool)
		pod := test.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[corev1.LabelInstanceTypeStable]).To(Equal("test-instance1"))
	})
	Context("MinValues", func() {
		It("should schedule respecting the minValues from instance-type requirements", func() {
			var instanceTypes []*cloudprovider.InstanceType
			// Create fake InstanceTypeOptions where one instances can fit 2 pods and another one can fit only 1 pod.
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.0,
				},
			}
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1))
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts2))
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on instance-type requirement.
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"instance-type-1", "instance-type-2"},
					},
					MinValues: lo.ToPtr(2),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// 2 pods are created with resources such that both fit together only in one of the 2 InstanceTypes created above.
			// Both of these should schedule on a instance-type-2 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)

			// This ensures that the pods are scheduled in 2 different nodes.
			Expect(node1.Name).ToNot(Equal(node2.Name))

			// Ensures that NodeClaims are created with 2 instanceTypes
			Expect(len(supportedInstanceTypes(cloudProvider.CreateCalls[0]))).To(BeNumerically(">=", 2))
		})
		It("should schedule respecting the minValues in Gt operator", func() {
			// custom key that will help us with numerical values to be used for Gt operator
			instanceGeneration := "karpenter/numerical-value"
			var instanceTypes []*cloudprovider.InstanceType
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.0,
				},
			}
			opts3 := fake.InstanceTypeOptions{
				Name:             "instance-type-3",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts3.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.2,
				},
			}

			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts1, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "2")))
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts2, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "3")))
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts3, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "4")))
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on instance generation using Gt operator in requirement.
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      instanceGeneration,
						Operator: corev1.NodeSelectorOpGt,
						Values:   []string{"2"},
					},
					MinValues: lo.ToPtr(2),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// 2 pods are created with resources such that both fit together only in one of the 2 InstanceTypes created above.
			// Both of these should schedule on a instance-type-3 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)

			// This ensures that the pods are scheduled in 2 different nodes.
			Expect(node1.Name).ToNot(Equal(node2.Name))

			// Ensures that NodeClaims are created with 2 instanceTypes
			Expect(len(supportedInstanceTypes(cloudProvider.CreateCalls[0]))).To(BeNumerically(">=", 2))
		})
		It("scheduler should fail if the minValues in Gt operator is not satisfied", func() {
			// custom key that will help us with numerical values to be used for Gt operator
			instanceGeneration := "karpenter/numerical-value"
			var instanceTypes []*cloudprovider.InstanceType
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.0,
				},
			}
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts1, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "2")))
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts2, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "3")))
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on instance generation using Gt operator in requirement.
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      instanceGeneration,
						Operator: corev1.NodeSelectorOpExists,
					},
					MinValues: lo.ToPtr(2),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// Both of these should schedule on a instance-type-2 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      instanceGeneration,
						Operator: corev1.NodeSelectorOpGt,
						Values:   []string{"2"},
					},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
				NodeRequirements: []corev1.NodeSelectorRequirement{
					{
						Key:      instanceGeneration,
						Operator: corev1.NodeSelectorOpGt,
						Values:   []string{"2"},
					},
				},
			})

			ExpectApplied(ctx, env.Client, pod1)
			ExpectApplied(ctx, env.Client, pod2)
			results, _ := prov.Schedule(ctx)
			for _, v := range results.PodErrors {
				Expect(v.Error()).To(ContainSubstring(`minValues requirement is not met for label(s) (label(s)=[karpenter/numerical-value])`))
			}
			ExpectNotScheduled(ctx, env.Client, pod1)
			ExpectNotScheduled(ctx, env.Client, pod2)
		})
		It("should schedule respecting the minValues in Lt operator", func() {
			// custom key that will help us with numerical values to be used for Lt operator
			instanceGeneration := "karpenter/numerical-value"
			var instanceTypes []*cloudprovider.InstanceType
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.0,
				},
			}
			opts3 := fake.InstanceTypeOptions{
				Name:             "instance-type-3",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts3.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.2,
				},
			}
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts1, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "2")))
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts2, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "3")))
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts3, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "4")))
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on instance generation using Lt operator in requirement.
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      instanceGeneration,
						Operator: corev1.NodeSelectorOpLt,
						Values:   []string{"4"},
					},
					MinValues: lo.ToPtr(2),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// 2 pods are created with resources such that both fit together only in one of the 2 InstanceTypes created above.
			// Both of these should schedule on a instance-type-2 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)

			// This ensures that the pods are scheduled in 2 different nodes.
			Expect(node1.Name).ToNot(Equal(node2.Name))

			// Ensures that NodeClaims are created with 2 instanceTypes
			Expect(len(supportedInstanceTypes(cloudProvider.CreateCalls[0]))).To(BeNumerically(">=", 2))
		})
		It("scheduler should fail if the minValues in Lt operator is not satisfied", func() {
			// custom key that will help us with numerical values to be used for Lt operator
			instanceGeneration := "karpenter/numerical-value"
			var instanceTypes []*cloudprovider.InstanceType
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.2,
				},
			}
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts1, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "2")))
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts2, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "4")))
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on instance generation using Lt operator in requirement.
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      instanceGeneration,
						Operator: corev1.NodeSelectorOpLt,
						Values:   []string{"4"},
					},
					MinValues: lo.ToPtr(2),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// Both of these should schedule on a instance-type-1 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			ExpectNotScheduled(ctx, env.Client, pod1)
			ExpectNotScheduled(ctx, env.Client, pod2)
		})
		It("should schedule considering the max of the minValues of In and NotIn operators in the instance-type requirements", func() {
			var instanceTypes []*cloudprovider.InstanceType
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.0,
				},
			}
			opts3 := fake.InstanceTypeOptions{
				Name:             "instance-type-3",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts3.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.2,
				},
			}
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1))
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts2))
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts3))
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on both In and NotIn operators for instance-type requirement.
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"instance-type-1", "instance-type-2", "instance-type-3"},
					},
					MinValues: lo.ToPtr(1),
				},
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{"instance-type-3"},
					},
					MinValues: lo.ToPtr(2),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// Both of these should schedule on a instance-type-2 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)

			// This ensures that the pods are scheduled in 2 different nodes.
			Expect(node1.Name).ToNot(Equal(node2.Name))

			// Ensures that NodeClaims are created with 2 instanceTypes which is the max of both the minValues of operators.
			Expect(len(supportedInstanceTypes(cloudProvider.CreateCalls[0]))).To(BeNumerically(">=", 2))
		})
		It("should schedule considering the max of the minValues of Gt and Lt operators.", func() {
			// custom key that will help us with numerical values to be used for Gt operator
			instanceGeneration := "karpenter/numerical-value"
			var instanceTypes []*cloudprovider.InstanceType
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.0,
				},
			}
			opts3 := fake.InstanceTypeOptions{
				Name:             "instance-type-3",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts3.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.2,
				},
			}
			opts4 := fake.InstanceTypeOptions{
				Name:             "instance-type-4",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts4.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.2,
				},
			}
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts1, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "2")))
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts2, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "3")))
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts3, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "4")))
			instanceTypes = append(instanceTypes, fake.NewInstanceTypeWithCustomRequirement(opts3, scheduler.NewRequirement(instanceGeneration, corev1.NodeSelectorOpIn, "5")))
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on instance generation using Gt operator in requirement.
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      instanceGeneration,
						Operator: corev1.NodeSelectorOpGt,
						Values:   []string{"2"},
					},
					MinValues: lo.ToPtr(1),
				},
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      instanceGeneration,
						Operator: corev1.NodeSelectorOpLt,
						Values:   []string{"5"},
					},
					MinValues: lo.ToPtr(2),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// Both of these should schedule on a instance-type-3 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)

			// This ensures that the pods are scheduled in 2 different nodes.
			Expect(node1.Name).ToNot(Equal(node2.Name))

			// Ensures that NodeClaims are created with 2 instanceTypes
			Expect(len(supportedInstanceTypes(cloudProvider.CreateCalls[0]))).To(BeNumerically(">=", 2))
		})
		It("schedule should fail if minimum number of InstanceTypes is not met as per the minValues in the requirement", func() {
			// Construct InstanceTypeOptions
			var instanceTypes []*cloudprovider.InstanceType
			for i, instanceType := range cloudProvider.InstanceTypes {
				if i < 10 {
					instanceTypes = append(instanceTypes, instanceType)
				}
			}
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on instance-type requirement such that it is more than
			// the number of instanceTypes that the scheduler has from the requirement.
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpExists,
					},
					MinValues: lo.ToPtr(11),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)
			pod := test.UnschedulablePod()

			// Pods are not scheduled since the requirements are not met
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("schedule should fail if minimum number of InstanceTypes is not met as per the minValues in the requirement after truncation", func() {
			var instanceTypes []*cloudprovider.InstanceType
			// Create fake InstanceTypeOptions where one instances can fit 2 pods and another one can fit only 1 pod.
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.0,
				},
			}
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1))
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts2))
			// We have the required InstanceTypes that meet the minValues requirement.
			cloudProvider.InstanceTypes = instanceTypes
			// The truncation is changed from the default to 1 for the ease of testing.
			// This will truncate the 2 InstanceTypes to 1 resulting in breaking the minValue requirement and hence should fail the scheduling.
			scheduling.MaxInstanceTypes = 1

			// Define NodePool that has minValues on instance-type requirement.
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"instance-type-1", "instance-type-2"},
					},
					MinValues: lo.ToPtr(2),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// 2 pods are created with resources such that both fit together only in one of the 2 InstanceTypes created above.
			// Both of these should schedule on a instance-type-2 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			// Pods are not scheduled since the requirements are not met
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			ExpectNotScheduled(ctx, env.Client, pod1)
			ExpectNotScheduled(ctx, env.Client, pod2)
		})
		It("should schedule and pick the max of minValues of InstanceTypes if multiple operators are used for the same requirement.", func() {
			// Create fake InstanceTypeOptions where one instances can fit 2 pods and another one can fit only 1 pod.
			var instanceTypes []*cloudprovider.InstanceType
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.0,
				},
			}
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1))
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts2))
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on instance-type requirement with multiple operators
			// like "In", "Exists"
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpExists,
					},
					MinValues: lo.ToPtr(1),
				},
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"instance-type-1", "instance-type-2"},
					},
					MinValues: lo.ToPtr(2),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// 2 pods are created with resources such that both fit together only in one of the 2 InstanceTypes created above.
			// Both of these should schedule on a instance-type-2 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)

			// This ensures that the pods are scheduled in 2 different nodes.
			Expect(node1.Name).ToNot(Equal(node2.Name))

			// Ensures that NodeClaims are created with 2 instanceTypes
			Expect(len(supportedInstanceTypes(cloudProvider.CreateCalls[0]))).To(BeNumerically(">=", 2))
		})
		It("should schedule and respect multiple requirement keys with minValues", func() {
			// Create fake InstanceTypeOptions where one instances can fit 2 pods and another one can fit only 1 pod.
			var instanceTypes []*cloudprovider.InstanceType
			opts1 := fake.InstanceTypeOptions{
				Name:             "instance-type-1",
				Architecture:     v1.ArchitectureArm64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}
			opts1.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        0.52,
				},
			}
			opts2 := fake.InstanceTypeOptions{
				Name:             "instance-type-2",
				Architecture:     v1.ArchitectureAmd64,
				OperatingSystems: sets.New(string(corev1.Linux)),
				Resources: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("4"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}
			opts2.Offerings = []*cloudprovider.Offering{
				{
					Available:    true,
					Requirements: scheduler.NewLabelRequirements(map[string]string{v1.CapacityTypeLabelKey: v1.CapacityTypeSpot, corev1.LabelTopologyZone: "test-zone-1-spot"}),
					Price:        1.0,
				},
			}
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts1))
			instanceTypes = append(instanceTypes, fake.NewInstanceType(opts2))
			cloudProvider.InstanceTypes = instanceTypes

			// Define NodePool that has minValues on multiple requirements
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelArchStable,
						Operator: corev1.NodeSelectorOpExists,
					},
					MinValues: lo.ToPtr(2),
				},
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      corev1.LabelInstanceTypeStable,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"instance-type-1", "instance-type-2"},
					},
					MinValues: lo.ToPtr(1),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool)

			// 2 pods are created with resources such that both fit together only in one of the 2 InstanceTypes created above.
			// Both of these should schedule on a instance-type-2 without the minValues requirement being specified.
			pod1 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})
			pod2 := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: corev1.ResourceRequirements{Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0.9"),
					corev1.ResourceMemory: resource.MustParse("0.9Gi")},
				},
			})

			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)

			// This ensures that the pods are scheduled in 2 different nodes.
			Expect(node1.Name).ToNot(Equal(node2.Name))

			// Ensures that NodeClaims are created with 2 instanceTypes
			Expect(len(supportedInstanceTypes(cloudProvider.CreateCalls[0]))).To(BeNumerically(">=", 2))
		})
	})
})

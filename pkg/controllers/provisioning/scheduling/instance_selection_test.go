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

package scheduling_test

import (
	"fmt"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/utils/resources"
	"github.com/mitchellh/hashstructure/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"math"
	"math/rand"
	"regexp"
)

var _ = Describe("Instance Type Selection", func() {
	var minPrice float64
	var instanceTypePrices map[string]float64
	nodePrice := func(n *v1.Node) float64 {
		return instanceTypePrices[n.Labels[v1.LabelInstanceTypeStable]]
	}

	BeforeEach(func() {
		// open up the provisioner to any instance types
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64, v1alpha5.ArchitectureAmd64},
			},
		}
		cloudProv.CreateCalls = nil
		cloudProv.InstanceTypes = fake.InstanceTypesAssorted()
		minPrice = math.MaxFloat64
		instanceTypePrices = map[string]float64{}
		for _, it := range cloudProv.InstanceTypes {
			instanceTypePrices[it.Name()] = it.Price()
			minPrice = math.Min(it.Price(), minPrice)
		}

		// add some randomness to instance type ordering to ensure we sort everywhere we need to
		rand.Shuffle(len(cloudProv.InstanceTypes), func(i, j int) {
			cloudProv.InstanceTypes[i], cloudProv.InstanceTypes[j] = cloudProv.InstanceTypes[j], cloudProv.InstanceTypes[i]
		})
	})

	// This set of tests ensure that we schedule on the cheapest valid instance type while also ensuring that all of the
	// instance types passed to the cloud provider are also valid per provisioner and node selector requirements.  In some
	// ways they repeat some other tests, but the testing regarding checking against all possible instance types
	// passed to the cloud provider is unique.
	It("should schedule on one of the cheapest instances", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
	})
	It("should schedule on one of the cheapest instances (pod arch = amd64)", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		// ensure that the entire list of instance types match the label
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelArchStable, v1alpha5.ArchitectureAmd64)
	})
	It("should schedule on one of the cheapest instances (pod arch = arm64)", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelArchStable, v1alpha5.ArchitectureArm64)
	})
	It("should schedule on one of the cheapest instances (prov arch = amd64)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelArchStable, v1alpha5.ArchitectureAmd64)
	})
	It("should schedule on one of the cheapest instances (prov arch = arm64)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelArchStable, v1alpha5.ArchitectureArm64)
	})
	It("should schedule on one of the cheapest instances (prov os = windows)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"windows"},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelOSStable, "windows")
	})
	It("should schedule on one of the cheapest instances (pod os = windows)", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"windows"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelOSStable, "windows")
	})
	It("should schedule on one of the cheapest instances (prov os = windows)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"windows"},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelOSStable, "windows")
	})
	It("should schedule on one of the cheapest instances (pod os = linux)", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"linux"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelOSStable, "linux")
	})
	It("should schedule on one of the cheapest instances (pod os = linux)", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"linux"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelOSStable, "linux")
	})
	It("should schedule on one of the cheapest instances (prov zone = test-zone-2)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-2"},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelTopologyZone, "test-zone-2")
	})
	It("should schedule on one of the cheapest instances (pod zone = test-zone-2)", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-2"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelTopologyZone, "test-zone-2")
	})
	It("should schedule on one of the cheapest instances (prov ct = spot)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha1.CapacityTypeSpot},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1alpha5.LabelCapacityType, v1alpha1.CapacityTypeSpot)
	})
	It("should schedule on one of the cheapest instances (pod ct = spot)", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha1.CapacityTypeSpot},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1alpha5.LabelCapacityType, v1alpha1.CapacityTypeSpot)
	})
	It("should schedule on one of the cheapest instances (prov ct = ondemand, prov zone = test-zone-1)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha1.CapacityTypeOnDemand},
			},
			{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-1"},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypeOptions, v1alpha1.CapacityTypeOnDemand, "test-zone-1")
	})
	It("should schedule on one of the cheapest instances (pod ct = spot, pod zone = test-zone-1)", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha1.CapacityTypeSpot},
			},
				{
					Key:      v1.LabelTopologyZone,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"test-zone-1"},
				},
			}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypeOptions, v1alpha1.CapacityTypeSpot, "test-zone-1")
	})
	It("should schedule on one of the cheapest instances (prov ct = spot, pod zone = test-zone-2)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha1.CapacityTypeSpot},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-2"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypeOptions, v1alpha1.CapacityTypeSpot, "test-zone-2")
	})
	It("should schedule on one of the cheapest instances (prov ct = ondemand/test-zone-1/arm64/windows)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64},
			},
			{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"windows"},
			},
			{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha1.CapacityTypeOnDemand},
			},
			{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-1"},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypeOptions, v1alpha1.CapacityTypeOnDemand, "test-zone-1")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelOSStable, "windows")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelArchStable, "arm64")
	})
	It("should schedule on one of the cheapest instances (prov = spot/test-zone-2, pod = amd64/linux)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			},
			{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"linux"},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha1.CapacityTypeSpot},
				},
				{
					Key:      v1.LabelTopologyZone,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"test-zone-2"},
				},
			}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypeOptions, v1alpha1.CapacityTypeSpot, "test-zone-2")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelOSStable, "linux")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelArchStable, "amd64")
	})
	It("should schedule on one of the cheapest instances (pod ct = spot/test-zone-2/amd64/linux)", func() {
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelArchStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.ArchitectureAmd64},
				},
				{
					Key:      v1.LabelOSStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"linux"},
				},
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha1.CapacityTypeSpot},
				},
				{
					Key:      v1.LabelTopologyZone,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"test-zone-2"},
				},
			}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypeOptions, v1alpha1.CapacityTypeSpot, "test-zone-2")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelOSStable, "linux")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypeOptions, v1.LabelArchStable, "amd64")
	})
	It("should not schedule if no instance type matches selector (pod arch = arm)", func() {
		// remove all Arm instance types
		cloudProv.InstanceTypes = filterInstanceTypes(cloudProv.InstanceTypes, func(i cloudprovider.InstanceType) bool {
			return i.Architecture() == v1alpha5.ArchitectureAmd64
		})

		Expect(len(cloudProv.InstanceTypes)).To(BeNumerically(">", 0))
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelArchStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.ArchitectureArm64},
				},
			}}))
		ExpectNotScheduled(ctx, env.Client, pod[0])
		Expect(cloudProv.CreateCalls).To(HaveLen(0))
	})
	It("should not schedule if no instance type matches selector (pod arch = arm zone=test-zone-2)", func() {
		// remove all Arm instance types in zone-2
		cloudProv.InstanceTypes = filterInstanceTypes(cloudProv.InstanceTypes, func(i cloudprovider.InstanceType) bool {
			for _, off := range i.Offerings() {
				if off.Zone == "test-zone-2" {
					return i.Architecture() == v1alpha5.ArchitectureAmd64
				}
			}
			return true
		})
		Expect(len(cloudProv.InstanceTypes)).To(BeNumerically(">", 0))
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelArchStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.ArchitectureArm64},
				},
				{
					Key:      v1.LabelTopologyZone,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"test-zone-2"},
				},
			}}))
		ExpectNotScheduled(ctx, env.Client, pod[0])
		Expect(cloudProv.CreateCalls).To(HaveLen(0))
	})
	It("should not schedule if no instance type matches selector (prov arch = arm / pod zone=test-zone-2)", func() {
		// remove all Arm instance types in zone-2
		cloudProv.InstanceTypes = filterInstanceTypes(cloudProv.InstanceTypes, func(i cloudprovider.InstanceType) bool {
			for _, off := range i.Offerings() {
				if off.Zone == "test-zone-2" {
					return i.Architecture() == v1alpha5.ArchitectureAmd64
				}
			}
			return true
		})

		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64},
			},
		}
		Expect(len(cloudProv.InstanceTypes)).To(BeNumerically(">", 0))
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1.LabelTopologyZone,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"test-zone-2"},
				},
			}}))
		ExpectNotScheduled(ctx, env.Client, pod[0])
		Expect(cloudProv.CreateCalls).To(HaveLen(0))
	})
	It("should schedule on an instance with enough resources", func() {
		// this is a pretty thorough exercise of scheduling, so we also check an invariant that scheduling doesn't
		// modify the instance type's Overhead() or Resources() maps so they can return the same map every time instead
		// of re-alllocating a new one per call
		resourceHashes := map[string]uint64{}
		overheadHashes := map[string]uint64{}
		for _, it := range cloudProv.InstanceTypes {
			var err error
			resourceHashes[it.Name()], err = hashstructure.Hash(it.Resources(), hashstructure.FormatV2, nil)
			Expect(err).To(BeNil())
			overheadHashes[it.Name()], err = hashstructure.Hash(it.Overhead(), hashstructure.FormatV2, nil)
		}
		ExpectApplied(ctx, env.Client, provisioner)
		// these values are constructed so that three of these pods can always fit on at least one of our instance types
		for _, cpu := range []float64{0.1, 1.0, 2, 2.5, 4, 8, 16} {
			for _, mem := range []float64{0.1, 1.0, 2, 4, 8, 16, 32} {
				cloudProv.CreateCalls = nil
				opts := test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%0.1f", cpu)),
						v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%0.1fGi", mem)),
					}}}
				pods := ExpectProvisioned(ctx, env.Client, controller,
					test.UnschedulablePod(opts), test.UnschedulablePod(opts), test.UnschedulablePod(opts))
				nodeNames := sets.NewString()
				for _, p := range pods {
					node := ExpectScheduled(ctx, env.Client, p)
					nodeNames.Insert(node.Name)
				}
				// should fit on one node
				Expect(nodeNames).To(HaveLen(1))
				totalPodResources := resources.RequestsForPods(pods...)
				for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
					totalReserved := resources.Merge(totalPodResources, it.Overhead())
					// the total pod resources in CPU and memory + instance overhead should always be less than the
					// resources available on every viable instance has
					Expect(totalReserved.Cpu().Cmp(it.Resources()[v1.ResourceCPU])).To(Equal(-1))
					Expect(totalReserved.Memory().Cmp(it.Resources()[v1.ResourceMemory])).To(Equal(-1))
				}
			}
		}
		for _, it := range cloudProv.InstanceTypes {
			resourceHash, err := hashstructure.Hash(it.Resources(), hashstructure.FormatV2, nil)
			Expect(err).To(BeNil())
			overheadHash, err := hashstructure.Hash(it.Overhead(), hashstructure.FormatV2, nil)
			Expect(err).To(BeNil())
			Expect(resourceHash).To(Equal(resourceHashes[it.Name()]), fmt.Sprintf("expected %s Resources() to not be modified by scheduling", it.Name()))
			Expect(overheadHash).To(Equal(overheadHashes[it.Name()]), fmt.Sprintf("expected %s Overhead() to not be modified by scheduling", it.Name()))
		}
	})
})

var _ = Describe("Instance Type Filtering", func() {
	BeforeEach(func() {
		// open up the provisioner to any instance types
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64, v1alpha5.ArchitectureAmd64},
			},
		}
		cloudProv.CreateCalls = nil
		cloudProv.InstanceTypes = fake.InstanceTypesAssorted()
		// add some randomness to instance type ordering to ensure we sort everywhere we need to
		rand.Shuffle(len(cloudProv.InstanceTypes), func(i, j int) {
			cloudProv.InstanceTypes[i], cloudProv.InstanceTypes[j] = cloudProv.InstanceTypes[j], cloudProv.InstanceTypes[i]
		})
	})
	It("should not filter instance types if no filter is specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		// should provide the list of all instance types
		Expect(len(cloudProv.CreateCalls[0].InstanceTypeOptions)).To(Equal(len(cloudProv.InstanceTypes)))
	})
	It("should filter out instances with cpu less than the minimum specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MinResources: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("16"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			cpu := it.Resources()[v1.ResourceCPU]
			Expect(cpu.AsApproximateFloat64()).To(BeNumerically(">=", 16))
		}
	})
	It("should filter out instances with cpu greater than the maximum specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MaxResources: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("16"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			cpu := it.Resources()[v1.ResourceCPU]
			Expect(cpu.AsApproximateFloat64()).To(BeNumerically("<=", 16))
		}
	})
	It("should filter out instances with cpu not in the range specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MinResources: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("8"),
				},
				MaxResources: v1.ResourceList{
					v1.ResourceCPU: resource.MustParse("16"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			cpu := it.Resources()[v1.ResourceCPU]
			Expect(cpu.AsApproximateFloat64()).To(BeNumerically(">=", 8))
			Expect(cpu.AsApproximateFloat64()).To(BeNumerically("<=", 16))
		}
	})
	It("should filter out instances with memory less than the minimum specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MinResources: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("128Gi"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			mem := it.Resources()[v1.ResourceMemory]
			Expect(mem.AsApproximateFloat64()).To(BeNumerically(">=", 128*1024*1024*1024))
		}
	})
	It("should filter out instances with memory greater than the maximum specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MaxResources: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("32Gi"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			mem := it.Resources()[v1.ResourceMemory]
			Expect(mem.AsApproximateFloat64()).To(BeNumerically("<=", 32*1024*1024*1024))
		}
	})
	It("should filter out instances memory not in the range specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MinResources: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("16Gi"),
				},
				MaxResources: v1.ResourceList{
					v1.ResourceMemory: resource.MustParse("32Gi"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			mem := it.Resources()[v1.ResourceMemory]
			Expect(mem.AsApproximateFloat64()).To(BeNumerically(">=", 16*1024*1024*1024))
			Expect(mem.AsApproximateFloat64()).To(BeNumerically("<=", 32*1024*1024*1024))
		}
	})
	It("should support combined cpu and memory filters", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MinResources: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("8"),
					v1.ResourceMemory: resource.MustParse("16Gi"),
				},
				MaxResources: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("16"),
					v1.ResourceMemory: resource.MustParse("32Gi"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			mem := it.Resources()[v1.ResourceMemory]
			Expect(mem.AsApproximateFloat64()).To(BeNumerically(">=", 16*1024*1024*1024))
			Expect(mem.AsApproximateFloat64()).To(BeNumerically("<=", 32*1024*1024*1024))
			cpu := it.Resources()[v1.ResourceCPU]
			Expect(cpu.AsApproximateFloat64()).To(BeNumerically(">=", 8))
			Expect(cpu.AsApproximateFloat64()).To(BeNumerically("<=", 16))
		}
	})
	It("should filter out instances with less memory per cpu than the minimum specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MemoryPerCPU: &v1alpha5.MinMax{
					Min: presource("8Gi"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		Expect(len(cloudProv.CreateCalls[0].InstanceTypeOptions)).To(BeNumerically(">", 0))
		Expect(len(cloudProv.CreateCalls[0].InstanceTypeOptions)).ToNot(Equal(len(cloudProv.InstanceTypes)))
		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			mem := it.Resources()[v1.ResourceMemory]
			cpu := it.Resources()[v1.ResourceCPU]
			ratio := mem.AsApproximateFloat64() / cpu.AsApproximateFloat64()
			Expect(ratio).To(BeNumerically(">=", 8*1024*1024*1024))
		}
	})
	It("should filter out instances with more memory per cpu than the maximum specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MemoryPerCPU: &v1alpha5.MinMax{
					Max: presource("8Gi"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		Expect(len(cloudProv.CreateCalls[0].InstanceTypeOptions)).To(BeNumerically(">", 0))
		Expect(len(cloudProv.CreateCalls[0].InstanceTypeOptions)).ToNot(Equal(len(cloudProv.InstanceTypes)))
		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			mem := it.Resources()[v1.ResourceMemory]
			cpu := it.Resources()[v1.ResourceCPU]
			memMib := mem.AsApproximateFloat64() / (1024 * 1024)
			ratio := memMib / cpu.AsApproximateFloat64()
			Expect(ratio).To(BeNumerically("<=", 8*1024))
		}
	})
	It("should filter out instances with memory per cpu not in the range specified", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				MemoryPerCPU: &v1alpha5.MinMax{
					Min: presource("8Gi"),
					Max: presource("16Gi"),
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		Expect(len(cloudProv.CreateCalls[0].InstanceTypeOptions)).To(BeNumerically(">", 0))
		Expect(len(cloudProv.CreateCalls[0].InstanceTypeOptions)).ToNot(Equal(len(cloudProv.InstanceTypes)))
		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			mem := it.Resources()[v1.ResourceMemory]
			cpu := it.Resources()[v1.ResourceCPU]
			memMib := mem.AsApproximateFloat64() / (1024 * 1024)
			ratio := memMib / cpu.AsApproximateFloat64()
			Expect(ratio).To(BeNumerically(">=", 8*1024))
			Expect(ratio).To(BeNumerically("<=", 16*1024))
		}
	})
	It("should filter out instances by the NameIncludeExpressions", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				NameIncludeExpressions: []string{
					"li..x",
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		regExp := regexp.MustCompile("li..x")
		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			Expect(regExp.MatchString(it.Name())).To(BeTrue())
		}
	})
	It("should filter out instances by the NameIncludeExpressions, multiple terms are OR'd", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				NameIncludeExpressions: []string{
					"linux",
					"amd64",
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		regExp1 := regexp.MustCompile("linux")
		regExp2 := regexp.MustCompile("amd64")
		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			Expect(regExp1.MatchString(it.Name()) || regExp2.MatchString(it.Name())).To(BeTrue())
		}
	})

	It("should filter out instances by the NameExcludeExpressions", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				NameExcludeExpressions: []string{
					"li..x",
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		regExp := regexp.MustCompile("li..x")
		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			Expect(regExp.MatchString(it.Name())).To(BeFalse())
		}
	})
	It("should filter out instances by the NameExcludeExpressions, multiple terms are OR'd", func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{
			InstanceTypeFilter: &v1alpha5.InstanceTypeFilter{
				NameExcludeExpressions: []string{
					"linux",
					"amd64",
				},
			},
		})
		ExpectApplied(ctx, env.Client, provisioner)
		pod := ExpectProvisioned(ctx, env.Client, controller, test.UnschedulablePod())
		ExpectScheduled(ctx, env.Client, pod[0])

		regExp1 := regexp.MustCompile("linux")
		regExp2 := regexp.MustCompile("amd64")
		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			Expect(regExp1.MatchString(it.Name()) || regExp2.MatchString(it.Name())).To(BeFalse())
		}
	})
})

func presource(s string) *resource.Quantity {
	v := resource.MustParse(s)
	return &v
}

func filterInstanceTypes(types []cloudprovider.InstanceType, pred func(i cloudprovider.InstanceType) bool) []cloudprovider.InstanceType {
	var ret []cloudprovider.InstanceType
	for _, it := range types {
		if pred(it) {
			ret = append(ret, it)
		}
	}
	return ret
}

func ExpectInstancesWithOffering(instanceTypes []cloudprovider.InstanceType, capacityType string, zone string) {
	for _, it := range instanceTypes {
		matched := false
		for _, offering := range it.Offerings() {
			if offering.CapacityType == capacityType && offering.Zone == zone {
				matched = true
			}
		}
		Expect(matched).To(BeTrue(), fmt.Sprintf("expected to find zone %s / capacity type %s in an offering", zone, capacityType))
	}
}

func ExpectInstancesWithLabel(instanceTypes []cloudprovider.InstanceType, label string, value string) {
	for _, it := range instanceTypes {
		switch label {
		case v1.LabelArchStable:
			Expect(it.Architecture()).To(Equal(value))
		case v1.LabelOSStable:
			Expect(it.OperatingSystems().Has(value)).To(BeTrue(), fmt.Sprintf("expected to find an OS of %s", value))
		case v1.LabelTopologyZone:
			{
				matched := false
				for _, offering := range it.Offerings() {
					if offering.Zone == value {
						matched = true
						break
					}
				}
				Expect(matched).To(BeTrue(), fmt.Sprintf("expected to find zone %s in an offering", value))
			}
		case v1alpha5.LabelCapacityType:
			{
				matched := false
				for _, offering := range it.Offerings() {
					if offering.CapacityType == value {
						matched = true
						break
					}
				}
				Expect(matched).To(BeTrue(), fmt.Sprintf("expected to find caapacity type %s in an offering", value))
			}
		default:
			Fail(fmt.Sprintf("unsupported label %s in test", label))
		}
	}
}

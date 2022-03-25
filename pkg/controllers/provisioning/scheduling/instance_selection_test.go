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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"math"
	"math/rand"
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
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
	})
	It("should schedule on one of the cheapest instances (pod arch = amd64)", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		// ensure that the entire list of instance types match the label
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelArchStable, v1alpha5.ArchitectureAmd64)
	})
	It("should schedule on one of the cheapest instances (pod arch = arm64)", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelArchStable, v1alpha5.ArchitectureArm64)
	})
	It("should schedule on one of the cheapest instances (prov arch = amd64)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelArchStable, v1alpha5.ArchitectureAmd64)
	})
	It("should schedule on one of the cheapest instances (prov arch = arm64)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelArchStable, v1alpha5.ArchitectureArm64)
	})
	It("should schedule on one of the cheapest instances (prov os = windows)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"windows"},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelOSStable, "windows")
	})
	It("should schedule on one of the cheapest instances (pod os = windows)", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"windows"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelOSStable, "windows")
	})
	It("should schedule on one of the cheapest instances (prov os = windows)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"windows"},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelOSStable, "windows")
	})
	It("should schedule on one of the cheapest instances (pod os = linux)", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"linux"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelOSStable, "linux")
	})
	It("should schedule on one of the cheapest instances (pod os = linux)", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"linux"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelOSStable, "linux")
	})
	It("should schedule on one of the cheapest instances (prov zone = test-zone-2)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-2"},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelTopologyZone, "test-zone-2")
	})
	It("should schedule on one of the cheapest instances (pod zone = test-zone-2)", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-2"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelTopologyZone, "test-zone-2")
	})
	It("should schedule on one of the cheapest instances (prov ct = spot)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha1.CapacityTypeSpot},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1alpha5.LabelCapacityType, v1alpha1.CapacityTypeSpot)
	})
	It("should schedule on one of the cheapest instances (pod ct = spot)", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha1.CapacityTypeSpot},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1alpha5.LabelCapacityType, v1alpha1.CapacityTypeSpot)
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
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypes, v1alpha1.CapacityTypeOnDemand, "test-zone-1")
	})
	It("should schedule on one of the cheapest instances (pod ct = spot, pod zone = test-zone-1)", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
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
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypes, v1alpha1.CapacityTypeSpot, "test-zone-1")
	})
	It("should schedule on one of the cheapest instances (prov ct = spot, pod zone = test-zone-2)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1alpha5.LabelCapacityType,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha1.CapacityTypeSpot},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-2"},
			}}}))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypes, v1alpha1.CapacityTypeSpot, "test-zone-2")
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
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())
		node := ExpectScheduled(ctx, env.Client, pod[0])
		Expect(nodePrice(node)).To(Equal(minPrice))
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypes, v1alpha1.CapacityTypeOnDemand, "test-zone-1")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelOSStable, "windows")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelArchStable, "arm64")
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
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
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
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypes, v1alpha1.CapacityTypeSpot, "test-zone-2")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelOSStable, "linux")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelArchStable, "amd64")
	})
	It("should schedule on one of the cheapest instances (pod ct = spot/test-zone-2/amd64/linux)", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
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
		ExpectInstancesWithOffering(cloudProv.CreateCalls[0].InstanceTypes, v1alpha1.CapacityTypeSpot, "test-zone-2")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelOSStable, "linux")
		ExpectInstancesWithLabel(cloudProv.CreateCalls[0].InstanceTypes, v1.LabelArchStable, "amd64")
	})
	It("should not schedule if no instance type matches selector (pod arch = arm)", func() {
		// remove all Arm instance types
		cloudProv.InstanceTypes = filterInstanceTypes(cloudProv.InstanceTypes, func(i cloudprovider.InstanceType) bool {
			return i.Architecture() == v1alpha5.ArchitectureAmd64
		})

		Expect(len(cloudProv.InstanceTypes)).To(BeNumerically(">", 0))
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
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
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
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
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
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
		// these values are constructed so that three of these pods can always fit on at least one of our instance types
		for _, cpu := range []float64{0.1, 1.0, 2, 2.5, 4, 8, 16} {
			for _, mem := range []float64{0.1, 1.0, 2, 4, 8, 16, 32} {
				cloudProv.CreateCalls = nil
				opts := test.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%0.1f", cpu)),
						v1.ResourceMemory: resource.MustParse(fmt.Sprintf("%0.1fGi", mem)),
					}}}
				pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(opts), test.UnschedulablePod(opts), test.UnschedulablePod(opts))
				nodeNames := sets.NewString()
				for _, p := range pods {
					node := ExpectScheduled(ctx, env.Client, p)
					nodeNames.Insert(node.Name)
				}
				// should fit on one node
				Expect(nodeNames).To(HaveLen(1))
				totalPodResources := resources.RequestsForPods(pods...)
				for _, it := range cloudProv.CreateCalls[0].InstanceTypes {
					totalReserved := resources.Merge(totalPodResources, it.Overhead())
					// the total pod resources in CPU and memory + instance overhead should always be less than the
					// resources available on every viable instance has
					Expect(totalReserved.Cpu().Cmp(it.Resources()[v1.ResourceCPU])).To(Equal(-1))
					Expect(totalReserved.Memory().Cmp(it.Resources()[v1.ResourceMemory])).To(Equal(-1))
				}
			}
		}
	})
})

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

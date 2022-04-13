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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/provisioning/scheduling"
	"github.com/aws/karpenter/pkg/controllers/selection"
	"github.com/aws/karpenter/pkg/test"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "github.com/aws/karpenter/pkg/test/expectations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context
var provisioner *v1alpha5.Provisioner
var provisioners *provisioning.Controller
var selectionController *selection.Controller
var env *test.Environment
var cloudProv *fake.CloudProvider

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Scheduling")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProv = &fake.CloudProvider{}
		registry.RegisterOrDie(ctx, cloudProv)
		provisioners = provisioning.NewController(ctx, e.Client, corev1.NewForConfigOrDie(e.Config), cloudProv)
		selectionController = selection.NewController(e.Client, provisioners)
	})
	Expect(env.Start()).To(Succeed(), "Failed to start environment")
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	provisioner = &v1alpha5.Provisioner{
		ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
		Spec:       v1alpha5.ProvisionerSpec{},
	}
	// reset instance types
	cloudProv.InstanceTypes = fake.CloudProvider{}.InstanceTypes
	cloudProv.CreateCalls = nil
	provisioner.SetDefaults(ctx)
})

var _ = AfterEach(func() {
	ExpectProvisioningCleanedUp(ctx, env.Client, provisioners)
})

var _ = Describe("Custom Constraints", func() {
	Context("Provisioner with Labels", func() {
		It("should schedule unconstrained pods that don't have matching node selectors", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should not schedule pods that have conflicting node selectors", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeSelector: map[string]string{"test-key": "different-value"}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule pods that have node selectors with undefined key", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeSelector: map[string]string{"test-key": "test-value"}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule pods that have matching requirements", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value", "another-value"}},
				}},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should not schedule pods that have conflicting requirements", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"another-value"}},
				}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule pods that have matching preferences", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodePreferences: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"another-value", "test-value"}},
				}},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should not schedule pods with have conflicting preferences", func() {
			provisioner.Spec.Labels = map[string]string{"test-key": "test-value"}
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodePreferences: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-value"}},
				}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
	})
	Context("Well Known Labels", func() {
		It("should use provisioner constraints", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should use node selectors", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-2"}},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should not schedule nodes with a hostname selector", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeSelector: map[string]string{v1.LabelHostname: "red-node"}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule the pod if nodeselector unknown", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "unknown"}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule if node selector outside of provisioner constraints", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-2"}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule compatible requirements with Operator=In", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-3"}},
				}},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
		})
		It("should not schedule incompatible preferences and requirements with Operator=In", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"unknown"}},
				}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule compatible requirements with Operator=NotIn", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-1", "test-zone-2", "unknown"}},
				}},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
		})
		It("should not schedule incompatible preferences and requirements with Operator=NotIn", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}},
					}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule compatible preferences and requirements with Operator=In", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2", "unknown"}}},
				},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should not schedule incompatible preferences and requirements with Operator=In", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"unknown"}}},
				},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule compatible preferences and requirements with Operator=NotIn", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-1", "test-zone-3"}}},
				},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should not schedule incompatible preferences and requirements with Operator=NotIn", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3", "unknown"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}},
				},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule compatible node selectors, preferences and requirements", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-3"},
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}}},
				},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
		})
		It("should not schedule incompatible node selectors, preferences and requirements", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-3"},
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"}}},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-2", "test-zone-3"}}},
				},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should combine multidimensional node selectors, preferences and requirements", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeSelector: map[string]string{
						v1.LabelTopologyZone:       "test-zone-3",
						v1.LabelInstanceTypeStable: "arm-instance-type",
					},
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"}},
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"default-instance-type", "arm-instance-type"}},
					},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"unknown"}},
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpNotIn, Values: []string{"unknown"}},
					},
				},
			))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelInstanceTypeStable, "arm-instance-type"))
		})
		It("should not combine incompatible multidimensional node selectors, preferences and requirements", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeSelector: map[string]string{
						v1.LabelTopologyZone:       "test-zone-3",
						v1.LabelInstanceTypeStable: "arm-instance-type",
					},
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"}},
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"default-instance-type", "arm-instance-type"}},
					},
					NodePreferences: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-3"}},
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpNotIn, Values: []string{"arm-instance-type"}},
					},
				},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
	})
	Context("Constraints Validation", func() {
		It("should not schedule pods that have node selectors with restricted labels", func() {
			for label := range v1alpha5.RestrictedLabels {
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
					test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: label, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
					}}))[0]
				ExpectNotScheduled(ctx, env.Client, pod)
			}
		})
		It("should not schedule pods that have node selectors with restricted domains", func() {
			for domain := range v1alpha5.RestrictedLabelDomains {
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
					test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: domain + "/test", Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
					}}))[0]
				ExpectNotScheduled(ctx, env.Client, pod)
			}
		})
		It("should schedule pods that have node selectors with label in restricted domains exceptions list", func() {
			var requirements []v1.NodeSelectorRequirement
			for domain := range v1alpha5.LabelDomainExceptions {
				requirements = append(requirements, v1.NodeSelectorRequirement{Key: domain + "/test", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}})
			}
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(requirements...)
			for domain := range v1alpha5.LabelDomainExceptions {
				pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
					test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: domain + "/test", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}},
					}}))[0]
				node := ExpectScheduled(ctx, env.Client, pod)
				Expect(node.Labels).ToNot(HaveKeyWithValue(domain+"/test", "test-value"))
			}
		})
		It("should schedule pods that have node selectors with label in restricted label exceptions list", func() {
			schedulable := []*v1.Pod{
				// Constrained by zone
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1"}}),
				// Constrained by instanceType
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "default-instance-type"}}),
				// Constrained by architecture
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelArchStable: "arm64"}}),
				// Constrained by operatingSystem
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1.LabelOSStable: "linux"}}),
				// Constrained by capacity type
				test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.LabelCapacityType: "spot"}}),
			}
			for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, schedulable...) {
				ExpectScheduled(ctx, env.Client, pod)
			}
		})
	})
	Context("Scheduling Logics", func() {
		It("should not schedule pods that have node selectors with In operator and undefined key", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}},
				}}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule pods that have node selectors with NotIn operator and undefined key", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-value"}},
				}}))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).ToNot(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should not schedule pods that have node selectors with Exists operator and undefined key", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpExists},
				}}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule pods that with DoesNotExists operator and undefined key", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpDoesNotExist},
				}}))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).ToNot(HaveKey("test-key"))
		})
		It("should schedule unconstrained pods that don't have matching node selectors", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should schedule pods that have node selectors with matching value and In operator", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}},
				}}))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should not schedule pods that have node selectors with matching value and NotIn operator", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-value"}},
				}}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule the pod with Exists operator and defined key", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpExists},
				}},
			))[0]
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should not schedule the pod with DoesNotExists operator and defined key", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpDoesNotExist},
				}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should not schedule pods that have node selectors with different value and In operator", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"another-value"}},
				}}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should schedule pods that have node selectors with different value and NotIn operator", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}})
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpNotIn, Values: []string{"another-value"}},
				}}))[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue("test-key", "test-value"))
		})
		It("should schedule compatible pods to the same node", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value", "another-value"}})
			pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}},
				}}),
				test.UnschedulablePod(test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpNotIn, Values: []string{"another-value"}},
				}}))
			node1 := ExpectScheduled(ctx, env.Client, pods[0])
			node2 := ExpectScheduled(ctx, env.Client, pods[1])
			Expect(node1.Labels).To(HaveKeyWithValue("test-key", "test-value"))
			Expect(node2.Labels).To(HaveKeyWithValue("test-key", "test-value"))
			Expect(node1.Name).To(Equal(node2.Name))
		})
		It("should schedule incompatible pods to the different node", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value", "another-value"}})
			pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"test-value"}},
				}}),
				test.UnschedulablePod(test.PodOptions{NodeRequirements: []v1.NodeSelectorRequirement{
					{Key: "test-key", Operator: v1.NodeSelectorOpIn, Values: []string{"another-value"}},
				}}))
			node1 := ExpectScheduled(ctx, env.Client, pods[0])
			node2 := ExpectScheduled(ctx, env.Client, pods[1])
			Expect(node1.Labels).To(HaveKeyWithValue("test-key", "test-value"))
			Expect(node2.Labels).To(HaveKeyWithValue("test-key", "another-value"))
			Expect(node1.Name).ToNot(Equal(node2.Name))
		})
		It("Exists operator should not overwrite the existing value", func() {
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
				test.PodOptions{
					NodeRequirements: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"non-existent-zone"}},
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists},
					}},
			))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
	})
})

var _ = Describe("Preferential Fallback", func() {
	Context("Required", func() {
		It("should not relax the final term", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
				v1.NodeSelectorRequirement{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"default-instance-type"}},
			)
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}}, // Should not be relaxed
				}},
			}}}}
			// Don't relax
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should relax multiple terms", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
				}},
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
				}},
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
				}},
				{MatchExpressions: []v1.NodeSelectorRequirement{
					{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2"}}, // OR operator, never get to this one
				}},
			}}}}
			// Remove first term
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// Remove second term
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// Success
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-1"))
		})
	})
	Context("Preferred", func() {
		It("should relax all terms", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
				},
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
				},
			}}}
			// Remove first term
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// Remove second term
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// Success
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should relax to use lighter weights", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}})
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 100, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelInstanceTypeStable, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-3"}},
					}},
				},
				{
					Weight: 50, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2"}},
					}},
				},
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{ // OR operator, never get to this one
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
					}},
				},
			}}}
			// Remove heaviest term
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// Success
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-2"))
		})
		It("should schedule even if preference is conflicting with requirement", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test-zone-3"}},
					}},
				},
			},
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{NodeSelectorTerms: []v1.NodeSelectorTerm{
					{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-3"}}, // Should not be relaxed
					}},
				}},
			}}
			// Remove first term
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// Success
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
		})
		It("should schedule even if preference requirements are conflicting", func() {
			pod := test.UnschedulablePod()
			pod.Spec.Affinity = &v1.Affinity{NodeAffinity: &v1.NodeAffinity{PreferredDuringSchedulingIgnoredDuringExecution: []v1.PreferredSchedulingTerm{
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"invalid"}},
					}},
				},
				{
					Weight: 1, Preference: v1.NodeSelectorTerm{MatchExpressions: []v1.NodeSelectorRequirement{
						{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"invalid"}},
					}},
				},
			}}}
			// Remove first term
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectNotScheduled(ctx, env.Client, pod)
			// Success
			pod = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod)[0]
			ExpectScheduled(ctx, env.Client, pod)
		})
	})
})

var _ = Describe("Topology", func() {
	labels := map[string]string{"test": "test"}

	It("should ignore unknown topology keys", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: []v1.TopologySpreadConstraint{{
				TopologyKey:       "unknown",
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}},
		))[0]
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	Context("Zonal", func() {
		It("should balance pods across zones", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1, 2))
		})
		It("should respect provisioner zonal constraints", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}})
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2))
		})
		It("should respect provisioner zonal constraints (existing pod)", func() {
			Skip("enable after we don't violate max-skew")
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels},
					NodeSelector: map[string]string{
						v1.LabelTopologyZone: "test-zone-3",
					},
				}))
			ExpectScheduled(ctx, env.Client, pod[0])

			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}})
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			// we should have unschedulable pods now, the provisioner can only schedule to zone-1/zone-2, but because of the existing
			// pod in zone-3 it can put a max of two per zone before it would violate max skew
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 2, 2))
		})
		It("should not violate max-skew when unsat = do not schedule", func() {
			Skip("enable after pod-affinity is finished")
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			// force this pod onto zone-1
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}})
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}))
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))

			// now only allow scheduling pods on zone-2 and zone-3
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-2", "test-zone-3"}})
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(10, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)

			// max skew of 1, so test-zone-2/3 will have 2 nodes each and the rest of the pods will fail to schedule
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 2, 2))
		})
		It("should only count running/scheduled pods with matching labels scheduled to nodes with a corresponding domain", func() {
			wrongNamespace := strings.ToLower(randomdata.SillyName())
			firstNode := test.Node(test.NodeOptions{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1.LabelTopologyZone: "test-zone-1"}}})
			secondNode := test.Node(test.NodeOptions{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1.LabelTopologyZone: "test-zone-2"}}})
			thirdNode := test.Node(test.NodeOptions{}) // missing topology domain
			ExpectCreated(ctx, env.Client, provisioner, firstNode, secondNode, thirdNode, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: wrongNamespace}})
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.Pod(test.PodOptions{NodeName: firstNode.Name}),                                                                                                                         // ignored, missing labels
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}}),                                                                                                    // ignored, pending
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: thirdNode.Name}),                                                                          // ignored, no domain on node
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels, Namespace: wrongNamespace}, NodeName: firstNode.Name}),                                               // ignored, wrong namespace
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels, DeletionTimestamp: &metav1.Time{Time: time.Now().Add(10 * time.Second)}}, NodeName: firstNode.Name}), // ignored, terminating
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name, Phase: v1.PodFailed}),                                                     // ignored, phase=Failed
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name, Phase: v1.PodSucceeded}),                                                  // ignored, phase=Succeeded
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: secondNode.Name}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			nodes := v1.NodeList{}
			Expect(env.Client.List(ctx, &nodes)).To(Succeed())
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2, 1))
		})
		It("should match all pods when labelSelector is not specified", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))
		})
		It("should handle interdependent selectors", func() {
			Skip("enable after scheduler handles non-self selecting topology")
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{TopologySpreadConstraints: topology})...,
			)
			// This is weird, but the topology label selector is used for determining domain counts. The pod that
			// owns the topology is what the spread actually applies to.  In this test case, there are no pods matching
			// the label selector, so the max skew is zero.  This means we can pack all the pods onto the same node since
			// it doesn't violate the topology spread constraint (i.e. adding new pods doesn't increase skew since the
			// pods we are adding don't count toward skew). This behavior is called out at
			// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/ , though it's not
			// recommended for users.
			nodeNames := sets.NewString()
			for _, p := range pods {
				nodeNames.Insert(p.Spec.NodeName)
			}
			Expect(nodeNames).To(HaveLen(1))
		})
	})

	Context("Hostname", func() {
		It("should balance pods across nodes", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1, 1, 1))
		})
		It("should balance pods on the same hostname up to maxskew", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           4,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4))
		})
		It("balance multiple deployments with hostname topology spread", func() {
			Skip("enable after pod-affinity is finished")
			// Issue #1425
			spreadPod := func(appName string) test.PodOptions {
				return test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": appName,
						},
					},
					TopologySpreadConstraints: []v1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       v1.LabelHostname,
							WhenUnsatisfiable: v1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": appName},
							},
						},
					},
				}
			}

			scheduled := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(spreadPod("app1")), test.UnschedulablePod(spreadPod("app1")),
				test.UnschedulablePod(spreadPod("app2")), test.UnschedulablePod(spreadPod("app2")))

			for _, p := range scheduled {
				ExpectScheduled(ctx, env.Client, p)
			}
			nodes := v1.NodeList{}
			Expect(env.Client.List(ctx, &nodes)).To(Succeed())
			// this wasn't part of #1425, but ensures that we launch the minimum number of nodes
			Expect(nodes.Items).To(HaveLen(2))
		})
		It("balance multiple deployments with hostname topology spread & varying arch", func() {
			// Issue #1425
			spreadPod := func(appName, arch string) test.PodOptions {
				return test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": appName,
						},
					},
					NodeRequirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{arch},
						},
					},
					TopologySpreadConstraints: []v1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       v1.LabelHostname,
							WhenUnsatisfiable: v1.DoNotSchedule,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": appName},
							},
						},
					},
				}
			}

			scheduled := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(spreadPod("app1", v1alpha5.ArchitectureAmd64)), test.UnschedulablePod(spreadPod("app1", v1alpha5.ArchitectureAmd64)),
				test.UnschedulablePod(spreadPod("app2", v1alpha5.ArchitectureArm64)), test.UnschedulablePod(spreadPod("app2", v1alpha5.ArchitectureArm64)))

			for _, p := range scheduled {
				ExpectScheduled(ctx, env.Client, p)
			}
			nodes := v1.NodeList{}
			Expect(env.Client.List(ctx, &nodes)).To(Succeed())
			// same test as the previous one, but now the architectures are different so we need four nodes in total
			Expect(nodes.Items).To(HaveLen(4))
		})
	})

	Context("CapacityType", func() {
		It("should balance pods across capacity types", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2))
		})
		It("should respect provisioner capacity type constraints", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand}})
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2))
		})
		It("should not violate max-skew when unsat = do not schedule", func() {
			Skip("enable we don't violate max-skew")
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			// force this pod onto spot
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeSpot}})
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}))
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))

			// now only allow scheduling pods on on-demand
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeOnDemand}})
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)

			// max skew of 1, so on-demand will have 2 pods and the rest of the pods will fail to schedule
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 2))
		})
		It("should violate max-skew when unsat = schedule anyway (capacity type)", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.ScheduleAnyway,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeSpot}})
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}))
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))

			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeOnDemand}})
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)

			// max skew of 1, on-demand will end up with 5 pods even though spot has a single pod
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 5))
		})
		It("should only count running/scheduled pods with matching labels scheduled to nodes with a corresponding domain", func() {
			wrongNamespace := strings.ToLower(randomdata.SillyName())
			firstNode := test.Node(test.NodeOptions{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1alpha5.LabelCapacityType: v1alpha1.CapacityTypeSpot}}})
			secondNode := test.Node(test.NodeOptions{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{v1alpha5.LabelCapacityType: v1alpha1.CapacityTypeOnDemand}}})
			thirdNode := test.Node(test.NodeOptions{}) // missing topology capacity type
			ExpectCreated(ctx, env.Client, provisioner, firstNode, secondNode, thirdNode, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: wrongNamespace}})
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.Pod(test.PodOptions{NodeName: firstNode.Name}),                                                                                                                         // ignored, missing labels
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}}),                                                                                                    // ignored, pending
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: thirdNode.Name}),                                                                          // ignored, no domain on node
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels, Namespace: wrongNamespace}, NodeName: firstNode.Name}),                                               // ignored, wrong namespace
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels, DeletionTimestamp: &metav1.Time{Time: time.Now().Add(10 * time.Second)}}, NodeName: firstNode.Name}), // ignored, terminating
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name, Phase: v1.PodFailed}),                                                     // ignored, phase=Failed
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name, Phase: v1.PodSucceeded}),                                                  // ignored, phase=Succeeded
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: firstNode.Name}),
				test.Pod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, NodeName: secondNode.Name}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
				test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology}),
			)
			nodes := v1.NodeList{}
			Expect(env.Client.List(ctx, &nodes)).To(Succeed())
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 3))
		})
		It("should match all pods when labelSelector is not specified", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.DoNotSchedule,
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				test.UnschedulablePod(),
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1))
		})
		It("should handle interdependent selectors", func() {
			Skip("enable after scheduler handles non-self selecting topology")
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{TopologySpreadConstraints: topology})...,
			)
			// This is weird, but the topology label selector is used for determining domain counts. The pod that
			// owns the topology is what the spread actually applies to.  In this test case, there are no pods matching
			// the label selector, so the max skew is zero.  This means we can pack all the pods onto the same node since
			// it doesn't violate the topology spread constraint (i.e. adding new pods doesn't increase skew since the
			// pods we are adding don't count toward skew). This behavior is called out at
			// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/ , though it's not
			// recommended for users.
			nodeNames := sets.NewString()
			for _, p := range pods {
				nodeNames.Insert(p.Spec.NodeName)
			}
			Expect(nodeNames).To(HaveLen(1))
		})
	})

	Context("Combined Hostname and Zonal Topology", func() {
		It("should spread pods while respecting both constraints", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           3,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(2, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(3, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(2, 2, 1))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4, 3, 3))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(11, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(7, 7, 7))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))
		})
	})

	Context("Combined Hostname and Capacity Type Topology", func() {
		It("should spread pods while respecting both constraints", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           3,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(2, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(1, 1))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(3, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 2))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 5))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(11, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(11, 10))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 3)))
		})
	})

	Context("Combined Zonal and Capacity Type Topology", func() {
		It("should spread pods while respecting both constraints", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(2, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 1)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 1)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(3, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 3)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 2)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 5)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 4)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(11, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 11)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 7)))
		})
	})

	Context("Combined Hostname, Zonal, and Capacity Type Topology", func() {
		It("should spread pods while respecting all constraints", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}, {
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           2,
			}, {
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           3,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(2, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 1)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 2)))
			ExpectSkew(ctx, env.Client, "default", &topology[2]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(3, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 3)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 2)))
			ExpectSkew(ctx, env.Client, "default", &topology[2]).ToNot(ContainElements(BeNumerically(">", 3)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 5)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 4)))
			ExpectSkew(ctx, env.Client, "default", &topology[2]).ToNot(ContainElements(BeNumerically(">", 5)))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(11, test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: labels}, TopologySpreadConstraints: topology})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).ToNot(ContainElements(BeNumerically(">", 11)))
			ExpectSkew(ctx, env.Client, "default", &topology[1]).ToNot(ContainElements(BeNumerically(">", 8)))
			ExpectSkew(ctx, env.Client, "default", &topology[2]).ToNot(ContainElements(BeNumerically(">", 9)))
		})
	})

	// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/#interaction-with-node-affinity-and-node-selectors
	Context("Combined Zonal Topology and Node Affinity", func() {
		It("should limit spread options by nodeSelector", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				append(
					MakePods(5, test.PodOptions{
						ObjectMeta:                metav1.ObjectMeta{Labels: labels},
						TopologySpreadConstraints: topology,
						NodeSelector:              map[string]string{v1.LabelTopologyZone: "test-zone-1"},
					}),
					MakePods(10, test.PodOptions{
						ObjectMeta:                metav1.ObjectMeta{Labels: labels},
						TopologySpreadConstraints: topology,
						NodeSelector:              map[string]string{v1.LabelTopologyZone: "test-zone-2"},
					})...,
				)...,
			)
			// we limit the zones of each pod via node selectors, which causes the topology spreads to only consider
			// the single zone as the only valid domain for the topology spread allowing us to schedule multiple pods per domain
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 10))
		})
		It("should limit spread options by node requirements", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(10, test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
					NodeRequirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelTopologyZone,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"test-zone-1", "test-zone-2"},
						},
					},
				})...)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 5))
		})
		It("should limit spread options by node affinity", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelTopologyZone,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			// need to limit the provisioner to only zone-1, zone-2 or else it will know that test-zone-3 has 0 pods and won't violate
			// the max-skew
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2"}})
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(6, test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
					NodeRequirements: []v1.NodeSelectorRequirement{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{
						"test-zone-1", "test-zone-2",
					}}},
				})...)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 3))

			// open the provisioner back to up so it can see all zones again
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-2", "test-zone-3"}})

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, MakePods(1, test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
				NodeRequirements: []v1.NodeSelectorRequirement{{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{
					"test-zone-2", "test-zone-3",
				}}},
			})...)

			// it will schedule on the currently empty zone-3 even though max-skew is violated as it improves max-skew
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 3, 1))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
				})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(4, 4, 4))
		})
	})

	// https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/#interaction-with-node-affinity-and-node-selectors
	Context("Combined Capacity Type Topology and Node Affinity", func() {
		It("should limit spread options by nodeSelector", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.ScheduleAnyway,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				append(
					MakePods(5, test.PodOptions{
						ObjectMeta:                metav1.ObjectMeta{Labels: labels},
						TopologySpreadConstraints: topology,
						NodeSelector:              map[string]string{v1alpha5.LabelCapacityType: v1alpha1.CapacityTypeSpot},
					}),
					MakePods(5, test.PodOptions{
						ObjectMeta:                metav1.ObjectMeta{Labels: labels},
						TopologySpreadConstraints: topology,
						NodeSelector:              map[string]string{v1alpha5.LabelCapacityType: v1alpha1.CapacityTypeOnDemand},
					})...,
				)...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 5))
		})
		It("should limit spread options by node affinity (capacity type)", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1alpha5.LabelCapacityType,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			// need to limit the provisioner to spot or else it will know that on-demand has 0 pods and won't violate
			// the max-skew
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeSpot}})
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(3, test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
					NodeRequirements: []v1.NodeSelectorRequirement{{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{
						v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand,
					}}},
				})...)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3))

			// open the provisioner back to up so it can see all capacity types
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{v1alpha1.CapacityTypeSpot, v1alpha1.CapacityTypeOnDemand}})

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, MakePods(1, test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
				NodeRequirements: []v1.NodeSelectorRequirement{{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpIn, Values: []string{
					v1alpha1.CapacityTypeOnDemand,
				}}},
			})...)

			// it will schedule on the currently empty on-demand even though max-skew is violated as it improves max-skew
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(3, 1))

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
				MakePods(5, test.PodOptions{
					ObjectMeta:                metav1.ObjectMeta{Labels: labels},
					TopologySpreadConstraints: topology,
				})...,
			)
			ExpectSkew(ctx, env.Client, "default", &topology[0]).To(ConsistOf(5, 4))
		})
	})

	Context("Pod Affinity/Anti-Affinity", func() {
		It("should schedule a pod with empty pod affinity and anti-affinity", func() {
			ExpectCreated(ctx, env.Client)
			pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(test.PodOptions{
				PodRequirements:     []v1.PodAffinityTerm{},
				PodAntiRequirements: []v1.PodAffinityTerm{},
			}))[0]
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should respect pod affinity (hostname)", func() {
			Skip("enable after pod affinity support")
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			affLabels := map[string]string{"security": "s2"}

			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{PodRequirements: []v1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: v1.LabelHostname,
			}}})

			var pods []*v1.Pod
			pods = append(pods, MakePods(10, test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			})...)
			pods = append(pods, affPod1)
			pods = append(pods, affPod2)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			n1 := ExpectScheduled(ctx, env.Client, affPod1)
			n2 := ExpectScheduled(ctx, env.Client, affPod2)
			// should be scheduled on the same node
			Expect(n1.Name).To(Equal(n2.Name))
		})
		It("should respect pod affinity (zone)", func() {
			// TODO: We can pass this test, but it's probably not worth the effort.  We know that the zonal affinity is
			// satisfied since the pods can end up on the same node. I can't think of a good way to make this work other
			// that possibly tightening constraints to (e.g. zonal -> hostname) see if that would help in a scheduling
			// after everything else fails.
			Skip("skip until we make a decision")
			affLabels := map[string]string{"security": "s2"}
			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{PodRequirements: []v1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: v1.LabelTopologyZone,
			}}})

			// create
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, affPod1, affPod2)
			n1 := ExpectScheduled(ctx, env.Client, affPod1)
			n2 := ExpectScheduled(ctx, env.Client, affPod2)
			// should be scheduled on the same node
			Expect(n1.Name).To(Equal(n2.Name))
		})
		It("should respect self pod affinity (hostname)", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}

			pods := MakePods(3, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				PodRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelHostname,
				}},
			})

			pods = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			nodeNames := map[string]struct{}{}
			for _, p := range pods {
				n := ExpectScheduled(ctx, env.Client, p)
				nodeNames[n.Name] = struct{}{}
			}
			Expect(len(nodeNames)).To(Equal(1))
		})
		It("should respect self pod affinity for first empty topology domain only (hostname)", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}
			pods := MakePods(10, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				PodRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelHostname,
				}},
			})
			pods = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			nodeNames := map[string]struct{}{}
			unscheduledCount := 0
			scheduledCount := 0
			for _, p := range pods {
				p = ExpectPodExists(ctx, env.Client, p.Name, p.Namespace)
				if p.Spec.NodeName == "" {
					unscheduledCount++
				} else {
					nodeNames[p.Spec.NodeName] = struct{}{}
					scheduledCount++
				}
			}
			// the node can only hold 5 pods, so we should get a single node with 5 pods and 5 unschedulable pods
			Expect(len(nodeNames)).To(Equal(1))
			Expect(scheduledCount).To(BeNumerically("==", 5))
			Expect(unscheduledCount).To(BeNumerically("==", 5))
		})
		It("should respect self pod affinity for first empty topology domain only (hostname/constrained zones)", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}

			// put one pod in test-zone-1, this shouldn't affect the remainder of the test as the node selectors are
			// disjoint between this pod and the other 10 pods
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				NodeSelector: map[string]string{
					v1.LabelTopologyZone: "test-zone-1",
				},
				PodRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelHostname,
				}},
			}))

			pods := MakePods(10, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				NodeRequirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1.LabelTopologyZone,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"test-zone-2", "test-zone-3"},
					},
				},
				PodRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelHostname,
				}},
			})
			pods = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			nodeNames := map[string]struct{}{}
			unscheduledCount := 0
			scheduledCount := 0
			for _, p := range pods {
				p = ExpectPodExists(ctx, env.Client, p.Name, p.Namespace)
				if p.Spec.NodeName == "" {
					unscheduledCount++
				} else {
					nodeNames[p.Spec.NodeName] = struct{}{}
					scheduledCount++
				}
			}
			// the node can only hold 5 pods, so we should get a single node with 5 pods and 5 unschedulable pods
			Expect(len(nodeNames)).To(Equal(1))
			Expect(scheduledCount).To(BeNumerically("==", 5))
			Expect(unscheduledCount).To(BeNumerically("==", 5))
		})
		It("should respect self pod affinity (zone)", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}

			pods := MakePods(3, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				PodRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelTopologyZone,
				}},
			})

			pods = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			nodeNames := map[string]struct{}{}
			for _, p := range pods {
				n := ExpectScheduled(ctx, env.Client, p)
				nodeNames[n.Name] = struct{}{}
			}
			Expect(len(nodeNames)).To(Equal(1))
		})
		It("should respect self pod affinity (zone w/ constraint)", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}
			// the pod needs to provide it's own zonal affinity, but we further limit it to only being on test-zone-3
			pods := MakePods(3, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: affLabels,
				},
				PodRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelTopologyZone,
				}},
				NodeRequirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1.LabelTopologyZone,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"test-zone-3"},
					},
				},
			})
			pods = ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			nodeNames := map[string]struct{}{}
			for _, p := range pods {
				n := ExpectScheduled(ctx, env.Client, p)
				nodeNames[n.Name] = struct{}{}
				Expect(n.Labels[v1.LabelTopologyZone]).To(Equal("test-zone-3"))
			}
			Expect(len(nodeNames)).To(Equal(1))
		})
		It("should allow violation of preferred pod affinity", func() {
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			affPod2 := test.UnschedulablePod(test.PodOptions{PodPreferences: []v1.WeightedPodAffinityTerm{{
				Weight: 50,
				PodAffinityTerm: v1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"security": "s2"},
					},
					TopologyKey: v1.LabelHostname,
				},
			}}})

			var pods []*v1.Pod
			pods = append(pods, MakePods(10, test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			})...)

			pods = append(pods, affPod2)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			// should be scheduled as the pod it has affinity to doesn't exist, but it's only a preference and not a
			// hard constraints
			ExpectScheduled(ctx, env.Client, affPod2)

		})
		It("should allow violation of preferred pod anti-affinity", func() {
			affPods := MakePods(10, test.PodOptions{PodAntiPreferences: []v1.WeightedPodAffinityTerm{
				{
					Weight: 50,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
						TopologyKey: v1.LabelTopologyZone,
					},
				},
			}})

			var pods []*v1.Pod
			pods = append(pods, MakePods(3, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{{
					TopologyKey:       v1.LabelTopologyZone,
					WhenUnsatisfiable: v1.DoNotSchedule,
					LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
					MaxSkew:           1,
				}},
			})...)

			pods = append(pods, affPods...)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			for _, aff := range affPods {
				ExpectScheduled(ctx, env.Client, aff)
			}

		})
		It("should separate nodes using simple pod anti-affinity on hostname", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}
			// pod affinity/anti-affinity are bidirectional, so run this a few times to ensure we handle it regardless
			// of pod scheduling order
			for i := 0; i < 10; i++ {
				affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})
				// affPod2 will avoid affPod1
				affPod2 := test.UnschedulablePod(test.PodOptions{PodAntiRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelHostname,
				}}})

				ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, affPod2, affPod1)
				n1 := ExpectScheduled(ctx, env.Client, affPod1)
				n2 := ExpectScheduled(ctx, env.Client, affPod2)
				// should not be scheduled on the same node
				Expect(n1.Name).ToNot(Equal(n2.Name))
			}
		})
		It("should not violate pod anti-affinity on zone", func() {
			affLabels := map[string]string{"security": "s2"}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
				},
				NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
				},
				NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
				},
				NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{
				PodAntiRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelTopologyZone,
				}}})

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, zone1Pod, zone2Pod, zone3Pod, affPod)
			// the three larger zone specific pods should get scheduled first due to first fit descending onto one
			// node per zone.
			ExpectScheduled(ctx, env.Client, zone1Pod)
			ExpectScheduled(ctx, env.Client, zone2Pod)
			ExpectScheduled(ctx, env.Client, zone3Pod)
			// the pod with anti-affinity
			ExpectNotScheduled(ctx, env.Client, affPod)
		})
		It("should violate preferred pod anti-affinity on zone (inverse)", func() {
			affLabels := map[string]string{"security": "s2"}
			anti := []v1.WeightedPodAffinityTerm{
				{
					Weight: 10,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: affLabels,
						},
						TopologyKey: v1.LabelTopologyZone,
					},
				},
			}
			rr := v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
			}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, zone1Pod, zone2Pod, zone3Pod, affPod)
			// three pods with anti-affinity will schedule first due to first fit-descending
			ExpectScheduled(ctx, env.Client, zone1Pod)
			ExpectScheduled(ctx, env.Client, zone2Pod)
			ExpectScheduled(ctx, env.Client, zone3Pod)
			// the anti-affinity was a preference, so this can schedule
			ExpectScheduled(ctx, env.Client, affPod)
		})
		It("should not violate pod anti-affinity on zone (inverse)", func() {
			Skip("enable after pod-affinity is finished")
			affLabels := map[string]string{"security": "s2"}
			anti := []v1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: v1.LabelTopologyZone,
			}}
			rr := v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
			}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, zone1Pod, zone2Pod, zone3Pod, affPod)
			// three pods with anti-affinity will schedule first due to first fit-descending
			ExpectScheduled(ctx, env.Client, zone1Pod)
			ExpectScheduled(ctx, env.Client, zone2Pod)
			ExpectScheduled(ctx, env.Client, zone3Pod)
			// this pod with no anti-affinity rules can't schedule. It has no anti-affinity rules, but every zone has a
			// pod with anti-affinity rules that prevent it from scheduling
			ExpectNotScheduled(ctx, env.Client, affPod)
		})
		It("should not violate pod anti-affinity on zone (Schrödinger)", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}
			anti := []v1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: v1.LabelTopologyZone,
			}}
			zoneAnywherePod := test.UnschedulablePod(test.PodOptions{
				PodAntiRequirements: anti,
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
				},
			})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, zoneAnywherePod, affPod)
			// the pod with anti-affinity will schedule first due to first fit-descending, but we don't know which zone it landed in
			node1 := ExpectScheduled(ctx, env.Client, zoneAnywherePod)

			// this pod cannot schedule since the pod with anti-affinity could potentially be in any zone
			affPod = ExpectNotScheduled(ctx, env.Client, affPod)

			// a second batching will now allow the pod to schedule as the zoneAnywherePod has been committed to a zone
			// by the actual node creation
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, affPod)
			node2 := ExpectScheduled(ctx, env.Client, affPod)

			Expect(node1.Labels[v1.LabelTopologyZone]).ToNot(Equal(node2.Labels[v1.LabelTopologyZone]))

		})
		It("should not violate pod anti-affinity on zone (inverse w/existing nodes)", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}
			anti := []v1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: v1.LabelTopologyZone,
			}}
			rr := v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
			}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiRequirements:  anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			// provision these so we get three nodes that exist in the cluster with anti-affinity to a pod that we will
			// then try to schedule
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, zone1Pod, zone2Pod, zone3Pod)
			ExpectScheduled(ctx, env.Client, zone1Pod)
			ExpectScheduled(ctx, env.Client, zone2Pod)
			ExpectScheduled(ctx, env.Client, zone3Pod)

			// this pod with no anti-affinity rules can't schedule. It has no anti-affinity rules, but every zone has an
			// existing pod (not from this batch) with anti-affinity rules that prevent it from scheduling
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, affPod)
			ExpectNotScheduled(ctx, env.Client, affPod)
		})
		It("should violate preferred pod anti-affinity on zone (inverse w/existing nodes)", func() {
			affLabels := map[string]string{"security": "s2"}
			anti := []v1.WeightedPodAffinityTerm{
				{
					Weight: 10,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: affLabels,
						},
						TopologyKey: v1.LabelTopologyZone,
					},
				},
			}
			rr := v1.ResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
			}
			zone1Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-1"}})
			zone2Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-2"}})
			zone3Pod := test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: rr,
				PodAntiPreferences:   anti,
				NodeSelector:         map[string]string{v1.LabelTopologyZone: "test-zone-3"}})

			affPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			// provision these so we get three nodes that exist in the cluster with anti-affinity to a pod that we will
			// then try to schedule
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, zone1Pod, zone2Pod, zone3Pod)
			ExpectScheduled(ctx, env.Client, zone1Pod)
			ExpectScheduled(ctx, env.Client, zone2Pod)
			ExpectScheduled(ctx, env.Client, zone3Pod)

			// this pod with no anti-affinity rules can schedule, though it couldn't if the anti-affinity were required
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, affPod)
			ExpectScheduled(ctx, env.Client, affPod)
		})
		It("should allow violation of a pod affinity preference with a conflicting required constraint", func() {
			affLabels := map[string]string{"security": "s2"}
			constraint := v1.TopologySpreadConstraint{
				MaxSkew:           1,
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: labels,
				},
			}
			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})
			affPods := MakePods(3, test.PodOptions{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				// limit these pods to one per host
				TopologySpreadConstraints: []v1.TopologySpreadConstraint{constraint},
				// with a preference to the other pod
				PodPreferences: []v1.WeightedPodAffinityTerm{{
					Weight: 50,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: affLabels,
						},
						TopologyKey: v1.LabelHostname,
					},
				}}})
			pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, append(affPods, affPod1)...)
			// all pods should be scheduled since the affinity term is just a preference
			for _, pod := range pods {
				ExpectScheduled(ctx, env.Client, pod)
			}
			// and we'll get three nodes due to the topology spread
			ExpectSkew(ctx, env.Client, "", &constraint).To(ConsistOf(1, 1, 1))
		})
		It("should support pod anti-affinity with a zone topology", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}

			// affPods will avoid being scheduled in the same zone
			createPods := func() []*v1.Pod {
				return MakePods(3, test.PodOptions{
					ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
					PodAntiRequirements: []v1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: affLabels,
						},
						TopologyKey: v1.LabelTopologyZone,
					}}})
			}

			top := &v1.TopologySpreadConstraint{TopologyKey: v1.LabelTopologyZone}

			// One of the downsides of late committal is that absent other constraints, it takes multiple batches of
			// scheduling for zonal anti-affinities to work themselves out.  The first schedule, we know that the pod
			// will land in test-zone-1, test-zone-2, or test-zone-3, but don't know which it collapses to until the
			// node is actually created.

			// one pod pod will schedule
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, createPods()...)
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1))

			// second pod in a second zone
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, createPods()...)
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1, 1))

			// third pod in the last zone
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, createPods()...)
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1, 1, 1))

			// and nothing else can schedule
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, createPods()...)
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1, 1, 1))
		})
		It("should not schedule pods with affinity to a non-existent pod", func() {
			affLabels := map[string]string{"security": "s2"}
			affPods := MakePods(10, test.PodOptions{
				PodRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelTopologyZone,
				}}})

			pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, affPods...)
			// the pod we have affinity to is not in the cluster, so all of these pods are unschedulable
			for _, p := range pods {
				ExpectNotScheduled(ctx, env.Client, p)
			}
		})
		It("should support pod affinity with zone topology (unconstrained target)", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}

			// the pod that the others have an affinity to
			targetPod := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels}})

			// affPods all want to schedule in the same zone as targetPod, but can't as it's zone is undetermined
			affPods := MakePods(10, test.PodOptions{
				PodRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelTopologyZone,
				}}})

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, append(affPods, targetPod)...)
			top := &v1.TopologySpreadConstraint{TopologyKey: v1.LabelTopologyZone}
			// these pods can't schedule as the pod they have affinity to isn't limited to any particular zone
			for i := range affPods {
				ExpectNotScheduled(ctx, env.Client, affPods[i])
				affPods[i] = ExpectPodExists(ctx, env.Client, affPods[i].Name, affPods[i].Namespace)
			}
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(1))

			// now that targetPod has been scheduled to a node, it's zone is committed and the pods with affinity to it
			// should schedule in the same zone
			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, affPods...)
			for _, pod := range affPods {
				ExpectScheduled(ctx, env.Client, pod)
			}
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(11))
		})
		It("should support pod affinity with zone topology (constrained target)", func() {
			Skip("enable after pod affinity support")
			affLabels := map[string]string{"security": "s2"}

			// the pod that the others have an affinity to
			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels},
				NodeRequirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1.LabelTopologyZone,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"test-zone-1"},
					},
				}})

			// affPods will all be scheduled in the same zone as affPod1
			affPods := MakePods(10, test.PodOptions{
				PodRequirements: []v1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: affLabels,
					},
					TopologyKey: v1.LabelTopologyZone,
				}}})

			affPods = append(affPods, affPod1)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, affPods...)
			top := &v1.TopologySpreadConstraint{TopologyKey: v1.LabelTopologyZone}
			ExpectSkew(ctx, env.Client, "default", top).To(ConsistOf(11))
		})
		It("should handle multiple dependent affinities", func() {
			Skip("enable after pod affinity support")
			dbLabels := map[string]string{"type": "db", "spread": "spread"}
			webLabels := map[string]string{"type": "web", "spread": "spread"}
			cacheLabels := map[string]string{"type": "cache", "spread": "spread"}
			uiLabels := map[string]string{"type": "ui", "spread": "spread"}

			for i := 0; i < 50; i++ {
				// we have to schedule DB -> Web -> Cache -> UI in that order or else there are pod affinity violations
				pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
					test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: dbLabels}}),
					test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: webLabels},
						PodRequirements: []v1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{MatchLabels: dbLabels},
							TopologyKey:   v1.LabelHostname},
						}}),
					test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: cacheLabels},
						PodRequirements: []v1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{MatchLabels: webLabels},
							TopologyKey:   v1.LabelHostname},
						}}),
					test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: uiLabels},
						PodRequirements: []v1.PodAffinityTerm{{
							LabelSelector: &metav1.LabelSelector{MatchLabels: cacheLabels},
							TopologyKey:   v1.LabelHostname},
						}}),
				)
				for _, pod := range pods {
					ExpectScheduled(ctx, env.Client, pod)
					ExpectDeleted(ctx, env.Client, pod)
				}
			}
		})
		It("should filter pod affinity topologies by namespace, no matching pods", func() {
			Skip("enable after pod-affinity is finished")
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			ExpectCreated(ctx, env.Client, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other-ns-no-match"}})
			affLabels := map[string]string{"security": "s2"}

			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels, Namespace: "other-ns-no-match"}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{PodRequirements: []v1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				TopologyKey: v1.LabelHostname,
			}}})

			var pods []*v1.Pod
			// creates 10 nodes due to topo spread
			pods = append(pods, MakePods(10, test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			})...)
			pods = append(pods, affPod1)
			pods = append(pods, affPod2)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)

			// the target pod gets scheduled
			ExpectScheduled(ctx, env.Client, affPod1)
			// but the one with affinity does not since the target pod is not in the same namespace and doesn't
			// match the namespace list or namespace selector
			ExpectNotScheduled(ctx, env.Client, affPod2)
		})
		It("should filter pod affinity topologies by namespace, matching pods namespace list", func() {
			Skip("enable after pod-affinity is finished")
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			ExpectCreated(ctx, env.Client, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other-ns-list"}})
			affLabels := map[string]string{"security": "s2"}

			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels, Namespace: "other-ns-list"}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{PodRequirements: []v1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				Namespaces:  []string{"other-ns-list"},
				TopologyKey: v1.LabelHostname,
			}}})

			var pods []*v1.Pod
			// create 10 nodes
			pods = append(pods, MakePods(10, test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			})...)
			// put our target pod on one of them
			pods = append(pods, affPod1)
			// and our pod with affinity should schedule on the same node
			pods = append(pods, affPod2)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			n1 := ExpectScheduled(ctx, env.Client, affPod1)
			n2 := ExpectScheduled(ctx, env.Client, affPod2)
			// should be scheduled on the same node
			Expect(n1.Name).To(Equal(n2.Name))
		})
		It("should filter pod affinity topologies by namespace, matching pods namespace selector", func() {
			Skip("enable after pod-affinity is finished")
			topology := []v1.TopologySpreadConstraint{{
				TopologyKey:       v1.LabelHostname,
				WhenUnsatisfiable: v1.DoNotSchedule,
				LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
				MaxSkew:           1,
			}}

			ExpectCreated(ctx, env.Client, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "empty-ns-selector", Labels: map[string]string{"foo": "bar"}}})
			affLabels := map[string]string{"security": "s2"}

			affPod1 := test.UnschedulablePod(test.PodOptions{ObjectMeta: metav1.ObjectMeta{Labels: affLabels, Namespace: "empty-ns-selector"}})
			// affPod2 will try to get scheduled with affPod1
			affPod2 := test.UnschedulablePod(test.PodOptions{PodRequirements: []v1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: affLabels,
				},
				// select all pods in all namespaces since the selector is empty
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{}},
				TopologyKey:       v1.LabelHostname,
			}}})

			var pods []*v1.Pod
			// create 10 nodes
			pods = append(pods, MakePods(10, test.PodOptions{
				ObjectMeta:                metav1.ObjectMeta{Labels: labels},
				TopologySpreadConstraints: topology,
			})...)
			// put our target pod on one of them
			pods = append(pods, affPod1)
			// and our pod with affinity should schedule on the same node
			pods = append(pods, affPod2)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pods...)
			n1 := ExpectScheduled(ctx, env.Client, affPod1)
			n2 := ExpectScheduled(ctx, env.Client, affPod2)
			// should be scheduled on the same node due to the empty namespace selector
			Expect(n1.Name).To(Equal(n2.Name))
		})
	})
})

var _ = Describe("Taints", func() {
	It("should taint nodes with provisioner taints", func() {
		provisioner.Spec.Taints = []v1.Taint{{Key: "test", Value: "bar", Effect: v1.TaintEffectNoSchedule}}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{Tolerations: []v1.Toleration{{Effect: v1.TaintEffectNoSchedule, Operator: v1.TolerationOpExists}}},
		))[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Spec.Taints).To(ContainElement(provisioner.Spec.Taints[0]))
	})
	It("should schedule pods that tolerate provisioner constraints", func() {
		provisioner.Spec.Taints = []v1.Taint{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}}
		for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			// Tolerates with OpExists
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoSchedule}}}),
			// Tolerates with OpEqual
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Value: "test-value", Operator: v1.TolerationOpEqual, Effect: v1.TaintEffectNoSchedule}}}),
		) {
			ExpectScheduled(ctx, env.Client, pod)
		}
		for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			// Missing toleration
			test.UnschedulablePod(),
			// key mismatch with OpExists
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "invalid", Operator: v1.TolerationOpExists}}}),
			// value mismatch
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpEqual, Effect: v1.TaintEffectNoSchedule}}}),
		) {
			ExpectNotScheduled(ctx, env.Client, pod)
		}
	})
	It("should provision nodes with taints and schedule pods, ignoring missing tolerations in the taintsToIgnore list", func() {
		provisioner.Spec.Taints = []v1.Taint{{Key: "ignore-me", Value: "nothing-to-see-here", Effect: v1.TaintEffectNoSchedule}}
		provisioner.Spec.TaintsToIgnore = []v1.Taint{{Key: "ignore-me", Value: "nothing-to-see-here", Effect: v1.TaintEffectNoSchedule}}

		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)
	})
	It("should not generate taints for OpExists", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoExecute}}}),
		)[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Spec.Taints).To(HaveLen(2)) // Expect no taints generated beyond defaults
	})
	It("should generate taints for pod tolerations", func() {
		Skip("until taint generation is reimplemented")
		pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			// Matching pods schedule together on a node with a matching taint
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			}),
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			}),
			// Key is different, generate new node with a taint for this key
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "another-test-key", Operator: v1.TolerationOpEqual, Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			}),
			// Value is different, generate new node with a taint for this value
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "another-test-value", Effect: v1.TaintEffectNoSchedule}},
			}),
			// Effect is different, generate new node with a taint for this value
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "test-value", Effect: v1.TaintEffectNoExecute}},
			}),
			// Missing effect, generate a new node with a taints for all effects
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{
				{Key: "test-key", Operator: v1.TolerationOpEqual, Value: "test-value"}},
			}),
			// // No taint generated
			test.UnschedulablePod(test.PodOptions{Tolerations: []v1.Toleration{{Key: "test-key", Operator: v1.TolerationOpExists, Effect: v1.TaintEffectNoExecute}}}),
		)
		for i, expectedTaintsPerNode := range [][]v1.Taint{
			{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			{{Key: "another-test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}},
			{{Key: "test-key", Value: "another-test-value", Effect: v1.TaintEffectNoSchedule}},
			{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoExecute}},
			{{Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoSchedule}, {Key: "test-key", Value: "test-value", Effect: v1.TaintEffectNoExecute}},
		} {
			node := ExpectScheduled(ctx, env.Client, pods[i])
			for _, taint := range expectedTaintsPerNode {
				Expect(node.Spec.Taints).To(ContainElement(taint))
			}
		}
		node := ExpectScheduled(ctx, env.Client, pods[len(pods)-1])
		Expect(node.Spec.Taints).To(HaveLen(2)) // Expect no taints generated beyond defaults
	})
})

var _ = Describe("Instance Type Compatibility", func() {
	It("should not schedule if requesting more resources than any instance type has", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceCPU: resource.MustParse("512"),
					}},
			}))
		ExpectNotScheduled(ctx, env.Client, pod[0])
	})
	It("should launch pods with different archs on different instances", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64, v1alpha5.ArchitectureAmd64},
			},
		}
		nodeNames := sets.NewString()
		for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{v1.LabelArchStable: v1alpha5.ArchitectureAmd64},
			}),
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{v1.LabelArchStable: v1alpha5.ArchitectureArm64},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should exclude instance types that are not supported by the pod constraints (node affinity/instance type)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				NodeRequirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1.LabelInstanceTypeStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"arm-instance-type"},
					},
				}}))
		// arm instance type conflicts with the provisioner limitation of AMD only
		ExpectNotScheduled(ctx, env.Client, pod[0])
	})
	It("should exclude instance types that are not supported by the pod constraints (node affinity/operating system)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				NodeRequirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1.LabelOSStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"ios"},
					},
				}}))
		// there's an instance with an OS of ios, but it has an arm processor so the provider requirements will
		// exclude it
		ExpectNotScheduled(ctx, env.Client, pod[0])
	})
	It("should exclude instance types that are not supported by the provider constraints (arch)", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			},
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{v1.ResourceCPU: resource.MustParse("14")}}}))
		// only the ARM instance has enough CPU, but it's not allowed per the provisioner
		ExpectNotScheduled(ctx, env.Client, pod[0])
	})
	It("should launch pods with different operating systems on different instances", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64, v1alpha5.ArchitectureAmd64},
			},
		}
		nodeNames := sets.NewString()
		for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{v1.LabelOSStable: "linux"},
			}),
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{v1.LabelOSStable: "windows"},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should launch pods with different instance type node selectors on different instances", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64, v1alpha5.ArchitectureAmd64},
			},
		}
		nodeNames := sets.NewString()
		for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{v1.LabelInstanceType: "small-instance-type"},
			}),
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{v1.LabelInstanceTypeStable: "default-instance-type"},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should launch pods with different zone selectors on different instances", func() {
		provisioner.Spec.Requirements.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureArm64, v1alpha5.ArchitectureAmd64},
			},
		}
		nodeNames := sets.NewString()
		for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-1"},
			}),
			test.UnschedulablePod(test.PodOptions{
				NodeSelector: map[string]string{v1.LabelTopologyZone: "test-zone-2"},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should launch pods with resources that aren't on any single instance type on different instances", func() {
		cloudProv.InstanceTypes = fake.InstanceTypes(5)
		const fakeGPU1 = "karpenter.sh/super-great-gpu"
		const fakeGPU2 = "karpenter.sh/even-better-gpu"
		cloudProv.InstanceTypes[0].Resources()[fakeGPU1] = resource.MustParse("25")
		cloudProv.InstanceTypes[1].Resources()[fakeGPU2] = resource.MustParse("25")

		nodeNames := sets.NewString()
		for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Limits: v1.ResourceList{fakeGPU1: resource.MustParse("1")},
				},
			}),
			// Should pack onto a different instance since no instance type has both GPUs
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Limits: v1.ResourceList{fakeGPU2: resource.MustParse("1")},
				},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			nodeNames.Insert(node.Name)
		}
		Expect(nodeNames.Len()).To(Equal(2))
	})
	It("should fail to schedule a pod with resources requests that aren't on a single instance type", func() {
		cloudProv.InstanceTypes = fake.InstanceTypes(5)
		const fakeGPU1 = "karpenter.sh/super-great-gpu"
		const fakeGPU2 = "karpenter.sh/even-better-gpu"
		cloudProv.InstanceTypes[0].Resources()[fakeGPU1] = resource.MustParse("25")
		cloudProv.InstanceTypes[1].Resources()[fakeGPU2] = resource.MustParse("25")

		pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						fakeGPU1: resource.MustParse("1"),
						fakeGPU2: resource.MustParse("1")},
				},
			}))
		ExpectNotScheduled(ctx, env.Client, pods[0])
	})

})

var _ = Describe("Networking constraints", func() {
	Context("HostPort", func() {
		It("shouldn't co-locate pods that use the same HostPort and protocol", func() {
			Skip("enable after scheduler is aware of hostport usage")
			port := v1.ContainerPort{
				Name:          "test-port",
				HostPort:      80,
				ContainerPort: 1234,
				Protocol:      "TCP",
			}
			pod1 := test.UnschedulablePod()
			pod1.Spec.Containers[0].Ports = append(pod1.Spec.Containers[0].Ports, port)
			pod2 := test.UnschedulablePod()
			pod2.Spec.Containers[0].Ports = append(pod2.Spec.Containers[0].Ports, port)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)
			Expect(node1.Name).ToNot(Equal(node2.Name))
		})
		It("should co-locate pods that use the same HostPort but a different protocol", func() {
			port := v1.ContainerPort{
				Name:          "test-port",
				HostPort:      80,
				ContainerPort: 1234,
				Protocol:      "TCP",
			}
			pod1 := test.UnschedulablePod()
			pod1.Spec.Containers[0].Ports = append(pod1.Spec.Containers[0].Ports, port)
			pod2 := test.UnschedulablePod()
			port.Protocol = "UDP"
			pod2.Spec.Containers[0].Ports = append(pod2.Spec.Containers[0].Ports, port)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)
			Expect(node1.Name).To(Equal(node2.Name))
		})
		It("should co-locate pods that don't use HostPort", func() {
			port := v1.ContainerPort{
				Name:          "test-port",
				ContainerPort: 1234,
				Protocol:      "TCP",
			}
			pod1 := test.UnschedulablePod()
			pod1.Spec.Containers[0].Ports = append(pod1.Spec.Containers[0].Ports, port)
			pod2 := test.UnschedulablePod()
			pod2.Spec.Containers[0].Ports = append(pod2.Spec.Containers[0].Ports, port)

			ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, pod1, pod2)
			node1 := ExpectScheduled(ctx, env.Client, pod1)
			node2 := ExpectScheduled(ctx, env.Client, pod2)
			Expect(node1.Name).To(Equal(node2.Name))
		})
	})
})

var _ = Describe("Binpacking", func() {
	It("should schedule a small pod on the smallest instance", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("100M"),
				},
			}}))[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("small-instance-type"))
	})
	It("should schedule a small pod on the smallest possible instance type", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("2000M"),
				},
			}}))[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("small-instance-type"))
	})
	It("should schedule multiple small pods on the smallest possible instance type", func() {
		opts := test.PodOptions{
			Conditions: []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("10M"),
				},
			}}
		pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.Pods(5, opts)...)
		nodeNames := sets.NewString()
		for _, p := range pods {
			node := ExpectScheduled(ctx, env.Client, p)
			nodeNames.Insert(node.Name)
			Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("small-instance-type"))
		}
		Expect(nodeNames).To(HaveLen(1))
	})
	It("should create new nodes when a node is at capacity", func() {
		opts := test.PodOptions{
			NodeSelector: map[string]string{v1.LabelArchStable: "amd64"},
			Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("1.8G"),
				},
			}}
		pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.Pods(40, opts)...)
		nodeNames := sets.NewString()
		for _, p := range pods {
			node := ExpectScheduled(ctx, env.Client, p)
			nodeNames.Insert(node.Name)
			Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("default-instance-type"))
		}
		Expect(nodeNames).To(HaveLen(20))
	})
	It("should pack small and large pods together", func() {
		largeOpts := test.PodOptions{
			NodeSelector: map[string]string{v1.LabelArchStable: "amd64"},
			Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("1.8G"),
				},
			}}
		smallOpts := test.PodOptions{
			NodeSelector: map[string]string{v1.LabelArchStable: "amd64"},
			Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("400M"),
				},
			}}

		// Two large pods are all that will fit on the default-instance type (the largest instance type) which will create
		// twenty nodes. This leaves just enough room on each of those nodes for one additional small pod per node, so we
		// should only end up with 20 nodes total.
		provPods := append(test.Pods(40, largeOpts), test.Pods(20, smallOpts)...)
		pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, provPods...)
		nodeNames := sets.NewString()
		for _, p := range pods {
			node := ExpectScheduled(ctx, env.Client, p)
			nodeNames.Insert(node.Name)
			Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("default-instance-type"))
		}
		Expect(nodeNames).To(HaveLen(20))
	})
	It("should pack nodes tightly", func() {
		cloudProv.InstanceTypes = fake.InstanceTypes(5)
		var nodes []*v1.Node
		for _, pod := range ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4.5")},
				},
			}),
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1")},
				},
			})) {
			node := ExpectScheduled(ctx, env.Client, pod)
			nodes = append(nodes, node)
		}
		Expect(nodes).To(HaveLen(2))
		// the first pod consumes nearly all CPU of the largest instance type with no room for the second pod, the
		// second pod is much smaller in terms of resources and should get a smaller node
		Expect(nodes[0].Labels[v1.LabelInstanceTypeStable]).ToNot(Equal(nodes[1].Labels[v1.LabelInstanceTypeStable]))
	})
	It("should handle zero-quantity resource requests", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: v1.ResourceList{"foo.com/weird-resources": resource.MustParse("0")},
					Limits:   v1.ResourceList{"foo.com/weird-resources": resource.MustParse("0")},
				},
			}))
		// requesting a resource of quantity zero of a type unsupported by any instance is fine
		ExpectScheduled(ctx, env.Client, pod[0])
	})
	It("should not schedule pods that exceed every instance type's capacity", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("2Ti"),
				},
			}}))[0]
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should create new nodes when a node is at capacity due to pod limits per node", func() {
		opts := test.PodOptions{
			NodeSelector: map[string]string{v1.LabelArchStable: "amd64"},
			Conditions:   []v1.PodCondition{{Type: v1.PodScheduled, Reason: v1.PodReasonUnschedulable, Status: v1.ConditionFalse}},
			ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("1m"),
					v1.ResourceCPU:    resource.MustParse("1m"),
				},
			}}
		pods := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.Pods(25, opts)...)
		nodeNames := sets.NewString()
		// all of the test instance types support 5 pods each, so we use the 5 instances of the smallest one for our 25 pods
		for _, p := range pods {
			node := ExpectScheduled(ctx, env.Client, p)
			nodeNames.Insert(node.Name)
			Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("small-instance-type"))
		}
		Expect(nodeNames).To(HaveLen(5))
	})
	It("should take into account initContainer resource requests when binpacking", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("1Gi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
			},
				InitResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceMemory: resource.MustParse("1Gi"),
						v1.ResourceCPU:    resource.MustParse("2"),
					},
				},
			}))[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("default-instance-type"))
	})
	It("should not schedule pods when initContainer resource requests are greater than available instance types", func() {
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("1Gi"),
					v1.ResourceCPU:    resource.MustParse("1"),
				},
			},
				InitResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceMemory: resource.MustParse("1Ti"),
						v1.ResourceCPU:    resource.MustParse("2"),
					},
				},
			}))[0]
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should select for valid instance types, regardless of price", func() {
		// capacity sizes and prices don't correlate here, regardless we should filter and see that all three instance types
		// are valid before preferring the cheapest one 'large'
		cloudProv.InstanceTypes = []cloudprovider.InstanceType{
			fake.NewInstanceType(fake.InstanceTypeOptions{
				Name:  "medium",
				Price: 3.00,
				Resources: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("2"),
					v1.ResourceMemory: resource.MustParse("2Gi"),
				},
			}),
			fake.NewInstanceType(fake.InstanceTypeOptions{
				Name:  "small",
				Price: 2.00,
				Resources: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("1"),
					v1.ResourceMemory: resource.MustParse("1Gi"),
				},
			}),
			fake.NewInstanceType(fake.InstanceTypeOptions{
				Name:  "large",
				Price: 1.00,
				Resources: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("4"),
					v1.ResourceMemory: resource.MustParse("4Gi"),
				},
			}),
		}
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(
			test.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					v1.ResourceCPU:    resource.MustParse("1m"),
					v1.ResourceMemory: resource.MustParse("1Mi"),
				},
			}},
		))
		node := ExpectScheduled(ctx, env.Client, pod[0])
		// large is the cheapest, so we should pick it, but the other two types are also valid options
		Expect(node.Labels[v1.LabelInstanceTypeStable]).To(Equal("large"))
		// all three options should be passed to the cloud provider
		possibleInstanceType := sets.NewString()
		for _, it := range cloudProv.CreateCalls[0].InstanceTypeOptions {
			possibleInstanceType.Insert(it.Name())
		}
		Expect(possibleInstanceType).To(Equal(sets.NewString("small", "medium", "large")))
	})
})

func MakePods(count int, options test.PodOptions) (pods []*v1.Pod) {
	for i := 0; i < count; i++ {
		pods = append(pods, test.UnschedulablePod(options))
	}
	return pods
}

func ExpectSkew(ctx context.Context, c client.Client, namespace string, constraint *v1.TopologySpreadConstraint) Assertion {
	nodes := &v1.NodeList{}
	Expect(c.List(ctx, nodes)).To(Succeed())
	pods := &v1.PodList{}
	Expect(c.List(ctx, pods, scheduling.TopologyListOptions(namespace, constraint))).To(Succeed())
	skew := map[string]int{}
	for i, pod := range pods.Items {
		if scheduling.IgnoredForTopology(&pods.Items[i]) {
			continue
		}
		for _, node := range nodes.Items {
			if pod.Spec.NodeName == node.Name {
				if constraint.TopologyKey == v1.LabelHostname {
					skew[node.Name]++ // Check node name since hostname labels aren't applied
				}
				if constraint.TopologyKey == v1.LabelTopologyZone {
					if key, ok := node.Labels[constraint.TopologyKey]; ok {
						skew[key]++
					}
				}
				if constraint.TopologyKey == v1alpha5.LabelCapacityType {
					if key, ok := node.Labels[constraint.TopologyKey]; ok {
						skew[key]++
					}
				}
			}
		}
	}
	return Expect(skew)
}

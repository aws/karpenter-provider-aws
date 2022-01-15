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

package selection_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/fake"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/selection"
	"github.com/aws/karpenter/pkg/test"

	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers/Scheduling")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(ctx, func(e *test.Environment) {
		cloudProvider := &fake.CloudProvider{}
		registry.RegisterOrDie(ctx, cloudProvider)
		provisioners = provisioning.NewController(ctx, e.Client, corev1.NewForConfigOrDie(e.Config), cloudProvider)
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
	provisioner.SetDefaults(ctx)
})

var _ = AfterEach(func() {
	ExpectProvisioningCleanedUp(ctx, env.Client, provisioners)
})

var _ = Describe("Volume Topology Requirements", func() {
	var storageClass *storagev1.StorageClass
	BeforeEach(func() {
		storageClass = test.StorageClass(test.StorageClassOptions{Zones: []string{"test-zone-2", "test-zone-3"}})
	})
	It("should not schedule if invalid pvc", func() {
		ExpectCreated(ctx, env.Client)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{"invalid"},
		}))[0]
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should schedule to storage class zones if volume does not exist", func() {
		persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{StorageClassName: &storageClass.Name})
		ExpectCreated(ctx, env.Client, storageClass, persistentVolumeClaim)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			NodeRequirements: []v1.NodeSelectorRequirement{{
				Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1", "test-zone-3"},
			}},
		}))[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
	})
	It("should not schedule if storage class zones are incompatible", func() {
		persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{StorageClassName: &storageClass.Name})
		ExpectCreated(ctx, env.Client, storageClass, persistentVolumeClaim)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			NodeRequirements: []v1.NodeSelectorRequirement{{
				Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"},
			}},
		}))[0]
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should schedule to volume zones if volume already bound", func() {
		persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{Zones: []string{"test-zone-3"}})
		persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: persistentVolume.Name, StorageClassName: &storageClass.Name})
		ExpectCreated(ctx, env.Client, storageClass, persistentVolumeClaim, persistentVolume)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
		}))[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels).To(HaveKeyWithValue(v1.LabelTopologyZone, "test-zone-3"))
	})
	It("should not schedule if volume zones are incompatible", func() {
		persistentVolume := test.PersistentVolume(test.PersistentVolumeOptions{Zones: []string{"test-zone-3"}})
		persistentVolumeClaim := test.PersistentVolumeClaim(test.PersistentVolumeClaimOptions{VolumeName: persistentVolume.Name, StorageClassName: &storageClass.Name})
		ExpectCreated(ctx, env.Client, storageClass, persistentVolumeClaim, persistentVolume)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod(test.PodOptions{
			PersistentVolumeClaims: []string{persistentVolumeClaim.Name},
			NodeRequirements: []v1.NodeSelectorRequirement{{
				Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"},
			}},
		}))[0]
		ExpectNotScheduled(ctx, env.Client, pod)
	})
})

var _ = Describe("Preferential Fallback", func() {
	Context("Required", func() {
		It("should not relax the final term", func() {
			provisioner.Spec.Requirements = v1alpha5.NewRequirements(
				v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test-zone-1"}},
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
	})
})

var _ = Describe("Multiple Provisioners", func() {
	It("should schedule to an explicitly selected provisioner", func() {
		provisioner2 := provisioner.DeepCopy()
		provisioner2.Name = "provisioner2"
		ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner2)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{v1alpha5.ProvisionerNameLabelKey: provisioner2.Name}}),
		)[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1alpha5.ProvisionerNameLabelKey]).To(Equal(provisioner2.Name))
	})
	It("should schedule to a provisioner by labels", func() {
		provisioner2 := provisioner.DeepCopy()
		provisioner2.Name = "provisioner2"
		provisioner2.Spec.Labels = map[string]string{"foo": "bar"}
		provisioner.Spec.Labels = map[string]string{"foo": "baz"}
		ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner2)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner,
			test.UnschedulablePod(test.PodOptions{NodeSelector: map[string]string{"foo": "bar"}}),
		)[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1alpha5.ProvisionerNameLabelKey]).To(Equal(provisioner2.Name))
	})
	It("should prioritize provisioners alphabetically if multiple match", func() {
		provisioner2 := provisioner.DeepCopy()
		provisioner2.Name = "aaaaaaaaa"
		ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner2)
		pod := ExpectProvisioned(ctx, env.Client, selectionController, provisioners, provisioner, test.UnschedulablePod())[0]
		node := ExpectScheduled(ctx, env.Client, pod)
		Expect(node.Labels[v1alpha5.ProvisionerNameLabelKey]).To(Equal(provisioner2.Name))
	})
})

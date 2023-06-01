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

package machine_test

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter-core/pkg/utils/functional"
	"github.com/aws/karpenter-core/pkg/utils/resources"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awstest "github.com/aws/karpenter/pkg/test"
)

var _ = Describe("StandaloneMachine", func() {
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	BeforeEach(func() {
		nodeTemplate = awstest.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{AWS: v1alpha1.AWS{
			SecurityGroupSelector: map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
			SubnetSelector:        map[string]string{"karpenter.sh/discovery": settings.FromContext(env.Context).ClusterName},
		}})
	})
	// For standalone machines, there is no Provisioner owner, so we just list all machines and delete them all
	AfterEach(func() {
		env.CleanupObjects(functional.Pair[client.Object, client.ObjectList]{First: &v1alpha5.Machine{}, Second: &v1alpha5.MachineList{}})
	})
	It("should create a standard machine within the 'c' instance family", func() {
		machine := test.Machine(v1alpha5.Machine{
			Spec: v1alpha5.MachineSpec{
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceCategory,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"c"},
					},
				},
				MachineTemplateRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			},
		})
		env.ExpectCreated(nodeTemplate, machine)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		machine = env.EventuallyExpectCreatedMachineCount("==", 1)[0]
		Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.LabelInstanceCategory, "c"))
		env.EventuallyExpectMachinesReady(machine)
	})
	It("should create a standard machine based on resource requests", func() {
		machine := test.Machine(v1alpha5.Machine{
			Spec: v1alpha5.MachineSpec{
				Resources: v1alpha5.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("3"),
						v1.ResourceMemory: resource.MustParse("64Gi"),
					},
				},
				MachineTemplateRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			},
		})
		env.ExpectCreated(nodeTemplate, machine)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		machine = env.EventuallyExpectCreatedMachineCount("==", 1)[0]
		Expect(resources.Fits(machine.Spec.Resources.Requests, node.Status.Allocatable))
		env.EventuallyExpectMachinesReady(machine)
	})
	It("should create a machine propagating all the machine spec details", func() {
		machine := test.Machine(v1alpha5.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"custom-annotation": "custom-value",
				},
				Labels: map[string]string{
					"custom-label": "custom-value",
				},
			},
			Spec: v1alpha5.MachineSpec{
				Taints: []v1.Taint{
					{
						Key:    "custom-taint",
						Effect: v1.TaintEffectNoSchedule,
						Value:  "custom-value",
					},
					{
						Key:    "other-custom-taint",
						Effect: v1.TaintEffectNoExecute,
						Value:  "other-custom-value",
					},
				},
				Resources: v1alpha5.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("3"),
						v1.ResourceMemory: resource.MustParse("16Gi"),
					},
				},
				Kubelet: &v1alpha5.KubeletConfiguration{
					ContainerRuntime: lo.ToPtr("containerd"),
					MaxPods:          lo.ToPtr[int32](110),
					PodsPerCore:      lo.ToPtr[int32](10),
					SystemReserved: v1.ResourceList{
						v1.ResourceCPU:              resource.MustParse("200m"),
						v1.ResourceMemory:           resource.MustParse("200Mi"),
						v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					KubeReserved: v1.ResourceList{
						v1.ResourceCPU:              resource.MustParse("200m"),
						v1.ResourceMemory:           resource.MustParse("200Mi"),
						v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					EvictionHard: map[string]string{
						"memory.available":   "5%",
						"nodefs.available":   "5%",
						"nodefs.inodesFree":  "5%",
						"imagefs.available":  "5%",
						"imagefs.inodesFree": "5%",
						"pid.available":      "3%",
					},
					EvictionSoft: map[string]string{
						"memory.available":   "10%",
						"nodefs.available":   "10%",
						"nodefs.inodesFree":  "10%",
						"imagefs.available":  "10%",
						"imagefs.inodesFree": "10%",
						"pid.available":      "6%",
					},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available":   {Duration: time.Minute * 2},
						"nodefs.available":   {Duration: time.Minute * 2},
						"nodefs.inodesFree":  {Duration: time.Minute * 2},
						"imagefs.available":  {Duration: time.Minute * 2},
						"imagefs.inodesFree": {Duration: time.Minute * 2},
						"pid.available":      {Duration: time.Minute * 2},
					},
					EvictionMaxPodGracePeriod:   lo.ToPtr[int32](120),
					ImageGCHighThresholdPercent: lo.ToPtr[int32](50),
					ImageGCLowThresholdPercent:  lo.ToPtr[int32](10),
				},
				MachineTemplateRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			},
		})
		env.ExpectCreated(nodeTemplate, machine)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		Expect(node.Annotations).To(HaveKeyWithValue("custom-annotation", "custom-value"))
		Expect(node.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
		Expect(node.Spec.Taints).To(ContainElements(
			v1.Taint{
				Key:    "custom-taint",
				Effect: v1.TaintEffectNoSchedule,
				Value:  "custom-value",
			},
			v1.Taint{
				Key:    "other-custom-taint",
				Effect: v1.TaintEffectNoExecute,
				Value:  "other-custom-value",
			},
		))
		Expect(node.OwnerReferences).To(ContainElement(
			metav1.OwnerReference{
				APIVersion:         v1alpha5.SchemeGroupVersion.String(),
				Kind:               "Machine",
				Name:               machine.Name,
				UID:                machine.UID,
				BlockOwnerDeletion: lo.ToPtr(true),
			},
		))
		env.EventuallyExpectCreatedMachineCount("==", 1)
		env.EventuallyExpectMachinesReady(machine)
	})
	It("should remove the cloudProvider machine when the cluster machine is deleted", func() {
		machine := test.Machine(v1alpha5.Machine{
			Spec: v1alpha5.MachineSpec{
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceCategory,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"c"},
					},
				},
				MachineTemplateRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			},
		})
		env.ExpectCreated(nodeTemplate, machine)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		machine = env.EventuallyExpectCreatedMachineCount("==", 1)[0]

		instanceID := env.ExpectParsedProviderID(node.Spec.ProviderID)
		env.GetInstance(node.Name)

		// Node is deleted and now should be not found
		env.ExpectDeleted(machine)
		env.EventuallyExpectNotFound(machine, node)

		Eventually(func(g Gomega) {
			g.Expect(lo.FromPtr(env.GetInstanceByID(instanceID).State.Name)).To(Equal("shutting-down"))
		}, time.Second*10).Should(Succeed())
	})
	It("should delete a machine from the node termination finalizer", func() {
		machine := test.Machine(v1alpha5.Machine{
			Spec: v1alpha5.MachineSpec{
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceCategory,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"c"},
					},
				},
				MachineTemplateRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			},
		})
		env.ExpectCreated(nodeTemplate, machine)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		machine = env.EventuallyExpectCreatedMachineCount("==", 1)[0]

		instanceID := env.ExpectParsedProviderID(node.Spec.ProviderID)
		env.GetInstance(node.Name)

		// Delete the node and expect both the node and machine to be gone as well as the instance to be shutting-down
		env.ExpectDeleted(node)
		env.EventuallyExpectNotFound(machine, node)

		Eventually(func(g Gomega) {
			g.Expect(lo.FromPtr(env.GetInstanceByID(instanceID).State.Name)).To(Equal("shutting-down"))
		}, time.Second*10).Should(Succeed())
	})
	It("should create a machine with custom labels passed through the userData", func() {
		customAMI := env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
		// Update the userData for the instance input with the correct provisionerName
		rawContent, err := os.ReadFile("testdata/al2_userdata_custom_labels_input.sh")
		Expect(err).ToNot(HaveOccurred())

		// Create userData that adds custom labels through the --kubelet-extra-args
		nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyCustom
		nodeTemplate.Spec.AMISelector = map[string]string{"aws-ids": customAMI}
		nodeTemplate.Spec.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), settings.FromContext(env.Context).ClusterName,
			settings.FromContext(env.Context).ClusterEndpoint, env.ExpectCABundle()))))

		machine := test.Machine(v1alpha5.Machine{
			Spec: v1alpha5.MachineSpec{
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceCategory,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"c"},
					},
					{
						Key:      v1.LabelArchStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"amd64"},
					},
				},
				MachineTemplateRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			},
		})
		env.ExpectCreated(nodeTemplate, machine)
		node := env.EventuallyExpectInitializedNodeCount("==", 1)[0]
		Expect(node.Labels).To(HaveKeyWithValue("custom-label", "custom-value"))
		Expect(node.Labels).To(HaveKeyWithValue("custom-label2", "custom-value2"))

		env.EventuallyExpectCreatedMachineCount("==", 1)
		env.EventuallyExpectMachinesReady(machine)
	})
	It("should delete a machine after the registration timeout when the node doesn't register", func() {
		customAMI := env.GetCustomAMI("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", 1)
		// Update the userData for the instance input with the correct provisionerName
		rawContent, err := os.ReadFile("testdata/al2_userdata_input.sh")
		Expect(err).ToNot(HaveOccurred())

		// Create userData that adds custom labels through the --kubelet-extra-args
		nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyCustom
		nodeTemplate.Spec.AMISelector = map[string]string{"aws-ids": customAMI}

		// Giving bad clusterName and clusterEndpoint to the userData
		nodeTemplate.Spec.UserData = lo.ToPtr(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(string(rawContent), "badName", "badEndpoint", env.ExpectCABundle()))))

		machine := test.Machine(v1alpha5.Machine{
			Spec: v1alpha5.MachineSpec{
				Requirements: []v1.NodeSelectorRequirement{
					{
						Key:      v1alpha1.LabelInstanceCategory,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"c"},
					},
					{
						Key:      v1.LabelArchStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{"amd64"},
					},
				},
				MachineTemplateRef: &v1alpha5.MachineTemplateRef{
					Name: nodeTemplate.Name,
				},
			},
		})

		env.ExpectCreated(nodeTemplate, machine)
		machine = env.EventuallyExpectCreatedMachineCount("==", 1)[0]

		// Expect that the machine eventually launches and has false Registration/Initialization
		Eventually(func(g Gomega) {
			temp := &v1alpha5.Machine{}
			g.Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(machine), temp)).To(Succeed())
			g.Expect(temp.StatusConditions().GetCondition(v1alpha5.MachineLaunched).IsTrue()).To(BeTrue())
			g.Expect(temp.StatusConditions().GetCondition(v1alpha5.MachineRegistered).IsFalse()).To(BeTrue())
			g.Expect(temp.StatusConditions().GetCondition(v1alpha5.MachineInitialized).IsFalse()).To(BeTrue())
		}).Should(Succeed())

		// Expect that the machine is eventually de-provisioned due to the registration timeout
		env.EventuallyExpectNotFoundAssertion(machine).WithTimeout(time.Minute * 20).Should(Succeed())
	})
})

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

package machinehydration_test

//var ctx context.Context
//var env *test.Environment
//var hydrationController controller.Controller
//
//func TestAPIs(t *testing.T) {
//	ctx = TestContextWithLogger(t)
//	RegisterFailHandler(Fail)
//	RunSpecs(t, "Machine")
//}
//
//var _ = BeforeSuite(func() {
//	env = test.NewEnvironment(scheme.Scheme, apis.CRDs...)
//	ctx = settings.ToContext(ctx, test.Settings())
//	cp = &fakeCloudProvider{}
//	hydrationController = machinehydration.NewController(env.Client, cp)
//})
//
//var _ = AfterSuite(func() {
//	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
//})
//
//var _ = Describe("MachineHydration", func() {
//	BeforeEach(func() {
//		cp = &fakeCloudProvider{}
//	})
//	AfterEach(func() {
//		ExpectCleanedUp(ctx, env.Client)
//	})
//
//	Context("Success", func() {
//		It("should hydrate a machine from a node", func() {
//			provisioner := test.Provisioner(test.ProvisionerOptions{
//				ProviderRef: &v1alpha5.ProviderRef{
//					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
//					Kind:       "NodeTemplate",
//					Name:       "default",
//				},
//			})
//			node := test.Node(test.NodeOptions{
//				ObjectMeta: metav1.ObjectMeta{
//					Labels: map[string]string{
//						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
//						v1alpha5.LabelNodeInitialized:    "true",
//					},
//				},
//				ProviderID: test.ProviderID(),
//				Taints: []v1.Taint{
//					{
//						Key:    "testkey",
//						Value:  "testvalue",
//						Effect: v1.TaintEffectNoSchedule,
//					},
//				},
//				Allocatable: v1.ResourceList{
//					v1.ResourceCPU:              resource.MustParse("1"),
//					v1.ResourceMemory:           resource.MustParse("1Mi"),
//					v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
//				},
//				Capacity: v1.ResourceList{
//					v1.ResourceCPU:              resource.MustParse("2"),
//					v1.ResourceMemory:           resource.MustParse("2Mi"),
//					v1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
//				},
//			})
//			ExpectApplied(ctx, env.Client, provisioner, node)
//			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
//
//			machineList := &v1alpha5.MachineList{}
//			Expect(env.Client.List(ctx, machineList)).To(Succeed())
//			Expect(machineList.Items).To(HaveLen(1))
//			machine := machineList.Items[0]
//
//			// Expect machine to have populated fields from the node
//			Expect(machine.Spec.Taints).To(Equal(provisioner.Spec.Taints))
//			Expect(machine.Spec.MachineTemplateRef.APIVersion).To(Equal(provisioner.Spec.ProviderRef.APIVersion))
//			Expect(machine.Spec.MachineTemplateRef.Kind).To(Equal(provisioner.Spec.ProviderRef.Kind))
//			Expect(machine.Spec.MachineTemplateRef.Name).To(Equal(provisioner.Spec.ProviderRef.Name))
//		})
//		It("should hydrate a machine with expected requirements from node labels", func() {
//			provisioner := test.Provisioner(test.ProvisionerOptions{
//				ProviderRef: &v1alpha5.ProviderRef{
//					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
//					Kind:       "NodeTemplate",
//					Name:       "default",
//				},
//			})
//			node := test.Node(test.NodeOptions{
//				ObjectMeta: metav1.ObjectMeta{
//					Labels: map[string]string{
//						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
//						v1alpha5.LabelNodeInitialized:    "true",
//						v1.LabelInstanceTypeStable:       "default-instance-type",
//						v1.LabelTopologyRegion:           "test-zone-1",
//						v1.LabelTopologyZone:             "test-zone-1a",
//						v1.LabelOSStable:                 string(v1.Linux),
//						v1.LabelArchStable:               "amd64",
//					},
//				},
//				ProviderID: test.ProviderID(),
//			})
//			ExpectApplied(ctx, env.Client, provisioner, node)
//			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
//
//			machineList := &v1alpha5.MachineList{}
//			Expect(env.Client.List(ctx, machineList)).To(Succeed())
//			Expect(machineList.Items).To(HaveLen(1))
//			machine := machineList.Items[0]
//
//			Expect(machine.Spec.Requirements).To(HaveLen(6))
//			Expect(machine.Spec.Requirements).To(ContainElements(
//				v1.NodeSelectorRequirement{
//					Key:      v1alpha5.ProvisionerNameLabelKey,
//					Operator: v1.NodeSelectorOpIn,
//					Values:   []string{provisioner.Name},
//				},
//				v1.NodeSelectorRequirement{
//					Key:      v1.LabelInstanceTypeStable,
//					Operator: v1.NodeSelectorOpIn,
//					Values:   []string{"default-instance-type"},
//				},
//				v1.NodeSelectorRequirement{
//					Key:      v1.LabelTopologyRegion,
//					Operator: v1.NodeSelectorOpIn,
//					Values:   []string{"test-zone-1"},
//				},
//				v1.NodeSelectorRequirement{
//					Key:      v1.LabelTopologyZone,
//					Operator: v1.NodeSelectorOpIn,
//					Values:   []string{"test-zone-1a"},
//				},
//				v1.NodeSelectorRequirement{
//					Key:      v1.LabelOSStable,
//					Operator: v1.NodeSelectorOpIn,
//					Values:   []string{string(v1.Linux)},
//				},
//				v1.NodeSelectorRequirement{
//					Key:      v1.LabelArchStable,
//					Operator: v1.NodeSelectorOpIn,
//					Values:   []string{v1alpha5.ArchitectureAmd64},
//				},
//			))
//		})
//		It("should hydrate a machine with expected kubelet from provisioner kubelet configuration", func() {
//			provisioner := test.Provisioner(test.ProvisionerOptions{
//				ProviderRef: &v1alpha5.ProviderRef{
//					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
//					Kind:       "NodeTemplate",
//					Name:       "default",
//				},
//				Kubelet: &v1alpha5.KubeletConfiguration{
//					ClusterDNS:       []string{"10.0.0.1"},
//					ContainerRuntime: lo.ToPtr("containerd"),
//					MaxPods:          lo.ToPtr[int32](10),
//				},
//			})
//			node := test.Node(test.NodeOptions{
//				ObjectMeta: metav1.ObjectMeta{
//					Labels: map[string]string{
//						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
//						v1alpha5.LabelNodeInitialized:    "true",
//					},
//				},
//				ProviderID: test.ProviderID(),
//				Taints: []v1.Taint{
//					{
//						Key:    "testkey",
//						Value:  "testvalue",
//						Effect: v1.TaintEffectNoSchedule,
//					},
//				},
//				Allocatable: v1.ResourceList{
//					v1.ResourceCPU:              resource.MustParse("1"),
//					v1.ResourceMemory:           resource.MustParse("1Mi"),
//					v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
//				},
//				Capacity: v1.ResourceList{
//					v1.ResourceCPU:              resource.MustParse("2"),
//					v1.ResourceMemory:           resource.MustParse("2Mi"),
//					v1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
//				},
//			})
//			ExpectApplied(ctx, env.Client, provisioner, node)
//			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
//
//			machineList := &v1alpha5.MachineList{}
//			Expect(env.Client.List(ctx, machineList)).To(Succeed())
//			Expect(machineList.Items).To(HaveLen(1))
//			machine := machineList.Items[0]
//
//			Expect(machine.Spec.Kubelet).ToNot(BeNil())
//			Expect(machine.Spec.Kubelet.ClusterDNS[0]).To(Equal("10.0.0.1"))
//			Expect(lo.FromPtr(machine.Spec.Kubelet.ContainerRuntime)).To(Equal("containerd"))
//			Expect(lo.FromPtr(machine.Spec.Kubelet.MaxPods)).To(BeNumerically("==", 10))
//		})
//		It("should hydrate many machines from many nodes", func() {
//			provisioner := test.Provisioner(test.ProvisionerOptions{
//				ProviderRef: &v1alpha5.ProviderRef{
//					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
//					Kind:       "NodeTemplate",
//					Name:       "default",
//				},
//			})
//			ExpectApplied(ctx, env.Client, provisioner)
//
//			var nodes []*v1.Node
//			for i := 0; i < 1000; i++ {
//				node := test.Node(test.NodeOptions{
//					ObjectMeta: metav1.ObjectMeta{
//						Labels: map[string]string{
//							v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
//							v1alpha5.LabelNodeInitialized:    "true",
//						},
//					},
//					ProviderID: test.ProviderID(),
//					Allocatable: v1.ResourceList{
//						v1.ResourceCPU: resource.MustParse("1"),
//					},
//					Capacity: v1.ResourceList{
//						v1.ResourceCPU: resource.MustParse("2"),
//					},
//				})
//				ExpectApplied(ctx, env.Client, node)
//				ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
//				nodes = append(nodes, node)
//			}
//
//			machineList := &v1alpha5.MachineList{}
//			Expect(env.Client.List(ctx, machineList)).To(Succeed())
//			Expect(machineList.Items).To(HaveLen(1000))
//
//			providerIDMap := lo.SliceToMap(machineList.Items, func(m v1alpha5.Machine) (string, *v1alpha5.Machine) {
//				return m.Status.ProviderID, &m
//			})
//			for _, node := range nodes {
//				_, ok := providerIDMap[node.Spec.ProviderID]
//				Expect(ok).To(BeTrue())
//			}
//		})
//		It("should not hydrate a machine for a node that is already hydrated", func() {
//			provisioner := test.Provisioner(test.ProvisionerOptions{
//				ProviderRef: &v1alpha5.ProviderRef{
//					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
//					Kind:       "NodeTemplate",
//					Name:       "default",
//				},
//			})
//			node := test.Node(test.NodeOptions{
//				ObjectMeta: metav1.ObjectMeta{
//					Labels: map[string]string{
//						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
//						v1alpha5.LabelNodeInitialized:    "true",
//					},
//				},
//				ProviderID: test.ProviderID(),
//			})
//			m := test.Machine(v1alpha5.Machine{
//				Status: v1alpha5.MachineStatus{
//					ProviderID: node.Spec.ProviderID, // Same providerID as the node
//				},
//			})
//			ExpectApplied(ctx, env.Client, provisioner, node, m)
//
//			machineList := &v1alpha5.MachineList{}
//			Expect(env.Client.List(ctx, machineList)).To(Succeed())
//			Expect(machineList.Items).To(HaveLen(1))
//
//			// Expect that we go to hydrate machines, and we don't add extra machines for the existing one
//			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
//			Expect(env.Client.List(ctx, machineList)).To(Succeed())
//			Expect(machineList.Items).To(HaveLen(1))
//		})
//		It("should pull the hydrated machine's name from the HydrateMachine cloudProvider call", func() {
//			provisioner := test.Provisioner(test.ProvisionerOptions{
//				ProviderRef: &v1alpha5.ProviderRef{
//					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
//					Kind:       "NodeTemplate",
//					Name:       "default",
//				},
//			})
//			node := test.Node(test.NodeOptions{
//				ObjectMeta: metav1.ObjectMeta{
//					Labels: map[string]string{
//						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
//						v1alpha5.LabelNodeInitialized:    "true",
//					},
//				},
//				ProviderID: test.ProviderID(),
//			})
//			ExpectApplied(ctx, env.Client, provisioner, node)
//
//			expectedName := "my-custom-machine"
//
//			cp.HydrateMachineAssertions = []func(context.Context, *v1alpha5.Machine) error{
//				func(_ context.Context, in *v1alpha5.Machine) error {
//					in.Name = expectedName // Name of this machine is the same as the existing one
//					return nil
//				},
//			}
//
//			// Expect that we go to hydrate machines, and we don't add extra machines for the existing one
//			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
//			machineList := &v1alpha5.MachineList{}
//			Expect(env.Client.List(ctx, machineList)).To(Succeed())
//			Expect(machineList.Items).To(HaveLen(1))
//			machine := machineList.Items[0]
//
//			// Expect that we hydrated the machine based on the cloudProvider response
//			Expect(machine.Status.ProviderID).To(Equal(node.Spec.ProviderID))
//			Expect(machine.Name).To(Equal(expectedName))
//		})
//	})
//	Context("Failure", func() {
//		It("should not hydrate a node without a provisioner label", func() {
//			provisioner := test.Provisioner(test.ProvisionerOptions{
//				ProviderRef: &v1alpha5.ProviderRef{
//					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
//					Kind:       "NodeTemplate",
//					Name:       "default",
//				},
//			})
//			node := test.Node(test.NodeOptions{
//				ObjectMeta: metav1.ObjectMeta{
//					Labels: map[string]string{
//						v1alpha5.LabelNodeInitialized: "true",
//					},
//				},
//				ProviderID: test.ProviderID(),
//			})
//			ExpectApplied(ctx, env.Client, provisioner, node)
//			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
//
//			machineList := &v1alpha5.MachineList{}
//			Expect(env.Client.List(ctx, machineList)).To(Succeed())
//			Expect(machineList.Items).To(HaveLen(0))
//		})
//		It("should not hydrate a node without a provisioner that exists", func() {
//			node := test.Node(test.NodeOptions{
//				ObjectMeta: metav1.ObjectMeta{
//					Labels: map[string]string{
//						v1alpha5.ProvisionerNameLabelKey: "default",
//						v1alpha5.LabelNodeInitialized:    "true",
//					},
//				},
//				ProviderID: test.ProviderID(),
//			})
//			ExpectApplied(ctx, env.Client, node)
//			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
//
//			machineList := &v1alpha5.MachineList{}
//			Expect(env.Client.List(ctx, machineList)).To(Succeed())
//			Expect(machineList.Items).To(HaveLen(0))
//		})
//	})
//})

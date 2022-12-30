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

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "knative.dev/pkg/logging/testing"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	corecloudprovider "github.com/aws/karpenter-core/pkg/cloudprovider"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter-core/pkg/apis"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	awscache "github.com/aws/karpenter/pkg/cache"
	"github.com/aws/karpenter/pkg/cloudprovider"
	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/controllers/machinehydration"
	"github.com/aws/karpenter/pkg/fake"
)

var ctx context.Context
var env *coretest.Environment
var unavailableOfferingsCache *awscache.UnavailableOfferings
var ec2API *fake.EC2API
var cloudProvider *cloudprovider.CloudProvider
var hydrationController controller.Controller

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Machine")
}

var _ = BeforeSuite(func() {
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...), coretest.WithFieldIndexers(func(c cache.Cache) error {
		return c.IndexField(ctx, &v1alpha5.Machine{}, "status.providerID", func(o client.Object) []string {
			machine := o.(*v1alpha5.Machine)
			return []string{machine.Status.ProviderID}
		})
	}))
	unavailableOfferingsCache = awscache.NewUnavailableOfferings()
	ec2API = &fake.EC2API{}
	cloudProvider = cloudprovider.New(awscontext.Context{
		Context: corecloudprovider.Context{
			Context:             ctx,
			RESTConfig:          env.Config,
			KubernetesInterface: env.KubernetesInterface,
			KubeClient:          env.Client,
			EventRecorder:       coretest.NewEventRecorder(),
			Clock:               &clock.FakeClock{},
			StartAsync:          nil,
		},
		Session:                   mock.Session,
		UnavailableOfferingsCache: unavailableOfferingsCache,
		EC2API:                    ec2API,
	})
	hydrationController = machinehydration.NewController(env.IndexedClient, cloudProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("MachineHydration", func() {
	BeforeEach(func() {
		ec2API.Reset()

	})
	AfterEach(func() {
		EventuallyExpectIndexedClientCleanedUp(ctx, env.Client, env.IndexedClient)
	})

	Context("Success", func() {
		It("should hydrate a machine from a node", func() {
			provisioner := coretest.Provisioner(coretest.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{
					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
					Kind:       "NodeTemplate",
					Name:       "default",
				},
			})
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1alpha5.LabelNodeInitialized:    "true",
					},
				},
				ProviderID: fake.RandomProviderID(),
				Taints: []v1.Taint{
					{
						Key:    "testkey",
						Value:  "testvalue",
						Effect: v1.TaintEffectNoSchedule,
					},
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("1"),
					v1.ResourceMemory:           resource.MustParse("1Mi"),
					v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
				},
				Capacity: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("2"),
					v1.ResourceMemory:           resource.MustParse("2Mi"),
					v1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			})
			ExpectApplied(ctx, env.IndexedClient, provisioner, node)
			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))

			machineList := &v1alpha5.MachineList{}
			Expect(env.Client.List(ctx, machineList)).To(Succeed())
			Expect(machineList.Items).To(HaveLen(1))
			machine := machineList.Items[0]

			// Expect machine to have populated fields from the node
			Expect(machine.Spec.Taints).To(Equal(provisioner.Spec.Taints))
			Expect(machine.Spec.MachineTemplateRef.APIVersion).To(Equal(provisioner.Spec.ProviderRef.APIVersion))
			Expect(machine.Spec.MachineTemplateRef.Kind).To(Equal(provisioner.Spec.ProviderRef.Kind))
			Expect(machine.Spec.MachineTemplateRef.Name).To(Equal(provisioner.Spec.ProviderRef.Name))
		})
		It("should hydrate a machine with expected requirements from node labels", func() {
			provisioner := coretest.Provisioner(coretest.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{
					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
					Kind:       "NodeTemplate",
					Name:       "default",
				},
			})
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1alpha5.LabelNodeInitialized:    "true",
						v1.LabelInstanceTypeStable:       "default-instance-type",
						v1.LabelTopologyRegion:           "coretest-zone-1",
						v1.LabelTopologyZone:             "coretest-zone-1a",
						v1.LabelOSStable:                 string(v1.Linux),
						v1.LabelArchStable:               "amd64",
					},
				},
				ProviderID: fake.RandomProviderID(),
			})
			ExpectApplied(ctx, env.IndexedClient, provisioner, node)
			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))

			machineList := &v1alpha5.MachineList{}
			Expect(env.Client.List(ctx, machineList)).To(Succeed())
			Expect(machineList.Items).To(HaveLen(1))
			machine := machineList.Items[0]

			Expect(machine.Spec.Requirements).To(HaveLen(6))
			Expect(machine.Spec.Requirements).To(ContainElements(
				v1.NodeSelectorRequirement{
					Key:      v1alpha5.ProvisionerNameLabelKey,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{provisioner.Name},
				},
				v1.NodeSelectorRequirement{
					Key:      v1.LabelInstanceTypeStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"default-instance-type"},
				},
				v1.NodeSelectorRequirement{
					Key:      v1.LabelTopologyRegion,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"coretest-zone-1"},
				},
				v1.NodeSelectorRequirement{
					Key:      v1.LabelTopologyZone,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{"coretest-zone-1a"},
				},
				v1.NodeSelectorRequirement{
					Key:      v1.LabelOSStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{string(v1.Linux)},
				},
				v1.NodeSelectorRequirement{
					Key:      v1.LabelArchStable,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.ArchitectureAmd64},
				},
			))
		})
		It("should hydrate a machine with expected kubelet from provisioner kubelet configuration", func() {
			provisioner := coretest.Provisioner(coretest.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{
					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
					Kind:       "NodeTemplate",
					Name:       "default",
				},
				Kubelet: &v1alpha5.KubeletConfiguration{
					ClusterDNS:       []string{"10.0.0.1"},
					ContainerRuntime: lo.ToPtr("containerd"),
					MaxPods:          lo.ToPtr[int32](10),
				},
			})
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1alpha5.LabelNodeInitialized:    "true",
					},
				},
				ProviderID: fake.RandomProviderID(),
				Taints: []v1.Taint{
					{
						Key:    "testkey",
						Value:  "testvalue",
						Effect: v1.TaintEffectNoSchedule,
					},
				},
				Allocatable: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("1"),
					v1.ResourceMemory:           resource.MustParse("1Mi"),
					v1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
				},
				Capacity: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("2"),
					v1.ResourceMemory:           resource.MustParse("2Mi"),
					v1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			})
			ExpectApplied(ctx, env.IndexedClient, provisioner, node)
			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))

			machineList := &v1alpha5.MachineList{}
			Expect(env.Client.List(ctx, machineList)).To(Succeed())
			Expect(machineList.Items).To(HaveLen(1))
			machine := machineList.Items[0]

			Expect(machine.Spec.Kubelet).ToNot(BeNil())
			Expect(machine.Spec.Kubelet.ClusterDNS[0]).To(Equal("10.0.0.1"))
			Expect(lo.FromPtr(machine.Spec.Kubelet.ContainerRuntime)).To(Equal("containerd"))
			Expect(lo.FromPtr(machine.Spec.Kubelet.MaxPods)).To(BeNumerically("==", 10))
		})
		It("should hydrate many machines from many nodes", func() {
			provisioner := coretest.Provisioner(coretest.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{
					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
					Kind:       "NodeTemplate",
					Name:       "default",
				},
			})
			ExpectApplied(ctx, env.Client, provisioner)

			var nodes []*v1.Node
			for i := 0; i < 1000; i++ {
				node := coretest.Node(coretest.NodeOptions{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
							v1alpha5.LabelNodeInitialized:    "true",
						},
					},
					ProviderID: fake.RandomProviderID(),
					Allocatable: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("1"),
					},
					Capacity: v1.ResourceList{
						v1.ResourceCPU: resource.MustParse("2"),
					},
				})
				ExpectApplied(ctx, env.IndexedClient, node)
				ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
				nodes = append(nodes, node)
			}

			machineList := &v1alpha5.MachineList{}
			Expect(env.Client.List(ctx, machineList)).To(Succeed())
			Expect(machineList.Items).To(HaveLen(1000))

			providerIDMap := lo.SliceToMap(machineList.Items, func(m v1alpha5.Machine) (string, *v1alpha5.Machine) {
				return m.Status.ProviderID, &m
			})
			for _, node := range nodes {
				_, ok := providerIDMap[node.Spec.ProviderID]
				Expect(ok).To(BeTrue())
			}
		})
		It("should not hydrate a machine for a node that is already hydrated", func() {
			provisioner := coretest.Provisioner(coretest.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{
					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
					Kind:       "NodeTemplate",
					Name:       "default",
				},
			})
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1alpha5.LabelNodeInitialized:    "true",
					},
				},
				ProviderID: fake.RandomProviderID(),
			})
			m := coretest.Machine(v1alpha5.Machine{
				Status: v1alpha5.MachineStatus{
					ProviderID: node.Spec.ProviderID, // Same providerID as the node
				},
			})
			ExpectApplied(ctx, env.IndexedClient, provisioner, node, m)

			machineList := &v1alpha5.MachineList{}
			Expect(env.Client.List(ctx, machineList)).To(Succeed())
			Expect(machineList.Items).To(HaveLen(1))

			// Expect that we go to hydrate machines, and we don't add extra machines for the existing one
			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
			Expect(env.Client.List(ctx, machineList)).To(Succeed())
			Expect(machineList.Items).To(HaveLen(1))
		})
		It("should pull the hydrated machine's name from the HydrateMachine cloudProvider call", func() {
			provisioner := coretest.Provisioner(coretest.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{
					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
					Kind:       "NodeTemplate",
					Name:       "default",
				},
			})
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
						v1alpha5.LabelNodeInitialized:    "true",
					},
				},
				ProviderID: fake.RandomProviderID(),
			})
			ExpectApplied(ctx, env.IndexedClient, provisioner, node)

			expectedName := "my-custom-machine"

			// Set the DescribeInstancesOutput to return an instance with a MachineName label
			ec2API.DescribeInstancesOutput.Set(&ec2.DescribeInstancesOutput{
				Reservations: []*ec2.Reservation{
					{
						Instances: []*ec2.Instance{
							{
								Tags: []*ec2.Tag{
									{
										Key:   aws.String(v1alpha5.MachineNameLabelKey),
										Value: aws.String(expectedName),
									},
								},
							},
						},
					},
				},
			})

			// Expect that we go to hydrate machines, and we don't add extra machines for the existing one
			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))
			machineList := &v1alpha5.MachineList{}
			Expect(env.Client.List(ctx, machineList)).To(Succeed())
			Expect(machineList.Items).To(HaveLen(1))
			machine := machineList.Items[0]

			// Expect that we hydrated the machine based on the cloudProvider response
			Expect(machine.Status.ProviderID).To(Equal(node.Spec.ProviderID))
			Expect(machine.Name).To(Equal(expectedName))
		})
	})
	Context("Failure", func() {
		It("should not hydrate a node without a provisioner label", func() {
			provisioner := coretest.Provisioner(coretest.ProvisionerOptions{
				ProviderRef: &v1alpha5.ProviderRef{
					APIVersion: v1alpha5.TestingGroup + "v1alpha1",
					Kind:       "NodeTemplate",
					Name:       "default",
				},
			})
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.LabelNodeInitialized: "true",
					},
				},
				ProviderID: fake.RandomProviderID(),
			})
			ExpectApplied(ctx, env.IndexedClient, provisioner, node)
			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))

			machineList := &v1alpha5.MachineList{}
			Expect(env.Client.List(ctx, machineList)).To(Succeed())
			Expect(machineList.Items).To(HaveLen(0))
		})
		It("should not hydrate a node without a provisioner that exists", func() {
			node := coretest.Node(coretest.NodeOptions{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1alpha5.ProvisionerNameLabelKey: "default",
						v1alpha5.LabelNodeInitialized:    "true",
					},
				},
				ProviderID: fake.RandomProviderID(),
			})
			ExpectApplied(ctx, env.IndexedClient, node)
			ExpectReconcileSucceeded(ctx, hydrationController, client.ObjectKeyFromObject(node))

			machineList := &v1alpha5.MachineList{}
			Expect(env.Client.List(ctx, machineList)).To(Succeed())
			Expect(machineList.Items).To(HaveLen(0))
		})
	})
})

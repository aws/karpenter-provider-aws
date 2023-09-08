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

package link_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	. "knative.dev/pkg/logging/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/events"
	"github.com/aws/karpenter-core/pkg/operator/controller"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/nodeclaim/link"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils"
)

var ctx context.Context
var awsEnv *test.Environment
var env *coretest.Environment
var linkController controller.Controller
var cloudProvider *cloudprovider.CloudProvider

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Machine")
}

var _ = BeforeSuite(func() {
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	awsEnv = test.NewEnvironment(ctx, env)
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.SubnetProvider)
	linkController = link.NewController(env.Client, cloudProvider)
})
var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	awsEnv.Reset()
})

var _ = Describe("MachineLink", func() {
	var instanceID string
	var providerID string
	var provisioner *v1alpha5.Provisioner
	var nodeTemplate *v1alpha1.AWSNodeTemplate

	BeforeEach(func() {
		instanceID = fake.InstanceID()
		providerID = fmt.Sprintf("aws:///test-zone-1a/%s", instanceID)
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{})
		provisioner = test.Provisioner(coretest.ProvisionerOptions{
			ProviderRef: &v1alpha5.MachineTemplateRef{
				APIVersion: "testing/v1alpha1",
				Kind:       "NodeTemplate",
				Name:       nodeTemplate.Name,
			},
		})

		// Store the instance as existing at DescribeInstances
		awsEnv.EC2API.Instances.Store(
			instanceID,
			&ec2.Instance{
				State: &ec2.InstanceState{
					Name: aws.String(ec2.InstanceStateNameRunning),
				},
				Tags: []*ec2.Tag{
					{
						Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", settings.FromContext(ctx).ClusterName)),
						Value: aws.String("owned"),
					},
					{
						Key:   aws.String(v1alpha5.ProvisionerNameLabelKey),
						Value: aws.String(provisioner.Name),
					},
				},
				PrivateDnsName: aws.String(fake.PrivateDNSName()),
				Placement: &ec2.Placement{
					AvailabilityZone: aws.String("test-zone-1a"),
				},
				InstanceId:   aws.String(instanceID),
				InstanceType: aws.String("m5.large"),
			},
		)
	})
	AfterEach(func() {
		ExpectCleanedUp(ctx, env.Client)
	})

	It("should link an instance with basic spec set", func() {
		provisioner.Spec.Taints = []v1.Taint{
			{
				Key:    "testkey",
				Value:  "testvalue",
				Effect: v1.TaintEffectNoSchedule,
			},
		}
		provisioner.Spec.StartupTaints = []v1.Taint{
			{
				Key:    "othertestkey",
				Value:  "othertestvalue",
				Effect: v1.TaintEffectNoExecute,
			},
		}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		ExpectInstanceExists(awsEnv.EC2API, instanceID)
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(1))
		machine := machineList.Items[0]

		// Expect machine to have populated fields from the node
		Expect(machine.Spec.Taints).To(Equal(provisioner.Spec.Taints))
		Expect(machine.Spec.StartupTaints).To(Equal(provisioner.Spec.StartupTaints))
		Expect(machine.Spec.MachineTemplateRef.Kind).To(Equal(provisioner.Spec.ProviderRef.Kind))
		Expect(machine.Spec.MachineTemplateRef.Name).To(Equal(provisioner.Spec.ProviderRef.Name))

		// Expect machine has linking annotation to get machine details
		Expect(machine.Annotations).To(HaveKeyWithValue(v1alpha5.MachineLinkedAnnotationKey, providerID))
		instance := ExpectInstanceExists(awsEnv.EC2API, instanceID)
		ExpectManagedByTagExists(instance)
	})
	It("should link and instance with expected requirements and labels", func() {
		provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{
			{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-1a", "test-zone-1b", "test-zone-1c"},
			},
			{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{string(v1.Linux), string(v1.Windows)},
			},
			{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			},
		}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		ExpectInstanceExists(awsEnv.EC2API, instanceID)
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(1))
		machine := machineList.Items[0]

		Expect(machine.Spec.Requirements).To(HaveLen(3))
		Expect(machine.Spec.Requirements).To(ContainElements(
			v1.NodeSelectorRequirement{
				Key:      v1.LabelTopologyZone,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{"test-zone-1a", "test-zone-1b", "test-zone-1c"},
			},
			v1.NodeSelectorRequirement{
				Key:      v1.LabelOSStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{string(v1.Linux), string(v1.Windows)},
			},
			v1.NodeSelectorRequirement{
				Key:      v1.LabelArchStable,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v1alpha5.ArchitectureAmd64},
			},
		))

		// Expect machine has linking annotation to get machine details
		Expect(machine.Annotations).To(HaveKeyWithValue(v1alpha5.MachineLinkedAnnotationKey, providerID))
		instance := ExpectInstanceExists(awsEnv.EC2API, instanceID)
		ExpectManagedByTagExists(instance)
	})
	It("should link an instance with expected kubelet from provisioner kubelet configuration", func() {
		provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
			ClusterDNS:       []string{"10.0.0.1"},
			ContainerRuntime: lo.ToPtr("containerd"),
			MaxPods:          lo.ToPtr[int32](10),
		}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(1))
		machine := machineList.Items[0]

		Expect(machine.Spec.Kubelet).ToNot(BeNil())
		Expect(machine.Spec.Kubelet.ClusterDNS[0]).To(Equal("10.0.0.1"))
		Expect(lo.FromPtr(machine.Spec.Kubelet.ContainerRuntime)).To(Equal("containerd"))
		Expect(lo.FromPtr(machine.Spec.Kubelet.MaxPods)).To(BeNumerically("==", 10))

		// Expect machine has linking annotation to get machine details
		Expect(machine.Annotations).To(HaveKeyWithValue(v1alpha5.MachineLinkedAnnotationKey, providerID))
		instance := ExpectInstanceExists(awsEnv.EC2API, instanceID)
		ExpectManagedByTagExists(instance)
	})
	It("should link many instances to many machines", func() {
		awsEnv.EC2API.Reset() // Reset so we don't store the extra instance
		ExpectApplied(ctx, env.Client, provisioner)

		// Generate 100 instances that have different instanceIDs
		var ids []string
		for i := 0; i < 100; i++ {
			instanceID = fake.InstanceID()
			awsEnv.EC2API.EC2Behavior.Instances.Store(
				instanceID,
				&ec2.Instance{
					State: &ec2.InstanceState{
						Name: aws.String(ec2.InstanceStateNameRunning),
					},
					Tags: []*ec2.Tag{
						{
							Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", settings.FromContext(ctx).ClusterName)),
							Value: aws.String("owned"),
						},
						{
							Key:   aws.String(v1alpha5.ProvisionerNameLabelKey),
							Value: aws.String(provisioner.Name),
						},
					},
					PrivateDnsName: aws.String(fake.PrivateDNSName()),
					Placement: &ec2.Placement{
						AvailabilityZone: aws.String("test-zone-1a"),
					},
					InstanceId:   aws.String(instanceID),
					InstanceType: aws.String("m5.large"),
				},
			)
			ids = append(ids, instanceID)
		}

		// Generate a reconcile loop to link the machines
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(100))

		machineInstanceIDs := sets.NewString(lo.Map(machineList.Items, func(m v1alpha5.Machine, _ int) string {
			return lo.Must(utils.ParseInstanceID(m.Annotations[v1alpha5.MachineLinkedAnnotationKey]))
		})...)

		Expect(machineInstanceIDs).To(HaveLen(len(ids)))
		for _, id := range ids {
			Expect(machineInstanceIDs.Has(id)).To(BeTrue())
			instance := ExpectInstanceExists(awsEnv.EC2API, id)
			ExpectManagedByTagExists(instance)
		}
	})
	It("should link an instance using provider and no providerRef", func() {
		raw := &runtime.RawExtension{}
		lo.Must0(raw.UnmarshalJSON(lo.Must(json.Marshal(v1alpha1.AWS{
			AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
			SubnetSelector:        map[string]string{"*": "*"},
			SecurityGroupSelector: map[string]string{"*": "*"},
		}))))
		provisioner.Spec.ProviderRef = nil
		provisioner.Spec.Provider = raw

		ExpectApplied(ctx, env.Client, provisioner)
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(1))
		machine := machineList.Items[0]
		Expect(machine.Annotations).To(HaveKey(v1alpha5.ProviderCompatabilityAnnotationKey))

		// Expect machine has linking annotation to get machine details
		Expect(machine.Annotations).To(HaveKeyWithValue(v1alpha5.MachineLinkedAnnotationKey, providerID))
		instance := ExpectInstanceExists(awsEnv.EC2API, instanceID)
		ExpectManagedByTagExists(instance)
	})
	It("should link an instance without node template existence", func() {
		// No node template has been applied here
		ExpectApplied(ctx, env.Client, provisioner)
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(1))
		machine := machineList.Items[0]
		Expect(machine.Spec.MachineTemplateRef.Kind).To(Equal(provisioner.Spec.ProviderRef.Kind))
		Expect(machine.Spec.MachineTemplateRef.Name).To(Equal(provisioner.Spec.ProviderRef.Name))

		// Expect machine has linking annotation to get machine details
		Expect(machine.Annotations).To(HaveKeyWithValue(v1alpha5.MachineLinkedAnnotationKey, providerID))
		instance := ExpectInstanceExists(awsEnv.EC2API, instanceID)
		ExpectManagedByTagExists(instance)
	})
	It("should link an instance that was re-owned with a provisioner-name label", func() {
		awsEnv.EC2API.Reset() // Reset so we don't store the extra instance

		// Don't include the provisioner-name tag
		awsEnv.EC2API.EC2Behavior.Instances.Store(
			instanceID,
			&ec2.Instance{
				State: &ec2.InstanceState{
					Name: aws.String(ec2.InstanceStateNameRunning),
				},
				Tags: []*ec2.Tag{
					{
						Key:   aws.String(fmt.Sprintf("kubernetes.io/cluster/%s", settings.FromContext(ctx).ClusterName)),
						Value: aws.String("owned"),
					},
				},
				PrivateDnsName: aws.String(fake.PrivateDNSName()),
				Placement: &ec2.Placement{
					AvailabilityZone: aws.String("test-zone-1a"),
				},
				InstanceId:   aws.String(instanceID),
				InstanceType: aws.String("m5.large"),
			},
		)
		node := coretest.Node(coretest.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					v1alpha5.ProvisionerNameLabelKey: provisioner.Name,
				},
			},
			ProviderID: providerID,
		})
		ExpectApplied(ctx, env.Client, node, provisioner, nodeTemplate)
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(1))
		machine := machineList.Items[0]
		Expect(machine.Annotations).To(HaveKeyWithValue(v1alpha5.MachineLinkedAnnotationKey, providerID))
	})
	It("should not link an instance without a provisioner tag", func() {
		instance := ExpectInstanceExists(awsEnv.EC2API, instanceID)
		instance.Tags = lo.Reject(instance.Tags, func(t *ec2.Tag, _ int) bool {
			return aws.StringValue(t.Key) == v1alpha5.ProvisionerNameLabelKey
		})
		awsEnv.EC2API.Instances.Store(instanceID, instance)

		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(0))
	})
	It("should not link an instance without a provisioner that exists on the cluster", func() {
		// No provisioner has been applied here
		ExpectApplied(ctx, env.Client, nodeTemplate)
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(0))

		// Expect that the instance was left alone if the provisioner wasn't found
		ExpectInstanceExists(awsEnv.EC2API, instanceID)
	})
	It("should not link an instance for an instance that is already linked", func() {
		m := coretest.Machine(v1alpha5.Machine{
			Status: v1alpha5.MachineStatus{
				ProviderID: providerID,
			},
		})
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate, m)

		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(1))

		// Expect that we go to link machines, and we don't add extra machines from the existing one
		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(1))
	})
	It("should not link an instance that is terminated", func() {
		// Update the state of the existing instance
		instance := ExpectInstanceExists(awsEnv.EC2API, instanceID)
		instance.State.Name = aws.String(ec2.InstanceStateNameTerminated)
		awsEnv.EC2API.Instances.Store(instanceID, instance)

		ExpectReconcileSucceeded(ctx, linkController, client.ObjectKey{})
		machineList := &v1alpha5.MachineList{}
		Expect(env.Client.List(ctx, machineList)).To(Succeed())
		Expect(machineList.Items).To(HaveLen(0))
	})
})

func ExpectInstanceExists(api *fake.EC2API, instanceID string) *ec2.Instance {
	raw, ok := api.Instances.Load(instanceID)
	Expect(ok).To(BeTrue())
	return raw.(*ec2.Instance)
}

func ExpectManagedByTagExists(instance *ec2.Instance) *ec2.Tag {
	tag, ok := lo.Find(instance.Tags, func(t *ec2.Tag) bool {
		return aws.StringValue(t.Key) == v1alpha5.MachineManagedByAnnotationKey
	})
	Expect(ok).To(BeTrue())
	return tag
}

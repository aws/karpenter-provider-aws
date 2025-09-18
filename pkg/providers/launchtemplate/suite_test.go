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

package launchtemplate_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	admv1alpha1 "github.com/awslabs/amazon-eks-ami/nodeadm/api/v1alpha1"
	"github.com/awslabs/operatorpkg/object"
	opstatus "github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	corecloudprovider "sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning"
	"sigs.k8s.io/karpenter/pkg/controllers/state"
	"sigs.k8s.io/karpenter/pkg/events"
	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/cloudprovider"
	"github.com/aws/karpenter-provider-aws/pkg/controllers/nodeclass"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter-provider-aws/pkg/providers/amifamily/bootstrap/mime"
	"github.com/aws/karpenter-provider-aws/pkg/providers/instancetype"
	"github.com/aws/karpenter-provider-aws/pkg/providers/launchtemplate"
	"github.com/aws/karpenter-provider-aws/pkg/test"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var fakeClock *clock.FakeClock
var prov *provisioning.Provisioner
var cluster *state.Cluster
var cloudProvider *cloudprovider.CloudProvider
var recorder events.Recorder

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "LaunchTemplateProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(
		coretest.WithCRDs(test.DisableCapacityReservationIDValidation(apis.CRDs)...),
		coretest.WithCRDs(v1alpha1.CRDs...),
		coretest.WithFieldIndexers(coretest.NodePoolNodeClassRefFieldIndexer(ctx)),
	)
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)

	fakeClock = &clock.FakeClock{}
	recorder = events.NewRecorder(&record.FakeRecorder{})
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, recorder,
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.CapacityReservationProvider, awsEnv.InstanceTypeStore)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, recorder, cloudProvider, cluster, fakeClock)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options(coretest.OptionsFields{FeatureGates: coretest.FeatureGates{ReservedCapacity: lo.ToPtr(true)}}))
	ctx = options.ToContext(ctx, test.Options())
	cluster.Reset()
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("LaunchTemplate Provider", func() {
	var nodePool *karpv1.NodePool
	var nodeClass *v1.EC2NodeClass
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(
			v1.EC2NodeClass{
				Status: v1.EC2NodeClassStatus{
					InstanceProfile: "test-profile",
					SecurityGroups: []v1.SecurityGroup{
						{
							ID: "sg-test1",
						},
						{
							ID: "sg-test2",
						},
						{
							ID: "sg-test3",
						},
					},
					Subnets: []v1.Subnet{
						{
							ID:   "subnet-test1",
							Zone: "test-zone-1a",
						},
						{
							ID:   "subnet-test2",
							Zone: "test-zone-1b",
						},
						{
							ID:   "subnet-test3",
							Zone: "test-zone-1c",
						},
					},
				},
			},
		)
		nodeClass.StatusConditions().SetTrue(opstatus.ConditionReady)
		nodePool = coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					ObjectMeta: karpv1.ObjectMeta{
						// TODO @joinnis: Move this into the coretest.NodePool function
						Labels: map[string]string{coretest.DiscoveryLabel: "unspecified"},
					},
					Spec: karpv1.NodeClaimTemplateSpec{
						Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      karpv1.CapacityTypeLabelKey,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{karpv1.CapacityTypeOnDemand},
								},
							},
						},
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass).Group,
							Kind:  object.GVK(nodeClass).Kind,
							Name:  nodeClass.Name,
						},
					},
				},
			},
		})
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass) // Hydrate the subnet cache
		Expect(err).To(BeNil())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypes(ctx)).To(Succeed())
		Expect(awsEnv.InstanceTypesProvider.UpdateInstanceTypeOfferings(ctx)).To(Succeed())
	})
	It("should not add the do not sync taints label to nodes when AMI type is custom", func() {
		labels := launchtemplate.InjectDoNotSyncTaintsLabel("Custom", make(map[string]string))
		Expect(labels).To(HaveLen(0))
	})

	It("should add the do not sync taints label to nodes when AMI type is al2", func() {
		labels := launchtemplate.InjectDoNotSyncTaintsLabel("AL2", make(map[string]string))
		Expect(labels).To(HaveLen(1))
		Expect(labels).Should(HaveKeyWithValue(karpv1.NodeDoNotSyncTaintsLabelKey, "true"))
	})

	It("should add the do not sync taints label to nodes when AMI type is al2023", func() {
		labels := launchtemplate.InjectDoNotSyncTaintsLabel("AL2023", make(map[string]string))
		Expect(labels).To(HaveLen(1))
		Expect(labels).Should(HaveKeyWithValue(karpv1.NodeDoNotSyncTaintsLabelKey, "true"))
	})

	It("should add the do not sync taints label to nodes when AMI type is br", func() {
		labels := launchtemplate.InjectDoNotSyncTaintsLabel("Bottlerocket", make(map[string]string))
		Expect(labels).To(HaveLen(1))
		Expect(labels).Should(HaveKeyWithValue(karpv1.NodeDoNotSyncTaintsLabelKey, "true"))
	})

	It("should add the do not sync taints label to nodes when AMI type is windows", func() {
		labels := launchtemplate.InjectDoNotSyncTaintsLabel("Windows", make(map[string]string))
		Expect(labels).To(HaveLen(1))
		Expect(labels).Should(HaveKeyWithValue(karpv1.NodeDoNotSyncTaintsLabelKey, "true"))
	})
	It("should create unique launch templates for multiple identical nodeClasses", func() {
		nodeClass2 := test.EC2NodeClass(v1.EC2NodeClass{
			Status: v1.EC2NodeClassStatus{
				InstanceProfile: "test-profile",
				Subnets:         nodeClass.Status.Subnets,
				SecurityGroups:  nodeClass.Status.SecurityGroups,
				AMIs:            nodeClass.Status.AMIs,
			},
		})
		_, err := awsEnv.SubnetProvider.List(ctx, nodeClass2) // Hydrate the subnet cache
		Expect(err).To(BeNil())
		nodePool2 := coretest.NodePool(karpv1.NodePool{
			Spec: karpv1.NodePoolSpec{
				Template: karpv1.NodeClaimTemplate{
					Spec: karpv1.NodeClaimTemplateSpec{
						Requirements: []karpv1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      karpv1.CapacityTypeLabelKey,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{karpv1.CapacityTypeSpot},
								},
							},
						},
						NodeClassRef: &karpv1.NodeClassReference{
							Group: object.GVK(nodeClass2).Group,
							Kind:  object.GVK(nodeClass2).Kind,
							Name:  nodeClass2.Name,
						},
					},
				},
			},
		})
		nodeClass2.Status.SecurityGroups = []v1.SecurityGroup{
			{
				ID: "sg-test1",
			},
			{
				ID: "sg-test2",
			},
			{
				ID: "sg-test3",
			},
		}
		nodeClass2.Status.Subnets = []v1.Subnet{
			{
				ID:   "subnet-test1",
				Zone: "test-zone-1a",
			},
			{
				ID:   "subnet-test2",
				Zone: "test-zone-1b",
			},
			{
				ID:   "subnet-test3",
				Zone: "test-zone-1c",
			},
		}
		nodeClass2.StatusConditions().SetTrue(opstatus.ConditionReady)

		pods := []*corev1.Pod{
			coretest.UnschedulablePod(coretest.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{
				{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.CapacityTypeSpot},
				},
			},
			}),
			coretest.UnschedulablePod(coretest.PodOptions{NodeRequirements: []corev1.NodeSelectorRequirement{
				{
					Key:      karpv1.CapacityTypeLabelKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{karpv1.CapacityTypeOnDemand},
				},
			},
			}),
		}
		ExpectApplied(ctx, env.Client, nodePool, nodeClass, nodePool2, nodeClass2)
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
		ltConfigCount := len(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop().LaunchTemplateConfigs) + len(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop().LaunchTemplateConfigs)
		Expect(ltConfigCount).To(BeNumerically("==", awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()))
		nodeClasses := [2]string{nodeClass.Name, nodeClass2.Name}
		awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
			for _, value := range ltInput.LaunchTemplateData.TagSpecifications[0].Tags {
				if *value.Key == v1.LabelNodeClass {
					Expect(*value.Value).To(BeElementOf(nodeClasses))
				}
			}
		})
	})
	It("should default to a generated launch template", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)

		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(BeNumerically("==", 1))
		createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(len(createFleetInput.LaunchTemplateConfigs)).To(BeNumerically("==", awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()))
		Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
		awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
			launchTemplate, ok := lo.Find(createFleetInput.LaunchTemplateConfigs, func(ltConfig ec2types.FleetLaunchTemplateConfigRequest) bool {
				return *ltConfig.LaunchTemplateSpecification.LaunchTemplateName == *ltInput.LaunchTemplateName
			})
			Expect(ok).To(BeTrue())
			Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted).To(Equal(aws.Bool(true)))
			Expect(*launchTemplate.LaunchTemplateSpecification.Version).To(Equal("$Latest"))
		})
	})
	It("should fail to provision if the instance profile isn't ready", func() {
		nodeClass.Status.InstanceProfile = ""
		nodeClass.StatusConditions().SetFalse(v1.ConditionTypeInstanceProfileReady, "reason", "message")
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	It("should use the instance profile on the EC2NodeClass when specified", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = aws.String("overridden-profile")
		nodeClass.Status.InstanceProfile = "overridden-profile"
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)
		Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
		awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
			Expect(*ltInput.LaunchTemplateData.IamInstanceProfile.Name).To(Equal("overridden-profile"))
		})
	})
	Context("Cache", func() {
		It("should use same launch template for equivalent constraints", func() {
			t1 := corev1.Toleration{
				Key:      "Abacus",
				Operator: "Equal",
				Value:    "Zebra",
				Effect:   "NoSchedule",
			}
			t2 := corev1.Toleration{
				Key:      "Zebra",
				Operator: "Equal",
				Value:    "Abacus",
				Effect:   "NoSchedule",
			}
			t3 := corev1.Toleration{
				Key:      "Boar",
				Operator: "Equal",
				Value:    "Abacus",
				Effect:   "NoSchedule",
			}

			// constrain the packer to a single launch template type
			rr := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:   resource.MustParse("24"),
					v1.ResourceNVIDIAGPU: resource.MustParse("1"),
				},
				Limits: corev1.ResourceList{v1.ResourceNVIDIAGPU: resource.MustParse("1")},
			}

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod1 := coretest.UnschedulablePod(coretest.PodOptions{
				Tolerations:          []corev1.Toleration{t1, t2, t3},
				ResourceRequirements: rr,
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod1)
			ExpectScheduled(ctx, env.Client, pod1)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			lts1 := sets.NewString()
			for _, ltConfig := range createFleetInput.LaunchTemplateConfigs {
				lts1.Insert(*ltConfig.LaunchTemplateSpecification.LaunchTemplateName)
			}

			pod2 := coretest.UnschedulablePod(coretest.PodOptions{
				Tolerations:          []corev1.Toleration{t2, t3, t1},
				ResourceRequirements: rr,
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod2)

			ExpectScheduled(ctx, env.Client, pod2)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput = awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			lts2 := sets.NewString()
			for _, ltConfig := range createFleetInput.LaunchTemplateConfigs {
				lts2.Insert(*ltConfig.LaunchTemplateSpecification.LaunchTemplateName)
			}
			Expect(lts1.Equal(lts2)).To(BeTrue())
		})
		It("should recover from an out-of-sync launch template cache", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: aws.Int32(1)}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 2))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				ltName := aws.ToString(ltInput.LaunchTemplateName)
				lt, ok := awsEnv.LaunchTemplateCache.Get(ltName)
				Expect(ok).To(Equal(true))
				// Remove expiration from cached LT
				awsEnv.LaunchTemplateCache.Set(ltName, lt, -1)
			})
			awsEnv.EC2API.CreateFleetBehavior.Error.Set(&smithy.GenericAPIError{
				Code:    "InvalidLaunchTemplateName.NotFoundException",
				Message: "The launch template name is invalid.",
			}, fake.MaxCalls(1))
			pod = coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			// should call fleet twice. Once will fail on invalid LT and the next will succeed
			Expect(awsEnv.EC2API.CreateFleetBehavior.FailedCalls()).To(BeNumerically("==", 1))
			Expect(awsEnv.EC2API.CreateFleetBehavior.SuccessfulCalls()).To(BeNumerically("==", 2))

		})
		// Testing launch template hash key will produce unique hashes
		It("should generate different launch template names based on amifamily option configuration", func() {
			options := []*amifamily.Options{
				{},
				{ClusterName: "test-name"},
				{ClusterEndpoint: "test-endpoint"},
				{ClusterCIDR: lo.ToPtr("test-cidr")},
				{InstanceProfile: "test-profile"},
				{InstanceStorePolicy: lo.ToPtr(v1.InstanceStorePolicyRAID0)},
				{SecurityGroups: []v1.SecurityGroup{{Name: "test-sg"}}},
				{Tags: map[string]string{"test-key": "test-value"}},
				{KubeDNSIP: net.ParseIP("192.0.0.2")},
				{AssociatePublicIPAddress: lo.ToPtr(true)},
				{NodeClassName: "test-name"},
			}
			launchtemplateResult := []string{}
			for _, option := range options {
				lt := &amifamily.LaunchTemplate{Options: option}
				launchtemplateResult = append(launchtemplateResult, launchtemplate.LaunchTemplateName(lt))
			}
			Expect(len(launchtemplateResult)).To(BeNumerically("==", 11))
			Expect(lo.Uniq(launchtemplateResult)).To(Equal(launchtemplateResult))
		})
		It("should not generate different launch template names based on CABundle and Labels", func() {
			options := []*amifamily.Options{
				{},
				{CABundle: lo.ToPtr("test-bundle")},
				{Labels: map[string]string{"test-key": "test-value"}},
			}
			launchtemplateResult := []string{}
			for _, option := range options {
				lt := &amifamily.LaunchTemplate{Options: option}
				launchtemplateResult = append(launchtemplateResult, launchtemplate.LaunchTemplateName(lt))
			}
			Expect(len(lo.Uniq(launchtemplateResult))).To(BeNumerically("==", 1))
			Expect(lo.Uniq(launchtemplateResult)[0]).To(Equal(launchtemplate.LaunchTemplateName(&amifamily.LaunchTemplate{Options: &amifamily.Options{}})))
		})
		It("should generate different launch template names based on kubelet configuration", func() {
			kubeletChanges := []*v1.KubeletConfiguration{
				{},
				{KubeReserved: map[string]string{string(corev1.ResourceCPU): "20"}},
				{SystemReserved: map[string]string{string(corev1.ResourceMemory): "10Gi"}},
				{EvictionHard: map[string]string{"memory.available": "52%"}},
				{EvictionSoft: map[string]string{"nodefs.available": "132%"}},
				{MaxPods: aws.Int32(20)},
			}
			launchtemplateResult := []string{}
			for _, kubelet := range kubeletChanges {
				lt := &amifamily.LaunchTemplate{UserData: bootstrap.EKS{Options: bootstrap.Options{KubeletConfig: kubelet}}}
				launchtemplateResult = append(launchtemplateResult, launchtemplate.LaunchTemplateName(lt))
			}
			Expect(len(launchtemplateResult)).To(BeNumerically("==", 6))
			Expect(lo.Uniq(launchtemplateResult)).To(Equal(launchtemplateResult))
		})
		It("should generate different launch template names based on bootstrap configuration", func() {
			bootstrapOptions := []*bootstrap.Options{
				{},
				{ClusterName: "test-name"},
				{ClusterEndpoint: "test-endpoint"},
				{ClusterCIDR: lo.ToPtr("test-cidr")},
				{Taints: []corev1.Taint{{Key: "test-key", Value: "test-value"}}},
				{Labels: map[string]string{"test-key": "test-value"}},
				{CABundle: lo.ToPtr("test-bundle")},
				{ContainerRuntime: lo.ToPtr("test-cri")},
				{CustomUserData: lo.ToPtr("test-cidr")},
			}
			launchtemplateResult := []string{}
			for _, option := range bootstrapOptions {
				lt := &amifamily.LaunchTemplate{UserData: bootstrap.EKS{Options: *option}}
				launchtemplateResult = append(launchtemplateResult, launchtemplate.LaunchTemplateName(lt))
			}
			Expect(len(launchtemplateResult)).To(BeNumerically("==", 9))
			Expect(lo.Uniq(launchtemplateResult)).To(Equal(launchtemplateResult))
		})
		It("should generate different launch template names based on launchtemplate option configuration", func() {
			launchtemplates := []*amifamily.LaunchTemplate{
				{},
				{BlockDeviceMappings: []*v1.BlockDeviceMapping{{DeviceName: lo.ToPtr("test-block")}}},
				{AMIID: "test-ami"},
				{DetailedMonitoring: true},
				{EFACount: 12},
				{CapacityType: "spot"},
			}
			launchtemplateResult := []string{}
			for _, lt := range launchtemplates {
				launchtemplateResult = append(launchtemplateResult, launchtemplate.LaunchTemplateName(lt))
			}
			Expect(len(launchtemplateResult)).To(BeNumerically("==", 6))
			Expect(lo.Uniq(launchtemplateResult)).To(Equal(launchtemplateResult))
		})
		It("should not generate different launch template names based on instance types", func() {
			launchtemplates := []*amifamily.LaunchTemplate{
				{},
				{InstanceTypes: []*corecloudprovider.InstanceType{{Name: "test-instance-type"}}},
			}
			launchtemplateResult := []string{}
			for _, lt := range launchtemplates {
				launchtemplateResult = append(launchtemplateResult, launchtemplate.LaunchTemplateName(lt))
			}
			Expect(len(lo.Uniq(launchtemplateResult))).To(BeNumerically("==", 1))
			Expect(lo.Uniq(launchtemplateResult)[0]).To(Equal(launchtemplate.LaunchTemplateName(&amifamily.LaunchTemplate{})))
		})
	})
	Context("Labels", func() {
		It("should apply labels to the node", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKey(corev1.LabelOSStable))
			Expect(node.Labels).To(HaveKey(corev1.LabelArchStable))
			Expect(node.Labels).To(HaveKey(corev1.LabelInstanceTypeStable))
		})
	})
	Context("Tags", func() {
		It("should request that tags be applied to both instances and volumes", func() {
			nodeClass.Spec.Tags = map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2value",
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.TagSpecifications).To(HaveLen(3))

			// tags should be included in instance, volume, and fleet tag specification
			Expect(createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2types.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, nodeClass.Spec.Tags)

			Expect(createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2types.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, nodeClass.Spec.Tags)

			Expect(createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2types.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, nodeClass.Spec.Tags)
		})
		It("should request that tags be applied to both network interfaces and spot instance requests", func() {
			nodeClass.Spec.Tags = map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2value",
			}
			nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
				{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      karpv1.CapacityTypeLabelKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{karpv1.CapacityTypeSpot},
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(i *ec2.CreateLaunchTemplateInput) {
				Expect(i.LaunchTemplateData.TagSpecifications).To(HaveLen(2))

				// tags should be included in instance, volume, and fleet tag specification
				Expect(i.LaunchTemplateData.TagSpecifications[0].ResourceType).To(Equal(ec2types.ResourceTypeNetworkInterface))
				ExpectTags(i.LaunchTemplateData.TagSpecifications[0].Tags, nodeClass.Spec.Tags)

				Expect(i.LaunchTemplateData.TagSpecifications[1].ResourceType).To(Equal(ec2types.ResourceTypeSpotInstancesRequest))
				ExpectTags(i.LaunchTemplateData.TagSpecifications[1].Tags, nodeClass.Spec.Tags)
			})
		})
		It("should override default tag names", func() {
			// these tags are defaulted, so ensure users can override them
			nodeClass.Spec.Tags = map[string]string{
				"Name": "myname",
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.TagSpecifications).To(HaveLen(3))

			// tags should be included in instance, volume, and fleet tag specification
			Expect(createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2types.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, nodeClass.Spec.Tags)

			Expect(createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2types.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, nodeClass.Spec.Tags)

			Expect(createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2types.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, nodeClass.Spec.Tags)
		})
	})
	Context("Block Device Mappings", func() {
		It("should default AL2 block device mappings", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(len(ltInput.LaunchTemplateData.BlockDeviceMappings)).To(Equal(1))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize)).To(Equal(int32(20)))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal(ec2types.VolumeType("gp3")))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops)).To(Equal(int32(0)))
			})
		})
		It("should default AL2023 block device mappings", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(len(ltInput.LaunchTemplateData.BlockDeviceMappings)).To(Equal(1))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize)).To(Equal(int32(20)))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal(ec2types.VolumeType("gp3")))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops)).To(Equal(int32(0)))
			})
		})
		It("should use custom block device mapping", func() {
			nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
						VolumeType:          aws.String("io2"),
						VolumeSize:          lo.ToPtr(resource.MustParse("200G")),
						IOPS:                aws.Int64(10_000),
						KMSKeyID:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					EBS: &v1.BlockDevice{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
						VolumeType:          aws.String("io2"),
						VolumeSize:          lo.ToPtr(resource.MustParse("200Gi")),
						IOPS:                aws.Int64(10_000),
						KMSKeyID:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs).To(Equal(&ec2types.LaunchTemplateEbsBlockDeviceRequest{
					VolumeSize:          aws.Int32(187),
					VolumeType:          ec2types.VolumeType("io2"),
					Iops:                aws.Int32(10_000),
					DeleteOnTermination: aws.Bool(true),
					Encrypted:           aws.Bool(true),
					KmsKeyId:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
				}))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs).To(Equal(&ec2types.LaunchTemplateEbsBlockDeviceRequest{
					VolumeSize:          aws.Int32(200),
					VolumeType:          "io2",
					Iops:                aws.Int32(10_000),
					DeleteOnTermination: aws.Bool(true),
					Encrypted:           aws.Bool(true),
					KmsKeyId:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
				}))
			})
		})
		It("should round up for custom block device mappings when specified in gigabytes", func() {
			nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
						VolumeType:          aws.String("io2"),
						VolumeSize:          lo.ToPtr(resource.MustParse("4G")),
						IOPS:                aws.Int64(10_000),
						KMSKeyID:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					EBS: &v1.BlockDevice{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
						VolumeType:          aws.String("io2"),
						VolumeSize:          lo.ToPtr(resource.MustParse("2G")),
						IOPS:                aws.Int64(10_000),
						KMSKeyID:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				// Both of these values are rounded up when converting to Gibibytes
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize)).To(BeNumerically("==", 4))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeSize)).To(BeNumerically("==", 2))
			})
		})
		It("should default bottlerocket second volume with root volume size", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(len(ltInput.LaunchTemplateData.BlockDeviceMappings)).To(Equal(2))
				// Bottlerocket control volume
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize)).To(Equal(int32(4)))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal(ec2types.VolumeType("gp3")))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops)).To(Equal(int32(0)))
				// Bottlerocket user volume
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeSize)).To(Equal(int32(20)))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeType).To(Equal(ec2types.VolumeType("gp3")))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs.Iops)).To(Equal(int32(0)))
			})
		})
		It("should not default block device mappings for custom AMIFamilies", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 2))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(len(ltInput.LaunchTemplateData.BlockDeviceMappings)).To(Equal(0))
			})
		})
		It("should use custom block device mapping for custom AMIFamilies", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
			nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
						VolumeType:          aws.String("io2"),
						VolumeSize:          lo.ToPtr(resource.MustParse("40Gi")),
						IOPS:                aws.Int64(10_000),
						KMSKeyID:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 2))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(len(ltInput.LaunchTemplateData.BlockDeviceMappings)).To(Equal(1))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize)).To(Equal(int32(40)))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal(ec2types.VolumeType("io2")))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops)).To(Equal(int32(10_000)))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.DeleteOnTermination)).To(BeTrue())
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted)).To(BeTrue())
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.KmsKeyId)).To(Equal("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"))
			})
		})
	})
	Context("Ephemeral Storage", func() {
		It("should pack pods when a daemonset has an ephemeral-storage request", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, coretest.DaemonSet(
				coretest.DaemonSetOptions{PodOptions: coretest.PodOptions{
					ResourceRequirements: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"),
							corev1.ResourceMemory:           resource.MustParse("1Gi"),
							corev1.ResourceEphemeralStorage: resource.MustParse("1Gi")}},
				}},
			))
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods with any ephemeral-storage request", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceEphemeralStorage: resource.MustParse("1G"),
				}}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods with large ephemeral-storage request", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
				}}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should not pack pods if the sum of pod ephemeral-storage and overhead exceeds node capacity", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceEphemeralStorage: resource.MustParse("19Gi"),
				}}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should pack pods if the pod's ephemeral-storage exceeds node capacity and instance storage is mounted", func() {
			nodeClass.Spec.InstanceStorePolicy = lo.ToPtr(v1.InstanceStorePolicyRAID0)
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					// Default node ephemeral-storage capacity is 20Gi
					corev1.ResourceEphemeralStorage: resource.MustParse("5000Gi"),
				}}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels[corev1.LabelInstanceTypeStable]).To(Equal("m6idn.32xlarge"))
			Expect(*node.Status.Capacity.StorageEphemeral()).To(Equal(resource.MustParse("7600G")))
		})
		It("should launch multiple nodes if sum of pod ephemeral-storage requests exceeds a single nodes capacity", func() {
			var nodes []*corev1.Node
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pods := []*corev1.Pod{
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
					},
				},
				}),
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
					},
				},
				}),
			}
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			for _, pod := range pods {
				nodes = append(nodes, ExpectScheduled(ctx, env.Client, pod))
			}
			Expect(nodes).To(HaveLen(2))
		})
		It("should only pack pods with ephemeral-storage requests that will fit on an available node", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pods := []*corev1.Pod{
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
					},
				},
				}),
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceEphemeralStorage: resource.MustParse("150Gi"),
					},
				},
				}),
			}
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pods...)
			ExpectScheduled(ctx, env.Client, pods[0])
			ExpectNotScheduled(ctx, env.Client, pods[1])
		})
		It("should not pack pod if no available instance types have enough storage", func() {
			ExpectApplied(ctx, env.Client, nodePool)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceEphemeralStorage: resource.MustParse("150Gi"),
				},
			},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should pack pods using the blockdevicemappings from the provider spec when defined", func() {
			nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(50, resource.Giga),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					EBS: &v1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceEphemeralStorage: resource.MustParse("25Gi"),
				},
			},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			// capacity isn't recorded on the node any longer, but we know the pod should schedule
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods using blockdevicemappings for Custom AMIFamily", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
			nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					EBS: &v1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(40, resource.Giga),
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					// this pod can only be satisfied if `/dev/xvdb` will house all the pods.
					corev1.ResourceEphemeralStorage: resource.MustParse("25Gi"),
				},
			},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			// capacity isn't recorded on the node any longer, but we know the pod should schedule
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods using the configured root volume in blockdevicemappings", func() {
			nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
			nodeClass.Spec.BlockDeviceMappings = []*v1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					EBS: &v1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(40, resource.Giga),
					},
					RootVolume: true,
				},
				{
					DeviceName: aws.String("/dev/xvdc"),
					EBS: &v1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					// this pod can only be satisfied if `/dev/xvdb` will house all the pods.
					corev1.ResourceEphemeralStorage: resource.MustParse("25Gi"),
				},
			},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			// capacity isn't recorded on the node any longer, but we know the pod should schedule
			ExpectScheduled(ctx, env.Client, pod)
		})
	})
	Context("AL2", func() {
		var info ec2types.InstanceTypeInfo
		BeforeEach(func() {
			var ok bool
			var instanceInfo []ec2types.InstanceTypeInfo
			out, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
				Filters: []ec2types.Filter{
					{
						Name:   aws.String("supported-virtualization-type"),
						Values: []string{"hvm"},
					},
					{
						Name:   aws.String("processor-info.supported-architecture"),
						Values: []string{"x86_64", "arm64"},
					},
				},
			})
			instanceInfo = out.InstanceTypes
			Expect(err).To(BeNil())
			info, ok = lo.Find(instanceInfo, func(i ec2types.InstanceTypeInfo) bool {
				return i.InstanceType == "m5.xlarge"
			})
			Expect(ok).To(BeTrue())
		})

		It("should calculate memory overhead based on eni limited pods", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				VMMemoryOverheadPercent: lo.ToPtr[float64](0),
			}))

			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}
			it := instancetype.NewInstanceType(ctx,
				info,
				"",
				nil,
				nil,
				nodeClass.Spec.BlockDeviceMappings,
				nodeClass.Spec.InstanceStorePolicy,
				nodeClass.Spec.Kubelet.MaxPods,
				nodeClass.Spec.Kubelet.PodsPerCore,
				nodeClass.Spec.Kubelet.KubeReserved,
				nodeClass.Spec.Kubelet.SystemReserved,
				nodeClass.Spec.Kubelet.EvictionHard,
				nodeClass.Spec.Kubelet.EvictionSoft,
				nodeClass.AMIFamily(),
				nil,
			)

			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("993Mi"))
		})
	})
	Context("Bottlerocket", func() {
		var info ec2types.InstanceTypeInfo
		BeforeEach(func() {
			var ok bool
			var instanceInfo []ec2types.InstanceTypeInfo
			out, err := awsEnv.EC2API.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
				Filters: []ec2types.Filter{
					{
						Name:   aws.String("supported-virtualization-type"),
						Values: []string{"hvm"},
					},
					{
						Name:   aws.String("processor-info.supported-architecture"),
						Values: []string{"x86_64", "arm64"},
					},
				},
			})
			instanceInfo = out.InstanceTypes
			Expect(err).To(BeNil())
			info, ok = lo.Find(instanceInfo, func(i ec2types.InstanceTypeInfo) bool {
				return i.InstanceType == "m5.xlarge"
			})
			Expect(ok).To(BeTrue())
		})

		It("should calculate memory overhead based on eni limited pods", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				VMMemoryOverheadPercent: lo.ToPtr[float64](0),
			}))

			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{}
			it := instancetype.NewInstanceType(ctx,
				info,
				"",
				nil,
				nil,
				nodeClass.Spec.BlockDeviceMappings,
				nodeClass.Spec.InstanceStorePolicy,
				nodeClass.Spec.Kubelet.MaxPods,
				nodeClass.Spec.Kubelet.PodsPerCore,
				nodeClass.Spec.Kubelet.KubeReserved,
				nodeClass.Spec.Kubelet.SystemReserved,
				nodeClass.Spec.Kubelet.EvictionHard,
				nodeClass.Spec.Kubelet.EvictionSoft,
				nodeClass.AMIFamily(),
				nil,
			)

			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("993Mi"))
		})
		It("should calculate memory overhead based on max pods", func() {
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				VMMemoryOverheadPercent: lo.ToPtr[float64](0),
			}))

			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: lo.ToPtr[int32](110)}
			it := instancetype.NewInstanceType(ctx,
				info,
				"",
				nil,
				nil,
				nodeClass.Spec.BlockDeviceMappings,
				nodeClass.Spec.InstanceStorePolicy,
				nodeClass.Spec.Kubelet.MaxPods,
				nodeClass.Spec.Kubelet.PodsPerCore,
				nodeClass.Spec.Kubelet.KubeReserved,
				nodeClass.Spec.Kubelet.SystemReserved,
				nodeClass.Spec.Kubelet.EvictionHard,
				nodeClass.Spec.Kubelet.EvictionSoft,
				nodeClass.AMIFamily(),
				nil,
			)
			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("1565Mi"))
		})
	})
	Context("User Data", func() {
		It("should specify --use-max-pods=false when using ENI-based pod density", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--use-max-pods false")
		})
		It("should specify --use-max-pods=false and --max-pods user value when user specifies maxPods in NodePool", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: aws.Int32(10)}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--use-max-pods false", "--max-pods=10")
		})
		It("should generate different launch templates for different maxPods values when specifying kubelet configuration", func() {
			// We validate that we no longer combine instance types into the same launch template with the same --max-pods values
			// that shouldn't have been combined but were combined due to a pointer error
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				ClusterDNS: []string{"test"},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			// We expect to generate 5 launch templates for our image/max-pods combination where we were only generating 2 before
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
		})
		It("should specify systemReserved when overriding system reserved values", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				SystemReserved: map[string]string{
					string(corev1.ResourceCPU):              "500m",
					string(corev1.ResourceMemory):           "1Gi",
					string(corev1.ResourceEphemeralStorage): "2Gi",
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				systemReserved := ExpectParseNodeConfigKubeletField[corev1.ResourceList](userData, "systemReserved")
				ExpectResources(corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("500m"),
					corev1.ResourceMemory:           resource.MustParse("1Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				}, systemReserved)
			}
		})
		It("should specify kubeReserved when overriding system reserved values", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				KubeReserved: map[string]string{
					string(corev1.ResourceCPU):              "500m",
					string(corev1.ResourceMemory):           "1Gi",
					string(corev1.ResourceEphemeralStorage): "2Gi",
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				kubeReserved := ExpectParseNodeConfigKubeletField[corev1.ResourceList](userData, "kubeReserved")
				ExpectResources(corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("500m"),
					corev1.ResourceMemory:           resource.MustParse("1Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				}, kubeReserved)
			}
		})
		It("should pass evictionHard threshold values when specified", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				EvictionHard: map[string]string{
					"memory.available":  "10%",
					"nodefs.available":  "15%",
					"nodefs.inodesFree": "5%",
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				evictionHard := ExpectParseNodeConfigKubeletField[map[string]string](userData, "evictionHard")
				Expect(evictionHard).To(HaveKeyWithValue("memory.available", "10%"))
				Expect(evictionHard).To(HaveKeyWithValue("nodefs.available", "15%"))
				Expect(evictionHard).To(HaveKeyWithValue("nodefs.inodesFree", "5%"))
			}
		})
		It("should pass evictionSoft threshold values when specified", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				EvictionSoft: map[string]string{
					"memory.available":  "10%",
					"nodefs.available":  "15%",
					"nodefs.inodesFree": "5%",
				},
				EvictionSoftGracePeriod: map[string]metav1.Duration{
					"memory.available":  {Duration: time.Minute},
					"nodefs.available":  {Duration: time.Second * 180},
					"nodefs.inodesFree": {Duration: time.Minute * 5},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				evictionSoft := ExpectParseNodeConfigKubeletField[map[string]string](userData, "evictionSoft")
				Expect(evictionSoft).To(HaveKeyWithValue("memory.available", "10%"))
				Expect(evictionSoft).To(HaveKeyWithValue("nodefs.available", "15%"))
				Expect(evictionSoft).To(HaveKeyWithValue("nodefs.inodesFree", "5%"))
			}
		})
		It("should pass evictionSoftGracePeriod values when specified", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				EvictionSoftGracePeriod: map[string]metav1.Duration{
					"memory.available":  {Duration: time.Minute},
					"nodefs.available":  {Duration: time.Second * 180},
					"nodefs.inodesFree": {Duration: time.Minute * 5},
				},
				EvictionSoft: map[string]string{
					"memory.available":  "10%",
					"nodefs.available":  "15%",
					"nodefs.inodesFree": "5%",
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				evictionSoftGracePeriod := ExpectParseNodeConfigKubeletField[map[string]string](userData, "evictionSoftGracePeriod")
				Expect(evictionSoftGracePeriod).To(HaveKeyWithValue("memory.available", "1m0s"))
				Expect(evictionSoftGracePeriod).To(HaveKeyWithValue("nodefs.available", "3m0s"))
				Expect(evictionSoftGracePeriod).To(HaveKeyWithValue("nodefs.inodesFree", "5m0s"))
			}
		})
		It("should pass evictionMaxPodGracePeriod when specified", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				EvictionMaxPodGracePeriod: aws.Int32(300),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				evictionMaxPodGracePeriod := ExpectParseNodeConfigKubeletField[int64](userData, "evictionMaxPodGracePeriod")
				Expect(evictionMaxPodGracePeriod).To(BeNumerically("==", 300))
			}
		})
		It("should specify podsPerCore", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				PodsPerCore: aws.Int32(2),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				podsPerCore := ExpectParseNodeConfigKubeletField[int64](userData, "podsPerCore")
				Expect(podsPerCore).To(BeNumerically("==", 2))
			}
		})
		It("should specify podsPerCore with maxPods enabled", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				PodsPerCore: aws.Int32(2),
				MaxPods:     aws.Int32(100),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				podsPerCore := ExpectParseNodeConfigKubeletField[int64](userData, "podsPerCore")
				Expect(podsPerCore).To(BeNumerically("==", 2))
				maxPods := ExpectParseNodeConfigKubeletField[int64](userData, "maxPods")
				Expect(maxPods).To(BeNumerically("==", 100))
			}
		})
		It("should specify clusterDNS when running in an ipv6 cluster", func() {
			awsEnv.LaunchTemplateProvider.KubeDNSIP = net.ParseIP("fd4b:121b:812b::a")
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				clusterDNS := ExpectParseNodeConfigKubeletField[[]string](userData, "clusterDNS")
				Expect(clusterDNS).To(HaveLen(1))
				Expect(clusterDNS[0]).To(Equal("fd4b:121b:812b::a"))
			}
		})
		It("should specify clusterDNS when running in an ipv4 cluster", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				clusterDNS := ExpectParseNodeConfigKubeletField[[]string](userData, "clusterDNS")
				Expect(clusterDNS).To(HaveLen(1))
				Expect(clusterDNS[0]).To(Equal("10.0.100.10"))
			}
		})
		It("should pass ImageGCHighThresholdPercent when specified", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				ImageGCHighThresholdPercent: aws.Int32(50),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				imageGCHighThresholdPercent := ExpectParseNodeConfigKubeletField[int64](userData, "imageGCHighThresholdPercent")
				Expect(imageGCHighThresholdPercent).To(BeNumerically("==", 50))
			}
		})
		It("should pass ImageGCLowThresholdPercent when specified", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				ImageGCLowThresholdPercent: aws.Int32(50),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				imageGCLowThresholdPercent := ExpectParseNodeConfigKubeletField[int64](userData, "imageGCLowThresholdPercent")
				Expect(imageGCLowThresholdPercent).To(BeNumerically("==", 50))
			}
		})
		It("should pass cpuCFSQuota when specified", func() {
			nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
				CPUCFSQuota: aws.Bool(false),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
				cpuCFSQuota := ExpectParseNodeConfigKubeletField[bool](userData, "cpuCFSQuota")
				Expect(cpuCFSQuota).To(BeFalse())
			}
		})
		It("should not pass any labels prefixed with the node-restriction.kubernetes.io domain", func() {
			nodePool.Spec.Template.Labels = lo.Assign(nodePool.Spec.Template.Labels, map[string]string{
				corev1.LabelNamespaceNodeRestriction + "/team":                        "team-1",
				corev1.LabelNamespaceNodeRestriction + "/custom-label":                "custom-value",
				"subdomain." + corev1.LabelNamespaceNodeRestriction + "/custom-label": "custom-value",
			})
			nodePool.Spec.Template.Spec.Requirements = lo.MapToSlice(nodePool.Spec.Template.Labels, func(k, v string) karpv1.NodeSelectorRequirementWithMinValues {
				return karpv1.NodeSelectorRequirementWithMinValues{
					NodeSelectorRequirement: corev1.NodeSelectorRequirement{
						Key:      k,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{v},
					},
				}
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataNotContaining(corev1.LabelNamespaceNodeRestriction)
		})
		It("should specify --local-disks raid0 when instance-store policy is set on AL2", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			nodeClass.Spec.InstanceStorePolicy = lo.ToPtr(v1.InstanceStorePolicyRAID0)
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)

			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--local-disks raid0")
		})
		It("should specify RAID0 bootstrap-command when instance-store policy is set on Bottlerocket", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			nodeClass.Spec.InstanceStorePolicy = lo.ToPtr(v1.InstanceStorePolicyRAID0)
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)

			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			ExpectLaunchTemplatesCreatedWithUserDataContaining(`
[settings.bootstrap-commands.000-mount-instance-storage]
commands = [['apiclient', 'ephemeral-storage', 'init'], ['apiclient', 'ephemeral-storage', 'bind', '--dirs', '/var/lib/containerd', '/var/lib/kubelet', '/var/log/pods']]
mode = 'always'
essential = true
`)
		})
		It("should merge bootstrap-commands when instance-store policy is set on Bottlerocket", func() {
			nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
			nodeClass.Spec.InstanceStorePolicy = lo.ToPtr(v1.InstanceStorePolicyRAID0)
			nodeClass.Spec.UserData = lo.ToPtr(`
[settings.bootstrap-commands.111-say-hello]
commands = [['echo', 'hello']]
mode = 'always'
essential = true
`)
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)

			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			ExpectLaunchTemplatesCreatedWithUserDataContaining(`
[settings.bootstrap-commands]
[settings.bootstrap-commands.000-mount-instance-storage]
commands = [['apiclient', 'ephemeral-storage', 'init'], ['apiclient', 'ephemeral-storage', 'bind', '--dirs', '/var/lib/containerd', '/var/lib/kubelet', '/var/log/pods']]
mode = 'always'
essential = true

[settings.bootstrap-commands.111-say-hello]
commands = [['echo', 'hello']]
mode = 'always'
essential = true
`)
		})
		Context("Bottlerocket", func() {
			BeforeEach(func() {
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "bottlerocket@latest"}}
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: lo.ToPtr[int32](110)}
			})
			It("should merge in custom user data", func() {
				content, err := os.ReadFile("testdata/br_userdata_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(fmt.Sprintf(string(content), karpv1.NodePoolLabelKey))
				nodePool.Spec.Template.Spec.Taints = []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoExecute}}
				nodePool.Spec.Template.Spec.StartupTaints = []corev1.Taint{{Key: "baz", Value: "bin", Effect: corev1.TaintEffectNoExecute}}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod(coretest.PodOptions{
					Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
				})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/br_userdata_merged.golden")
				Expect(err).To(BeNil())
				ExpectLaunchTemplatesCreatedWithUserData(fmt.Sprintf(string(content), nodeClass.Name, karpv1.NodePoolLabelKey, nodePool.Name))
			})
			It("should bootstrap when custom user data is empty", func() {
				nodePool.Spec.Template.Spec.Taints = []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoExecute}}
				nodePool.Spec.Template.Spec.StartupTaints = []corev1.Taint{{Key: "baz", Value: "bin", Effect: corev1.TaintEffectNoExecute}}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
				pod := coretest.UnschedulablePod(coretest.PodOptions{
					Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
				})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err := os.ReadFile("testdata/br_userdata_unmerged.golden")
				Expect(err).To(BeNil())
				ExpectLaunchTemplatesCreatedWithUserData(fmt.Sprintf(string(content), nodeClass.Name, karpv1.NodePoolLabelKey, nodePool.Name))
			})
			It("should not bootstrap when provider ref points to a non-existent EC2NodeClass resource", func() {
				nodePool.Spec.Template.Spec.NodeClassRef = &karpv1.NodeClassReference{
					Group: "doesnotexist",
					Kind:  "doesnotexist",
					Name:  "doesnotexist",
				}
				ExpectApplied(ctx, env.Client, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				// This will not be scheduled since we were pointed to a non-existent EC2NodeClass resource.
				ExpectNotScheduled(ctx, env.Client, pod)
			})
			It("should not bootstrap on invalid toml user data", func() {
				nodeClass.Spec.UserData = aws.String("#/bin/bash\n ./not-toml.sh")
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				// This will not be scheduled since userData cannot be generated for the prospective node.
				ExpectNotScheduled(ctx, env.Client, pod)
			})
			It("should override system reserved values in user data", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					SystemReserved: map[string]string{
						string(corev1.ResourceCPU):              "2",
						string(corev1.ResourceMemory):           "3Gi",
						string(corev1.ResourceEphemeralStorage): "10Gi",
					},
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 2))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(len(config.Settings.Kubernetes.SystemReserved)).To(Equal(3))
					Expect(config.Settings.Kubernetes.SystemReserved[corev1.ResourceCPU.String()]).To(Equal("2"))
					Expect(config.Settings.Kubernetes.SystemReserved[corev1.ResourceMemory.String()]).To(Equal("3Gi"))
					Expect(config.Settings.Kubernetes.SystemReserved[corev1.ResourceEphemeralStorage.String()]).To(Equal("10Gi"))
				})
			})
			It("should override kube reserved values in user data", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					KubeReserved: map[string]string{
						string(corev1.ResourceCPU):              "2",
						string(corev1.ResourceMemory):           "3Gi",
						string(corev1.ResourceEphemeralStorage): "10Gi",
					},
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(len(config.Settings.Kubernetes.KubeReserved)).To(Equal(3))
					Expect(config.Settings.Kubernetes.KubeReserved[corev1.ResourceCPU.String()]).To(Equal("2"))
					Expect(config.Settings.Kubernetes.KubeReserved[corev1.ResourceMemory.String()]).To(Equal("3Gi"))
					Expect(config.Settings.Kubernetes.KubeReserved[corev1.ResourceEphemeralStorage.String()]).To(Equal("10Gi"))
				})
			})
			It("should override soft eviction values in user data", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionSoft: map[string]string{"memory.available": "10%"},
					EvictionSoftGracePeriod: map[string]metav1.Duration{
						"memory.available": {Duration: time.Minute},
					},
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
				ExpectLaunchTemplatesCreatedWithUserDataContaining(`
[settings.kubernetes.eviction-soft]
'memory.available' = '10%'

[settings.kubernetes.eviction-soft-grace-period]
'memory.available' = '1m0s'
`)
			})
			It("should override max pod grace period in user data", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					MaxPods:                   aws.Int32(35),
					EvictionMaxPodGracePeriod: aws.Int32(10),
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 2))
				ExpectLaunchTemplatesCreatedWithUserDataContaining(`
[settings.kubernetes]
api-server = 'https://test-cluster'
cluster-certificate = 'Y2EtYnVuZGxlCg=='
cluster-name = 'test-cluster'
cluster-dns-ip = '10.0.100.10'
max-pods = 35
eviction-max-pod-grace-period = 10
`)
			})
			It("should override kube reserved values in user data", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					EvictionHard: map[string]string{
						"memory.available":  "10%",
						"nodefs.available":  "15%",
						"nodefs.inodesFree": "5%",
					},
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(len(config.Settings.Kubernetes.EvictionHard)).To(Equal(3))
					Expect(config.Settings.Kubernetes.EvictionHard["memory.available"]).To(Equal("10%"))
					Expect(config.Settings.Kubernetes.EvictionHard["nodefs.available"]).To(Equal("15%"))
					Expect(config.Settings.Kubernetes.EvictionHard["nodefs.inodesFree"]).To(Equal("5%"))
				})
			})
			It("should specify max pods value when passing maxPods in configuration", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					MaxPods: aws.Int32(10),
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 2))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(config.Settings.Kubernetes.MaxPods).ToNot(BeNil())
					Expect(*config.Settings.Kubernetes.MaxPods).To(BeNumerically("==", 10))
				})
			})
			It("should pass ImageGCHighThresholdPercent when specified", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					ImageGCHighThresholdPercent: aws.Int32(50),
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(config.Settings.Kubernetes.ImageGCHighThresholdPercent).ToNot(BeNil())
					percent, err := strconv.Atoi(*config.Settings.Kubernetes.ImageGCHighThresholdPercent)
					Expect(err).ToNot(HaveOccurred())
					Expect(percent).To(BeNumerically("==", 50))
				})
			})
			It("should pass ImageGCLowThresholdPercent when specified", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					ImageGCLowThresholdPercent: aws.Int32(50),
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(config.Settings.Kubernetes.ImageGCLowThresholdPercent).ToNot(BeNil())
					percent, err := strconv.Atoi(*config.Settings.Kubernetes.ImageGCLowThresholdPercent)
					Expect(err).ToNot(HaveOccurred())
					Expect(percent).To(BeNumerically("==", 50))
				})
			})
			It("should pass ClusterDNSIP when discovered", func() {
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 2))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(config.Settings.Kubernetes.ClusterDNSIP).ToNot(BeNil())
					Expect(*config.Settings.Kubernetes.ClusterDNSIP).To(Equal("10.0.100.10"))
				})
			})
			It("should pass CPUCFSQuota when specified", func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{
					CPUCFSQuota: aws.Bool(false),
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(config.Settings.Kubernetes.CPUCFSQuota).ToNot(BeNil())
					Expect(*config.Settings.Kubernetes.CPUCFSQuota).To(BeFalse())
				})
			})
			It("should specify labels in the Kubelet flags when specified in NodePool", func() {
				desiredLabels := map[string]string{
					"test-label-1": "value-1",
					"test-label-2": "value-2",
				}
				nodePool.Spec.Template.Labels = desiredLabels

				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML([]byte(userData))).To(Succeed())
					for k, v := range desiredLabels {
						Expect(config.Settings.Kubernetes.NodeLabels).To(HaveKeyWithValue(k, v))
					}
				}
			})
			It("should specify labels in the Kubelet flags when single value requirements are specified in NodePool", func() {
				desiredLabels := map[string]string{
					"test-label-1": "value-1",
					"test-label-2": "value-2",
				}
				nodePool.Spec.Template.Spec.Requirements = lo.MapToSlice(desiredLabels, func(k, v string) karpv1.NodeSelectorRequirementWithMinValues {
					return karpv1.NodeSelectorRequirementWithMinValues{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      k,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{v},
						},
					}
				})

				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML([]byte(userData))).To(Succeed())
					for k, v := range desiredLabels {
						Expect(config.Settings.Kubernetes.NodeLabels).To(HaveKeyWithValue(k, v))
					}
				}
			})
		})
		Context("AL2 Custom UserData", func() {
			BeforeEach(func() {
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: lo.ToPtr[int32](110)}
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2@latest"}}
			})
			It("should merge in custom user data", func() {
				content, err := os.ReadFile("testdata/al2_userdata_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(string(content))
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/al2_userdata_merged.golden")
				Expect(err).To(BeNil())
				expectedUserData := fmt.Sprintf(string(content), nodeClass.Name, karpv1.NodePoolLabelKey, nodePool.Name)
				ExpectLaunchTemplatesCreatedWithUserData(expectedUserData)
			})
			It("should merge in custom user data when Content-Type is before MIME-Version", func() {
				content, err := os.ReadFile("testdata/al2_userdata_content_type_first_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(string(content))
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/al2_userdata_merged.golden")
				Expect(err).To(BeNil())
				expectedUserData := fmt.Sprintf(string(content), nodeClass.Name, karpv1.NodePoolLabelKey, nodePool.Name)
				ExpectLaunchTemplatesCreatedWithUserData(expectedUserData)
			})
			It("should merge in custom user data not in multi-part mime format", func() {
				content, err := os.ReadFile("testdata/al2_no_mime_userdata_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(string(content))
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/al2_userdata_merged.golden")
				Expect(err).To(BeNil())
				expectedUserData := fmt.Sprintf(string(content), nodeClass.Name, karpv1.NodePoolLabelKey, nodePool.Name)
				ExpectLaunchTemplatesCreatedWithUserData(expectedUserData)
			})
			It("should handle empty custom user data", func() {
				nodeClass.Spec.UserData = nil
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err := os.ReadFile("testdata/al2_userdata_unmerged.golden")
				Expect(err).To(BeNil())
				expectedUserData := fmt.Sprintf(string(content), nodeClass.Name, karpv1.NodePoolLabelKey, nodePool.Name)
				ExpectLaunchTemplatesCreatedWithUserData(expectedUserData)
			})
		})
		Context("AL2023", func() {
			BeforeEach(func() {
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "al2023@latest"}}
			})
			Context("Kubelet", func() {
				It("should specify taints in the KubeletConfiguration when specified in NodePool", func() {
					desiredTaints := []corev1.Taint{
						{
							Key:    "test-taint-1",
							Effect: corev1.TaintEffectNoSchedule,
						},
						{
							Key:    "test-taint-2",
							Effect: corev1.TaintEffectNoExecute,
						},
					}
					nodePool.Spec.Template.Spec.Taints = desiredTaints
					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					pod := coretest.UnschedulablePod(coretest.UnscheduleablePodOptions(coretest.PodOptions{
						Tolerations: []corev1.Toleration{{
							Operator: corev1.TolerationOpExists,
						}},
					}))
					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectScheduled(ctx, env.Client, pod)
					for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
						configs := ExpectParseNodeConfigs(userData)
						Expect(len(configs)).To(Equal(1))
						taintsRaw, ok := configs[0].Spec.Kubelet.Config["registerWithTaints"]
						Expect(ok).To(BeTrue())
						taints := []corev1.Taint{}
						Expect(yaml.Unmarshal(taintsRaw.Raw, &taints)).To(Succeed())
						Expect(len(taints)).To(Equal(3))
						Expect(taints).To(ContainElements(lo.Map(desiredTaints, func(t corev1.Taint, _ int) interface{} {
							return interface{}(t)
						})))
					}
				})
				It("should specify labels in the Kubelet flags when specified in NodePool", func() {
					desiredLabels := map[string]string{
						"test-label-1": "value-1",
						"test-label-2": "value-2",
					}
					nodePool.Spec.Template.Labels = desiredLabels

					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					pod := coretest.UnschedulablePod()
					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectScheduled(ctx, env.Client, pod)
					for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
						configs := ExpectParseNodeConfigs(userData)
						Expect(len(configs)).To(Equal(1))
						labelFlag, ok := lo.Find(configs[0].Spec.Kubelet.Flags, func(flag string) bool {
							return strings.HasPrefix(flag, "--node-labels")
						})
						Expect(ok).To(BeTrue())
						for label, value := range desiredLabels {
							Expect(labelFlag).To(ContainSubstring(fmt.Sprintf("%s=%s", label, value)))
						}
					}
				})
				It("should specify labels in the Kubelet flags when single value requirements are specified in NodePool", func() {
					desiredLabels := map[string]string{
						"test-label-1": "value-1",
						"test-label-2": "value-2",
					}
					nodePool.Spec.Template.Spec.Requirements = lo.MapToSlice(desiredLabels, func(k, v string) karpv1.NodeSelectorRequirementWithMinValues {
						return karpv1.NodeSelectorRequirementWithMinValues{
							NodeSelectorRequirement: corev1.NodeSelectorRequirement{
								Key:      k,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{v},
							},
						}
					})

					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					pod := coretest.UnschedulablePod()
					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectScheduled(ctx, env.Client, pod)
					for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
						configs := ExpectParseNodeConfigs(userData)
						Expect(len(configs)).To(Equal(1))
						labelFlag, ok := lo.Find(configs[0].Spec.Kubelet.Flags, func(flag string) bool {
							return strings.HasPrefix(flag, "--node-labels")
						})
						Expect(ok).To(BeTrue())
						for label, value := range desiredLabels {
							Expect(labelFlag).To(ContainSubstring(fmt.Sprintf("%s=%s", label, value)))
						}
					}
				})
				DescribeTable(
					"should specify KubletConfiguration field when specified in NodePool",
					func(field string, kc v1.KubeletConfiguration) {
						nodeClass.Spec.Kubelet = &kc
						ExpectApplied(ctx, env.Client, nodePool, nodeClass)
						pod := coretest.UnschedulablePod()
						ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
						ExpectScheduled(ctx, env.Client, pod)

						// Convert provided KubeletConfiguration to an InlineConfig for comparison with NodeConfig
						inlineConfig := func() map[string]runtime.RawExtension {
							configYAML, err := yaml.Marshal(kc)
							Expect(err).To(BeNil())
							configMap := map[string]interface{}{}
							Expect(yaml.Unmarshal(configYAML, &configMap)).To(Succeed())
							return lo.MapValues(configMap, func(v interface{}, _ string) runtime.RawExtension {
								val, err := json.Marshal(v)
								Expect(err).To(BeNil())
								return runtime.RawExtension{Raw: val}
							})
						}()
						for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
							configs := ExpectParseNodeConfigs(userData)
							Expect(len(configs)).To(Equal(1))
							Expect(configs[0].Spec.Kubelet.Config[field]).To(Equal(inlineConfig[field]))
						}
					},
					Entry("systemReserved", "systemReserved", v1.KubeletConfiguration{
						SystemReserved: map[string]string{
							string(corev1.ResourceCPU):              "500m",
							string(corev1.ResourceMemory):           "1Gi",
							string(corev1.ResourceEphemeralStorage): "2Gi",
						},
					}),
					Entry("kubeReserved", "kubeReserved", v1.KubeletConfiguration{
						KubeReserved: map[string]string{
							string(corev1.ResourceCPU):              "500m",
							string(corev1.ResourceMemory):           "1Gi",
							string(corev1.ResourceEphemeralStorage): "2Gi",
						},
					}),
					Entry("evictionHard", "evictionHard", v1.KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available":  "10%",
							"nodefs.available":  "15%",
							"nodefs.inodesFree": "5%",
						},
					}),
					Entry("evictionSoft", "evictionSoft", v1.KubeletConfiguration{
						EvictionSoft: map[string]string{
							"memory.available":  "10%",
							"nodefs.available":  "15%",
							"nodefs.inodesFree": "5%",
						},
						EvictionSoftGracePeriod: map[string]metav1.Duration{
							"memory.available":  {Duration: time.Minute},
							"nodefs.available":  {Duration: time.Second * 180},
							"nodefs.inodesFree": {Duration: time.Minute * 5},
						},
					}),
					Entry("evictionSoftGracePeriod", "evictionSoftGracePeriod", v1.KubeletConfiguration{
						EvictionSoft: map[string]string{
							"memory.available":  "10%",
							"nodefs.available":  "15%",
							"nodefs.inodesFree": "5%",
						},
						EvictionSoftGracePeriod: map[string]metav1.Duration{
							"memory.available":  {Duration: time.Minute},
							"nodefs.available":  {Duration: time.Second * 180},
							"nodefs.inodesFree": {Duration: time.Minute * 5},
						},
					}),
					Entry("evictionMaxPodGracePeriod", "evictionMaxPodGracePeriod", v1.KubeletConfiguration{
						EvictionMaxPodGracePeriod: lo.ToPtr[int32](300),
					}),
					Entry("podsPerCore", "podsPerCore", v1.KubeletConfiguration{
						PodsPerCore: lo.ToPtr[int32](2),
					}),
					Entry("clusterDNS", "clusterDNS", v1.KubeletConfiguration{
						ClusterDNS: []string{"10.0.100.0"},
					}),
					Entry("imageGCHighThresholdPercent", "imageGCHighThresholdPercent", v1.KubeletConfiguration{
						ImageGCHighThresholdPercent: lo.ToPtr[int32](50),
					}),
					Entry("imageGCLowThresholdPercent", "imageGCLowThresholdPercent", v1.KubeletConfiguration{
						ImageGCLowThresholdPercent: lo.ToPtr[int32](50),
					}),
					Entry("cpuCFSQuota", "cpuCFSQuota", v1.KubeletConfiguration{
						CPUCFSQuota: lo.ToPtr(false),
					}),
				)
			})
			It("should set LocalDiskStrategy to Raid0 when specified by the InstanceStorePolicy", func() {
				nodeClass.Spec.InstanceStorePolicy = lo.ToPtr(v1.InstanceStorePolicyRAID0)
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				for _, userData := range ExpectUserDataExistsFromCreatedLaunchTemplates() {
					configs := ExpectParseNodeConfigs(userData)
					Expect(len(configs)).To(Equal(1))
					Expect(configs[0].Spec.Instance.LocalStorage.Strategy).To(Equal(admv1alpha1.LocalStorageRAID0))
				}
			})
			DescribeTable(
				"should merge custom user data",
				func(inputFile *string, mergedFile string) {
					if inputFile != nil {
						content, err := os.ReadFile("testdata/" + *inputFile)
						Expect(err).To(BeNil())
						nodeClass.Spec.UserData = lo.ToPtr(string(content))
					}
					nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: lo.ToPtr[int32](110)}
					ExpectApplied(ctx, env.Client, nodeClass, nodePool)
					pod := coretest.UnschedulablePod()
					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectScheduled(ctx, env.Client, pod)
					content, err := os.ReadFile("testdata/" + mergedFile)
					Expect(err).To(BeNil())
					expectedUserData := fmt.Sprintf(string(content), nodeClass.Name, karpv1.NodePoolLabelKey, nodePool.Name)
					ExpectLaunchTemplatesCreatedWithUserData(expectedUserData)
				},
				Entry("MIME", lo.ToPtr("al2023_mime_userdata_input.golden"), "al2023_mime_userdata_merged.golden"),
				Entry("YAML", lo.ToPtr("al2023_yaml_userdata_input.golden"), "al2023_yaml_userdata_merged.golden"),
				Entry("shell", lo.ToPtr("al2023_shell_userdata_input.golden"), "al2023_shell_userdata_merged.golden"),
				Entry("empty", nil, "al2023_userdata_unmerged.golden"),
			)
			It("should fail to create launch templates if cluster CIDR is unresolved", func() {
				awsEnv.LaunchTemplateProvider.ClusterCIDR.Store(nil)
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectNotScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(Equal(0))
			})
		})
		Context("Custom AMI Selector", func() {
			It("should use ami selector specified in EC2NodeClass", func() {
				nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				nodeClass.Status.AMIs = []v1.AMI{
					{
						ID: "ami-123",
						Requirements: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
						},
					},
				}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 1))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					Expect("ami-123").To(Equal(*ltInput.LaunchTemplateData.ImageId))
				})
			})
			It("should copy over userData untouched when AMIFamily is Custom", func() {
				nodeClass.Spec.UserData = aws.String("special user data")
				nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				nodeClass.Status.AMIs = []v1.AMI{
					{
						ID: "ami-123",
						Requirements: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
						},
					},
				}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				ExpectLaunchTemplatesCreatedWithUserData("special user data")
			})
			It("should correctly use ami selector with specific IDs in EC2NodeClass", func() {
				nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{ID: "ami-123"}, {ID: "ami-456"}}
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-123"),
						Architecture: "x86_64",
						Tags:         []ec2types.Tag{{Key: aws.String(corev1.LabelInstanceTypeStable), Value: aws.String("m5.large")}},
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
						State:        ec2types.ImageStateAvailable,
					},
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-456"),
						Architecture: "x86_64",
						Tags:         []ec2types.Tag{{Key: aws.String(corev1.LabelInstanceTypeStable), Value: aws.String("m5.xlarge")}},
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
						State:        ec2types.ImageStateAvailable,
					},
				}})
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				_, err := awsEnv.AMIProvider.List(ctx, nodeClass)
				Expect(err).To(BeNil())
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 2))
				actualFilter := awsEnv.EC2API.CalledWithDescribeImagesInput.Pop().Filters
				expectedFilter := []ec2types.Filter{
					{
						Name:   aws.String("image-id"),
						Values: []string{"ami-123", "ami-456"},
					},
					{
						Name:   aws.String("state"),
						Values: []string{string(ec2types.ImageStateAvailable)},
					},
				}
				Expect(actualFilter).To(Equal(expectedFilter))
			})
			It("should create multiple launch templates when multiple amis are discovered with non-equivalent requirements", func() {
				nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				nodeClass.Status.AMIs = []v1.AMI{
					{
						ID: "ami-123",
						Requirements: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureAmd64}},
						},
					},
					{
						ID: "ami-456",
						Requirements: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{karpv1.ArchitectureArm64}},
						},
					},
				}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 2))
				expectedImageIds := sets.New[string]("ami-123", "ami-456")
				actualImageIds := sets.New[string]()
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					actualImageIds.Insert(*ltInput.LaunchTemplateData.ImageId)
				})
				Expect(expectedImageIds.Equal(actualImageIds)).To(BeTrue())
			})
			It("should create a launch template with the newest compatible AMI when multiple amis are discovered", func() {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-123"),
						Architecture: "x86_64",
						CreationDate: aws.String("2020-01-01T12:00:00Z"),
						State:        ec2types.ImageStateAvailable,
					},
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-456"),
						Architecture: "x86_64",
						CreationDate: aws.String("2021-01-01T12:00:00Z"),
						State:        ec2types.ImageStateAvailable,
					},
					{
						// Incompatible because required ARM64
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-789"),
						Architecture: "arm64",
						CreationDate: aws.String("2022-01-01T12:00:00Z"),
						State:        ec2types.ImageStateAvailable,
					},
				}})
				nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{
					{
						NodeSelectorRequirement: corev1.NodeSelectorRequirement{
							Key:      corev1.LabelArchStable,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{karpv1.ArchitectureAmd64},
						},
					},
				}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)

				controller := nodeclass.NewController(awsEnv.Clock, env.Client, cloudProvider, recorder, fake.DefaultRegion, awsEnv.SubnetProvider, awsEnv.SecurityGroupProvider, awsEnv.AMIProvider, awsEnv.InstanceProfileProvider, awsEnv.InstanceTypesProvider, awsEnv.LaunchTemplateProvider, awsEnv.CapacityReservationProvider, awsEnv.EC2API, awsEnv.ValidationCache, awsEnv.RecreationCache, awsEnv.AMIResolver, options.FromContext(ctx).DisableDryRun)
				ExpectObjectReconciled(ctx, env.Client, controller, nodeClass)

				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				// one for validation and one for creation
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 2))
				awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					Expect("ami-456").To(Equal(*ltInput.LaunchTemplateData.ImageId))
				})
			})

			It("should fail if no amis match selector.", func() {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{}})
				nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				nodeClass.Status.AMIs = []v1.AMI{}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectNotScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(Equal(0))
			})
			It("should fail if no instanceType matches ami requirements.", func() {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []ec2types.Image{
					{Name: aws.String(coretest.RandomName()), ImageId: aws.String("ami-123"), Architecture: "newnew", CreationDate: aws.String("2022-01-01T12:00:00Z")}}})
				nodeClass.Spec.AMIFamily = lo.ToPtr(v1.AMIFamilyCustom)
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				nodeClass.Status.AMIs = []v1.AMI{
					{
						ID: "ami-123",
						Requirements: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{"newnew"}},
						},
					},
				}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectNotScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(Equal(0))
			})
			It("should choose amis from SSM if no selector specified in EC2NodeClass", func() {
				version := awsEnv.VersionProvider.Get(ctx)
				awsEnv.SSMAPI.Parameters = map[string]string{
					fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version): "test-ami-123",
				}
				nodeClass.Status.AMIs = []v1.AMI{
					{
						ID: "test-ami-123",
						Requirements: []corev1.NodeSelectorRequirement{
							{Key: corev1.LabelArchStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(karpv1.ArchitectureAmd64)}},
						},
					},
				}
				ExpectApplied(ctx, env.Client, nodeClass)
				ExpectApplied(ctx, env.Client, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				input := awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Pop()
				Expect(*input.LaunchTemplateData.ImageId).To(ContainSubstring("test-ami"))
			})
		})
		Context("Public IP Association", func() {
			DescribeTable(
				"should set 'AssociatePublicIPAddress' based on EC2NodeClass",
				func(setValue, expectedValue, isEFA bool) {
					nodeClass.Spec.AssociatePublicIPAddress = lo.ToPtr(setValue)
					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					pod := coretest.UnschedulablePod(lo.Ternary(isEFA, coretest.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{v1.ResourceEFA: resource.MustParse("2")},
							Limits:   corev1.ResourceList{v1.ResourceEFA: resource.MustParse("2")},
						},
					}, coretest.PodOptions{}))
					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectScheduled(ctx, env.Client, pod)
					input := awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Pop()
					Expect(*input.LaunchTemplateData.NetworkInterfaces[0].AssociatePublicIpAddress).To(Equal(expectedValue))
				},
				Entry("AssociatePublicIPAddress is true", true, true, false),
				Entry("AssociatePublicIPAddress is false", false, false, false),
				Entry("AssociatePublicIPAddress is true (EFA)", true, true, true),
				Entry("AssociatePublicIPAddress is false (EFA)", false, false, true),
			)
		})
		Context("IP Prefix Delegation", func() {
			DescribeTable(
				"should set 'IPv4PrefixCount' based on EC2NodeClass",
				func(setValue int) {
					nodeClass.Spec.IPPrefixCount = lo.ToPtr(int32(setValue))
					awsEnv.LaunchTemplateProvider.ClusterIPFamily = corev1.IPv4Protocol
					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					pod := coretest.UnschedulablePod()
					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectScheduled(ctx, env.Client, pod)
					input := awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Pop()
					Expect(*input.LaunchTemplateData.NetworkInterfaces[0].Ipv4PrefixCount).To(Equal(int32(setValue)))
				},
				Entry("IPv4PrefixCount is 0", 0),
				Entry("IPv4PrefixCount is 1", 1),
				Entry("IPv4PrefixCount is 10", 10),
				Entry("IPv4PrefixCount is 100", 100),
			)
			DescribeTable(
				"should set 'IPv6PrefixCount' based on EC2NodeClass",
				func(setValue int) {
					nodeClass.Spec.IPPrefixCount = lo.ToPtr(int32(setValue))
					awsEnv.LaunchTemplateProvider.ClusterIPFamily = corev1.IPv6Protocol
					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					pod := coretest.UnschedulablePod()
					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectScheduled(ctx, env.Client, pod)
					input := awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Pop()
					Expect(*input.LaunchTemplateData.NetworkInterfaces[0].Ipv6PrefixCount).To(Equal(int32(setValue)))
				},
				Entry("IPv6PrefixCount is 0", 0),
				Entry("IPv6PrefixCount is 1", 1),
				Entry("IPv6PrefixCount is 10", 10),
				Entry("IPv6PrefixCount is 100", 100),
			)
		})
		Context("Windows Custom UserData", func() {
			BeforeEach(func() {
				nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelOSStable, Operator: corev1.NodeSelectorOpIn, Values: []string{string(corev1.Windows)}}}}
				nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{Alias: "windows2022@latest"}}
				nodeClass.Spec.Kubelet = &v1.KubeletConfiguration{MaxPods: lo.ToPtr[int32](110)}
			})
			It("should merge and bootstrap with custom user data", func() {
				content, err := os.ReadFile("testdata/windows_userdata_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(string(content))
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
				pod := coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{
						corev1.LabelOSStable:     string(corev1.Windows),
						corev1.LabelWindowsBuild: "10.0.20348",
					},
				})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/windows_userdata_merged.golden")
				Expect(err).To(BeNil())
				ExpectLaunchTemplatesCreatedWithUserData(fmt.Sprintf(string(content), nodeClass.Name, karpv1.NodePoolLabelKey, nodePool.Name))
			})
			It("should bootstrap when custom user data is empty", func() {
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
				pod := coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{
						corev1.LabelOSStable:     string(corev1.Windows),
						corev1.LabelWindowsBuild: "10.0.20348",
					},
				})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err := os.ReadFile("testdata/windows_userdata_unmerged.golden")
				Expect(err).To(BeNil())
				ExpectLaunchTemplatesCreatedWithUserData(fmt.Sprintf(string(content), nodeClass.Name, karpv1.NodePoolLabelKey, nodePool.Name))
			})
		})
	})
	Context("Detailed Monitoring", func() {
		It("should default detailed monitoring to off", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(aws.ToBool(ltInput.LaunchTemplateData.Monitoring.Enabled)).To(BeFalse())
			})
		})
		It("should pass detailed monitoring setting to the launch template at creation", func() {
			nodeClass.Spec.DetailedMonitoring = aws.Bool(true)
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(aws.ToBool(ltInput.LaunchTemplateData.Monitoring.Enabled)).To(BeTrue())
			})
		})
	})
	Context("Instance Metadata", func() {
		It("should set the default instance metadata settings on instances", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(ltInput.LaunchTemplateData.MetadataOptions.HttpEndpoint).To(Equal(ec2types.LaunchTemplateInstanceMetadataEndpointStateEnabled))
				Expect(ltInput.LaunchTemplateData.MetadataOptions.HttpProtocolIpv6).To(Equal(ec2types.LaunchTemplateInstanceMetadataProtocolIpv6Disabled))
				Expect(lo.FromPtr(ltInput.LaunchTemplateData.MetadataOptions.HttpPutResponseHopLimit)).To(BeNumerically("==", 1))
				Expect(ltInput.LaunchTemplateData.MetadataOptions.HttpTokens).To(Equal(ec2types.LaunchTemplateHttpTokensStateRequired))
			})
		})
		It("should set instance metadata tags to disabled", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically("==", 5))
			awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(ltInput.LaunchTemplateData.MetadataOptions.InstanceMetadataTags).To(Equal(ec2types.LaunchTemplateInstanceMetadataTagsStateDisabled))
			})
		})
	})
	Context("Networking", func() {
		Context("launch template respect to DNS ip for ipfamily selection", func() {
			DescribeTable(
				"should select correct ipFamily based on DNS ip",
				func(ipFamily corev1.IPFamily) {
					provider := launchtemplate.NewDefaultProvider(
						ctx,
						awsEnv.LaunchTemplateCache,
						awsEnv.EC2API,
						awsEnv.EKSAPI,
						awsEnv.AMIResolver,
						awsEnv.SecurityGroupProvider,
						awsEnv.SubnetProvider,
						awsEnv.LaunchTemplateProvider.CABundle,
						make(chan struct{}),
						net.ParseIP(lo.Ternary(ipFamily == corev1.IPv4Protocol, "10.0.100.10", "fd01:99f0:d47b::a")),
						"https://test-cluster",
					)
					Expect(provider.ClusterIPFamily).To(Equal(ipFamily))
				},
				Entry("DNS has ipv4 address", corev1.IPv4Protocol),
				Entry("DNS has ipv6 address", corev1.IPv6Protocol),
			)
		})
		Context("should provision a v6 address and set v6 primary IP as true when running in an ipv6 cluster", func() {
			DescribeTable(
				"should set Primary IPv6 as true and provision a IPv6 address",
				func(isPublicAddressSet, isPublic, isEFA bool) {
					awsEnv.LaunchTemplateProvider.ClusterIPFamily = corev1.IPv6Protocol
					if isPublicAddressSet {
						nodeClass.Spec.AssociatePublicIPAddress = lo.ToPtr(isPublic)
					}
					ExpectApplied(ctx, env.Client, nodePool, nodeClass)
					pod := coretest.UnschedulablePod(lo.Ternary(isEFA, coretest.PodOptions{
						ResourceRequirements: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{v1.ResourceEFA: resource.MustParse("2")},
							Limits:   corev1.ResourceList{v1.ResourceEFA: resource.MustParse("2")},
						},
					}, coretest.PodOptions{}))
					ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
					ExpectScheduled(ctx, env.Client, pod)
					input := awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Pop()

					Expect(len(input.LaunchTemplateData.NetworkInterfaces)).To(BeNumerically(">=", 1))
					if !isPublicAddressSet && !isEFA {
						Expect(input.LaunchTemplateData.NetworkInterfaces[0].AssociatePublicIpAddress).To(BeNil())
					}
					if isEFA {
						Expect(lo.FromPtr(input.LaunchTemplateData.NetworkInterfaces[0].InterfaceType)).To(Equal(string(ec2types.NetworkInterfaceTypeEfa)))
						Expect(lo.FromPtr(input.LaunchTemplateData.NetworkInterfaces[0].AssociatePublicIpAddress)).To(Equal(isPublic))
					}
					Expect(lo.FromPtr(input.LaunchTemplateData.NetworkInterfaces[0].Ipv6AddressCount)).To(Equal(int32(1)))
					Expect(lo.FromPtr(input.LaunchTemplateData.NetworkInterfaces[0].PrimaryIpv6)).To(BeTrue())

				},
				Entry("AssociatePublicIPAddress is not set and EFA is false", false, true, false),
				Entry("AssociatePublicIPAddress is not set and EFA is true", false, false, true),
				Entry("AssociatePublicIPAddress is set as true and EFA is true", true, true, true),
				Entry("AssociatePublicIPAddress is set as false and EFA is false", true, false, false),
			)
		})
	})
	It("should generate a unique launch template per capacity reservation", func() {
		crs := []ec2types.CapacityReservation{
			{
				AvailabilityZone:       lo.ToPtr("test-zone-1a"),
				InstanceType:           lo.ToPtr("m5.large"),
				OwnerId:                lo.ToPtr("012345678901"),
				InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
				CapacityReservationId:  lo.ToPtr("cr-m5.large-1a-1"),
				AvailableInstanceCount: lo.ToPtr[int32](10),
				State:                  ec2types.CapacityReservationStateActive,
				ReservationType:        ec2types.CapacityReservationTypeDefault,
			},
			{
				AvailabilityZone:       lo.ToPtr("test-zone-1a"),
				InstanceType:           lo.ToPtr("m5.large"),
				OwnerId:                lo.ToPtr("012345678901"),
				InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
				CapacityReservationId:  lo.ToPtr("cr-m5.large-1a-2"),
				AvailableInstanceCount: lo.ToPtr[int32](15),
				State:                  ec2types.CapacityReservationStateActive,
				ReservationType:        ec2types.CapacityReservationTypeDefault,
			},
			{
				AvailabilityZone:       lo.ToPtr("test-zone-1b"),
				InstanceType:           lo.ToPtr("m5.large"),
				OwnerId:                lo.ToPtr("012345678901"),
				InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
				CapacityReservationId:  lo.ToPtr("cr-m5.large-1b-1"),
				AvailableInstanceCount: lo.ToPtr[int32](10),
				State:                  ec2types.CapacityReservationStateActive,
				ReservationType:        ec2types.CapacityReservationTypeDefault,
			},
			{
				AvailabilityZone:       lo.ToPtr("test-zone-1b"),
				InstanceType:           lo.ToPtr("m5.xlarge"),
				OwnerId:                lo.ToPtr("012345678901"),
				InstanceMatchCriteria:  ec2types.InstanceMatchCriteriaTargeted,
				CapacityReservationId:  lo.ToPtr("cr-m5.xlarge-1b-1"),
				AvailableInstanceCount: lo.ToPtr[int32](15),
				State:                  ec2types.CapacityReservationStateActive,
				ReservationType:        ec2types.CapacityReservationTypeDefault,
			},
		}
		awsEnv.EC2API.DescribeCapacityReservationsOutput.Set(&ec2.DescribeCapacityReservationsOutput{
			CapacityReservations: crs,
		})
		for _, cr := range crs {
			nodeClass.Status.CapacityReservations = append(nodeClass.Status.CapacityReservations, lo.Must(v1.CapacityReservationFromEC2(fakeClock, &cr)))
			awsEnv.CapacityReservationProvider.SetAvailableInstanceCount(*cr.CapacityReservationId, int(*cr.AvailableInstanceCount))
		}

		nodePool.Spec.Template.Spec.Requirements = []karpv1.NodeSelectorRequirementWithMinValues{{NodeSelectorRequirement: corev1.NodeSelectorRequirement{
			Key:      karpv1.CapacityTypeLabelKey,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{karpv1.CapacityTypeReserved},
		}}}
		pod := coretest.UnschedulablePod()
		ExpectApplied(ctx, env.Client, pod, nodePool, nodeClass)
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)

		launchTemplates := map[string]*ec2.CreateLaunchTemplateInput{}
		for awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len() != 0 {
			lt := awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Pop()
			launchTemplates[*lt.LaunchTemplateName] = lt
		}
		// We should have created 3 launch templates, rather than 4 since we only create 1 launch template per capacity pool
		Expect(launchTemplates).To(HaveLen(3))
		reservationIDs := lo.Uniq(lo.Map(lo.Values(launchTemplates), func(input *ec2.CreateLaunchTemplateInput, _ int) string {
			return *input.LaunchTemplateData.CapacityReservationSpecification.CapacityReservationTarget.CapacityReservationId
		}))
		Expect(reservationIDs).To(HaveLen(3))
		Expect(reservationIDs).To(ConsistOf(
			// We don't include the m5.large offering in 1a because we select the zonal offering with the highest capacity
			"cr-m5.large-1a-2",
			"cr-m5.large-1b-1",
			"cr-m5.xlarge-1b-1",
		))
		for _, input := range launchTemplates {
			Expect(input.LaunchTemplateData.CapacityReservationSpecification.CapacityReservationPreference).To(Equal(ec2types.CapacityReservationPreferenceCapacityReservationsOnly))
		}

		// Validate that we generate one override per launch template, and the override is for the instance pool associated
		// with the capacity reservation.
		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).ToNot(Equal(0))
		createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(createFleetInput.LaunchTemplateConfigs).To(HaveLen(3))
		for _, ltc := range createFleetInput.LaunchTemplateConfigs {
			Expect(ltc.Overrides).To(HaveLen(1))
			Expect(launchTemplates).To(HaveKey(*ltc.LaunchTemplateSpecification.LaunchTemplateName))
			lt := launchTemplates[*ltc.LaunchTemplateSpecification.LaunchTemplateName]
			cr, ok := lo.Find(crs, func(cr ec2types.CapacityReservation) bool {
				return *cr.CapacityReservationId == *lt.LaunchTemplateData.CapacityReservationSpecification.CapacityReservationTarget.CapacityReservationId
			})
			Expect(ok).To(BeTrue())
			Expect(*ltc.Overrides[0].AvailabilityZone).To(Equal(*cr.AvailabilityZone))
			Expect(ltc.Overrides[0].InstanceType).To(Equal(ec2types.InstanceType(*cr.InstanceType)))
		}
	})
	DescribeTable(
		"should set the capacity reservation specification according to the capacity reservation feature flag",
		func(enabled bool) {
			coreoptions.FromContext(ctx).FeatureGates.ReservedCapacity = enabled

			pod := coretest.UnschedulablePod()
			ExpectApplied(ctx, env.Client, pod, nodePool, nodeClass)
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)

			var launchTemplates []*ec2.CreateLaunchTemplateInput
			for awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len() != 0 {
				launchTemplates = append(launchTemplates, awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Pop())
			}
			for _, input := range launchTemplates {
				crs := input.LaunchTemplateData.CapacityReservationSpecification
				if !enabled {
					Expect(crs).To(BeNil())
				} else {
					Expect(*crs).To(Equal(ec2types.LaunchTemplateCapacityReservationSpecificationRequest{
						CapacityReservationPreference: ec2types.CapacityReservationPreferenceNone,
					}))
				}
			}
		},
		Entry("enabled", true),
		Entry("disabled", false),
	)
})

// ExpectTags verifies that the expected tags are a subset of the tags found
func ExpectTags(tags []ec2types.Tag, expected map[string]string) {
	GinkgoHelper()
	existingTags := lo.SliceToMap(tags, func(t ec2types.Tag) (string, string) { return *t.Key, *t.Value })
	for expKey, expValue := range expected {
		foundValue, ok := existingTags[expKey]
		Expect(ok).To(BeTrue(), fmt.Sprintf("expected to find tag %s in %s", expKey, existingTags))
		Expect(foundValue).To(Equal(expValue))
	}
}

func ExpectLaunchTemplatesCreatedWithUserDataContaining(substrings ...string) {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
	awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		for _, substring := range substrings {
			ExpectWithOffset(2, string(userData)).To(ContainSubstring(substring))
		}
	})
}

func ExpectLaunchTemplatesCreatedWithUserDataNotContaining(substrings ...string) {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
	awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		for _, substring := range substrings {
			ExpectWithOffset(2, string(userData)).ToNot(ContainSubstring(substring))
		}
	})
}

func ExpectLaunchTemplatesCreatedWithUserData(expected string) {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
	awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		// Newlines are always added for missing TOML fields, so strip them out before comparisons.
		actualUserData := strings.ReplaceAll(string(userData), "\n", "")
		expectedUserData := strings.ReplaceAll(expected, "\n", "")
		ExpectWithOffset(2, actualUserData).To(Equal(expectedUserData))
	})
}

func ExpectUserDataExistsFromCreatedLaunchTemplates() []string {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.Len()).To(BeNumerically(">=", 1))
	userDatas := []string{}
	awsEnv.EC2API.CreateLaunchTemplateBehavior.CalledWithInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		userDatas = append(userDatas, string(userData))
	})
	return userDatas
}

func ExpectParseNodeConfigs(userData string) []admv1alpha1.NodeConfig {
	GinkgoHelper()
	archive, err := mime.NewArchive(userData)
	Expect(err).To(BeNil())
	nodeConfigs := lo.FilterMap(archive, func(entry mime.Entry, _ int) (admv1alpha1.NodeConfig, bool) {
		config := admv1alpha1.NodeConfig{}
		if entry.ContentType != mime.ContentTypeNodeConfig {
			return config, false
		}
		err := yaml.Unmarshal([]byte(entry.Content), &config)
		Expect(err).To(BeNil())
		return config, true
	})
	Expect(len(nodeConfigs)).To(BeNumerically(">=", 1))
	return nodeConfigs
}

func ExpectParseNodeConfigKubeletField[T any](userData, fieldName string) T {
	GinkgoHelper()
	configs := ExpectParseNodeConfigs(userData)
	Expect(len(configs)).To(Equal(1))
	var ret T
	raw := configs[0].Spec.Kubelet.Config[fieldName]
	Expect(yaml.Unmarshal(raw.Raw, &ret)).To(Succeed())
	return ret
}

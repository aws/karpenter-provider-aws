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
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	clock "k8s.io/utils/clock/testing"
	. "knative.dev/pkg/logging/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	corev1beta1 "github.com/aws/karpenter-core/pkg/apis/v1beta1"
	"github.com/aws/karpenter-core/pkg/controllers/provisioning"
	"github.com/aws/karpenter-core/pkg/controllers/state"
	"github.com/aws/karpenter-core/pkg/events"
	coreoptions "github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/operator/options"
	"github.com/aws/karpenter/pkg/providers/amifamily/bootstrap"
	"github.com/aws/karpenter/pkg/providers/instancetype"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var fakeClock *clock.FakeClock
var prov *provisioning.Provisioner
var cluster *state.Cluster
var cloudProvider *cloudprovider.CloudProvider

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx = settings.ToContext(ctx, test.Settings())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)

	fakeClock = &clock.FakeClock{}
	cloudProvider = cloudprovider.New(awsEnv.InstanceTypesProvider, awsEnv.InstanceProvider, events.NewRecorder(&record.FakeRecorder{}),
		env.Client, awsEnv.AMIProvider, awsEnv.SecurityGroupProvider, awsEnv.SubnetProvider)
	cluster = state.NewCluster(fakeClock, env.Client, cloudProvider)
	prov = provisioning.NewProvisioner(env.Client, env.KubernetesInterface.CoreV1(), events.NewRecorder(&record.FakeRecorder{}), cloudProvider, cluster)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx = settings.ToContext(ctx, test.Settings())
	cluster.Reset()
	awsEnv.Reset()

	awsEnv.LaunchTemplateProvider.KubeDNSIP = net.ParseIP("10.0.100.10")
	awsEnv.LaunchTemplateProvider.ClusterEndpoint = "https://test-cluster"
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("LaunchTemplates", func() {
	var nodePool *corev1beta1.NodePool
	var nodeClass *v1beta1.EC2NodeClass
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass()
		nodePool = coretest.NodePool(corev1beta1.NodePool{
			Spec: corev1beta1.NodePoolSpec{
				Template: corev1beta1.NodeClaimTemplate{
					ObjectMeta: corev1beta1.ObjectMeta{
						// TODO @joinnis: Move this into the coretest.NodePool function
						Labels: map[string]string{coretest.DiscoveryLabel: "unspecified"},
					},
					Spec: corev1beta1.NodeClaimSpec{
						Requirements: []v1.NodeSelectorRequirement{
							{
								Key:      corev1beta1.CapacityTypeLabelKey,
								Operator: v1.NodeSelectorOpIn,
								Values:   []string{corev1beta1.CapacityTypeOnDemand},
							},
						},
						NodeClassRef: &corev1beta1.NodeClassReference{
							Name: nodeClass.Name,
						},
					},
				},
			},
		})
	})
	It("should default to a generated launch template", func() {
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectScheduled(ctx, env.Client, pod)

		Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(BeNumerically("==", 1))
		createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
		Expect(len(createFleetInput.LaunchTemplateConfigs)).To(BeNumerically("==", awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()))
		Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
		awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
			launchTemplate, ok := lo.Find(createFleetInput.LaunchTemplateConfigs, func(ltConfig *ec2.FleetLaunchTemplateConfigRequest) bool {
				return *ltConfig.LaunchTemplateSpecification.LaunchTemplateName == *ltInput.LaunchTemplateName
			})
			Expect(ok).To(BeTrue())
			Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted).To(Equal(aws.Bool(true)))
			Expect(*launchTemplate.LaunchTemplateSpecification.Version).To(Equal("$Latest"))
		})
	})
	It("should fail to provision if the instance profile isn't defined", func() {
		nodeClass.Status.InstanceProfile = ""
		ExpectApplied(ctx, env.Client, nodePool, nodeClass)
		pod := coretest.UnschedulablePod()
		ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
		ExpectNotScheduled(ctx, env.Client, pod)
	})
	Context("Cache", func() {
		It("should use same launch template for equivalent constraints", func() {
			t1 := v1.Toleration{
				Key:      "Abacus",
				Operator: "Equal",
				Value:    "Zebra",
				Effect:   "NoSchedule",
			}
			t2 := v1.Toleration{
				Key:      "Zebra",
				Operator: "Equal",
				Value:    "Abacus",
				Effect:   "NoSchedule",
			}
			t3 := v1.Toleration{
				Key:      "Boar",
				Operator: "Equal",
				Value:    "Abacus",
				Effect:   "NoSchedule",
			}

			// constrain the packer to a single launch template type
			rr := v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:            resource.MustParse("24"),
					v1beta1.ResourceNVIDIAGPU: resource.MustParse("1"),
				},
				Limits: v1.ResourceList{v1beta1.ResourceNVIDIAGPU: resource.MustParse("1")},
			}

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod1 := coretest.UnschedulablePod(coretest.PodOptions{
				Tolerations:          []v1.Toleration{t1, t2, t3},
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
				Tolerations:          []v1.Toleration{t2, t3, t1},
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
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{MaxPods: aws.Int32(1)}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				ltName := aws.StringValue(ltInput.LaunchTemplateName)
				lt, ok := awsEnv.LaunchTemplateCache.Get(ltName)
				Expect(ok).To(Equal(true))
				// Remove expiration from cached LT
				awsEnv.LaunchTemplateCache.Set(ltName, lt, -1)
			})
			awsEnv.EC2API.CreateFleetBehavior.Error.Set(awserr.New("InvalidLaunchTemplateName.NotFoundException", "", nil), fake.MaxCalls(1))
			pod = coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			// should call fleet twice. Once will fail on invalid LT and the next will succeed
			Expect(awsEnv.EC2API.CreateFleetBehavior.FailedCalls()).To(BeNumerically("==", 1))
			Expect(awsEnv.EC2API.CreateFleetBehavior.SuccessfulCalls()).To(BeNumerically("==", 2))

		})
	})
	Context("Labels", func() {
		It("should apply labels to the node", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKey(v1.LabelOSStable))
			Expect(node.Labels).To(HaveKey(v1.LabelArchStable))
			Expect(node.Labels).To(HaveKey(v1.LabelInstanceTypeStable))
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
			Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, nodeClass.Spec.Tags)

			Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, nodeClass.Spec.Tags)

			Expect(*createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, nodeClass.Spec.Tags)
		})
		It("should request that tags be applied to both network interfaces and spot instance requests", func() {
			nodeClass.Spec.Tags = map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2value",
			}
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirement{
				{
					Key:      corev1beta1.CapacityTypeLabelKey,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{corev1beta1.CapacityTypeSpot},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(i *ec2.CreateLaunchTemplateInput) {
				Expect(i.LaunchTemplateData.TagSpecifications).To(HaveLen(2))

				// tags should be included in instance, volume, and fleet tag specification
				Expect(*i.LaunchTemplateData.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeNetworkInterface))
				ExpectTags(i.LaunchTemplateData.TagSpecifications[0].Tags, nodeClass.Spec.Tags)

				Expect(*i.LaunchTemplateData.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeSpotInstancesRequest))
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
			Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, nodeClass.Spec.Tags)

			Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, nodeClass.Spec.Tags)

			Expect(*createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, nodeClass.Spec.Tags)
		})
		It("should merge global tags into launch template and volume tags", func() {
			nodeClass.Spec.Tags = map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2value",
			}
			settingsTags := map[string]string{
				"customTag1": "value1",
				"customTag2": "value2",
			}
			ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
				Tags: settingsTags,
			}))

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.TagSpecifications).To(HaveLen(3))

			// tags should be included in instance, volume, and fleet tag specification
			Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, settingsTags)

			Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, settingsTags)

			Expect(*createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, settingsTags)
		})
		It("should override global tags with provider tags", func() {
			nodeClass.Spec.Tags = map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2value",
			}
			settingsTags := map[string]string{
				"tag1": "custom1",
				"tag2": "custom2",
			}
			ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
				Tags: settingsTags,
			}))
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := awsEnv.EC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.TagSpecifications).To(HaveLen(3))

			// tags should be included in instance, volume, and fleet tag specification
			Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, nodeClass.Spec.Tags)
			ExpectTagsNotFound(createFleetInput.TagSpecifications[0].Tags, settingsTags)

			Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, nodeClass.Spec.Tags)
			ExpectTagsNotFound(createFleetInput.TagSpecifications[0].Tags, settingsTags)

			Expect(*createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, nodeClass.Spec.Tags)
			ExpectTagsNotFound(createFleetInput.TagSpecifications[0].Tags, settingsTags)
		})
	})
	Context("Block Device Mappings", func() {
		It("should default AL2 block device mappings", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(len(ltInput.LaunchTemplateData.BlockDeviceMappings)).To(Equal(1))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(int64(20)))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal("gp3"))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops).To(BeNil())
			})
		})
		It("should use custom block device mapping", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
			nodeClass.Spec.BlockDeviceMappings = []*v1beta1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1beta1.BlockDevice{
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
					EBS: &v1beta1.BlockDevice{
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
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs).To(Equal(&ec2.LaunchTemplateEbsBlockDeviceRequest{
					VolumeSize:          aws.Int64(187),
					VolumeType:          aws.String("io2"),
					Iops:                aws.Int64(10_000),
					DeleteOnTermination: aws.Bool(true),
					Encrypted:           aws.Bool(true),
					KmsKeyId:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
				}))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs).To(Equal(&ec2.LaunchTemplateEbsBlockDeviceRequest{
					VolumeSize:          aws.Int64(200),
					VolumeType:          aws.String("io2"),
					Iops:                aws.Int64(10_000),
					DeleteOnTermination: aws.Bool(true),
					Encrypted:           aws.Bool(true),
					KmsKeyId:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
				}))
			})
		})
		It("should round up for custom block device mappings when specified in gigabytes", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
			nodeClass.Spec.BlockDeviceMappings = []*v1beta1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1beta1.BlockDevice{
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
					EBS: &v1beta1.BlockDevice{
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
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				// Both of these values are rounded up when converting to Gibibytes
				Expect(aws.Int64Value(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize)).To(BeNumerically("==", 4))
				Expect(aws.Int64Value(ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeSize)).To(BeNumerically("==", 2))
			})
		})
		It("should default bottlerocket second volume with root volume size", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(len(ltInput.LaunchTemplateData.BlockDeviceMappings)).To(Equal(2))
				// Bottlerocket control volume
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(int64(4)))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal("gp3"))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops).To(BeNil())
				// Bottlerocket user volume
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeSize).To(Equal(int64(20)))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeType).To(Equal("gp3"))
				Expect(ltInput.LaunchTemplateData.BlockDeviceMappings[1].Ebs.Iops).To(BeNil())
			})
		})
		It("should not default block device mappings for custom AMIFamilies", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(len(ltInput.LaunchTemplateData.BlockDeviceMappings)).To(Equal(0))
			})
		})
		It("should use custom block device mapping for custom AMIFamilies", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
			nodeClass.Spec.BlockDeviceMappings = []*v1beta1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1beta1.BlockDevice{
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
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(len(ltInput.LaunchTemplateData.BlockDeviceMappings)).To(Equal(1))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(int64(40)))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal("io2"))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops).To(Equal(int64(10_000)))
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.DeleteOnTermination).To(BeTrue())
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted).To(BeTrue())
				Expect(*ltInput.LaunchTemplateData.BlockDeviceMappings[0].Ebs.KmsKeyId).To(Equal("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"))
			})
		})
	})
	Context("Ephemeral Storage", func() {
		It("should pack pods when a daemonset has an ephemeral-storage request", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass, coretest.DaemonSet(
				coretest.DaemonSetOptions{PodOptions: coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"),
							v1.ResourceMemory:           resource.MustParse("1Gi"),
							v1.ResourceEphemeralStorage: resource.MustParse("1Gi")}},
				}},
			))
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods with any ephemeral-storage request", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("1G"),
				}}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods with large ephemeral-storage request", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
				}}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should not pack pods if the sum of pod ephemeral-storage and overhead exceeds node capacity", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("19Gi"),
				}}})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should launch multiple nodes if sum of pod ephemeral-storage requests exceeds a single nodes capacity", func() {
			var nodes []*v1.Node
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pods := []*v1.Pod{
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
					},
				},
				}),
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
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
			pods := []*v1.Pod{
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
					},
				},
				}),
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("150Gi"),
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
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("150Gi"),
				},
			},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should pack pods using the blockdevicemappings from the provider spec when defined", func() {
			nodeClass.Spec.BlockDeviceMappings = []*v1beta1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1beta1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(50, resource.Giga),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					EBS: &v1beta1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceEphemeralStorage: resource.MustParse("25Gi"),
				},
			},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			// capacity isn't recorded on the node any longer, but we know the pod should schedule
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods using blockdevicemappings for Custom AMIFamily", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
			nodeClass.Spec.BlockDeviceMappings = []*v1beta1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1beta1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					EBS: &v1beta1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(40, resource.Giga),
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					// this pod can only be satisfied if `/dev/xvdb` will house all the pods.
					v1.ResourceEphemeralStorage: resource.MustParse("25Gi"),
				},
			},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			// capacity isn't recorded on the node any longer, but we know the pod should schedule
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods using the configured root volume in blockdevicemappings", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
			nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
			nodeClass.Spec.BlockDeviceMappings = []*v1beta1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1beta1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					EBS: &v1beta1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(40, resource.Giga),
					},
					RootVolume: true,
				},
				{
					DeviceName: aws.String("/dev/xvdc"),
					EBS: &v1beta1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					},
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
				Requests: map[v1.ResourceName]resource.Quantity{
					// this pod can only be satisfied if `/dev/xvdb` will house all the pods.
					v1.ResourceEphemeralStorage: resource.MustParse("25Gi"),
				},
			},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)

			// capacity isn't recorded on the node any longer, but we know the pod should schedule
			ExpectScheduled(ctx, env.Client, pod)
		})
	})
	Context("AL2", func() {
		var info *ec2.InstanceTypeInfo
		BeforeEach(func() {
			var ok bool
			var instanceInfo []*ec2.InstanceTypeInfo
			err := awsEnv.EC2API.DescribeInstanceTypesPagesWithContext(ctx, &ec2.DescribeInstanceTypesInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("supported-virtualization-type"),
						Values: []*string{aws.String("hvm")},
					},
					{
						Name:   aws.String("processor-info.supported-architecture"),
						Values: aws.StringSlice([]string{"x86_64", "arm64"}),
					},
				},
			}, func(page *ec2.DescribeInstanceTypesOutput, lastPage bool) bool {
				instanceInfo = append(instanceInfo, page.InstanceTypes...)
				return true
			})
			Expect(err).To(BeNil())
			info, ok = lo.Find(instanceInfo, func(i *ec2.InstanceTypeInfo) bool {
				return aws.StringValue(i.InstanceType) == "m5.xlarge"
			})
			Expect(ok).To(BeTrue())
		})

		It("should calculate memory overhead based on eni limited pods when ENI limited", func() {
			ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
				EnableENILimitedPodDensity: lo.ToPtr(false),
			}))

			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				VMMemoryOverheadPercent: lo.ToPtr[float64](0),
			}))

			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
			it := instancetype.NewInstanceType(ctx, info, nodePool.Spec.Template.Spec.Kubelet, "", nodeClass, nil)
			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("993Mi"))
		})
		It("should calculate memory overhead based on eni limited pods when not ENI limited", func() {
			ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
				EnableENILimitedPodDensity: lo.ToPtr(false),
			}))

			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				VMMemoryOverheadPercent: lo.ToPtr[float64](0),
			}))

			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
			it := instancetype.NewInstanceType(ctx, info, nodePool.Spec.Template.Spec.Kubelet, "", nodeClass, nil)
			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("993Mi"))
		})
	})
	Context("Bottlerocket", func() {
		var info *ec2.InstanceTypeInfo
		BeforeEach(func() {
			var ok bool
			var instanceInfo []*ec2.InstanceTypeInfo
			err := awsEnv.EC2API.DescribeInstanceTypesPagesWithContext(ctx, &ec2.DescribeInstanceTypesInput{
				Filters: []*ec2.Filter{
					{
						Name:   aws.String("supported-virtualization-type"),
						Values: []*string{aws.String("hvm")},
					},
					{
						Name:   aws.String("processor-info.supported-architecture"),
						Values: aws.StringSlice([]string{"x86_64", "arm64"}),
					},
				},
			}, func(page *ec2.DescribeInstanceTypesOutput, lastPage bool) bool {
				instanceInfo = append(instanceInfo, page.InstanceTypes...)
				return true
			})
			Expect(err).To(BeNil())
			info, ok = lo.Find(instanceInfo, func(i *ec2.InstanceTypeInfo) bool {
				return aws.StringValue(i.InstanceType) == "m5.xlarge"
			})
			Expect(ok).To(BeTrue())
		})

		It("should calculate memory overhead based on eni limited pods when ENI limited", func() {
			ctx = settings.ToContext(ctx, test.Settings())
			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				VMMemoryOverheadPercent: lo.ToPtr[float64](0),
			}))

			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
			it := instancetype.NewInstanceType(ctx, info, nodePool.Spec.Template.Spec.Kubelet, "", nodeClass, nil)
			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("993Mi"))
		})
		It("should calculate memory overhead based on max pods when not ENI limited", func() {
			ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
				EnableENILimitedPodDensity: lo.ToPtr(false),
			}))

			ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
				VMMemoryOverheadPercent: lo.ToPtr[float64](0),
			}))

			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
			it := instancetype.NewInstanceType(ctx, info, nodePool.Spec.Template.Spec.Kubelet, "", nodeClass, nil)
			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("1565Mi"))
		})
	})
	Context("User Data", func() {
		It("should specify --use-max-pods=false when using ENI-based pod density", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--use-max-pods false")
		})
		It("should specify --use-max-pods=false when not using ENI-based pod density", func() {
			ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
				EnableENILimitedPodDensity: lo.ToPtr(false),
			}))

			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--use-max-pods false", "--max-pods=110")
		})
		It("should specify --use-max-pods=false and --max-pods user value when user specifies maxPods in Provisioner", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{MaxPods: aws.Int32(10)}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--use-max-pods false", "--max-pods=10")
		})
		It("should specify --system-reserved when overriding system reserved values", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				SystemReserved: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("500m"),
					v1.ResourceMemory:           resource.MustParse("1Gi"),
					v1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
				Expect(err).To(BeNil())

				// Check whether the arguments are there for --system-reserved
				arg := "--system-reserved="
				i := strings.Index(string(userData), arg)
				rem := string(userData)[(i + len(arg)):]
				i = strings.Index(rem, "'")
				for k, v := range nodePool.Spec.Template.Spec.Kubelet.SystemReserved {
					Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v=%v", k.String(), v.String())))
				}
			})
		})
		It("should specify --kube-reserved when overriding system reserved values", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				KubeReserved: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("500m"),
					v1.ResourceMemory:           resource.MustParse("1Gi"),
					v1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
				Expect(err).To(BeNil())

				// Check whether the arguments are there for --kube-reserved
				arg := "--kube-reserved="
				i := strings.Index(string(userData), arg)
				rem := string(userData)[(i + len(arg)):]
				i = strings.Index(rem, "'")
				for k, v := range nodePool.Spec.Template.Spec.Kubelet.KubeReserved {
					Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v=%v", k.String(), v.String())))
				}
			})
		})
		It("should pass eviction hard threshold values when specified", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
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
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
				Expect(err).To(BeNil())

				// Check whether the arguments are there for --kube-reserved
				arg := "--eviction-hard="
				i := strings.Index(string(userData), arg)
				rem := string(userData)[(i + len(arg)):]
				i = strings.Index(rem, "'")
				for k, v := range nodePool.Spec.Template.Spec.Kubelet.EvictionHard {
					Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v<%v", k, v)))
				}
			})
		})
		It("should pass eviction soft threshold values when specified", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
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
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
				Expect(err).To(BeNil())

				// Check whether the arguments are there for --kube-reserved
				arg := "--eviction-soft="
				i := strings.Index(string(userData), arg)
				rem := string(userData)[(i + len(arg)):]
				i = strings.Index(rem, "'")
				for k, v := range nodePool.Spec.Template.Spec.Kubelet.EvictionSoft {
					Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v<%v", k, v)))
				}
			})
		})
		It("should pass eviction soft grace period values when specified", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
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
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
				Expect(err).To(BeNil())

				// Check whether the arguments are there for --kube-reserved
				arg := "--eviction-soft-grace-period="
				i := strings.Index(string(userData), arg)
				rem := string(userData)[(i + len(arg)):]
				i = strings.Index(rem, "'")
				for k, v := range nodePool.Spec.Template.Spec.Kubelet.EvictionSoftGracePeriod {
					Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v=%v", k, v.Duration.String())))
				}
			})
		})
		It("should pass eviction max pod grace period when specified", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				EvictionMaxPodGracePeriod: aws.Int32(300),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining(fmt.Sprintf("--eviction-max-pod-grace-period=%d", 300))
		})
		It("should specify --pods-per-core", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				PodsPerCore: aws.Int32(2),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining(fmt.Sprintf("--pods-per-core=%d", 2))
		})
		It("should specify --pods-per-core with --max-pods enabled", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				PodsPerCore: aws.Int32(2),
				MaxPods:     aws.Int32(100),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining(fmt.Sprintf("--pods-per-core=%d", 2), fmt.Sprintf("--max-pods=%d", 100))
		})
		It("should specify --container-runtime containerd by default", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--container-runtime containerd")
		})
		It("should specify --container-runtime containerd when using Neuron GPUs", func() {
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirement{{Key: v1beta1.LabelInstanceCategory, Operator: v1.NodeSelectorOpExists}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceCPU:            resource.MustParse("1"),
						v1beta1.ResourceAWSNeuron: resource.MustParse("1"),
					},
					Limits: map[v1.ResourceName]resource.Quantity{
						v1beta1.ResourceAWSNeuron: resource.MustParse("1"),
					},
				},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--container-runtime containerd")
		})
		It("should specify --container-runtime containerd when using Nvidia GPUs", func() {
			nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirement{{Key: v1beta1.LabelInstanceCategory, Operator: v1.NodeSelectorOpExists}}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceCPU:            resource.MustParse("1"),
						v1beta1.ResourceNVIDIAGPU: resource.MustParse("1"),
					},
					Limits: map[v1.ResourceName]resource.Quantity{
						v1beta1.ResourceNVIDIAGPU: resource.MustParse("1"),
					},
				},
			})
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--container-runtime containerd")
		})
		It("should specify --dns-cluster-ip and --ip-family when running in an ipv6 cluster", func() {
			awsEnv.LaunchTemplateProvider.KubeDNSIP = net.ParseIP("fd4b:121b:812b::a")
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--dns-cluster-ip 'fd4b:121b:812b::a'")
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--ip-family ipv6")
		})
		It("should specify --dns-cluster-ip when running in an ipv4 cluster", func() {
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--dns-cluster-ip '10.0.100.10'")
		})
		It("should pass ImageGCHighThresholdPercent when specified", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				ImageGCHighThresholdPercent: aws.Int32(50),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--image-gc-high-threshold=50")
		})
		It("should pass ImageGCLowThresholdPercent when specified", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				ImageGCLowThresholdPercent: aws.Int32(50),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--image-gc-low-threshold=50")
		})
		It("should pass --cpu-fs-quota when specified", func() {
			nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
				CPUCFSQuota: aws.Bool(false),
			}
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataContaining("--cpu-cfs-quota=false")
		})
		It("should not pass any labels prefixed with the node-restriction.kubernetes.io domain", func() {
			nodePool.Spec.Template.Labels = lo.Assign(nodePool.Spec.Template.Labels, map[string]string{
				v1.LabelNamespaceNodeRestriction + "/team":         "team-1",
				v1.LabelNamespaceNodeRestriction + "/custom-label": "custom-value",
			})
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			ExpectLaunchTemplatesCreatedWithUserDataNotContaining(v1.LabelNamespaceNodeRestriction)
		})
		Context("Bottlerocket", func() {
			It("should merge in custom user data", func() {
				ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
				}))
				content, err := os.ReadFile("testdata/br_userdata_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(fmt.Sprintf(string(content), corev1beta1.NodePoolLabelKey))
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				nodePool.Spec.Template.Spec.Taints = []v1.Taint{{Key: "foo", Value: "bar", Effect: v1.TaintEffectNoExecute}}
				nodePool.Spec.Template.Spec.StartupTaints = []v1.Taint{{Key: "baz", Value: "bin", Effect: v1.TaintEffectNoExecute}}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod(coretest.PodOptions{
					Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
				})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/br_userdata_merged.golden")
				Expect(err).To(BeNil())
				ExpectLaunchTemplatesCreatedWithUserData(fmt.Sprintf(string(content), corev1beta1.NodePoolLabelKey, nodePool.Name))
			})
			It("should bootstrap when custom user data is empty", func() {
				ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
				}))
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				nodePool.Spec.Template.Spec.Taints = []v1.Taint{{Key: "foo", Value: "bar", Effect: v1.TaintEffectNoExecute}}
				nodePool.Spec.Template.Spec.StartupTaints = []v1.Taint{{Key: "baz", Value: "bin", Effect: v1.TaintEffectNoExecute}}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
				pod := coretest.UnschedulablePod(coretest.PodOptions{
					Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
				})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err := os.ReadFile("testdata/br_userdata_unmerged.golden")
				Expect(err).To(BeNil())
				ExpectLaunchTemplatesCreatedWithUserData(fmt.Sprintf(string(content), corev1beta1.NodePoolLabelKey, nodePool.Name))
			})
			It("should not bootstrap when provider ref points to a non-existent resource", func() {
				ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
				}))

				nodePool.Spec.Template.Spec.NodeClassRef = &corev1beta1.NodeClassReference{Name: "doesnotexist"}
				ExpectApplied(ctx, env.Client, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				// This will not be scheduled since we were pointed to a non-existent awsnodetemplate resource.
				ExpectNotScheduled(ctx, env.Client, pod)
			})
			It("should not bootstrap on invalid toml user data", func() {
				nodeClass.Spec.UserData = aws.String("#/bin/bash\n ./not-toml.sh")
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				ExpectApplied(ctx, env.Client, nodeClass)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: nodeClass.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				// This will not be scheduled since userData cannot be generated for the prospective node.
				ExpectNotScheduled(ctx, env.Client, pod)
			})
			It("should override system reserved values in user data", func() {
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				ExpectApplied(ctx, env.Client, nodeClass)
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					SystemReserved: v1.ResourceList{
						v1.ResourceCPU:              resource.MustParse("2"),
						v1.ResourceMemory:           resource.MustParse("3Gi"),
						v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
					},
				}
				ExpectApplied(ctx, env.Client, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(len(config.Settings.Kubernetes.SystemReserved)).To(Equal(3))
					Expect(config.Settings.Kubernetes.SystemReserved[v1.ResourceCPU.String()]).To(Equal("2"))
					Expect(config.Settings.Kubernetes.SystemReserved[v1.ResourceMemory.String()]).To(Equal("3Gi"))
					Expect(config.Settings.Kubernetes.SystemReserved[v1.ResourceEphemeralStorage.String()]).To(Equal("10Gi"))
				})
			})
			It("should override kube reserved values in user data", func() {
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				ExpectApplied(ctx, env.Client, nodeClass)
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					KubeReserved: v1.ResourceList{
						v1.ResourceCPU:              resource.MustParse("2"),
						v1.ResourceMemory:           resource.MustParse("3Gi"),
						v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
					},
				}
				ExpectApplied(ctx, env.Client, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(len(config.Settings.Kubernetes.KubeReserved)).To(Equal(3))
					Expect(config.Settings.Kubernetes.KubeReserved[v1.ResourceCPU.String()]).To(Equal("2"))
					Expect(config.Settings.Kubernetes.KubeReserved[v1.ResourceMemory.String()]).To(Equal("3Gi"))
					Expect(config.Settings.Kubernetes.KubeReserved[v1.ResourceEphemeralStorage.String()]).To(Equal("10Gi"))
				})
			})
			It("should override kube reserved values in user data", func() {
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				ExpectApplied(ctx, env.Client, nodeClass)
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					EvictionHard: map[string]string{
						"memory.available":  "10%",
						"nodefs.available":  "15%",
						"nodefs.inodesFree": "5%",
					},
				}
				ExpectApplied(ctx, env.Client, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
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
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					MaxPods: aws.Int32(10),
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(config.Settings.Kubernetes.MaxPods).ToNot(BeNil())
					Expect(*config.Settings.Kubernetes.MaxPods).To(BeNumerically("==", 10))
				})
			})
			It("should pass ImageGCHighThresholdPercent when specified", func() {
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					ImageGCHighThresholdPercent: aws.Int32(50),
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
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
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					ImageGCLowThresholdPercent: aws.Int32(50),
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
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
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(config.Settings.Kubernetes.ClusterDNSIP).ToNot(BeNil())
					Expect(*config.Settings.Kubernetes.ClusterDNSIP).To(Equal("10.0.100.10"))
				})
			})
			It("should pass CPUCFSQuota when specified", func() {
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyBottlerocket
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{
					CPUCFSQuota: aws.Bool(false),
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					userData, err := base64.StdEncoding.DecodeString(*ltInput.LaunchTemplateData.UserData)
					Expect(err).To(BeNil())
					config := &bootstrap.BottlerocketConfig{}
					Expect(config.UnmarshalTOML(userData)).To(Succeed())
					Expect(config.Settings.Kubernetes.CPUCFSQuota).ToNot(BeNil())
					Expect(*config.Settings.Kubernetes.CPUCFSQuota).To(BeFalse())
				})
			})
		})
		Context("AL2 Custom UserData", func() {
			It("should merge in custom user data", func() {
				ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
				}))

				content, err := os.ReadFile("testdata/al2_userdata_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(string(content))
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/al2_userdata_merged.golden")
				Expect(err).To(BeNil())
				expectedUserData := fmt.Sprintf(string(content), corev1beta1.NodePoolLabelKey, nodePool.Name)
				ExpectLaunchTemplatesCreatedWithUserData(expectedUserData)
			})
			It("should merge in custom user data when Content-Type is before MIME-Version", func() {
				ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
				}))

				content, err := os.ReadFile("testdata/al2_userdata_content_type_first_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(string(content))
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/al2_userdata_merged.golden")
				Expect(err).To(BeNil())
				expectedUserData := fmt.Sprintf(string(content), corev1beta1.NodePoolLabelKey, nodePool.Name)
				ExpectLaunchTemplatesCreatedWithUserData(expectedUserData)
			})
			It("should merge in custom user data not in multi-part mime format", func() {
				ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
				}))

				content, err := os.ReadFile("testdata/al2_no_mime_userdata_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(string(content))
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/al2_userdata_merged.golden")
				Expect(err).To(BeNil())
				expectedUserData := fmt.Sprintf(string(content), corev1beta1.NodePoolLabelKey, nodePool.Name)
				ExpectLaunchTemplatesCreatedWithUserData(expectedUserData)
			})
			It("should handle empty custom user data", func() {
				ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
				}))
				nodeClass.Spec.UserData = nil
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err := os.ReadFile("testdata/al2_userdata_unmerged.golden")
				Expect(err).To(BeNil())
				expectedUserData := fmt.Sprintf(string(content), corev1beta1.NodePoolLabelKey, nodePool.Name)
				ExpectLaunchTemplatesCreatedWithUserData(expectedUserData)
			})
		})
		Context("Custom AMI Selector", func() {
			It("should use ami selector specified in AWSNodeTemplate", func() {
				nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
				}})
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					Expect("ami-123").To(Equal(*ltInput.LaunchTemplateData.ImageId))
				})
			})
			It("should copy over userData untouched when AMIFamily is Custom", func() {
				nodeClass.Spec.UserData = aws.String("special user data")
				nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyCustom
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
				}})
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				ExpectLaunchTemplatesCreatedWithUserData("special user data")
			})
			It("should correctly use ami selector with specific IDs in AWSNodeTemplate", func() {
				nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: "ami-123"}, {ID: "ami-456"}}
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						Tags:         []*ec2.Tag{{Key: aws.String(v1.LabelInstanceTypeStable), Value: aws.String("m5.large")}},
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-456"),
						Architecture: aws.String("x86_64"),
						Tags:         []*ec2.Tag{{Key: aws.String(v1.LabelInstanceTypeStable), Value: aws.String("m5.xlarge")}},
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
				}})
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 2))
				actualFilter := awsEnv.EC2API.CalledWithDescribeImagesInput.Pop().Filters
				expectedFilter := []*ec2.Filter{
					{
						Name:   aws.String("image-id"),
						Values: aws.StringSlice([]string{"ami-123", "ami-456"}),
					},
				}
				Expect(actualFilter).To(Equal(expectedFilter))
			})
			It("should create multiple launch templates when multiple amis are discovered with non-equivalent requirements", func() {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-456"),
						Architecture: aws.String("arm64"),
						CreationDate: aws.String("2022-08-10T12:00:00Z"),
					},
				}})
				nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 2))
				expectedImageIds := sets.New[string]("ami-123", "ami-456")
				actualImageIds := sets.New[string]()
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					actualImageIds.Insert(*ltInput.LaunchTemplateData.ImageId)
				})
				Expect(expectedImageIds.Equal(actualImageIds)).To(BeTrue())
			})
			It("should create a launch template with the newest compatible AMI when multiple amis are discovered", func() {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2020-01-01T12:00:00Z"),
					},
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-456"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2021-01-01T12:00:00Z"),
					},
					{
						// Incompatible because required ARM64
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("ami-789"),
						Architecture: aws.String("arm64"),
						CreationDate: aws.String("2022-01-01T12:00:00Z"),
					},
				}})
				nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				ExpectApplied(ctx, env.Client, nodeClass)
				nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirement{
					{
						Key:      v1.LabelArchStable,
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{corev1beta1.ArchitectureAmd64},
					},
				}
				ExpectApplied(ctx, env.Client, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
				awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
					Expect("ami-456").To(Equal(*ltInput.LaunchTemplateData.ImageId))
				})
			})

			It("should fail if no amis match selector.", func() {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{}})
				nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				ExpectApplied(ctx, env.Client, nodeClass)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: nodeClass.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectNotScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(0))
			})
			It("should fail if no instanceType matches ami requirements.", func() {
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{Name: aws.String(coretest.RandomName()), ImageId: aws.String("ami-123"), Architecture: aws.String("newnew"), CreationDate: aws.String("2022-01-01T12:00:00Z")}}})
				nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{Tags: map[string]string{"*": "*"}}}
				ExpectApplied(ctx, env.Client, nodeClass)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.MachineTemplateRef{Name: nodeClass.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectNotScheduled(ctx, env.Client, pod)
				Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(0))
			})
			It("should choose amis from SSM if no selector specified in AWSNodeTemplate", func() {
				version := lo.Must(awsEnv.VersionProvider.Get(ctx))
				awsEnv.SSMAPI.Parameters = map[string]string{
					fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2/recommended/image_id", version): "test-ami-123",
				}
				awsEnv.EC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						Name:         aws.String(coretest.RandomName()),
						ImageId:      aws.String("test-ami-123"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
				}})
				ExpectApplied(ctx, env.Client, nodeClass)
				ExpectApplied(ctx, env.Client, nodePool)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				input := awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Pop()
				Expect(*input.LaunchTemplateData.ImageId).To(ContainSubstring("test-ami"))
			})
		})
		Context("Subnet-based Launch Template Configration", func() {
			It("should explicitly set 'AssignPublicIPv4' to false in the Launch Template", func() {
				nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
					{Tags: map[string]string{"Name": "test-subnet-1"}},
					{Tags: map[string]string{"Name": "test-subnet-3"}},
				}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				input := awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Pop()
				Expect(*input.LaunchTemplateData.NetworkInterfaces[0].AssociatePublicIpAddress).To(BeFalse())
			})

			It("should not explicitly set 'AssignPublicIPv4' when the subnets are configured to assign public IPv4 addresses", func() {
				nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{{Tags: map[string]string{"Name": "test-subnet-2"}}}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				input := awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Pop()
				Expect(len(input.LaunchTemplateData.NetworkInterfaces)).To(BeNumerically("==", 0))
			})
		})
		Context("Kubelet Args", func() {
			It("should specify the --dns-cluster-ip flag when clusterDNSIP is set", func() {
				nodePool.Spec.Template.Spec.Kubelet = &corev1beta1.KubeletConfiguration{ClusterDNS: []string{"10.0.10.100"}}
				ExpectApplied(ctx, env.Client, nodePool, nodeClass)
				pod := coretest.UnschedulablePod()
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				ExpectLaunchTemplatesCreatedWithUserDataContaining("--dns-cluster-ip '10.0.10.100'")
			})
		})
		Context("Windows Custom UserData", func() {
			BeforeEach(func() {
				ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
				}))
				nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirement{{Key: v1.LabelOSStable, Operator: v1.NodeSelectorOpIn, Values: []string{string(v1.Windows)}}}
				nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyWindows2022
			})
			It("should merge and bootstrap with custom user data", func() {
				content, err := os.ReadFile("testdata/windows_userdata_input.golden")
				Expect(err).To(BeNil())
				nodeClass.Spec.UserData = aws.String(string(content))
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
				pod := coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{
						v1.LabelOSStable:     string(v1.Windows),
						v1.LabelWindowsBuild: "10.0.20348",
					},
				})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err = os.ReadFile("testdata/windows_userdata_merged.golden")
				Expect(err).To(BeNil())
				ExpectLaunchTemplatesCreatedWithUserData(fmt.Sprintf(string(content), corev1beta1.NodePoolLabelKey, nodePool.Name))
			})
			It("should bootstrap when custom user data is empty", func() {
				ExpectApplied(ctx, env.Client, nodeClass, nodePool)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
				pod := coretest.UnschedulablePod(coretest.PodOptions{
					NodeSelector: map[string]string{
						v1.LabelOSStable:     string(v1.Windows),
						v1.LabelWindowsBuild: "10.0.20348",
					},
				})
				ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
				ExpectScheduled(ctx, env.Client, pod)
				content, err := os.ReadFile("testdata/windows_userdata_unmerged.golden")
				Expect(err).To(BeNil())
				ExpectLaunchTemplatesCreatedWithUserData(fmt.Sprintf(string(content), corev1beta1.NodePoolLabelKey, nodePool.Name))
			})
		})
	})
	Context("Detailed Monitoring", func() {
		It("should default detailed monitoring to off", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(aws.BoolValue(ltInput.LaunchTemplateData.Monitoring.Enabled)).To(BeFalse())
			})
		})
		It("should pass detailed monitoring setting to the launch template at creation", func() {
			nodeClass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2
			nodeClass.Spec.DetailedMonitoring = aws.Bool(true)
			ExpectApplied(ctx, env.Client, nodePool, nodeClass)
			pod := coretest.UnschedulablePod()
			ExpectProvisioned(ctx, env.Client, cluster, cloudProvider, prov, pod)
			ExpectScheduled(ctx, env.Client, pod)
			Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
			awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(ltInput *ec2.CreateLaunchTemplateInput) {
				Expect(aws.BoolValue(ltInput.LaunchTemplateData.Monitoring.Enabled)).To(BeTrue())
			})
		})
	})
})

// ExpectTags verifies that the expected tags are a subset of the tags found
func ExpectTags(tags []*ec2.Tag, expected map[string]string) {
	GinkgoHelper()
	existingTags := lo.SliceToMap(tags, func(t *ec2.Tag) (string, string) { return *t.Key, *t.Value })
	for expKey, expValue := range expected {
		foundValue, ok := existingTags[expKey]
		Expect(ok).To(BeTrue(), fmt.Sprintf("expected to find tag %s in %s", expKey, existingTags))
		Expect(foundValue).To(Equal(expValue))
	}
}

func ExpectTagsNotFound(tags []*ec2.Tag, expectNotFound map[string]string) {
	GinkgoHelper()
	existingTags := lo.SliceToMap(tags, func(t *ec2.Tag) (string, string) { return *t.Key, *t.Value })
	for k, v := range expectNotFound {
		elem, ok := existingTags[k]
		Expect(!ok || v != elem).To(BeTrue())
	}
}

func ExpectLaunchTemplatesCreatedWithUserDataContaining(substrings ...string) {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
	awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		for _, substring := range substrings {
			ExpectWithOffset(2, string(userData)).To(ContainSubstring(substring))
		}
	})
}

func ExpectLaunchTemplatesCreatedWithUserDataNotContaining(substrings ...string) {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
	awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		for _, substring := range substrings {
			ExpectWithOffset(2, string(userData)).ToNot(ContainSubstring(substring))
		}
	})
}

func ExpectLaunchTemplatesCreatedWithUserData(expected string) {
	GinkgoHelper()
	Expect(awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.Len()).To(BeNumerically(">=", 1))
	awsEnv.EC2API.CalledWithCreateLaunchTemplateInput.ForEach(func(input *ec2.CreateLaunchTemplateInput) {
		userData, err := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
		ExpectWithOffset(2, err).To(BeNil())
		// Newlines are always added for missing TOML fields, so strip them out before comparisons.
		actualUserData := strings.Replace(string(userData), "\n", "", -1)
		expectedUserData := strings.Replace(expected, "\n", "", -1)
		ExpectWithOffset(2, actualUserData).To(Equal(expectedUserData))
	})
}

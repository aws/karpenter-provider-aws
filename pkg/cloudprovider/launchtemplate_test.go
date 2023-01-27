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

package cloudprovider

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"sort"
	"strings"
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
	"sigs.k8s.io/controller-runtime/pkg/client"

	awssettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider/amifamily/bootstrap"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/controllers/provisioning"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
)

var _ = Describe("LaunchTemplates", func() {
	It("should default to a generated launch template", func() {
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)

		Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))

		firstLt := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()

		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))

		createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
		launchTemplate := createFleetInput.LaunchTemplateConfigs[0].LaunchTemplateSpecification
		Expect(createFleetInput.LaunchTemplateConfigs).To(HaveLen(1))

		Expect(*createFleetInput.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName).
			To(Equal(*firstLt.LaunchTemplateName))
		Expect(firstLt.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted).To(Equal(aws.Bool(true)))
		Expect(*launchTemplate.Version).To(Equal("$Latest"))
	})
	It("should order spot launch template overrides by offering pricing", func() {
		provisioner = test.Provisioner(coretest.ProvisionerOptions{
			Requirements: []v1.NodeSelectorRequirement{
				{
					Key:      v1alpha5.LabelCapacityType,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{v1alpha5.CapacityTypeSpot},
				},
			},
			ProviderRef: &v1alpha5.ProviderRef{
				APIVersion: nodeTemplate.APIVersion,
				Kind:       nodeTemplate.Kind,
				Name:       nodeTemplate.Name,
			},
		})
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
		ExpectScheduled(ctx, env.Client, pod)

		Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
		Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
		createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()

		// Expect these values to be correctly ordered by price
		overrides := createFleetInput.LaunchTemplateConfigs[0].Overrides
		sort.Slice(overrides, func(i, j int) bool {
			return aws.Float64Value(overrides[i].Priority) < aws.Float64Value(overrides[j].Priority)
		})
		lastPrice := -math.MaxFloat64
		for _, override := range overrides {
			offeringPrice, ok := pricingProvider.SpotPrice(*override.InstanceType, *override.AvailabilityZone)
			Expect(ok).To(BeTrue())
			Expect(offeringPrice).To(BeNumerically(">=", lastPrice))
			lastPrice = offeringPrice
		}
	})
	Context("LaunchTemplateName", func() {
		It("should allow a launch template to be specified", func() {
			nodeTemplate.Spec.LaunchTemplateName = aws.String("test-launch-template")
			nodeTemplate.Spec.SecurityGroupSelector = nil
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			input := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(input.LaunchTemplateConfigs).To(HaveLen(1))
			launchTemplate := input.LaunchTemplateConfigs[0].LaunchTemplateSpecification
			Expect(*launchTemplate.LaunchTemplateName).To(Equal("test-launch-template"))
			Expect(*launchTemplate.Version).To(Equal("$Latest"))
		})
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
					v1.ResourceCPU:             resource.MustParse("24"),
					v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1"),
				},
				Limits: v1.ResourceList{v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1")},
			}

			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod1 := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{
					Tolerations:          []v1.Toleration{t1, t2, t3},
					ResourceRequirements: rr,
				}),
			)[0]
			ExpectScheduled(ctx, env.Client, pod1)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			name1 := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop().LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName

			pod2 := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{
					Tolerations:          []v1.Toleration{t2, t3, t1},
					ResourceRequirements: rr,
				}),
			)[0]

			ExpectScheduled(ctx, env.Client, pod2)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			name2 := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop().LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName
			Expect(name1).To(Equal(name2))
		})
		It("should recover from an out-of-sync launch template cache", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)

			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			firstLt := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			ltName := aws.StringValue(firstLt.LaunchTemplateName)
			lt, ok := launchTemplateCache.Get(ltName)
			Expect(ok).To(Equal(true))
			// Remove expiration from cached LT
			launchTemplateCache.Set(ltName, lt, -1)

			fakeEC2API.CreateFleetBehavior.Error.Set(awserr.New("InvalidLaunchTemplateName.NotFoundException", "", errors.New("")))
			pod = ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			// should call fleet twice. Once will fail on invalid LT and the next will succeed
			fleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(aws.StringValue(fleetInput.LaunchTemplateConfigs[0].LaunchTemplateSpecification.LaunchTemplateName)).To(Equal(ltName))
			ExpectScheduled(ctx, env.Client, pod)
		})
	})
	Context("Labels", func() {
		It("should apply labels to the node", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKey(v1.LabelOSStable))
			Expect(node.Labels).To(HaveKey(v1.LabelArchStable))
			Expect(node.Labels).To(HaveKey(v1.LabelInstanceTypeStable))
		})
		It("should apply provider labels to the node", func() {
			fakeEC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
				{
					ImageId:      aws.String("ami-123"),
					Architecture: aws.String("x86_64"),
					CreationDate: aws.String("2022-08-15T12:00:00Z"),
				},
				{
					ImageId:      aws.String("ami-456"),
					Architecture: aws.String("arm64"),
					CreationDate: aws.String("2022-08-10T12:00:00Z"),
				},
			}})
			nodeTemplate.Spec.AMISelector = map[string]string{"karpenter.sh/discovery": "my-cluster"}
			ExpectApplied(ctx, env.Client, nodeTemplate)
			newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
			ExpectApplied(ctx, env.Client, newProvisioner)
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(newProvisioner), newProvisioner)).To(Succeed())
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			node := ExpectScheduled(ctx, env.Client, pod)
			Expect(node.Labels).To(HaveKeyWithValue(v1alpha1.LabelInstanceAMIID, "ami-123"))
		})
	})
	Context("Tags", func() {
		It("should tag with provisioner name", func() {
			provisionerName := "the-provisioner"
			provisioner.Name = provisionerName
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.TagSpecifications).To(HaveLen(3))

			tags := map[string]string{
				v1alpha5.ProvisionerNameLabelKey: provisionerName,
				"Name":                           fmt.Sprintf("%s/%s", v1alpha5.ProvisionerNameLabelKey, provisionerName),
			}
			// tags should be included in instance, volume, and fleet tag specification
			Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, tags)

			Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, tags)

			Expect(*createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, tags)
		})
		It("should request that tags be applied to both instances and volumes", func() {
			nodeTemplate.Spec.Tags = map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2value",
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.TagSpecifications).To(HaveLen(3))

			// tags should be included in instance, volume, and fleet tag specification
			Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, nodeTemplate.Spec.Tags)

			Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, nodeTemplate.Spec.Tags)

			Expect(*createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, nodeTemplate.Spec.Tags)
		})
		It("should override default tag names", func() {
			// these tags are defaulted, so ensure users can override them
			nodeTemplate.Spec.Tags = map[string]string{
				v1alpha5.ProvisionerNameLabelKey: "myprovisioner",
				"Name":                           "myname",
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.TagSpecifications).To(HaveLen(3))

			// tags should be included in instance, volume, and fleet tag specification
			Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, nodeTemplate.Spec.Tags)

			Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, nodeTemplate.Spec.Tags)

			Expect(*createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, nodeTemplate.Spec.Tags)
		})
		It("should merge global tags into launch template and volume tags", func() {
			nodeTemplate.Spec.Tags = map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2value",
			}
			settingsTags := map[string]string{
				"customTag1": "value1",
				"customTag2": "value2",
			}

			// Inject custom settings into the current context
			settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{
				Tags: settingsTags,
			})
			ctx = settingsStore.InjectSettings(ctx)

			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
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
			nodeTemplate.Spec.Tags = map[string]string{
				"tag1": "tag1value",
				"tag2": "tag2value",
			}
			settingsTags := map[string]string{
				"tag1": "custom1",
				"tag2": "custom2",
			}

			// Inject custom settings into the current context
			settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{
				Tags: settingsTags,
			})
			ctx = settingsStore.InjectSettings(ctx)

			provisioningController = provisioning.NewController(env.Client, prov, recorder)

			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CreateFleetBehavior.CalledWithInput.Len()).To(Equal(1))
			createFleetInput := fakeEC2API.CreateFleetBehavior.CalledWithInput.Pop()
			Expect(createFleetInput.TagSpecifications).To(HaveLen(3))

			// tags should be included in instance, volume, and fleet tag specification
			Expect(*createFleetInput.TagSpecifications[0].ResourceType).To(Equal(ec2.ResourceTypeInstance))
			ExpectTags(createFleetInput.TagSpecifications[0].Tags, nodeTemplate.Spec.Tags)
			ExpectTagsNotFound(createFleetInput.TagSpecifications[0].Tags, settingsTags)

			Expect(*createFleetInput.TagSpecifications[1].ResourceType).To(Equal(ec2.ResourceTypeVolume))
			ExpectTags(createFleetInput.TagSpecifications[1].Tags, nodeTemplate.Spec.Tags)
			ExpectTagsNotFound(createFleetInput.TagSpecifications[0].Tags, settingsTags)

			Expect(*createFleetInput.TagSpecifications[2].ResourceType).To(Equal(ec2.ResourceTypeFleet))
			ExpectTags(createFleetInput.TagSpecifications[2].Tags, nodeTemplate.Spec.Tags)
			ExpectTagsNotFound(createFleetInput.TagSpecifications[0].Tags, settingsTags)
		})
	})
	Context("Block Device Mappings", func() {
		It("should default AL2 block device mappings", func() {
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyAL2
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(len(input.LaunchTemplateData.BlockDeviceMappings)).To(Equal(1))
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(int64(20)))
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal("gp3"))
			Expect(input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops).To(BeNil())
		})
		It("should use custom block device mapping", func() {
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyAL2
			nodeTemplate.Spec.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1alpha1.BlockDevice{
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
					EBS: &v1alpha1.BlockDevice{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
						VolumeType:          aws.String("io2"),
						VolumeSize:          lo.ToPtr(resource.MustParse("200Gi")),
						IOPS:                aws.Int64(10_000),
						KMSKeyID:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
					},
				},
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(input.LaunchTemplateData.BlockDeviceMappings[0].Ebs).To(Equal(&ec2.LaunchTemplateEbsBlockDeviceRequest{
				VolumeSize:          aws.Int64(186),
				VolumeType:          aws.String("io2"),
				Iops:                aws.Int64(10_000),
				DeleteOnTermination: aws.Bool(true),
				Encrypted:           aws.Bool(true),
				KmsKeyId:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
			}))
			Expect(input.LaunchTemplateData.BlockDeviceMappings[1].Ebs).To(Equal(&ec2.LaunchTemplateEbsBlockDeviceRequest{
				VolumeSize:          aws.Int64(200),
				VolumeType:          aws.String("io2"),
				Iops:                aws.Int64(10_000),
				DeleteOnTermination: aws.Bool(true),
				Encrypted:           aws.Bool(true),
				KmsKeyId:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
			}))
		})
		It("should default bottlerocket second volume with root volume size", func() {
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(len(input.LaunchTemplateData.BlockDeviceMappings)).To(Equal(2))
			// Bottlerocket control volume
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(int64(4)))
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal("gp3"))
			Expect(input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops).To(BeNil())
			// Bottlerocket user volume
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeSize).To(Equal(int64(20)))
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[1].Ebs.VolumeType).To(Equal("gp3"))
			Expect(input.LaunchTemplateData.BlockDeviceMappings[1].Ebs.Iops).To(BeNil())
		})
		It("should not default block device mappings for custom AMIFamilies", func() {
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyCustom
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(len(input.LaunchTemplateData.BlockDeviceMappings)).To(Equal(0))
		})
		It("should use custom block device mapping for custom AMIFamilies", func() {
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyCustom
			nodeTemplate.Spec.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1alpha1.BlockDevice{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
						VolumeType:          aws.String("io2"),
						VolumeSize:          lo.ToPtr(resource.MustParse("40Gi")),
						IOPS:                aws.Int64(10_000),
						KMSKeyID:            aws.String("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
					},
				},
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(len(input.LaunchTemplateData.BlockDeviceMappings)).To(Equal(1))
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeSize).To(Equal(int64(40)))
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.VolumeType).To(Equal("io2"))
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Iops).To(Equal(int64(10_000)))
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.DeleteOnTermination).To(BeTrue())
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.Encrypted).To(BeTrue())
			Expect(*input.LaunchTemplateData.BlockDeviceMappings[0].Ebs.KmsKeyId).To(Equal("arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab"))
		})
	})
	Context("Ephemeral Storage", func() {
		It("should pack pods when a daemonset has an ephemeral-storage request", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate, coretest.DaemonSet(
				coretest.DaemonSetOptions{PodOptions: coretest.PodOptions{
					ResourceRequirements: v1.ResourceRequirements{
						Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1"),
							v1.ResourceMemory:           resource.MustParse("1Gi"),
							v1.ResourceEphemeralStorage: resource.MustParse("1Gi")}},
				}},
			))
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())
			ExpectScheduled(ctx, env.Client, pod[0])
		})
		It("should pack pods with any ephemeral-storage request", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("1G"),
					}}}))
			ExpectScheduled(ctx, env.Client, pod[0])
		})
		It("should pack pods with large ephemeral-storage request", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
					}}}))
			ExpectScheduled(ctx, env.Client, pod[0])
		})
		It("should not pack pods if the sum of pod ephemeral-storage and overhead exceeds node capacity", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("19Gi"),
					}}}))
			ExpectNotScheduled(ctx, env.Client, pod[0])
		})
		It("should launch multiple nodes if sum of pod ephemeral-storage requests exceeds a single nodes capacity", func() {
			var nodes []*v1.Node
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pods := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
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
			)
			for _, pod := range pods {
				nodes = append(nodes, ExpectScheduled(ctx, env.Client, pod))
			}
			Expect(nodes).To(HaveLen(2))
		})
		It("should only pack pods with ephemeral-storage requests that will fit on an available node", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pods := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
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
			)
			ExpectScheduled(ctx, env.Client, pods[0])
			ExpectNotScheduled(ctx, env.Client, pods[1])
		})
		It("should not pack pod if no available instance types have enough storage", func() {
			ExpectApplied(ctx, env.Client, provisioner)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("150Gi"),
					},
				},
				}))[0]
			ExpectNotScheduled(ctx, env.Client, pod)
		})
		It("should pack pods using the blockdevicemappings from the provider spec when defined", func() {
			nodeTemplate.Spec.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
				DeviceName: aws.String("/dev/xvda"),
				EBS: &v1alpha1.BlockDevice{
					VolumeSize: resource.NewScaledQuantity(50, resource.Giga),
				},
			}}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceEphemeralStorage: resource.MustParse("25Gi"),
					},
				},
				}))[0]

			// capacity isn't recorded on the node any longer, but we know the pod should schedule
			ExpectScheduled(ctx, env.Client, pod)
		})
		It("should pack pods using blockdevicemappings for Custom AMIFamily", func() {
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyCustom
			nodeTemplate.Spec.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1alpha1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(20, resource.Giga),
					},
				},
				{
					DeviceName: aws.String("/dev/xvdb"),
					EBS: &v1alpha1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(40, resource.Giga),
					},
				},
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov,
				coretest.UnschedulablePod(coretest.PodOptions{ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						// this pod can only be satisfied if `/dev/xvdb` will house all the pods.
						v1.ResourceEphemeralStorage: resource.MustParse("25Gi"),
					},
				},
				}))[0]

			// capacity isn't recorded on the node any longer, but we know the pod should schedule
			ExpectScheduled(ctx, env.Client, pod)
		})
	})
	Context("AL2", func() {
		It("should calculate memory overhead based on eni limited pods when ENI limited", func() {
			settingsStore = coretest.SettingsStore{
				settings.ContextKey: test.Settings(),
				awssettings.ContextKey: test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
					VMMemoryOverheadPercent:    lo.ToPtr[float64](0),
				}),
			}
			ctx = settingsStore.InjectSettings(ctx)

			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyAL2
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("1093Mi"))
		})
		It("should calculate memory overhead based on eni limited pods when not ENI limited", func() {
			settingsStore = coretest.SettingsStore{
				settings.ContextKey: test.Settings(),
				awssettings.ContextKey: test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
					VMMemoryOverheadPercent:    lo.ToPtr[float64](0),
				}),
			}
			ctx = settingsStore.InjectSettings(ctx)

			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyAL2
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("1093Mi"))
		})
	})
	Context("Bottlerocket", func() {
		It("should calculate memory overhead based on eni limited pods when ENI limited", func() {
			settingsStore = coretest.SettingsStore{
				settings.ContextKey: test.Settings(),
				awssettings.ContextKey: test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(true),
					VMMemoryOverheadPercent:    lo.ToPtr[float64](0),
				}),
			}
			ctx = settingsStore.InjectSettings(ctx)

			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("1093Mi"))
		})
		It("should calculate memory overhead based on max pods when not ENI limited", func() {
			settingsStore = coretest.SettingsStore{
				settings.ContextKey: test.Settings(),
				awssettings.ContextKey: test.Settings(test.SettingOptions{
					EnableENILimitedPodDensity: lo.ToPtr(false),
					VMMemoryOverheadPercent:    lo.ToPtr[float64](0),
				}),
			}
			ctx = settingsStore.InjectSettings(ctx)

			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
			instanceInfo, err := instanceTypeProvider.getInstanceTypes(ctx)
			Expect(err).To(BeNil())
			it := NewInstanceType(ctx, instanceInfo["m5.xlarge"], provisioner.Spec.KubeletConfiguration, "", nodeTemplate, nil)
			overhead := it.Overhead.Total()
			Expect(overhead.Memory().String()).To(Equal("1665Mi"))
		})
	})
	Context("User Data", func() {
		It("should not specify --use-max-pods=false when using ENI-based pod density", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).NotTo(ContainSubstring("--use-max-pods false"))
		})
		It("should specify --use-max-pods=false when not using ENI-based pod density", func() {
			settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{EnableENILimitedPodDensity: lo.ToPtr(false)})
			ctx = settingsStore.InjectSettings(ctx)

			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring("--use-max-pods false"))
			Expect(string(userData)).To(ContainSubstring("--max-pods=110"))
		})
		It("should specify --use-max-pods=false and --max-pods user value when user specifies maxPods in Provisioner", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{MaxPods: aws.Int32(10)}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring("--use-max-pods false"))
			Expect(string(userData)).To(ContainSubstring("--max-pods=10"))
		})
		It("should specify --system-reserved when overriding system reserved values", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				SystemReserved: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("500m"),
					v1.ResourceMemory:           resource.MustParse("1Gi"),
					v1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)

			// Check whether the arguments are there for --system-reserved
			arg := "--system-reserved="
			i := strings.Index(string(userData), arg)
			rem := string(userData)[(i + len(arg)):]
			i = strings.Index(rem, "'")
			for k, v := range provisioner.Spec.KubeletConfiguration.SystemReserved {
				Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v=%v", k.String(), v.String())))
			}
		})
		It("should specify --kube-reserved when overriding system reserved values", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				KubeReserved: v1.ResourceList{
					v1.ResourceCPU:              resource.MustParse("500m"),
					v1.ResourceMemory:           resource.MustParse("1Gi"),
					v1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
				},
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)

			// Check whether the arguments are there for --kube-reserved
			arg := "--kube-reserved="
			i := strings.Index(string(userData), arg)
			rem := string(userData)[(i + len(arg)):]
			i = strings.Index(rem, "'")
			for k, v := range provisioner.Spec.KubeletConfiguration.KubeReserved {
				Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v=%v", k.String(), v.String())))
			}
		})
		It("should pass eviction hard threshold values when specified", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				EvictionHard: map[string]string{
					"memory.available":  "10%",
					"nodefs.available":  "15%",
					"nodefs.inodesFree": "5%",
				},
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)

			// Check whether the arguments are there for --kube-reserved
			arg := "--eviction-hard="
			i := strings.Index(string(userData), arg)
			rem := string(userData)[(i + len(arg)):]
			i = strings.Index(rem, "'")
			for k, v := range provisioner.Spec.KubeletConfiguration.EvictionHard {
				Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v<%v", k, v)))
			}
		})
		It("should pass eviction soft threshold values when specified", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				EvictionSoft: map[string]string{
					"memory.available":  "10%",
					"nodefs.available":  "15%",
					"nodefs.inodesFree": "5%",
				},
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)

			// Check whether the arguments are there for --kube-reserved
			arg := "--eviction-soft="
			i := strings.Index(string(userData), arg)
			rem := string(userData)[(i + len(arg)):]
			i = strings.Index(rem, "'")
			for k, v := range provisioner.Spec.KubeletConfiguration.EvictionSoft {
				Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v<%v", k, v)))
			}
		})
		It("should pass eviction soft grace period values when specified", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				EvictionSoftGracePeriod: map[string]metav1.Duration{
					"memory.available":  {Duration: time.Minute},
					"nodefs.available":  {Duration: time.Second * 180},
					"nodefs.inodesFree": {Duration: time.Minute * 5},
				},
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)

			// Check whether the arguments are there for --kube-reserved
			arg := "--eviction-soft-grace-period="
			i := strings.Index(string(userData), arg)
			rem := string(userData)[(i + len(arg)):]
			i = strings.Index(rem, "'")
			for k, v := range provisioner.Spec.KubeletConfiguration.EvictionSoftGracePeriod {
				Expect(rem[:i]).To(ContainSubstring(fmt.Sprintf("%v=%v", k, v.Duration.String())))
			}
		})
		It("should pass eviction max pod grace period when specified", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				EvictionMaxPodGracePeriod: aws.Int32(300),
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)

			Expect(string(userData)).To(ContainSubstring(fmt.Sprintf("--eviction-max-pod-grace-period=%d", 300)))
		})
		It("should specify --pods-per-core", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				PodsPerCore: aws.Int32(2),
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring(fmt.Sprintf("--pods-per-core=%d", 2)))
		})
		It("should specify --pods-per-core with --max-pods enabled", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				PodsPerCore: aws.Int32(2),
				MaxPods:     aws.Int32(100),
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring(fmt.Sprintf("--pods-per-core=%d", 2)))
			Expect(string(userData)).To(ContainSubstring(fmt.Sprintf("--max-pods=%d", 100)))
		})
		It("should specify --container-runtime containerd by default", func() {
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring("--container-runtime containerd"))
		})
		It("should specify dockerd if specified in the provisionerSpec", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{ContainerRuntime: aws.String("dockerd")}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring("--container-runtime dockerd"))
		})
		It("should specify --container-runtime containerd when using Neuron GPUs", func() {
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{{Key: v1alpha1.LabelInstanceCategory, Operator: v1.NodeSelectorOpExists}}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceCPU:             resource.MustParse("1"),
						v1alpha1.ResourceAWSNeuron: resource.MustParse("1"),
					},
					Limits: map[v1.ResourceName]resource.Quantity{
						v1alpha1.ResourceAWSNeuron: resource.MustParse("1"),
					},
				},
			}))[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring("--container-runtime containerd"))
		})
		It("should specify --container-runtime containerd when using Nvidia GPUs", func() {
			provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{{Key: v1alpha1.LabelInstanceCategory, Operator: v1.NodeSelectorOpExists}}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod(coretest.PodOptions{
				ResourceRequirements: v1.ResourceRequirements{
					Requests: map[v1.ResourceName]resource.Quantity{
						v1.ResourceCPU:             resource.MustParse("1"),
						v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1"),
					},
					Limits: map[v1.ResourceName]resource.Quantity{
						v1alpha1.ResourceNVIDIAGPU: resource.MustParse("1"),
					},
				},
			}))[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring("--container-runtime containerd"))
		})
		It("should specify --dns-cluster-ip and --ip-family when running in an ipv6 cluster", func() {
			launchTemplateProvider.kubeDNSIP = net.ParseIP("fd4b:121b:812b::a")
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring("--dns-cluster-ip 'fd4b:121b:812b::a'"))
			Expect(string(userData)).To(ContainSubstring("--ip-family ipv6"))
			Expect(*input.LaunchTemplateData.MetadataOptions.HttpProtocolIpv6).To(Equal(ec2.LaunchTemplateInstanceMetadataProtocolIpv6Enabled))
		})
		It("should pass ImageGCHighThresholdPercent when specified", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				ImageGCHighThresholdPercent: aws.Int32(50),
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring("--image-gc-high-threshold=50"))
		})
		It("should pass ImageGCLowThresholdPercent when specified", func() {
			provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{
				ImageGCLowThresholdPercent: aws.Int32(50),
			}
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
			Expect(string(userData)).To(ContainSubstring("--image-gc-low-threshold=50"))
		})
		Context("Bottlerocket", func() {
			It("should merge in custom user data", func() {
				settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{EnableENILimitedPodDensity: lo.ToPtr(false)})
				ctx = settingsStore.InjectSettings(ctx)

				content, _ := os.ReadFile("testdata/br_userdata_input.golden")
				nodeTemplate.Spec.UserData = aws.String(string(content))
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				provisioner.Spec.Taints = []v1.Taint{{Key: "foo", Value: "bar", Effect: v1.TaintEffectNoExecute}}
				provisioner.Spec.StartupTaints = []v1.Taint{{Key: "baz", Value: "bin", Effect: v1.TaintEffectNoExecute}}
				ExpectApplied(ctx, env.Client, nodeTemplate, provisioner)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(provisioner), provisioner)).To(Succeed())
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod(coretest.PodOptions{
					Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
				}))[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				content, _ = os.ReadFile("testdata/br_userdata_merged.golden")
				// Newlines are always added for missing TOML fields, so strip them out before comparisons.
				actualUserData := strings.Replace(string(userData), "\n", "", -1)
				expectedUserData := strings.Replace(fmt.Sprintf(string(content), provisioner.Name), "\n", "", -1)
				Expect(expectedUserData).To(Equal(actualUserData))
			})
			It("should bootstrap when custom user data is empty", func() {
				settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{EnableENILimitedPodDensity: lo.ToPtr(false)})
				ctx = settingsStore.InjectSettings(ctx)
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				provisioner.Spec.Taints = []v1.Taint{{Key: "foo", Value: "bar", Effect: v1.TaintEffectNoExecute}}
				provisioner.Spec.StartupTaints = []v1.Taint{{Key: "baz", Value: "bin", Effect: v1.TaintEffectNoExecute}}
				ExpectApplied(ctx, env.Client, nodeTemplate, provisioner)
				Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(provisioner), provisioner)).To(Succeed())
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod(coretest.PodOptions{
					Tolerations: []v1.Toleration{{Operator: v1.TolerationOpExists}},
				}))[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				content, _ := os.ReadFile("testdata/br_userdata_unmerged.golden")
				actualUserData := strings.Replace(string(userData), "\n", "", -1)
				expectedUserData := strings.Replace(fmt.Sprintf(string(content), provisioner.Name), "\n", "", -1)
				Expect(expectedUserData).To(Equal(actualUserData))
			})
			It("should not bootstrap when provider ref points to a non-existent resource", func() {
				settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{EnableENILimitedPodDensity: lo.ToPtr(false)})
				ctx = settingsStore.InjectSettings(ctx)

				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: "doesnotexist"}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				// This will not be scheduled since we were pointed to a non-existent awsnodetemplate resource.
				ExpectNotScheduled(ctx, env.Client, pod)
			})
			It("should not bootstrap on invalid toml user data", func() {
				nodeTemplate.Spec.UserData = aws.String("#/bin/bash\n ./not-toml.sh")
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				// This will not be scheduled since userData cannot be generated for the prospective node.
				ExpectNotScheduled(ctx, env.Client, pod)
			})
			It("should override system reserved values in user data", func() {
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				ExpectApplied(ctx, env.Client, nodeTemplate)
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					ProviderRef: &v1alpha5.ProviderRef{
						Name: nodeTemplate.Name,
					},
					Kubelet: &v1alpha5.KubeletConfiguration{
						SystemReserved: v1.ResourceList{
							v1.ResourceCPU:              resource.MustParse("2"),
							v1.ResourceMemory:           resource.MustParse("3Gi"),
							v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
						},
					},
				})
				ExpectApplied(ctx, env.Client, provisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				config := &bootstrap.BottlerocketConfig{}
				Expect(config.UnmarshalTOML(userData)).To(Succeed())
				Expect(len(config.Settings.Kubernetes.SystemReserved)).To(Equal(3))
				Expect(config.Settings.Kubernetes.SystemReserved[v1.ResourceCPU.String()]).To(Equal("2"))
				Expect(config.Settings.Kubernetes.SystemReserved[v1.ResourceMemory.String()]).To(Equal("3Gi"))
				Expect(config.Settings.Kubernetes.SystemReserved[v1.ResourceEphemeralStorage.String()]).To(Equal("10Gi"))
			})
			It("should override kube reserved values in user data", func() {
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				ExpectApplied(ctx, env.Client, nodeTemplate)
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					ProviderRef: &v1alpha5.ProviderRef{
						Name: nodeTemplate.Name,
					},
					Kubelet: &v1alpha5.KubeletConfiguration{
						KubeReserved: v1.ResourceList{
							v1.ResourceCPU:              resource.MustParse("2"),
							v1.ResourceMemory:           resource.MustParse("3Gi"),
							v1.ResourceEphemeralStorage: resource.MustParse("10Gi"),
						},
					},
				})
				ExpectApplied(ctx, env.Client, provisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				config := &bootstrap.BottlerocketConfig{}
				Expect(config.UnmarshalTOML(userData)).To(Succeed())
				Expect(len(config.Settings.Kubernetes.KubeReserved)).To(Equal(3))
				Expect(config.Settings.Kubernetes.KubeReserved[v1.ResourceCPU.String()]).To(Equal("2"))
				Expect(config.Settings.Kubernetes.KubeReserved[v1.ResourceMemory.String()]).To(Equal("3Gi"))
				Expect(config.Settings.Kubernetes.KubeReserved[v1.ResourceEphemeralStorage.String()]).To(Equal("10Gi"))
			})
			It("should override kube reserved values in user data", func() {
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				ExpectApplied(ctx, env.Client, nodeTemplate)
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					ProviderRef: &v1alpha5.ProviderRef{
						Name: nodeTemplate.Name,
					},
					Kubelet: &v1alpha5.KubeletConfiguration{
						EvictionHard: map[string]string{
							"memory.available":  "10%",
							"nodefs.available":  "15%",
							"nodefs.inodesFree": "5%",
						},
					},
				})
				ExpectApplied(ctx, env.Client, provisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				config := &bootstrap.BottlerocketConfig{}
				Expect(config.UnmarshalTOML(userData)).To(Succeed())
				Expect(len(config.Settings.Kubernetes.EvictionHard)).To(Equal(3))
				Expect(config.Settings.Kubernetes.EvictionHard["memory.available"]).To(Equal("10%"))
				Expect(config.Settings.Kubernetes.EvictionHard["nodefs.available"]).To(Equal("15%"))
				Expect(config.Settings.Kubernetes.EvictionHard["nodefs.inodesFree"]).To(Equal("5%"))
			})
			It("should specify max pods value when passing maxPods in configuration", func() {
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyBottlerocket
				provisioner = test.Provisioner(coretest.ProvisionerOptions{
					ProviderRef: &v1alpha5.ProviderRef{
						Name: nodeTemplate.Name,
					},
					Kubelet: &v1alpha5.KubeletConfiguration{
						MaxPods: aws.Int32(10),
					},
				})
				ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				config := &bootstrap.BottlerocketConfig{}
				Expect(config.UnmarshalTOML(userData)).To(Succeed())
				Expect(config.Settings.Kubernetes.MaxPods).ToNot(BeNil())
				Expect(*config.Settings.Kubernetes.MaxPods).To(BeNumerically("==", 10))
			})
		})
		Context("AL2 Custom UserData", func() {
			It("should merge in custom user data", func() {
				settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{EnableENILimitedPodDensity: lo.ToPtr(false)})
				ctx = settingsStore.InjectSettings(ctx)

				content, _ := os.ReadFile("testdata/al2_userdata_input.golden")
				nodeTemplate.Spec.UserData = aws.String(string(content))
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				content, _ = os.ReadFile("testdata/al2_userdata_merged.golden")
				expectedUserData := fmt.Sprintf(string(content), newProvisioner.Name)
				Expect(expectedUserData).To(Equal(string(userData)))
			})
			It("should handle empty custom user data", func() {
				settingsStore[awssettings.ContextKey] = test.Settings(test.SettingOptions{EnableENILimitedPodDensity: lo.ToPtr(false)})
				ctx = settingsStore.InjectSettings(ctx)
				nodeTemplate.Spec.UserData = nil
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				content, _ := os.ReadFile("testdata/al2_userdata_unmerged.golden")
				expectedUserData := fmt.Sprintf(string(content), newProvisioner.Name)
				Expect(expectedUserData).To(Equal(string(userData)))
			})
			It("should not bootstrap invalid MIME UserData", func() {
				nodeTemplate.Spec.UserData = aws.String("#/bin/bash\n ./not-mime.sh")
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				// This will not be scheduled since userData cannot be generated for the prospective node.
				ExpectNotScheduled(ctx, env.Client, pod)
			})
		})
		Context("Custom AMI Selector", func() {
			It("should use ami selector specified in AWSNodeTemplate", func() {
				nodeTemplate.Spec.AMISelector = map[string]string{"karpenter.sh/discovery": "my-cluster"}
				fakeEC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z")},
				}})
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				Expect("ami-123").To(Equal(*input.LaunchTemplateData.ImageId))
			})
			It("should copy over userData untouched when AMIFamily is Custom", func() {
				nodeTemplate.Spec.UserData = aws.String("special user data")
				nodeTemplate.Spec.AMISelector = map[string]string{"karpenter.sh/discovery": "my-cluster"}
				nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyCustom
				fakeEC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2022-08-15T12:00:00Z")},
				}})
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				Expect("special user data").To(Equal(string(userData)))
			})
			It("should correctly use ami selector with specific IDs in AWSNodeTemplate", func() {
				nodeTemplate.Spec.AMISelector = map[string]string{"aws-ids": "ami-123,ami-456"}
				fakeEC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						Tags:         []*ec2.Tag{{Key: aws.String(v1.LabelInstanceTypeStable), Value: aws.String("m5.large")}},
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
					{
						ImageId:      aws.String("ami-456"),
						Architecture: aws.String("x86_64"),
						Tags:         []*ec2.Tag{{Key: aws.String(v1.LabelInstanceTypeStable), Value: aws.String("m5.xlarge")}},
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
				}})
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(2))
				actualFilter := fakeEC2API.CalledWithDescribeImagesInput.Pop().Filters
				expectedFilter := []*ec2.Filter{
					{
						Name:   aws.String("image-id"),
						Values: aws.StringSlice([]string{"ami-123", "ami-456"}),
					},
				}
				Expect(actualFilter).To(Equal(expectedFilter))
			})
			It("should create multiple launch templates when multiple amis are discovered with non-equivalent requirements", func() {
				fakeEC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						Tags:         []*ec2.Tag{{Key: aws.String(v1.LabelInstanceTypeStable), Value: aws.String("m5.large")}},
						CreationDate: aws.String("2022-08-15T12:00:00Z"),
					},
					{
						ImageId:      aws.String("ami-456"),
						Architecture: aws.String("x86_64"),
						Tags:         []*ec2.Tag{{Key: aws.String(v1.LabelInstanceTypeStable), Value: aws.String("m5.xlarge")}},
						CreationDate: aws.String("2022-08-10T12:00:00Z"),
					},
				}})
				nodeTemplate.Spec.AMISelector = map[string]string{"karpenter.sh/discovery": "my-cluster"}
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(2))
				expectedImageIds := sets.NewString("ami-123", "ami-456")
				actualImageIds := sets.NewString(
					*fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().LaunchTemplateData.ImageId,
					*fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop().LaunchTemplateData.ImageId,
				)
				Expect(expectedImageIds.Equal(actualImageIds)).To(BeTrue())
			})
			It("should create a launch template with the newest compatible AMI when multiple amis are discovered", func() {
				fakeEC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{
						ImageId:      aws.String("ami-123"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2020-01-01T12:00:00Z"),
					},
					{
						ImageId:      aws.String("ami-456"),
						Architecture: aws.String("x86_64"),
						CreationDate: aws.String("2021-01-01T12:00:00Z"),
					},
					{
						// Incompatible because required ARM64
						ImageId:      aws.String("ami-789"),
						Architecture: aws.String("arm64"),
						CreationDate: aws.String("2022-01-01T12:00:00Z"),
					},
				}})
				nodeTemplate.Spec.AMISelector = map[string]string{"karpenter.sh/discovery": "my-cluster"}
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{
					ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name},
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{v1alpha5.ArchitectureAmd64},
						},
					},
				})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				Expect("ami-456").To(Equal(*input.LaunchTemplateData.ImageId))
			})

			It("should fail if no amis match selector.", func() {
				fakeEC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{}})
				nodeTemplate.Spec.AMISelector = map[string]string{"karpenter.sh/discovery": "my-cluster"}
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectNotScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(0))
			})
			It("should fail if no instanceType matches ami requirements.", func() {
				fakeEC2API.DescribeImagesOutput.Set(&ec2.DescribeImagesOutput{Images: []*ec2.Image{
					{ImageId: aws.String("ami-123"), Architecture: aws.String("newnew"), CreationDate: aws.String("2022-01-01T12:00:00Z")},
				}})
				nodeTemplate.Spec.AMISelector = map[string]string{"karpenter.sh/discovery": "my-cluster"}
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectNotScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(0))
			})
			It("should choose amis from SSM if no selector specified in AWSNodeTemplate", func() {
				ExpectApplied(ctx, env.Client, nodeTemplate)
				newProvisioner := test.Provisioner(coretest.ProvisionerOptions{ProviderRef: &v1alpha5.ProviderRef{Name: nodeTemplate.Name}})
				ExpectApplied(ctx, env.Client, newProvisioner)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				Expect(*input.LaunchTemplateData.ImageId).To(ContainSubstring("test-ami"))
			})
		})
		Context("Kubelet Args", func() {
			It("should specify the --dns-cluster-ip flag when clusterDNSIP is set", func() {
				provisioner.Spec.KubeletConfiguration = &v1alpha5.KubeletConfiguration{ClusterDNS: []string{"10.0.10.100"}}
				ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				userData, _ := base64.StdEncoding.DecodeString(*input.LaunchTemplateData.UserData)
				Expect(string(userData)).To(ContainSubstring("--dns-cluster-ip '10.0.10.100'"))
			})
		})
		Context("Instance Profile", func() {
			It("should use the default instance profile if none specified on the Provisioner", func() {
				ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				Expect(*input.LaunchTemplateData.IamInstanceProfile.Name).To(Equal("test-instance-profile"))
			})
			It("should use the instance profile on the Provisioner when specified", func() {
				nodeTemplate.Spec.InstanceProfile = aws.String("overridden-profile")
				ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
				pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
				ExpectScheduled(ctx, env.Client, pod)
				Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
				input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
				Expect(*input.LaunchTemplateData.IamInstanceProfile.Name).To(Equal("overridden-profile"))
			})
		})
	})
	Context("Detailed Monitoring", func() {
		It("should default detailed monitoring to off", func() {
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyAL2
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(aws.BoolValue(input.LaunchTemplateData.Monitoring.Enabled)).To(BeFalse())
		})
		It("should pass detailed monitoring setting to the launch template at creation", func() {
			nodeTemplate.Spec.AMIFamily = &v1alpha1.AMIFamilyAL2
			nodeTemplate.Spec.DetailedMonitoring = aws.Bool(true)
			ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
			pod := ExpectProvisioned(ctx, env.Client, cluster, recorder, provisioningController, prov, coretest.UnschedulablePod())[0]
			ExpectScheduled(ctx, env.Client, pod)
			Expect(fakeEC2API.CalledWithCreateLaunchTemplateInput.Len()).To(Equal(1))
			input := fakeEC2API.CalledWithCreateLaunchTemplateInput.Pop()
			Expect(aws.BoolValue(input.LaunchTemplateData.Monitoring.Enabled)).To(BeTrue())
		})
	})
})

// ExpectTags verifies that the expected tags are a subset of the tags found
func ExpectTags(tags []*ec2.Tag, expected map[string]string) {
	existingTags := lo.SliceToMap(tags, func(t *ec2.Tag) (string, string) { return *t.Key, *t.Value })
	for expKey, expValue := range expected {
		foundValue, ok := existingTags[expKey]
		ExpectWithOffset(1, ok).To(BeTrue(), fmt.Sprintf("expected to find tag %s in %s", expKey, existingTags))
		ExpectWithOffset(1, foundValue).To(Equal(expValue))
	}
}

func ExpectTagsNotFound(tags []*ec2.Tag, expectNotFound map[string]string) {
	existingTags := lo.SliceToMap(tags, func(t *ec2.Tag) (string, string) { return *t.Key, *t.Value })
	for k, v := range expectNotFound {
		elem, ok := existingTags[k]
		ExpectWithOffset(1, !ok || v != elem).To(BeTrue())
	}
}

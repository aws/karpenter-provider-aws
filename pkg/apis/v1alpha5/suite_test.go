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

package v1alpha5_test

import (
	"context"
	"math"
	"testing"

	"github.com/Pallinder/go-randomdata"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"go.uber.org/multierr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/scheduling"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	apisv1alpha5 "github.com/aws/karpenter/pkg/apis/v1alpha5"
)

var ctx context.Context

func TestV1Alpha5(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "v1alpha5")
	ctx = TestContextWithLogger(t)
}

var _ = Describe("Provisioner", func() {
	var provisioner *v1alpha5.Provisioner

	BeforeEach(func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{Provider: &v1alpha1.AWS{
			SubnetSelector:        map[string]string{"*": "*"},
			SecurityGroupSelector: map[string]string{"*": "*"},
		}})
	})

	Context("SetDefaults", func() {
		It("should default OS to linux", func() {
			SetDefaults(ctx, provisioner)
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1.LabelOSStable)).
				To(Equal(scheduling.NewRequirement(v1.LabelOSStable, v1.NodeSelectorOpIn, string(v1.Linux))))
		})
		It("should not default OS if set", func() {
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements,
				v1.NodeSelectorRequirement{Key: v1.LabelOSStable, Operator: v1.NodeSelectorOpDoesNotExist})
			SetDefaults(ctx, provisioner)
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1.LabelOSStable)).
				To(Equal(scheduling.NewRequirement(v1.LabelOSStable, v1.NodeSelectorOpDoesNotExist)))
		})
		It("should default architecture to amd64", func() {
			SetDefaults(ctx, provisioner)
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1.LabelArchStable)).
				To(Equal(scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, v1alpha5.ArchitectureAmd64)))
		})
		It("should not default architecture if set", func() {
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements,
				v1.NodeSelectorRequirement{Key: v1.LabelArchStable, Operator: v1.NodeSelectorOpDoesNotExist})
			SetDefaults(ctx, provisioner)
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1.LabelArchStable)).
				To(Equal(scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpDoesNotExist)))
		})
		It("should default capacity-type to on-demand", func() {
			SetDefaults(ctx, provisioner)
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1alpha5.LabelCapacityType)).
				To(Equal(scheduling.NewRequirement(v1alpha5.LabelCapacityType, v1.NodeSelectorOpIn, v1alpha1.CapacityTypeOnDemand)))
		})
		It("should not default capacity-type if set", func() {
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements,
				v1.NodeSelectorRequirement{Key: v1alpha5.LabelCapacityType, Operator: v1.NodeSelectorOpDoesNotExist})
			SetDefaults(ctx, provisioner)
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1alpha5.LabelCapacityType)).
				To(Equal(scheduling.NewRequirement(v1alpha5.LabelCapacityType, v1.NodeSelectorOpDoesNotExist)))
		})
		It("should default instance-category, generation to c m r, gen>1", func() {
			SetDefaults(ctx, provisioner)
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1alpha1.LabelInstanceCategory)).
				To(Equal(scheduling.NewRequirement(v1alpha1.LabelInstanceCategory, v1.NodeSelectorOpIn, "c", "m", "r")))
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1alpha1.LabelInstanceGeneration)).
				To(Equal(scheduling.NewRequirement(v1alpha1.LabelInstanceGeneration, v1.NodeSelectorOpGt, "2")))
		})
		It("should not default instance-category, generation if set", func() {
			provisioner.Spec.Requirements = append(provisioner.Spec.Requirements,
				v1.NodeSelectorRequirement{Key: v1alpha1.LabelInstanceCategory, Operator: v1.NodeSelectorOpExists})
			SetDefaults(ctx, provisioner)
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1alpha1.LabelInstanceCategory)).
				To(Equal(scheduling.NewRequirement(v1alpha1.LabelInstanceCategory, v1.NodeSelectorOpExists)))
			Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1alpha1.LabelInstanceGeneration)).
				To(Equal(scheduling.NewRequirement(v1alpha1.LabelInstanceGeneration, v1.NodeSelectorOpExists)))
		})
		It("should not default instance-category if any instance label is set", func() {
			for _, label := range []string{v1.LabelInstanceTypeStable, v1alpha1.LabelInstanceFamily} {
				provisioner.Spec.Requirements = []v1.NodeSelectorRequirement{{Key: label, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}}}
				SetDefaults(ctx, provisioner)
				Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(label)).
					To(Equal(scheduling.NewRequirement(label, v1.NodeSelectorOpIn, "test")))
				Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1alpha1.LabelInstanceCategory)).
					To(Equal(scheduling.NewRequirement(v1alpha1.LabelInstanceCategory, v1.NodeSelectorOpExists)))
				Expect(scheduling.NewNodeSelectorRequirements(provisioner.Spec.Requirements...).Get(v1alpha1.LabelInstanceGeneration)).
					To(Equal(scheduling.NewRequirement(v1alpha1.LabelInstanceGeneration, v1.NodeSelectorOpExists)))
			}
		})
	})

	Context("Validate", func() {

		It("should validate", func() {
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		It("should succeed if provider undefined", func() {
			provisioner.Spec.Provider = nil
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})

		Context("SubnetSelector", func() {
			It("should not allow empty string keys or values", func() {
				provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
				Expect(err).ToNot(HaveOccurred())
				for key, value := range map[string]string{
					"":    "value",
					"key": "",
				} {
					provider.SubnetSelector = map[string]string{key: value}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				}
			})
		})
		Context("SecurityGroupSelector", func() {
			It("should not allow with a custom launch template", func() {
				provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
				Expect(err).ToNot(HaveOccurred())
				provider.LaunchTemplateName = aws.String("my-lt")
				provider.SecurityGroupSelector = map[string]string{"key": "value"}
				Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
			})
			It("should not allow empty string keys or values", func() {
				provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
				Expect(err).ToNot(HaveOccurred())
				for key, value := range map[string]string{
					"":    "value",
					"key": "",
				} {
					provider.SecurityGroupSelector = map[string]string{key: value}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				}
			})
		})

		Context("Labels", func() {
			It("should not allow unrecognized labels with the aws label prefix", func() {
				provisioner.Spec.Labels = map[string]string{v1alpha1.LabelDomain + "/" + randomdata.SillyName(): randomdata.SillyName()}
				Expect(Validate(ctx, provisioner)).ToNot(Succeed())
			})
			It("should support well known labels", func() {
				for _, label := range []string{
					v1alpha1.LabelInstanceHypervisor,
					v1alpha1.LabelInstanceFamily,
					v1alpha1.LabelInstanceSize,
					v1alpha1.LabelInstanceCPU,
					v1alpha1.LabelInstanceMemory,
					v1alpha1.LabelInstanceGPUName,
					v1alpha1.LabelInstanceGPUManufacturer,
					v1alpha1.LabelInstanceGPUCount,
					v1alpha1.LabelInstanceGPUMemory,
				} {
					provisioner.Spec.Labels = map[string]string{label: randomdata.SillyName()}
					Expect(provisioner.Validate(ctx)).To(Succeed())
				}
			})
		})
		Context("MetadataOptions", func() {
			It("should not allow with a custom launch template", func() {
				provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
				Expect(err).ToNot(HaveOccurred())
				provider.LaunchTemplateName = aws.String("my-lt")
				provider.MetadataOptions = &v1alpha1.MetadataOptions{}
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
			})
			It("should allow missing values", func() {
				provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{}
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			Context("HTTPEndpoint", func() {
				It("should allow enum values", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					for i := range ec2.LaunchTemplateInstanceMetadataEndpointState_Values() {
						value := ec2.LaunchTemplateInstanceMetadataEndpointState_Values()[i]
						provider.MetadataOptions = &v1alpha1.MetadataOptions{
							HTTPEndpoint: &value,
						}
						provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
						Expect(provisioner.Validate(ctx)).To(Succeed())
					}
				})
				It("should not allow non-enum values", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPEndpoint: aws.String(randomdata.SillyName()),
					}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
			})
			Context("HTTPProtocolIpv6", func() {
				It("should allow enum values", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					for i := range ec2.LaunchTemplateInstanceMetadataProtocolIpv6_Values() {
						value := ec2.LaunchTemplateInstanceMetadataProtocolIpv6_Values()[i]
						provider.MetadataOptions = &v1alpha1.MetadataOptions{
							HTTPProtocolIPv6: &value,
						}
						provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
						Expect(provisioner.Validate(ctx)).To(Succeed())
					}
				})
				It("should not allow non-enum values", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPProtocolIPv6: aws.String(randomdata.SillyName()),
					}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
			})
			Context("HTTPPutResponseHopLimit", func() {
				It("should validate inside accepted range", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPPutResponseHopLimit: aws.Int64(int64(randomdata.Number(1, 65))),
					}
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).To(Succeed())
				})
				It("should not validate outside accepted range", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{}
					// We expect to be able to invalidate any hop limit between
					// [math.MinInt64, 1). But, to avoid a panic here, we can't
					// exceed math.MaxInt for the difference between bounds of
					// the random number range. So we divide the range
					// approximately in half and test on both halves.
					provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(math.MinInt64, math.MinInt64/2)))
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
					provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(math.MinInt64/2, 1)))
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())

					provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(65, math.MaxInt64)))
					provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
			})
			Context("HTTPTokens", func() {
				It("should allow enum values", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					for _, value := range ec2.LaunchTemplateHttpTokensState_Values() {
						provider.MetadataOptions = &v1alpha1.MetadataOptions{
							HTTPTokens: aws.String(value),
						}
						provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
						Expect(provisioner.Validate(ctx)).To(Succeed())
					}
				})
				It("should not allow non-enum values", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPTokens: aws.String(randomdata.SillyName()),
					}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
			})
			Context("BlockDeviceMappings", func() {
				It("should not allow with a custom launch template", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.LaunchTemplateName = aws.String("my-lt")
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(1, resource.Giga),
						},
					}}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
				It("should validate minimal device mapping", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(1, resource.Giga),
						},
					}}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
				It("should validate ebs device mapping with snapshotID only", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							SnapshotID: aws.String("snap-0123456789"),
						},
					}}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
				It("should not allow volume size below minimum", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(100, resource.Mega),
						},
					}}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
				It("should not allow volume size above max", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(65, resource.Tera),
						},
					}}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
				It("should not allow nil device name", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						EBS: &v1alpha1.BlockDevice{
							VolumeSize: resource.NewScaledQuantity(65, resource.Tera),
						},
					}}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
				It("should not allow nil volume size", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
						EBS:        &v1alpha1.BlockDevice{},
					}}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
				It("should not allow empty ebs block", func() {
					provider, err := v1alpha1.DeserializeProvider(provisioner.Spec.Provider.Raw)
					Expect(err).ToNot(HaveOccurred())
					provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
						DeviceName: aws.String("/dev/xvda"),
					}}
					Expect(Validate(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))).ToNot(Succeed())
				})
			})
		})
	})
})

func SetDefaults(ctx context.Context, provisioner *v1alpha5.Provisioner) {
	prov := apisv1alpha5.Provisioner(*provisioner)
	prov.SetDefaults(ctx)
	*provisioner = v1alpha5.Provisioner(prov)
}

func Validate(ctx context.Context, provisioner *v1alpha5.Provisioner) error {
	return multierr.Combine(
		lo.ToPtr(apisv1alpha5.Provisioner(*provisioner)).Validate(ctx),
		provisioner.Validate(ctx),
	)
}

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
	"k8s.io/apimachinery/pkg/api/resource"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	apisv1alpha5 "github.com/aws/karpenter/pkg/apis/v1alpha5"
)

var provisioner *v1alpha5.Provisioner
var ctx context.Context

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "v1alpha5")
	ctx = TestContextWithLogger(t)
}

var _ = Describe("Validate", func() {
	BeforeEach(func() {
		provisioner = test.Provisioner(test.ProvisionerOptions{Provider: &v1alpha1.AWS{
			AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
			SubnetSelector:        map[string]string{"*": "*"},
			SecurityGroupSelector: map[string]string{"*": "*"},
		}})
	})
	It("should validate", func() {
		Expect(provisioner.Validate(ctx)).To(Succeed())
	})
	It("should succeed if provider undefined", func() {
		provisioner.Spec.Provider = nil
		Expect(provisioner.Validate(ctx)).To(Succeed())
	})

	Context("SubnetSelector", func() {
		It("should not allow empty string keys or values", func() {
			provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			for key, value := range map[string]string{
				"":    "value",
				"key": "",
			} {
				provider.SubnetSelector = map[string]string{key: value}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			}
		})
	})
	Context("SecurityGroupSelector", func() {
		It("should not allow with a custom launch template", func() {
			provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			provider.LaunchTemplateName = aws.String("my-lt")
			provider.SecurityGroupSelector = map[string]string{"key": "value"}
			ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
		})
		It("should not allow empty string keys or values", func() {
			provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			for key, value := range map[string]string{
				"":    "value",
				"key": "",
			} {
				provider.SecurityGroupSelector = map[string]string{key: value}
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			}
		})
	})

	Context("Labels", func() {
		It("should not allow unrecognized labels with the aws label prefix", func() {
			provisioner.Spec.Labels = map[string]string{v1alpha1.LabelDomain + "/" + randomdata.SillyName(): randomdata.SillyName()}
			ExpectNotValid(ctx, provisioner)
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
			provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			provider.LaunchTemplateName = aws.String("my-lt")
			provider.MetadataOptions = &v1alpha1.MetadataOptions{}
			provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
			ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
		})
		It("should allow missing values", func() {
			provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			provider.MetadataOptions = &v1alpha1.MetadataOptions{}
			provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		Context("HTTPEndpoint", func() {
			It("should allow enum values", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
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
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{
					HTTPEndpoint: aws.String(randomdata.SillyName()),
				}
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
		})
		Context("HTTPProtocolIpv6", func() {
			It("should allow enum values", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
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
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{
					HTTPProtocolIPv6: aws.String(randomdata.SillyName()),
				}
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
		})
		Context("HTTPPutResponseHopLimit", func() {
			It("should validate inside accepted range", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{
					HTTPPutResponseHopLimit: aws.Int64(int64(randomdata.Number(1, 65))),
				}
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).To(Succeed())
			})
			It("should not validate outside accepted range", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{}
				// We expect to be able to invalidate any hop limit between
				// [math.MinInt64, 1). But, to avoid a panic here, we can't
				// exceed math.MaxInt for the difference between bounds of
				// the random number range. So we divide the range
				// approximately in half and test on both halves.
				provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(math.MinInt64, math.MinInt64/2)))
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
				provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(math.MinInt64/2, 1)))
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))

				provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(65, math.MaxInt64)))
				provisioner = test.Provisioner(test.ProvisionerOptions{Provider: provider})
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
		})
		Context("HTTPTokens", func() {
			It("should allow enum values", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
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
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{
					HTTPTokens: aws.String(randomdata.SillyName()),
				}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
		})
		Context("BlockDeviceMappings", func() {
			It("should not allow with a custom launch template", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.LaunchTemplateName = aws.String("my-lt")
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1alpha1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(1, resource.Giga),
					},
				}}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
			It("should validate minimal device mapping", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1alpha1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(1, resource.Giga),
					},
				}}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
			It("should validate ebs device mapping with snapshotID only", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1alpha1.BlockDevice{
						SnapshotID: aws.String("snap-0123456789"),
					},
				}}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
			It("should not allow volume size below minimum", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1alpha1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(100, resource.Mega),
					},
				}}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
			It("should not allow volume size above max", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
					EBS: &v1alpha1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(65, resource.Tera),
					},
				}}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
			It("should not allow nil device name", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					EBS: &v1alpha1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(65, resource.Tera),
					},
				}}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
			It("should not allow nil volume size", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
					EBS:        &v1alpha1.BlockDevice{},
				}}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
			It("should not allow empty ebs block", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
				}}
				ExpectNotValid(ctx, test.Provisioner(test.ProvisionerOptions{Provider: provider}))
			})
		})
	})
})

func ExpectNotValid(ctx context.Context, provisioner *v1alpha5.Provisioner) {
	Expect(multierr.Combine(
		lo.ToPtr(apisv1alpha5.Provisioner(*provisioner)).Validate(ctx),
		provisioner.Validate(ctx),
	)).ToNot(Succeed())
}

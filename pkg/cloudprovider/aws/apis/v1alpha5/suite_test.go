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
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha5"

	provisioningv1alpha5 "github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/cloudprovider/aws/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	. "knative.dev/pkg/logging/testing"
)

var ctx context.Context

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "cloudprovider/aws/apis/v1alpha1")
}

var _ = Describe("Validation", func() {
	var provisioner v1alpha5.Provisioner
	BeforeEach(func() {
		provisioner = Provisioner(test.ProvisionerOptions{})
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
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
	})
	Context("SecurityGroupSelector", func() {
		It("should not allow with a custom launch template", func() {
			provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			provider.LaunchTemplateName = aws.String("my-lt")
			provider.SecurityGroupSelector = map[string]string{"key": "value"}
			provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should not allow empty string keys or values", func() {
			provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			for key, value := range map[string]string{
				"":    "value",
				"key": "",
			} {
				provider.SecurityGroupSelector = map[string]string{key: value}
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			}
		})
	})
	Context("Labels", func() {
		It("should not allow unrecognized labels with the aws label prefix", func() {
			Expect(provisioningv1alpha5.RestrictedLabelDomains.List()).To(ContainElement(v1alpha1.LabelDomain))
		})
		It("should support well known labels", func() {
			Expect(provisioningv1alpha5.WellKnownLabels.List()).To(ContainElements(
				v1alpha1.LabelInstanceHypervisor,
				v1alpha1.LabelInstanceFamily,
				v1alpha1.LabelInstanceSize,
				v1alpha1.LabelInstanceCPU,
				v1alpha1.LabelInstanceMemory,
				v1alpha1.LabelInstanceGPUName,
				v1alpha1.LabelInstanceGPUManufacturer,
				v1alpha1.LabelInstanceGPUCount,
				v1alpha1.LabelInstanceGPUMemory,
			))
		})
	})
	Context("MetadataOptions", func() {
		It("should not allow with a custom launch template", func() {
			provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			provider.LaunchTemplateName = aws.String("my-lt")
			provider.MetadataOptions = &v1alpha1.MetadataOptions{}
			provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
			Expect(provisioner.Validate(ctx)).ToNot(Succeed())
		})
		It("should allow missing values", func() {
			provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
			Expect(err).ToNot(HaveOccurred())
			provider.MetadataOptions = &v1alpha1.MetadataOptions{}
			provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
			Expect(provisioner.Validate(ctx)).To(Succeed())
		})
		Context("HTTPEndpoint", func() {
			It("should allow enum values", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				for _, value := range ec2.LaunchTemplateInstanceMetadataEndpointState_Values() {
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPEndpoint: &value,
					}
					provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).To(Succeed())
				}
			})
			It("should not allow non-enum values", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{
					HTTPEndpoint: aws.String(randomdata.SillyName()),
				}
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
		})
		Context("HTTPProtocolIpv6", func() {
			It("should allow enum values", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				for _, value := range ec2.LaunchTemplateInstanceMetadataProtocolIpv6_Values() {
					provider.MetadataOptions = &v1alpha1.MetadataOptions{
						HTTPProtocolIPv6: &value,
					}
					provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).To(Succeed())
				}
			})
			It("should not allow non-enum values", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{
					HTTPProtocolIPv6: aws.String(randomdata.SillyName()),
				}
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
		})
		Context("HTTPPutResponseHopLimit", func() {
			It("should validate inside accepted range", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{
					HTTPPutResponseHopLimit: aws.Int64(int64(randomdata.Number(1, 65))),
				}
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
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
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
				provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(math.MinInt64/2, 1)))
				provisioner = Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())

				provider.MetadataOptions.HTTPPutResponseHopLimit = aws.Int64(int64(randomdata.Number(65, math.MaxInt64)))
				provisioner = Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
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
					provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
					Expect(provisioner.Validate(ctx)).To(Succeed())
				}
			})
			It("should not allow non-enum values", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.MetadataOptions = &v1alpha1.MetadataOptions{
					HTTPTokens: aws.String(randomdata.SillyName()),
				}
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
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
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
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
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).To(Succeed())
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
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).To(Succeed())
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
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
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
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should not allow nil device name", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					EBS: &v1alpha1.BlockDevice{
						VolumeSize: resource.NewScaledQuantity(65, resource.Tera),
					},
				}}
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should not allow nil volume size", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
					EBS:        &v1alpha1.BlockDevice{},
				}}
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
			It("should not allow empty ebs block", func() {
				provider, err := v1alpha1.Deserialize(provisioner.Spec.Provider)
				Expect(err).ToNot(HaveOccurred())
				provider.BlockDeviceMappings = []*v1alpha1.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
				}}
				provisioner := Provisioner(test.ProvisionerOptions{Provider: provider})
				Expect(provisioner.Validate(ctx)).ToNot(Succeed())
			})
		})
	})
})

func Provisioner(options test.ProvisionerOptions) v1alpha5.Provisioner {
	if options.Provider == nil {
		options.Provider = &v1alpha1.AWS{}
	}
	provider := options.Provider.(*v1alpha1.AWS)
	if provider.AMIFamily == nil {
		provider.AMIFamily = aws.String(v1alpha1.AMIFamilyAL2)
	}
	if provider.SubnetSelector == nil {
		provider.SubnetSelector = map[string]string{"foo": "bar"}
	}
	if provider.SecurityGroupSelector == nil {
		provider.SecurityGroupSelector = map[string]string{"foo": "bar"}
	}
	from := test.Provisioner(options)
	to := v1alpha5.Provisioner(*from)
	// Apply provider specific defaults
	to.SetDefaults(context.Background())
	return to
}

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

package v1beta1_test

import (
	"github.com/imdario/mergo"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/apis"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Webhook/Validation", func() {
	var nc *v1beta1.EC2NodeClass

	BeforeEach(func() {
		nc = test.EC2NodeClass()
	})
	It("should succeed if just specifying role", func() {
		Expect(nc.Validate(ctx)).To(Succeed())
	})
	It("should succeed if just specifying instance profile", func() {
		nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		nc.Spec.Role = ""
		Expect(nc.Validate(ctx)).To(Succeed())
	})
	It("should fail if specifying both instance profile and role", func() {
		nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		Expect(nc.Validate(ctx)).ToNot(Succeed())
	})
	It("should fail if not specifying one of instance profile and role", func() {
		nc.Spec.Role = ""
		Expect(nc.Validate(ctx)).ToNot(Succeed())
	})
	Context("UserData", func() {
		It("should succeed if user data is empty", func() {
			Expect(nc.Validate(ctx)).To(Succeed())
		})
	})
	Context("Tags", func() {
		It("should succeed when tags are empty", func() {
			nc.Spec.Tags = map[string]string{}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should succeed if tags aren't in restricted tag keys", func() {
			nc.Spec.Tags = map[string]string{
				"karpenter.sh/custom-key": "value",
				"karpenter.sh/managed":    "true",
				"kubernetes.io/role/key":  "value",
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should succeed by validating that regex is properly escaped", func() {
			nc.Spec.Tags = map[string]string{
				"karpenterzsh/nodepool": "value",
			}
			Expect(nc.Validate(ctx)).To(Succeed())
			nc.Spec.Tags = map[string]string{
				"kubernetesbio/cluster/test": "value",
			}
			Expect(nc.Validate(ctx)).To(Succeed())
			nc.Spec.Tags = map[string]string{
				"karpenterzsh/managed-by": "test",
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should fail if tags contain a restricted domain key", func() {
			nc.Spec.Tags = map[string]string{
				"karpenter.sh/nodepool": "value",
			}
			Expect(nc.Validate(ctx)).To(Not(Succeed()))
			nc.Spec.Tags = map[string]string{
				"kubernetes.io/cluster/test": "value",
			}
			Expect(nc.Validate(ctx)).To(Not(Succeed()))
			nc.Spec.Tags = map[string]string{
				"karpenter.sh/managed-by": "test",
			}
			Expect(nc.Validate(ctx)).To(Not(Succeed()))
			nc.Spec.Tags = map[string]string{
				v1beta1.LabelNodeClass: "test",
			}
			Expect(nc.Validate(ctx)).To(Not(Succeed()))
			nc.Spec.Tags = map[string]string{
				"karpenter.sh/nodeclaim": "test",
			}
			Expect(nc.Validate(ctx)).To(Not(Succeed()))
		})
	})
	Context("SubnetSelectorTerms", func() {
		It("should succeed with a valid subnet selector on tags", func() {
			nc.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should succeed with a valid subnet selector on id", func() {
			nc.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					ID: "subnet-12345749",
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should fail when subnet selector terms is set to nil", func() {
			nc.Spec.SubnetSelectorTerms = nil
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when no subnet selector terms exist", func() {
			nc.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a subnet selector term has no values", func() {
			nc.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a subnet selector term has no tag map values", func() {
			nc.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a subnet selector term has a tag map key that is empty", func() {
			nc.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"test": "",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a subnet selector term has a tag map value that is empty", func() {
			nc.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when the last subnet selector is invalid", func() {
			nc.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
				{
					Tags: map[string]string{
						"test2": "testvalue2",
					},
				},
				{
					Tags: map[string]string{
						"test3": "testvalue3",
					},
				},
				{
					Tags: map[string]string{
						"": "testvalue4",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when specifying id with tags", func() {
			nc.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					ID: "subnet-12345749",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
	})
	Context("SecurityGroupSelectorTerms", func() {
		It("should succeed with a valid security group selector on tags", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should succeed with a valid security group selector on id", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					ID: "sg-12345749",
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should succeed with a valid security group selector on name", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Name: "testname",
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should fail when security group selector terms is set to nil", func() {
			nc.Spec.SecurityGroupSelectorTerms = nil
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when no security group selector terms exist", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a security group selector term has no values", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a security group selector term has no tag map values", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a security group selector term has a tag map key that is empty", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"test": "",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a security group selector term has a tag map value that is empty", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when the last security group selector is invalid", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
				{
					Tags: map[string]string{
						"test2": "testvalue2",
					},
				},
				{
					Tags: map[string]string{
						"test3": "testvalue3",
					},
				},
				{
					Tags: map[string]string{
						"": "testvalue4",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when specifying id with tags", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					ID: "sg-12345749",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when specifying id with name", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					ID:   "sg-12345749",
					Name: "my-security-group",
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when specifying name with tags", func() {
			nc.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Name: "my-security-group",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
	})
	Context("AMISelectorTerms", func() {
		It("should fail when amiSelectorTerms is nil", func() {
			nc.Spec.AMISelectorTerms = nil
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when len(amiSelectorTerms) is zero", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		DescribeTable(
			"should succeed with a valid eksOptimized familiy",
			func(family string) {
				nc.Spec.AMIFamily = &family
				nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{
					EKSOptimized: &v1beta1.EKSOptimized{Family: family},
				}}
				Expect(nc.Validate(ctx)).To(Succeed())
			},
			Entry("AL2", v1beta1.AMIFamilyAL2),
			Entry("AL2023", v1beta1.AMIFamilyAL2023),
			Entry("Bottlerocket", v1beta1.AMIFamilyBottlerocket),
			Entry("Windows2019", v1beta1.AMIFamilyWindows2019),
			Entry("Windows2022", v1beta1.AMIFamilyWindows2022),
			Entry("Ubuntu", v1beta1.AMIFamilyUbuntu),
		)
		It("should fail with an empty eksOptimized AMI Family", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{
				EKSOptimized: &v1beta1.EKSOptimized{},
			}}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail with an invalid eksOptimized AMI Family", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{
				EKSOptimized: &v1beta1.EKSOptimized{Family: "foo"},
			}}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when eksOptimized family is specified with other fields", func() {
			for _, modifier := range []*v1beta1.AMISelectorTerm{
				{Tags: map[string]string{"*": "*"}},
				{ID: "foo"},
				{Name: "foo"},
				{Owner: "foo"},
			} {
				nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{
					EKSOptimized: &v1beta1.EKSOptimized{Family: v1beta1.AMIFamilyAL2},
				}}
				Expect(mergo.Merge(&nc.Spec.AMISelectorTerms[0], modifier)).To(Succeed())
				Expect(nc.Validate(ctx)).ToNot(Succeed())
			}
		})
		It("should fail when EKSOptimized is specified with other terms", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{EKSOptimized: &v1beta1.EKSOptimized{Family: v1beta1.AMIFamilyAL2}},
				{ID: "foo"},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when the eksOptimized family doesn't match amiFamily", func() {
			families := []string{
				v1beta1.AMIFamilyAL2,
				v1beta1.AMIFamilyAL2023,
				v1beta1.AMIFamilyUbuntu,
				v1beta1.AMIFamilyBottlerocket,
				v1beta1.AMIFamilyWindows2019,
				v1beta1.AMIFamilyWindows2022,
			}
			for i := range families {
				for j := range families {
					if i == j {
						continue
					}
					nc = test.EC2NodeClass(v1beta1.EC2NodeClass{
						Spec: v1beta1.EC2NodeClassSpec{
							AMIFamily: &families[i],
							AMISelectorTerms: []v1beta1.AMISelectorTerm{{
								EKSOptimized: &v1beta1.EKSOptimized{Family: families[j]},
							}},
						},
					})
					Expect(nc.Validate(ctx)).ToNot(Succeed())
				}
			}
		})
		It("should succed when the eksOptimized family matches amiFamily", func() {
			for _, family := range []string{
				v1beta1.AMIFamilyAL2,
				v1beta1.AMIFamilyAL2023,
				v1beta1.AMIFamilyUbuntu,
				v1beta1.AMIFamilyBottlerocket,
				v1beta1.AMIFamilyWindows2019,
				v1beta1.AMIFamilyWindows2022,
			} {
				nc = test.EC2NodeClass(v1beta1.EC2NodeClass{
					Spec: v1beta1.EC2NodeClassSpec{
						AMIFamily: lo.ToPtr(family),
						AMISelectorTerms: []v1beta1.AMISelectorTerm{{
							EKSOptimized: &v1beta1.EKSOptimized{Family: family},
						}},
					},
				})
				Expect(nc.Validate(ctx)).To(Succeed())
			}
		})
		It("should succed when the amiFamily is 'Custom' for any optimized ami", func() {
			for _, family := range []string{
				v1beta1.AMIFamilyAL2,
				v1beta1.AMIFamilyAL2023,
				v1beta1.AMIFamilyUbuntu,
				v1beta1.AMIFamilyBottlerocket,
				v1beta1.AMIFamilyWindows2019,
				v1beta1.AMIFamilyWindows2022,
			} {
				nc = test.EC2NodeClass(v1beta1.EC2NodeClass{
					Spec: v1beta1.EC2NodeClassSpec{
						AMIFamily: lo.ToPtr(v1beta1.AMIFamilyCustom),
						AMISelectorTerms: []v1beta1.AMISelectorTerm{{
							EKSOptimized: &v1beta1.EKSOptimized{Family: family},
						}},
					},
				})
				Expect(nc.Validate(ctx)).To(Succeed())
			}
		})
		It("should succeed with a valid ami selector on tags", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should succeed with a valid ami selector on id", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					ID: "sg-12345749",
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should succeed with a valid ami selector on name", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Name: "testname",
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should succeed with a valid ami selector on name and owner", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Name:  "testname",
					Owner: "testowner",
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should succeed when an ami selector term has an owner key with tags", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Owner: "testowner",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).To(Succeed())
		})
		It("should fail when a ami selector term has no values", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a ami selector term has no tag map values", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a ami selector term has a tag map key that is empty", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"test": "",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when a ami selector term has a tag map value that is empty", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when the last ami selector is invalid", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
				{
					Tags: map[string]string{
						"test2": "testvalue2",
					},
				},
				{
					Tags: map[string]string{
						"test3": "testvalue3",
					},
				},
				{
					Tags: map[string]string{
						"": "testvalue4",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when specifying id with tags", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					ID: "ami-12345749",
					Tags: map[string]string{
						"test": "testvalue",
					},
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when specifying id with name", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					ID:   "ami-12345749",
					Name: "my-custom-ami",
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
		It("should fail when specifying id with owner", func() {
			nc.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					ID:    "ami-12345749",
					Owner: "123456789",
				},
			}
			Expect(nc.Validate(ctx)).ToNot(Succeed())
		})
	})
	Context("BlockDeviceMappings", func() {
		It("should fail if more than one root volume is specified", func() {
			nodeClass := test.EC2NodeClass(v1beta1.EC2NodeClass{
				Spec: v1beta1.EC2NodeClassSpec{
					BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
							EBS: &v1beta1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(50, resource.Giga),
							},

							RootVolume: true,
						},
						{
							DeviceName: aws.String("map-device-2"),
							EBS: &v1beta1.BlockDevice{
								VolumeSize: resource.NewScaledQuantity(50, resource.Giga),
							},

							RootVolume: true,
						},
					},
				},
			})
			Expect(nodeClass.Validate(ctx)).To(Not(Succeed()))
		})
	})
	Context("Role Immutability", func() {
		It("should fail when updating the role", func() {
			nc.Spec.Role = "test-role"
			Expect(nc.Validate(ctx)).To(Succeed())

			updateCtx := apis.WithinUpdate(ctx, nc.DeepCopy())
			nc.Spec.Role = "test-role2"
			Expect(nc.Validate(updateCtx)).ToNot(Succeed())
		})
		It("should fail to switch between an unmanaged and managed instance profile", func() {
			nc.Spec.Role = ""
			nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(nc.Validate(ctx)).To(Succeed())

			updateCtx := apis.WithinUpdate(ctx, nc.DeepCopy())
			nc.Spec.Role = "test-role"
			nc.Spec.InstanceProfile = nil
			Expect(nc.Validate(updateCtx)).ToNot(Succeed())
		})
		It("should fail to switch between a managed and unmanaged instance profile", func() {
			nc.Spec.Role = "test-role"
			nc.Spec.InstanceProfile = nil
			Expect(nc.Validate(ctx)).To(Succeed())

			updateCtx := apis.WithinUpdate(ctx, nc.DeepCopy())
			nc.Spec.Role = ""
			nc.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(nc.Validate(updateCtx)).ToNot(Succeed())
		})
	})
})

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

package v1_test

import (
	"fmt"

	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/karpenter/pkg/test"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-sdk-go/service/ec2"

	. "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
)

var _ = Describe("Convert v1 to v1beta1 EC2NodeClass API", func() {
	var (
		v1ec2nodeclass      *EC2NodeClass
		v1beta1ec2nodeclass *v1beta1.EC2NodeClass
	)

	BeforeEach(func() {
		v1ec2nodeclass = &EC2NodeClass{}
		v1beta1ec2nodeclass = &v1beta1.EC2NodeClass{}
	})

	It("should convert v1 ec2nodeclass metadata", func() {
		v1ec2nodeclass.ObjectMeta = test.ObjectMeta(metav1.ObjectMeta{
			Annotations: map[string]string{"foo": "bar"},
		})
		Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
		Expect(v1beta1ec2nodeclass.ObjectMeta).To(BeEquivalentTo(v1ec2nodeclass.ObjectMeta))
	})
	Context("EC2NodeClass Spec", func() {
		It("should convert v1 ec2nodeclass subnet selector terms", func() {
			v1ec2nodeclass.Spec.SubnetSelectorTerms = []SubnetSelectorTerm{
				{
					Tags: map[string]string{"test-key-1": "test-value-1"},
					ID:   "test-id-1",
				},
				{
					Tags: map[string]string{"test-key-2": "test-value-2"},
					ID:   "test-id-2",
				},
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1ec2nodeclass.Spec.SubnetSelectorTerms {
				Expect(v1beta1ec2nodeclass.Spec.SubnetSelectorTerms[i].Tags).To(Equal(v1ec2nodeclass.Spec.SubnetSelectorTerms[i].Tags))
				Expect(v1beta1ec2nodeclass.Spec.SubnetSelectorTerms[i].ID).To(Equal(v1ec2nodeclass.Spec.SubnetSelectorTerms[i].ID))
			}
		})
		It("should convert v1 ec2nodeclass securitygroup selector terms", func() {
			v1ec2nodeclass.Spec.SecurityGroupSelectorTerms = []SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{"test-key-1": "test-value-1"},
					ID:   "test-id-1",
					Name: "test-name-1",
				},
				{
					Tags: map[string]string{"test-key-2": "test-value-2"},
					ID:   "test-id-2",
					Name: "test-name-2",
				},
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1ec2nodeclass.Spec.SecurityGroupSelectorTerms {
				Expect(v1beta1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].Tags).To(Equal(v1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].Tags))
				Expect(v1beta1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].ID).To(Equal(v1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].ID))
				Expect(v1beta1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].Name).To(Equal(v1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].Name))
			}
		})
		It("should convert v1 ec2nodeclass ami selector terms", func() {
			v1ec2nodeclass.Spec.AMISelectorTerms = []AMISelectorTerm{
				{
					Tags:  map[string]string{"test-key-1": "test-value-1"},
					ID:    "test-id-1",
					Name:  "test-name-1",
					Owner: "test-owner-1",
				},
				{
					Tags:  map[string]string{"test-key-2": "test-value-2"},
					ID:    "test-id-2",
					Name:  "test-name-2",
					Owner: "test-owner-1",
				},
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1ec2nodeclass.Spec.AMISelectorTerms {
				Expect(v1beta1ec2nodeclass.Spec.AMISelectorTerms[i].Tags).To(Equal(v1ec2nodeclass.Spec.AMISelectorTerms[i].Tags))
				Expect(v1beta1ec2nodeclass.Spec.AMISelectorTerms[i].ID).To(Equal(v1ec2nodeclass.Spec.AMISelectorTerms[i].ID))
				Expect(v1beta1ec2nodeclass.Spec.AMISelectorTerms[i].Name).To(Equal(v1ec2nodeclass.Spec.AMISelectorTerms[i].Name))
				Expect(v1beta1ec2nodeclass.Spec.AMISelectorTerms[i].Owner).To(Equal(v1ec2nodeclass.Spec.AMISelectorTerms[i].Owner))
			}
		})
		It("should convert v1 ec2nodeclass associate public ip address ", func() {
			v1ec2nodeclass.Spec.AssociatePublicIPAddress = lo.ToPtr(true)
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.AssociatePublicIPAddress)).To(BeTrue())
		})
		It("should convert v1 ec2nodeclass alias", func() {
			v1ec2nodeclass.Spec.AMISelectorTerms = []AMISelectorTerm{{Alias: "al2023@latest"}}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.AMIFamily)).To(Equal(v1beta1.AMIFamilyAL2023))
		})
		It("should convert v1 ec2nodeclass ami selector terms with the Ubuntu compatibility annotation", func() {
			v1ec2nodeclass.Annotations = lo.Assign(v1ec2nodeclass.Annotations, map[string]string{
				AnnotationUbuntuCompatibilityKey: fmt.Sprintf("%s,%s", AnnotationUbuntuCompatibilityAMIFamily, AnnotationUbuntuCompatibilityBlockDeviceMappings),
			})
			v1ec2nodeclass.Spec.AMIFamily = lo.ToPtr(AMIFamilyAL2)
			v1ec2nodeclass.Spec.AMISelectorTerms = []AMISelectorTerm{{ID: "ami-01234567890abcdef"}}
			v1ec2nodeclass.Spec.BlockDeviceMappings = []*BlockDeviceMapping{{
				DeviceName: lo.ToPtr("/dev/sda1"),
				RootVolume: true,
				EBS: &BlockDevice{
					Encrypted:  lo.ToPtr(true),
					VolumeType: lo.ToPtr(ec2.VolumeTypeGp3),
					VolumeSize: lo.ToPtr(resource.MustParse("20Gi")),
				},
			}}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1beta1ec2nodeclass.Annotations).ToNot(HaveKey(AnnotationUbuntuCompatibilityKey))
			Expect(len(v1beta1ec2nodeclass.Spec.BlockDeviceMappings)).To(Equal(0))
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.AMIFamily)).To(Equal(v1beta1.AMIFamilyUbuntu))
			Expect(v1beta1ec2nodeclass.Spec.AMISelectorTerms).To(Equal([]v1beta1.AMISelectorTerm{{ID: "ami-01234567890abcdef"}}))
		})
		It("should convert v1 ec2nodeclass ami selector terms with the Ubuntu compatibility annotation and custom BlockDeviceMappings", func() {
			v1ec2nodeclass.Annotations = lo.Assign(v1ec2nodeclass.Annotations, map[string]string{AnnotationUbuntuCompatibilityKey: AnnotationUbuntuCompatibilityAMIFamily})
			v1ec2nodeclass.Spec.AMIFamily = lo.ToPtr(AMIFamilyAL2)
			v1ec2nodeclass.Spec.AMISelectorTerms = []AMISelectorTerm{{ID: "ami-01234567890abcdef"}}
			v1ec2nodeclass.Spec.BlockDeviceMappings = []*BlockDeviceMapping{{
				DeviceName: lo.ToPtr("/dev/sdb1"),
				RootVolume: true,
				EBS: &BlockDevice{
					Encrypted:  lo.ToPtr(false),
					VolumeType: lo.ToPtr(ec2.VolumeTypeGp2),
					VolumeSize: lo.ToPtr(resource.MustParse("40Gi")),
				},
			}}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1beta1ec2nodeclass.Annotations).ToNot(HaveKey(AnnotationUbuntuCompatibilityKey))
			Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings).To(Equal([]*v1beta1.BlockDeviceMapping{{
				DeviceName: lo.ToPtr("/dev/sdb1"),
				RootVolume: true,
				EBS: &v1beta1.BlockDevice{
					Encrypted:  lo.ToPtr(false),
					VolumeType: lo.ToPtr(ec2.VolumeTypeGp2),
					VolumeSize: lo.ToPtr(resource.MustParse("40Gi")),
				},
			}}))
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.AMIFamily)).To(Equal(v1beta1.AMIFamilyUbuntu))
			Expect(v1beta1ec2nodeclass.Spec.AMISelectorTerms).To(Equal([]v1beta1.AMISelectorTerm{{ID: "ami-01234567890abcdef"}}))
		})
		It("should convert v1 ec2nodeclass user data", func() {
			v1ec2nodeclass.Spec.UserData = lo.ToPtr("test user data")
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.UserData)).To(Equal(lo.FromPtr(v1ec2nodeclass.Spec.UserData)))
		})
		It("should convert v1 ec2nodeclass role", func() {
			v1ec2nodeclass.Spec.Role = "test-role"
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1beta1ec2nodeclass.Spec.Role).To(Equal(v1ec2nodeclass.Spec.Role))
		})
		It("should convert v1 ec2nodeclass instance profile", func() {
			v1ec2nodeclass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.InstanceProfile)).To(Equal(lo.FromPtr(v1ec2nodeclass.Spec.InstanceProfile)))
		})
		It("should convert v1 ec2nodeclass tags", func() {
			v1ec2nodeclass.Spec.Tags = map[string]string{
				"test-key-tag-1": "test-value-tag-1",
				"test-key-tag-2": "test-value-tag-2",
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1beta1ec2nodeclass.Spec.Tags).To(Equal(v1ec2nodeclass.Spec.Tags))
		})
		It("should convert v1 ec2nodeclass block device mapping", func() {
			v1ec2nodeclass.Spec.BlockDeviceMappings = []*BlockDeviceMapping{
				{
					EBS: &BlockDevice{
						DeleteOnTermination: lo.ToPtr(true),
						Encrypted:           lo.ToPtr(true),
						IOPS:                lo.ToPtr(int64(45123)),
						KMSKeyID:            lo.ToPtr("test-kms-id"),
						SnapshotID:          lo.ToPtr("test-snapshot-id"),
						Throughput:          lo.ToPtr(int64(4512433)),
						VolumeSize:          lo.ToPtr(resource.MustParse("54G")),
						VolumeType:          lo.ToPtr("test-type"),
					},
					DeviceName: lo.ToPtr("test-device"),
					RootVolume: true,
				},
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1ec2nodeclass.Spec.BlockDeviceMappings {
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].RootVolume).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].RootVolume))
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].DeviceName).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].DeviceName))
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.DeleteOnTermination).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.DeleteOnTermination))
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.Encrypted).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.Encrypted))
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.IOPS).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.IOPS))
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.KMSKeyID).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.KMSKeyID))
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.SnapshotID).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.SnapshotID))
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.Throughput).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.Throughput))
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.VolumeSize).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.VolumeSize))
				Expect(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.VolumeType).To(Equal(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.VolumeType))
			}
		})
		It("should convert v1 ec2nodeclass instance store policy", func() {
			v1ec2nodeclass.Spec.InstanceStorePolicy = lo.ToPtr(InstanceStorePolicyRAID0)
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(string(lo.FromPtr(v1beta1ec2nodeclass.Spec.InstanceStorePolicy))).To(Equal(string(lo.FromPtr(v1ec2nodeclass.Spec.InstanceStorePolicy))))
		})
		It("should convert v1 ec2nodeclass detailed monitoring", func() {
			v1ec2nodeclass.Spec.DetailedMonitoring = lo.ToPtr(true)
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.DetailedMonitoring)).To(Equal(lo.FromPtr(v1ec2nodeclass.Spec.DetailedMonitoring)))
		})
		It("should convert v1 ec2nodeclass metadata options", func() {
			v1ec2nodeclass.Spec.MetadataOptions = &MetadataOptions{
				HTTPEndpoint:            lo.ToPtr("test-endpoint"),
				HTTPProtocolIPv6:        lo.ToPtr("test-protocol"),
				HTTPPutResponseHopLimit: lo.ToPtr(int64(54)),
				HTTPTokens:              lo.ToPtr("test-token"),
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.MetadataOptions.HTTPEndpoint)).To(Equal(lo.FromPtr(v1ec2nodeclass.Spec.MetadataOptions.HTTPEndpoint)))
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.MetadataOptions.HTTPProtocolIPv6)).To(Equal(lo.FromPtr(v1ec2nodeclass.Spec.MetadataOptions.HTTPProtocolIPv6)))
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.MetadataOptions.HTTPPutResponseHopLimit)).To(Equal(lo.FromPtr(v1ec2nodeclass.Spec.MetadataOptions.HTTPPutResponseHopLimit)))
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.MetadataOptions.HTTPTokens)).To(Equal(lo.FromPtr(v1ec2nodeclass.Spec.MetadataOptions.HTTPTokens)))
		})
		It("should convert v1 ec2nodeclass context", func() {
			v1ec2nodeclass.Spec.Context = lo.ToPtr("test-context")
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1beta1ec2nodeclass.Spec.Context)).To(Equal(lo.FromPtr(v1ec2nodeclass.Spec.Context)))
		})
	})
	Context("EC2NodeClass Status", func() {
		It("should convert v1 ec2nodeclass subnet", func() {
			v1ec2nodeclass.Status.Subnets = []Subnet{
				{
					ID:     "test-id",
					Zone:   "test-zone",
					ZoneID: "test-zone-id",
				},
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1ec2nodeclass.Status.Subnets {
				Expect(v1beta1ec2nodeclass.Status.Subnets[i].ID).To(Equal(v1ec2nodeclass.Status.Subnets[i].ID))
				Expect(v1beta1ec2nodeclass.Status.Subnets[i].Zone).To(Equal(v1ec2nodeclass.Status.Subnets[i].Zone))
				Expect(v1beta1ec2nodeclass.Status.Subnets[i].ZoneID).To(Equal(v1ec2nodeclass.Status.Subnets[i].ZoneID))
			}
		})
		It("should convert v1 ec2nodeclass security group ", func() {
			v1ec2nodeclass.Status.SecurityGroups = []SecurityGroup{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1ec2nodeclass.Status.SecurityGroups {
				Expect(v1beta1ec2nodeclass.Status.SecurityGroups[i].ID).To(Equal(v1ec2nodeclass.Status.SecurityGroups[i].ID))
				Expect(v1beta1ec2nodeclass.Status.SecurityGroups[i].Name).To(Equal(v1ec2nodeclass.Status.SecurityGroups[i].Name))
			}
		})
		It("should convert v1 ec2nodeclass ami", func() {
			v1ec2nodeclass.Status.AMIs = []AMI{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1ec2nodeclass.Status.AMIs {
				Expect(v1beta1ec2nodeclass.Status.AMIs[i].ID).To(Equal(v1ec2nodeclass.Status.AMIs[i].ID))
				Expect(v1beta1ec2nodeclass.Status.AMIs[i].Name).To(Equal(v1ec2nodeclass.Status.AMIs[i].Name))

			}
		})
		It("should convert v1 ec2nodeclass instance profile", func() {
			v1ec2nodeclass.Status.InstanceProfile = "test-instance-profile"
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1beta1ec2nodeclass.Status.InstanceProfile).To(Equal(v1ec2nodeclass.Status.InstanceProfile))
		})
		It("should convert v1 ec2nodeclass conditions", func() {
			v1ec2nodeclass.Status.Conditions = []status.Condition{
				{
					Status: status.ConditionReady,
					Reason: "test-reason",
				},
			}
			Expect(v1ec2nodeclass.ConvertTo(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1beta1ec2nodeclass.Status.Conditions).To(Equal(v1beta1ec2nodeclass.Status.Conditions))
		})
	})
})

var _ = Describe("Convert v1beta1 to v1 EC2NodeClass API", func() {
	var (
		v1ec2nodeclass      *EC2NodeClass
		v1beta1ec2nodeclass *v1beta1.EC2NodeClass
	)

	BeforeEach(func() {
		v1ec2nodeclass = &EC2NodeClass{}
		v1beta1ec2nodeclass = &v1beta1.EC2NodeClass{}
	})

	It("should convert v1beta1 ec2nodeclass metadata", func() {
		v1beta1ec2nodeclass.ObjectMeta = test.ObjectMeta(metav1.ObjectMeta{
			Annotations: map[string]string{"foo": "bar"},
		})
		Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
		Expect(v1ec2nodeclass.ObjectMeta).To(BeEquivalentTo(v1beta1ec2nodeclass.ObjectMeta))
	})
	Context("EC2NodeClass Spec", func() {
		It("should convert v1beta1 ec2nodeclass subnet selector terms", func() {
			v1beta1ec2nodeclass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"test-key-1": "test-value-1"},
					ID:   "test-id-1",
				},
				{
					Tags: map[string]string{"test-key-2": "test-value-2"},
					ID:   "test-id-2",
				},
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1beta1ec2nodeclass.Spec.SubnetSelectorTerms {
				Expect(v1ec2nodeclass.Spec.SubnetSelectorTerms[i].Tags).To(Equal(v1beta1ec2nodeclass.Spec.SubnetSelectorTerms[i].Tags))
				Expect(v1ec2nodeclass.Spec.SubnetSelectorTerms[i].ID).To(Equal(v1beta1ec2nodeclass.Spec.SubnetSelectorTerms[i].ID))
			}
		})
		It("should convert v1beta1 ec2nodeclass securitygroup selector terms", func() {
			v1beta1ec2nodeclass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
				{
					Tags: map[string]string{"test-key-1": "test-value-1"},
					ID:   "test-id-1",
					Name: "test-name-1",
				},
				{
					Tags: map[string]string{"test-key-2": "test-value-2"},
					ID:   "test-id-2",
					Name: "test-name-2",
				},
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1beta1ec2nodeclass.Spec.SecurityGroupSelectorTerms {
				Expect(v1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].Tags).To(Equal(v1beta1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].Tags))
				Expect(v1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].ID).To(Equal(v1beta1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].ID))
				Expect(v1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].Name).To(Equal(v1beta1ec2nodeclass.Spec.SecurityGroupSelectorTerms[i].Name))
			}
		})
		It("should convert v1beta1 ec2nodeclass ami selector terms", func() {
			v1beta1ec2nodeclass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
				{
					Tags:  map[string]string{"test-key-1": "test-value-1"},
					ID:    "test-id-1",
					Name:  "test-name-1",
					Owner: "test-owner-1",
				},
				{
					Tags:  map[string]string{"test-key-2": "test-value-2"},
					ID:    "test-id-2",
					Name:  "test-name-2",
					Owner: "test-owner-1",
				},
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1beta1ec2nodeclass.Spec.AMISelectorTerms {
				Expect(v1ec2nodeclass.Spec.AMISelectorTerms[i].Tags).To(Equal(v1beta1ec2nodeclass.Spec.AMISelectorTerms[i].Tags))
				Expect(v1ec2nodeclass.Spec.AMISelectorTerms[i].ID).To(Equal(v1beta1ec2nodeclass.Spec.AMISelectorTerms[i].ID))
				Expect(v1ec2nodeclass.Spec.AMISelectorTerms[i].Name).To(Equal(v1beta1ec2nodeclass.Spec.AMISelectorTerms[i].Name))
				Expect(v1ec2nodeclass.Spec.AMISelectorTerms[i].Owner).To(Equal(v1beta1ec2nodeclass.Spec.AMISelectorTerms[i].Owner))
			}
		})
		It("should convert v1beta1 ec2nodeclass associate public ip address ", func() {
			v1beta1ec2nodeclass.Spec.AssociatePublicIPAddress = lo.ToPtr(true)
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.AssociatePublicIPAddress)).To(BeTrue())
		})
		It("should convert v1beta1 ec2nodeclass ami family", func() {
			v1beta1ec2nodeclass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2023
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1ec2nodeclass.Spec.AMISelectorTerms).To(ContainElement(AMISelectorTerm{Alias: "al2023@latest"}))
		})
		It("should convert v1beta1 ec2nodeclass ami family with non-custom ami family and ami selector terms", func() {
			v1beta1ec2nodeclass.Spec.AMIFamily = &v1beta1.AMIFamilyAL2023
			v1beta1ec2nodeclass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{
				ID: "ami-0123456789abcdef",
			}}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.AMIFamily)).To(Equal(AMIFamilyAL2023))
			Expect(v1ec2nodeclass.Spec.AMISelectorTerms).To(Equal([]AMISelectorTerm{{
				ID: "ami-0123456789abcdef",
			}}))
		})
		It("should convert v1beta1 ec2nodeclass when amiFamily is Ubuntu (with amiSelectorTerms)", func() {
			v1beta1ec2nodeclass.Spec.AMIFamily = &v1beta1.AMIFamilyUbuntu
			v1beta1ec2nodeclass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: "ami-0123456789abcdef"}}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1ec2nodeclass.Annotations).To(HaveKeyWithValue(
				AnnotationUbuntuCompatibilityKey,
				fmt.Sprintf("%s,%s", AnnotationUbuntuCompatibilityAMIFamily, AnnotationUbuntuCompatibilityBlockDeviceMappings),
			))
			Expect(v1ec2nodeclass.AMIFamily()).To(Equal(AMIFamilyAL2))
			Expect(v1ec2nodeclass.Spec.AMISelectorTerms).To(Equal([]AMISelectorTerm{{ID: "ami-0123456789abcdef"}}))
			Expect(v1ec2nodeclass.Spec.BlockDeviceMappings).To(Equal([]*BlockDeviceMapping{{
				DeviceName: lo.ToPtr("/dev/sda1"),
				RootVolume: true,
				EBS: &BlockDevice{
					Encrypted:  lo.ToPtr(true),
					VolumeType: lo.ToPtr(ec2.VolumeTypeGp3),
					VolumeSize: lo.ToPtr(resource.MustParse("20Gi")),
				},
			}}))
		})
		It("should convert v1beta1 ec2nodeclass when amiFamily is Ubuntu (with amiSelectorTerms and custom BlockDeviceMappings)", func() {
			v1beta1ec2nodeclass.Spec.AMIFamily = &v1beta1.AMIFamilyUbuntu
			v1beta1ec2nodeclass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{{ID: "ami-0123456789abcdef"}}
			v1beta1ec2nodeclass.Spec.BlockDeviceMappings = []*v1beta1.BlockDeviceMapping{{
				DeviceName: lo.ToPtr("/dev/sdb1"),
				RootVolume: true,
				EBS: &v1beta1.BlockDevice{
					Encrypted:  lo.ToPtr(false),
					VolumeType: lo.ToPtr(ec2.VolumeTypeGp2),
					VolumeSize: lo.ToPtr(resource.MustParse("40Gi")),
				},
			}}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1ec2nodeclass.Annotations).To(HaveKeyWithValue(AnnotationUbuntuCompatibilityKey, AnnotationUbuntuCompatibilityAMIFamily))
			Expect(v1ec2nodeclass.AMIFamily()).To(Equal(AMIFamilyAL2))
			Expect(v1ec2nodeclass.Spec.AMISelectorTerms).To(Equal([]AMISelectorTerm{{ID: "ami-0123456789abcdef"}}))
			Expect(v1ec2nodeclass.Spec.BlockDeviceMappings).To(Equal([]*BlockDeviceMapping{{
				DeviceName: lo.ToPtr("/dev/sdb1"),
				RootVolume: true,
				EBS: &BlockDevice{
					Encrypted:  lo.ToPtr(false),
					VolumeType: lo.ToPtr(ec2.VolumeTypeGp2),
					VolumeSize: lo.ToPtr(resource.MustParse("40Gi")),
				},
			}}))
		})
		It("should convert v1beta1 ec2nodeclass when amiFamily is Ubuntu (without amiSelectorTerms) but mark incompatible", func() {
			v1beta1ec2nodeclass.Spec.AMIFamily = lo.ToPtr(v1beta1.AMIFamilyUbuntu)
			v1beta1ec2nodeclass.Spec.AMISelectorTerms = nil
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1ec2nodeclass.UbuntuIncompatible()).To(BeTrue())
		})
		It("should convert v1beta1 ec2nodeclass user data", func() {
			v1beta1ec2nodeclass.Spec.UserData = lo.ToPtr("test user data")
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.UserData)).To(Equal(lo.FromPtr(v1beta1ec2nodeclass.Spec.UserData)))
		})
		It("should convert v1beta1 ec2nodeclass role", func() {
			v1beta1ec2nodeclass.Spec.Role = "test-role"
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1ec2nodeclass.Spec.Role).To(Equal(v1beta1ec2nodeclass.Spec.Role))
		})
		It("should convert v1beta1 ec2nodeclass instance profile", func() {
			v1beta1ec2nodeclass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.InstanceProfile)).To(Equal(lo.FromPtr(v1beta1ec2nodeclass.Spec.InstanceProfile)))
		})
		It("should convert v1beta1 ec2nodeclass tags", func() {
			v1beta1ec2nodeclass.Spec.Tags = map[string]string{
				"test-key-tag-1": "test-value-tag-1",
				"test-key-tag-2": "test-value-tag-2",
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1ec2nodeclass.Spec.Tags).To(Equal(v1beta1ec2nodeclass.Spec.Tags))
		})
		It("should convert v1beta1 ec2nodeclass block device mapping", func() {
			v1beta1ec2nodeclass.Spec.BlockDeviceMappings = []*v1beta1.BlockDeviceMapping{
				{
					EBS: &v1beta1.BlockDevice{
						DeleteOnTermination: lo.ToPtr(true),
						Encrypted:           lo.ToPtr(true),
						IOPS:                lo.ToPtr(int64(45123)),
						KMSKeyID:            lo.ToPtr("test-kms-id"),
						SnapshotID:          lo.ToPtr("test-snapshot-id"),
						Throughput:          lo.ToPtr(int64(4512433)),
						VolumeSize:          lo.ToPtr(resource.MustParse("54G")),
						VolumeType:          lo.ToPtr("test-type"),
					},
					DeviceName: lo.ToPtr("test-device"),
					RootVolume: true,
				},
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1beta1ec2nodeclass.Spec.BlockDeviceMappings {
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].RootVolume).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].RootVolume))
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].DeviceName).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].DeviceName))
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.DeleteOnTermination).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.DeleteOnTermination))
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.Encrypted).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.Encrypted))
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.IOPS).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.IOPS))
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.KMSKeyID).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.KMSKeyID))
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.SnapshotID).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.SnapshotID))
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.Throughput).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.Throughput))
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.VolumeSize).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.VolumeSize))
				Expect(v1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.VolumeType).To(Equal(v1beta1ec2nodeclass.Spec.BlockDeviceMappings[i].EBS.VolumeType))
			}
		})
		It("should convert v1beta1 ec2nodeclass instance store policy", func() {
			v1beta1ec2nodeclass.Spec.InstanceStorePolicy = lo.ToPtr(v1beta1.InstanceStorePolicyRAID0)
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(string(lo.FromPtr(v1ec2nodeclass.Spec.InstanceStorePolicy))).To(Equal(string(lo.FromPtr(v1beta1ec2nodeclass.Spec.InstanceStorePolicy))))
		})
		It("should convert v1beta1 ec2nodeclass detailed monitoring", func() {
			v1beta1ec2nodeclass.Spec.DetailedMonitoring = lo.ToPtr(true)
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.DetailedMonitoring)).To(Equal(lo.FromPtr(v1beta1ec2nodeclass.Spec.DetailedMonitoring)))
		})
		It("should convert v1beta1 ec2nodeclass metadata options", func() {
			v1beta1ec2nodeclass.Spec.MetadataOptions = &v1beta1.MetadataOptions{
				HTTPEndpoint:            lo.ToPtr("test-endpoint"),
				HTTPProtocolIPv6:        lo.ToPtr("test-protocol"),
				HTTPPutResponseHopLimit: lo.ToPtr(int64(54)),
				HTTPTokens:              lo.ToPtr("test-token"),
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.MetadataOptions.HTTPEndpoint)).To(Equal(lo.FromPtr(v1beta1ec2nodeclass.Spec.MetadataOptions.HTTPEndpoint)))
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.MetadataOptions.HTTPProtocolIPv6)).To(Equal(lo.FromPtr(v1beta1ec2nodeclass.Spec.MetadataOptions.HTTPProtocolIPv6)))
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.MetadataOptions.HTTPPutResponseHopLimit)).To(Equal(lo.FromPtr(v1beta1ec2nodeclass.Spec.MetadataOptions.HTTPPutResponseHopLimit)))
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.MetadataOptions.HTTPTokens)).To(Equal(lo.FromPtr(v1beta1ec2nodeclass.Spec.MetadataOptions.HTTPTokens)))
		})
		It("should convert v1beta1 ec2nodeclass context", func() {
			v1beta1ec2nodeclass.Spec.Context = lo.ToPtr("test-context")
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(lo.FromPtr(v1ec2nodeclass.Spec.Context)).To(Equal(lo.FromPtr(v1beta1ec2nodeclass.Spec.Context)))
		})
	})
	Context("EC2NodeClass Status", func() {
		It("should convert v1beta1 ec2nodeclass subnet", func() {
			v1beta1ec2nodeclass.Status.Subnets = []v1beta1.Subnet{
				{
					ID:     "test-id",
					Zone:   "test-zone",
					ZoneID: "test-zone-id",
				},
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1beta1ec2nodeclass.Status.Subnets {
				Expect(v1ec2nodeclass.Status.Subnets[i].ID).To(Equal(v1beta1ec2nodeclass.Status.Subnets[i].ID))
				Expect(v1ec2nodeclass.Status.Subnets[i].Zone).To(Equal(v1beta1ec2nodeclass.Status.Subnets[i].Zone))
				Expect(v1ec2nodeclass.Status.Subnets[i].ZoneID).To(Equal(v1beta1ec2nodeclass.Status.Subnets[i].ZoneID))
			}
		})
		It("should convert v1beta1 ec2nodeclass security group ", func() {
			v1beta1ec2nodeclass.Status.SecurityGroups = []v1beta1.SecurityGroup{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1beta1ec2nodeclass.Status.SecurityGroups {
				Expect(v1ec2nodeclass.Status.SecurityGroups[i].ID).To(Equal(v1beta1ec2nodeclass.Status.SecurityGroups[i].ID))
				Expect(v1ec2nodeclass.Status.SecurityGroups[i].Name).To(Equal(v1beta1ec2nodeclass.Status.SecurityGroups[i].Name))
			}
		})
		It("should convert v1beta1 ec2nodeclass ami", func() {
			v1beta1ec2nodeclass.Status.AMIs = []v1beta1.AMI{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			for i := range v1beta1ec2nodeclass.Status.AMIs {
				Expect(v1ec2nodeclass.Status.AMIs[i].ID).To(Equal(v1beta1ec2nodeclass.Status.AMIs[i].ID))
				Expect(v1ec2nodeclass.Status.AMIs[i].Name).To(Equal(v1beta1ec2nodeclass.Status.AMIs[i].Name))

			}
		})
		It("should convert v1beta1 ec2nodeclass instance profile", func() {
			v1beta1ec2nodeclass.Status.InstanceProfile = "test-instance-profile"
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1ec2nodeclass.Status.InstanceProfile).To(Equal(v1beta1ec2nodeclass.Status.InstanceProfile))
		})
		It("should convert v1beta1 ec2nodeclass conditions", func() {
			v1beta1ec2nodeclass.Status.Conditions = []status.Condition{
				{
					Status: status.ConditionReady,
					Reason: "test-reason",
				},
			}
			Expect(v1ec2nodeclass.ConvertFrom(ctx, v1beta1ec2nodeclass)).To(Succeed())
			Expect(v1ec2nodeclass.Status.Conditions).To(Equal(v1beta1ec2nodeclass.Status.Conditions))
		})
	})
})

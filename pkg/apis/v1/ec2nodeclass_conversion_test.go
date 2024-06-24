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
	"github.com/awslabs/operatorpkg/status"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
)

var v1EC2NodeClass *EC2NodeClass
var v1beta1EC2NodeClass *v1beta1.EC2NodeClass

var _ = Describe("Convert v1 to v1beta1 EC2NodeClass API", func() {
	BeforeEach(func() {
		v1EC2NodeClass = &EC2NodeClass{}
		v1beta1EC2NodeClass = &v1beta1.EC2NodeClass{}
	})

	Context("MetaData", func() {
		It("should convert v1 ec2nodeclass Name", func() {
			v1EC2NodeClass.Name = "test-name-v1"
			Expect(v1beta1EC2NodeClass.Name).To(BeEmpty())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1beta1EC2NodeClass.Name).To(Equal(v1EC2NodeClass.Name))
		})
		It("should convert v1 ec2nodeclass UID", func() {
			v1EC2NodeClass.UID = types.UID("test-name-v1-uuid")
			Expect(v1beta1EC2NodeClass.UID).To(BeEmpty())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1beta1EC2NodeClass.UID).To(Equal(v1EC2NodeClass.UID))
		})
	})
	Context("EC2NodeClass Spec", func() {
		It("should convert v1 ec2nodeclass subnet selector terms", func() {
			v1EC2NodeClass.Spec.SubnetSelectorTerms = []SubnetSelectorTerm{
				{
					Tags: map[string]string{"test-key-1": "test-value-1"},
					ID:   "test-id-1",
				},
				{
					Tags: map[string]string{"test-key-2": "test-value-2"},
					ID:   "test-id-2",
				},
			}
			Expect(len(v1beta1EC2NodeClass.Spec.SubnetSelectorTerms)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1EC2NodeClass.Spec.SubnetSelectorTerms {
				Expect(v1beta1EC2NodeClass.Spec.SubnetSelectorTerms[i].Tags).To(Equal(v1EC2NodeClass.Spec.SubnetSelectorTerms[i].Tags))
				Expect(v1beta1EC2NodeClass.Spec.SubnetSelectorTerms[i].ID).To(Equal(v1EC2NodeClass.Spec.SubnetSelectorTerms[i].ID))
			}
		})
		It("should convert v1 ec2nodeclass securitygroup selector terms", func() {
			v1EC2NodeClass.Spec.SecurityGroupSelectorTerms = []SecurityGroupSelectorTerm{
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
			Expect(len(v1beta1EC2NodeClass.Spec.SecurityGroupSelectorTerms)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1EC2NodeClass.Spec.SecurityGroupSelectorTerms {
				Expect(v1beta1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].Tags).To(Equal(v1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].Tags))
				Expect(v1beta1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].ID).To(Equal(v1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].ID))
				Expect(v1beta1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].Name).To(Equal(v1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].Name))
			}
		})
		It("should convert v1 ec2nodeclass ami selector terms", func() {
			v1EC2NodeClass.Spec.AMISelectorTerms = []AMISelectorTerm{
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
			Expect(len(v1beta1EC2NodeClass.Spec.AMISelectorTerms)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1EC2NodeClass.Spec.AMISelectorTerms {
				Expect(v1beta1EC2NodeClass.Spec.AMISelectorTerms[i].Tags).To(Equal(v1EC2NodeClass.Spec.AMISelectorTerms[i].Tags))
				Expect(v1beta1EC2NodeClass.Spec.AMISelectorTerms[i].ID).To(Equal(v1EC2NodeClass.Spec.AMISelectorTerms[i].ID))
				Expect(v1beta1EC2NodeClass.Spec.AMISelectorTerms[i].Name).To(Equal(v1EC2NodeClass.Spec.AMISelectorTerms[i].Name))
				Expect(v1beta1EC2NodeClass.Spec.AMISelectorTerms[i].Owner).To(Equal(v1EC2NodeClass.Spec.AMISelectorTerms[i].Owner))
			}
		})
		It("should convert v1 ec2nodeclass associate public ip address ", func() {
			v1EC2NodeClass.Spec.AssociatePublicIPAddress = lo.ToPtr(true)
			Expect(v1beta1EC2NodeClass.Spec.AssociatePublicIPAddress).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.AssociatePublicIPAddress)).To(BeTrue())
		})
		It("should convert v1 ec2nodeclass ami family", func() {
			v1EC2NodeClass.Spec.AMIFamily = &AMIFamilyUbuntu
			Expect(v1beta1EC2NodeClass.Spec.AMIFamily).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.AMIFamily)).To(Equal(v1beta1.AMIFamilyUbuntu))
		})
		It("should convert v1 ec2nodeclass user data", func() {
			v1EC2NodeClass.Spec.UserData = lo.ToPtr("test user data")
			Expect(v1beta1EC2NodeClass.Spec.UserData).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.UserData)).To(Equal(lo.FromPtr(v1EC2NodeClass.Spec.UserData)))
		})
		It("should convert v1 ec2nodeclass role", func() {
			v1EC2NodeClass.Spec.Role = "test-role"
			Expect(v1beta1EC2NodeClass.Spec.Role).To(Equal(""))
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1beta1EC2NodeClass.Spec.Role).To(Equal(v1EC2NodeClass.Spec.Role))
		})
		It("should convert v1 ec2nodeclass instance profile", func() {
			v1EC2NodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(v1beta1EC2NodeClass.Spec.InstanceProfile).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.InstanceProfile)).To(Equal(lo.FromPtr(v1EC2NodeClass.Spec.InstanceProfile)))
		})
		It("should convert v1 ec2nodeclass tags", func() {
			v1EC2NodeClass.Spec.Tags = map[string]string{
				"test-key-tag-1": "test-value-tag-1",
				"test-key-tag-2": "test-value-tag-2",
			}
			Expect(v1beta1EC2NodeClass.Spec.Tags).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1beta1EC2NodeClass.Spec.Tags).To(Equal(v1EC2NodeClass.Spec.Tags))
		})
		It("should convert v1 ec2nodeclass block device mapping", func() {
			v1EC2NodeClass.Spec.BlockDeviceMappings = []*BlockDeviceMapping{
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
					RootVolume: false,
				},
			}
			Expect(v1beta1EC2NodeClass.Spec.Tags).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1EC2NodeClass.Spec.BlockDeviceMappings {
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].RootVolume).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].RootVolume))
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].DeviceName).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].DeviceName))
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.DeleteOnTermination).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.DeleteOnTermination))
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.Encrypted).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.Encrypted))
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.IOPS).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.IOPS))
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.KMSKeyID).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.KMSKeyID))
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.SnapshotID).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.SnapshotID))
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.Throughput).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.Throughput))
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.VolumeSize).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.VolumeSize))
				Expect(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.VolumeType).To(Equal(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.VolumeType))
			}
		})
		It("should convert v1 ec2nodeclass instance store policy", func() {
			v1EC2NodeClass.Spec.InstanceStorePolicy = lo.ToPtr(InstanceStorePolicyRAID0)
			Expect(v1beta1EC2NodeClass.Spec.InstanceStorePolicy).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(string(lo.FromPtr(v1beta1EC2NodeClass.Spec.InstanceStorePolicy))).To(Equal(string(lo.FromPtr(v1EC2NodeClass.Spec.InstanceStorePolicy))))
		})
		It("should convert v1 ec2nodeclass detailed monitoring", func() {
			v1EC2NodeClass.Spec.DetailedMonitoring = lo.ToPtr(true)
			Expect(v1beta1EC2NodeClass.Spec.DetailedMonitoring).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.DetailedMonitoring)).To(Equal(lo.FromPtr(v1EC2NodeClass.Spec.DetailedMonitoring)))
		})
		It("should convert v1 ec2nodeclass metadata options", func() {
			v1EC2NodeClass.Spec.MetadataOptions = &MetadataOptions{
				HTTPEndpoint:            lo.ToPtr("test-endpoint"),
				HTTPProtocolIPv6:        lo.ToPtr("test-protocal"),
				HTTPPutResponseHopLimit: lo.ToPtr(int64(54)),
				HTTPTokens:              lo.ToPtr("test-token"),
			}
			Expect(v1beta1EC2NodeClass.Spec.MetadataOptions).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.MetadataOptions.HTTPEndpoint)).To(Equal(lo.FromPtr(v1EC2NodeClass.Spec.MetadataOptions.HTTPEndpoint)))
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.MetadataOptions.HTTPProtocolIPv6)).To(Equal(lo.FromPtr(v1EC2NodeClass.Spec.MetadataOptions.HTTPProtocolIPv6)))
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.MetadataOptions.HTTPPutResponseHopLimit)).To(Equal(lo.FromPtr(v1EC2NodeClass.Spec.MetadataOptions.HTTPPutResponseHopLimit)))
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.MetadataOptions.HTTPTokens)).To(Equal(lo.FromPtr(v1EC2NodeClass.Spec.MetadataOptions.HTTPTokens)))
		})
		It("should convert v1 ec2nodeclass context", func() {
			v1EC2NodeClass.Spec.Context = lo.ToPtr("test-context")
			Expect(v1beta1EC2NodeClass.Spec.Context).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1beta1EC2NodeClass.Spec.Context)).To(Equal(lo.FromPtr(v1EC2NodeClass.Spec.Context)))
		})
	})
	Context("EC2NodeClass Status", func() {
		It("should convert v1 ec2nodeclass subnet", func() {
			v1EC2NodeClass.Status.Subnets = []Subnet{
				{
					ID:     "test-id",
					Zone:   "test-zone",
					ZoneID: "test-zone-id",
				},
			}
			Expect(len(v1beta1EC2NodeClass.Status.Subnets)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1EC2NodeClass.Status.Subnets {
				Expect(v1beta1EC2NodeClass.Status.Subnets[i].ID).To(Equal(v1EC2NodeClass.Status.Subnets[i].ID))
				Expect(v1beta1EC2NodeClass.Status.Subnets[i].Zone).To(Equal(v1EC2NodeClass.Status.Subnets[i].Zone))
				Expect(v1beta1EC2NodeClass.Status.Subnets[i].ZoneID).To(Equal(v1EC2NodeClass.Status.Subnets[i].ZoneID))
			}
		})
		It("should convert v1 ec2nodeclass security group ", func() {
			v1EC2NodeClass.Status.SecurityGroups = []SecurityGroup{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			}
			Expect(len(v1beta1EC2NodeClass.Status.SecurityGroups)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1EC2NodeClass.Status.SecurityGroups {
				Expect(v1beta1EC2NodeClass.Status.SecurityGroups[i].ID).To(Equal(v1EC2NodeClass.Status.SecurityGroups[i].ID))
				Expect(v1beta1EC2NodeClass.Status.SecurityGroups[i].Name).To(Equal(v1EC2NodeClass.Status.SecurityGroups[i].Name))
			}
		})
		It("should convert v1 ec2nodeclass ami", func() {
			v1EC2NodeClass.Status.AMIs = []AMI{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			}
			Expect(len(v1beta1EC2NodeClass.Status.AMIs)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1EC2NodeClass.Status.AMIs {
				Expect(v1beta1EC2NodeClass.Status.AMIs[i].ID).To(Equal(v1EC2NodeClass.Status.AMIs[i].ID))
				Expect(v1beta1EC2NodeClass.Status.AMIs[i].Name).To(Equal(v1EC2NodeClass.Status.AMIs[i].Name))

			}
		})
		It("should convert v1 ec2nodeclass instance profile", func() {
			v1EC2NodeClass.Status.InstanceProfile = "test-instance-profile"
			Expect(v1beta1EC2NodeClass.Status.InstanceProfile).To(Equal(""))
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1beta1EC2NodeClass.Status.InstanceProfile).To(Equal(v1EC2NodeClass.Status.InstanceProfile))
		})
		It("should convert v1 ec2nodeclass conditions", func() {
			v1EC2NodeClass.Status.Conditions = []status.Condition{
				{
					Status: status.ConditionReady,
					Reason: "test-reason",
				},
			}
			Expect(v1beta1EC2NodeClass.Status.Conditions).To(BeNil())
			Expect(v1EC2NodeClass.ConvertTo(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1beta1EC2NodeClass.Status.Conditions).To(Equal(v1beta1EC2NodeClass.Status.Conditions))
		})
	})
})

var _ = Describe("Convert v1beta1 to v1 EC2NodeClass API", func() {
	BeforeEach(func() {
		v1EC2NodeClass = &EC2NodeClass{}
		v1beta1EC2NodeClass = &v1beta1.EC2NodeClass{}
	})

	Context("MetaData", func() {
		It("should convert v1beta1 ec2nodeclass Name", func() {
			v1beta1EC2NodeClass.Name = "test-name-v1beta1"
			Expect(v1EC2NodeClass.Name).To(BeEmpty())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1EC2NodeClass.Name).To(Equal(v1beta1EC2NodeClass.Name))
		})
		It("should convert v1beta1 ec2nodeclass UID", func() {
			v1beta1EC2NodeClass.UID = types.UID("test-name-v1beta1-uuid")
			Expect(v1EC2NodeClass.UID).To(BeEmpty())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1EC2NodeClass.UID).To(Equal(v1beta1EC2NodeClass.UID))
		})
	})
	Context("EC2NodeClass Spec", func() {
		It("should convert v1beta1 ec2nodeclass subnet selector terms", func() {
			v1beta1EC2NodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
				{
					Tags: map[string]string{"test-key-1": "test-value-1"},
					ID:   "test-id-1",
				},
				{
					Tags: map[string]string{"test-key-2": "test-value-2"},
					ID:   "test-id-2",
				},
			}
			Expect(len(v1EC2NodeClass.Spec.SubnetSelectorTerms)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1beta1EC2NodeClass.Spec.SubnetSelectorTerms {
				Expect(v1EC2NodeClass.Spec.SubnetSelectorTerms[i].Tags).To(Equal(v1beta1EC2NodeClass.Spec.SubnetSelectorTerms[i].Tags))
				Expect(v1EC2NodeClass.Spec.SubnetSelectorTerms[i].ID).To(Equal(v1beta1EC2NodeClass.Spec.SubnetSelectorTerms[i].ID))
			}
		})
		It("should convert v1beta1 ec2nodeclass securitygroup selector terms", func() {
			v1beta1EC2NodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
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
			Expect(len(v1EC2NodeClass.Spec.SecurityGroupSelectorTerms)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1beta1EC2NodeClass.Spec.SecurityGroupSelectorTerms {
				Expect(v1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].Tags).To(Equal(v1beta1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].Tags))
				Expect(v1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].ID).To(Equal(v1beta1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].ID))
				Expect(v1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].Name).To(Equal(v1beta1EC2NodeClass.Spec.SecurityGroupSelectorTerms[i].Name))
			}
		})
		It("should convert v1beta1 ec2nodeclass ami selector terms", func() {
			v1beta1EC2NodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
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
			Expect(len(v1EC2NodeClass.Spec.AMISelectorTerms)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1beta1EC2NodeClass.Spec.AMISelectorTerms {
				Expect(v1EC2NodeClass.Spec.AMISelectorTerms[i].Tags).To(Equal(v1beta1EC2NodeClass.Spec.AMISelectorTerms[i].Tags))
				Expect(v1EC2NodeClass.Spec.AMISelectorTerms[i].ID).To(Equal(v1beta1EC2NodeClass.Spec.AMISelectorTerms[i].ID))
				Expect(v1EC2NodeClass.Spec.AMISelectorTerms[i].Name).To(Equal(v1beta1EC2NodeClass.Spec.AMISelectorTerms[i].Name))
				Expect(v1EC2NodeClass.Spec.AMISelectorTerms[i].Owner).To(Equal(v1beta1EC2NodeClass.Spec.AMISelectorTerms[i].Owner))
			}
		})
		It("should convert v1beta1 ec2nodeclass associate public ip address ", func() {
			v1beta1EC2NodeClass.Spec.AssociatePublicIPAddress = lo.ToPtr(true)
			Expect(v1EC2NodeClass.Spec.AssociatePublicIPAddress).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.AssociatePublicIPAddress)).To(BeTrue())
		})
		It("should convert v1beta1 ec2nodeclass ami family", func() {
			v1beta1EC2NodeClass.Spec.AMIFamily = &AMIFamilyUbuntu
			Expect(v1EC2NodeClass.Spec.AMIFamily).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.AMIFamily)).To(Equal(v1beta1.AMIFamilyUbuntu))
		})
		It("should convert v1beta1 ec2nodeclass user data", func() {
			v1beta1EC2NodeClass.Spec.UserData = lo.ToPtr("test user data")
			Expect(v1EC2NodeClass.Spec.UserData).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.UserData)).To(Equal(lo.FromPtr(v1beta1EC2NodeClass.Spec.UserData)))
		})
		It("should convert v1beta1 ec2nodeclass role", func() {
			v1beta1EC2NodeClass.Spec.Role = "test-role"
			Expect(v1EC2NodeClass.Spec.Role).To(Equal(""))
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1EC2NodeClass.Spec.Role).To(Equal(v1beta1EC2NodeClass.Spec.Role))
		})
		It("should convert v1beta1 ec2nodeclass instance profile", func() {
			v1beta1EC2NodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
			Expect(v1EC2NodeClass.Spec.InstanceProfile).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.InstanceProfile)).To(Equal(lo.FromPtr(v1beta1EC2NodeClass.Spec.InstanceProfile)))
		})
		It("should convert v1beta1 ec2nodeclass tags", func() {
			v1beta1EC2NodeClass.Spec.Tags = map[string]string{
				"test-key-tag-1": "test-value-tag-1",
				"test-key-tag-2": "test-value-tag-2",
			}
			Expect(v1EC2NodeClass.Spec.Tags).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1EC2NodeClass.Spec.Tags).To(Equal(v1beta1EC2NodeClass.Spec.Tags))
		})
		It("should convert v1beta1 ec2nodeclass block device mapping", func() {
			v1beta1EC2NodeClass.Spec.BlockDeviceMappings = []*v1beta1.BlockDeviceMapping{
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
					RootVolume: false,
				},
			}
			Expect(v1EC2NodeClass.Spec.Tags).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1beta1EC2NodeClass.Spec.BlockDeviceMappings {
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].RootVolume).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].RootVolume))
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].DeviceName).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].DeviceName))
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.DeleteOnTermination).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.DeleteOnTermination))
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.Encrypted).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.Encrypted))
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.IOPS).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.IOPS))
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.KMSKeyID).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.KMSKeyID))
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.SnapshotID).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.SnapshotID))
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.Throughput).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.Throughput))
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.VolumeSize).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.VolumeSize))
				Expect(v1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.VolumeType).To(Equal(v1beta1EC2NodeClass.Spec.BlockDeviceMappings[i].EBS.VolumeType))
			}
		})
		It("should convert v1beta1 ec2nodeclass instance store policy", func() {
			v1beta1EC2NodeClass.Spec.InstanceStorePolicy = lo.ToPtr(v1beta1.InstanceStorePolicyRAID0)
			Expect(v1EC2NodeClass.Spec.InstanceStorePolicy).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(string(lo.FromPtr(v1EC2NodeClass.Spec.InstanceStorePolicy))).To(Equal(string(lo.FromPtr(v1beta1EC2NodeClass.Spec.InstanceStorePolicy))))
		})
		It("should convert v1beta1 ec2nodeclass detailed monitoring", func() {
			v1beta1EC2NodeClass.Spec.DetailedMonitoring = lo.ToPtr(true)
			Expect(v1EC2NodeClass.Spec.DetailedMonitoring).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.DetailedMonitoring)).To(Equal(lo.FromPtr(v1beta1EC2NodeClass.Spec.DetailedMonitoring)))
		})
		It("should convert v1beta1 ec2nodeclass metadata options", func() {
			v1beta1EC2NodeClass.Spec.MetadataOptions = &v1beta1.MetadataOptions{
				HTTPEndpoint:            lo.ToPtr("test-endpoint"),
				HTTPProtocolIPv6:        lo.ToPtr("test-protocal"),
				HTTPPutResponseHopLimit: lo.ToPtr(int64(54)),
				HTTPTokens:              lo.ToPtr("test-token"),
			}
			Expect(v1EC2NodeClass.Spec.MetadataOptions).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.MetadataOptions.HTTPEndpoint)).To(Equal(lo.FromPtr(v1beta1EC2NodeClass.Spec.MetadataOptions.HTTPEndpoint)))
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.MetadataOptions.HTTPProtocolIPv6)).To(Equal(lo.FromPtr(v1beta1EC2NodeClass.Spec.MetadataOptions.HTTPProtocolIPv6)))
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.MetadataOptions.HTTPPutResponseHopLimit)).To(Equal(lo.FromPtr(v1beta1EC2NodeClass.Spec.MetadataOptions.HTTPPutResponseHopLimit)))
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.MetadataOptions.HTTPTokens)).To(Equal(lo.FromPtr(v1beta1EC2NodeClass.Spec.MetadataOptions.HTTPTokens)))
		})
		It("should convert v1beta1 ec2nodeclass context", func() {
			v1beta1EC2NodeClass.Spec.Context = lo.ToPtr("test-context")
			Expect(v1EC2NodeClass.Spec.Context).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(lo.FromPtr(v1EC2NodeClass.Spec.Context)).To(Equal(lo.FromPtr(v1beta1EC2NodeClass.Spec.Context)))
		})
	})
	Context("EC2NodeClass Status", func() {
		It("should convert v1beta1 ec2nodeclass subnet", func() {
			v1beta1EC2NodeClass.Status.Subnets = []v1beta1.Subnet{
				{
					ID:     "test-id",
					Zone:   "test-zone",
					ZoneID: "test-zone-id",
				},
			}
			Expect(len(v1EC2NodeClass.Status.Subnets)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1beta1EC2NodeClass.Status.Subnets {
				Expect(v1EC2NodeClass.Status.Subnets[i].ID).To(Equal(v1beta1EC2NodeClass.Status.Subnets[i].ID))
				Expect(v1EC2NodeClass.Status.Subnets[i].Zone).To(Equal(v1beta1EC2NodeClass.Status.Subnets[i].Zone))
				Expect(v1EC2NodeClass.Status.Subnets[i].ZoneID).To(Equal(v1beta1EC2NodeClass.Status.Subnets[i].ZoneID))
			}
		})
		It("should convert v1beta1 ec2nodeclass security group ", func() {
			v1beta1EC2NodeClass.Status.SecurityGroups = []v1beta1.SecurityGroup{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			}
			Expect(len(v1EC2NodeClass.Status.SecurityGroups)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1beta1EC2NodeClass.Status.SecurityGroups {
				Expect(v1EC2NodeClass.Status.SecurityGroups[i].ID).To(Equal(v1beta1EC2NodeClass.Status.SecurityGroups[i].ID))
				Expect(v1EC2NodeClass.Status.SecurityGroups[i].Name).To(Equal(v1beta1EC2NodeClass.Status.SecurityGroups[i].Name))
			}
		})
		It("should convert v1beta1 ec2nodeclass ami", func() {
			v1beta1EC2NodeClass.Status.AMIs = []v1beta1.AMI{
				{
					ID:   "test-id",
					Name: "test-name",
				},
			}
			Expect(len(v1EC2NodeClass.Status.AMIs)).To(BeNumerically("==", 0))
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			for i := range v1beta1EC2NodeClass.Status.AMIs {
				Expect(v1EC2NodeClass.Status.AMIs[i].ID).To(Equal(v1beta1EC2NodeClass.Status.AMIs[i].ID))
				Expect(v1EC2NodeClass.Status.AMIs[i].Name).To(Equal(v1beta1EC2NodeClass.Status.AMIs[i].Name))

			}
		})
		It("should convert v1beta1 ec2nodeclass instance profile", func() {
			v1beta1EC2NodeClass.Status.InstanceProfile = "test-instance-profile"
			Expect(v1EC2NodeClass.Status.InstanceProfile).To(Equal(""))
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1EC2NodeClass.Status.InstanceProfile).To(Equal(v1beta1EC2NodeClass.Status.InstanceProfile))
		})
		It("should convert v1beta1 ec2nodeclass conditions", func() {
			v1beta1EC2NodeClass.Status.Conditions = []status.Condition{
				{
					Status: status.ConditionReady,
					Reason: "test-reason",
				},
			}
			Expect(v1EC2NodeClass.Status.Conditions).To(BeNil())
			Expect(v1EC2NodeClass.ConvertFrom(ctx, v1beta1EC2NodeClass)).To(BeNil())
			Expect(v1EC2NodeClass.Status.Conditions).To(Equal(v1beta1EC2NodeClass.Status.Conditions))
		})
	})
})

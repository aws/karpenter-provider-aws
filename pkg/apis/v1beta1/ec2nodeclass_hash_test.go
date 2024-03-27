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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/imdario/mergo"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/aws/karpenter-provider-aws/pkg/apis/v1beta1"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hash", func() {
	const staticHash = "10790156025840984195"
	var nodeClass *v1beta1.EC2NodeClass
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1beta1.EC2NodeClass{
			Spec: v1beta1.EC2NodeClassSpec{
				AMIFamily: lo.ToPtr(v1beta1.AMIFamilyAL2023),
				Role:      "role-1",
				Tags: map[string]string{
					"keyTag-1": "valueTag-1",
					"keyTag-2": "valueTag-2",
				},
				Context:                  lo.ToPtr("fake-context"),
				DetailedMonitoring:       lo.ToPtr(false),
				AssociatePublicIPAddress: lo.ToPtr(false),
				MetadataOptions: &v1beta1.MetadataOptions{
					HTTPEndpoint:            lo.ToPtr("disabled"),
					HTTPProtocolIPv6:        lo.ToPtr("disabled"),
					HTTPPutResponseHopLimit: lo.ToPtr(int64(1)),
					HTTPTokens:              lo.ToPtr("optional"),
				},
				BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{
					{
						DeviceName: lo.ToPtr("map-device-1"),
						RootVolume: false,
						EBS: &v1beta1.BlockDevice{
							DeleteOnTermination: lo.ToPtr(false),
							Encrypted:           lo.ToPtr(false),
							IOPS:                lo.ToPtr(int64(0)),
							KMSKeyID:            lo.ToPtr("fakeKMSKeyID"),
							SnapshotID:          lo.ToPtr("fakeSnapshot"),
							Throughput:          lo.ToPtr(int64(0)),
							VolumeSize:          resource.NewScaledQuantity(2, resource.Giga),
							VolumeType:          lo.ToPtr("standard"),
						},
					},
					{
						DeviceName: lo.ToPtr("map-device-2"),
					},
				},
				UserData: aws.String("userdata-test-1"),
			},
		})
	})
	DescribeTable(
		"should match static hash on field value change",
		func(hash string, changes v1beta1.EC2NodeClass) {
			Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride, mergo.WithSliceDeepCopy)).To(Succeed())
			Expect(nodeClass.Hash()).To(Equal(hash))
		},
		Entry("Base EC2NodeClass", staticHash, v1beta1.EC2NodeClass{}),
		// Static fields, expect changed hash from base
		Entry("UserData", "18317182711135792962", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
		Entry("Tags", "7254882043893135054", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
		Entry("Context", "17271601354348855032", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
		Entry("DetailedMonitoring", "3320998103335094348", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
		Entry("AMIFamily", "11029247967399146065", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMIFamily: aws.String(v1beta1.AMIFamilyBottlerocket)}}),
		Entry("InstanceStorePolicy", "15591048753403695860", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{InstanceStorePolicy: lo.ToPtr(v1beta1.InstanceStorePolicyRAID0)}}),
		Entry("AssociatePublicIPAddress", "8788624850560996180", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AssociatePublicIPAddress: lo.ToPtr(true)}}),
		Entry("MetadataOptions HTTPEndpoint", "12130088184516131939", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPEndpoint: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPProtocolIPv6", "9851778617676567202", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPProtocolIPv6: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPPutResponseHopLimit", "10114972825726256442", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPPutResponseHopLimit: lo.ToPtr(int64(10))}}}),
		Entry("MetadataOptions HTTPTokens", "15328515228245883488", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPTokens: lo.ToPtr("required")}}}),
		Entry("BlockDeviceMapping DeviceName", "14855383487702710824", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{DeviceName: lo.ToPtr("map-device-test-3")}}}}),
		Entry("BlockDeviceMapping RootVolume", "9591488558660758449", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{RootVolume: true}}}}),
		Entry("BlockDeviceMapping DeleteOnTermination", "2802222466202766732", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{DeleteOnTermination: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping Encrypted", "16743053872042184219", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{Encrypted: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping IOPS", "17284705682110195253", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{IOPS: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping KMSKeyID", "9151019926310241707", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{KMSKeyID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping SnapshotID", "5250341140179985875", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{SnapshotID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping Throughput", "16711481758638864953", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{Throughput: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping VolumeType", "488614640133725370", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{VolumeType: lo.ToPtr("io1")}}}}}),

		// Behavior / Dynamic fields, expect same hash as base
		Entry("Modified AMISelector", staticHash, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMISelectorTerms: []v1beta1.AMISelectorTerm{{Tags: map[string]string{"ami-test-key": "ami-test-value"}}}}}),
		Entry("Modified SubnetSelector", staticHash, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{{Tags: map[string]string{"subnet-test-key": "subnet-test-value"}}}}}),
		Entry("Modified SecurityGroupSelector", staticHash, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{{Tags: map[string]string{"security-group-test-key": "security-group-test-value"}}}}}),
	)
	// We create a separate test for updating blockDeviceMapping volumeSize, since resource.Quantity is a struct, and mergo.WithSliceDeepCopy
	// doesn't work well with unexported fields, like the ones that are present in resource.Quantity
	It("should match static hash when updating blockDeviceMapping volumeSize", func() {
		nodeClass.Spec.BlockDeviceMappings[0].EBS.VolumeSize = resource.NewScaledQuantity(10, resource.Giga)
		Expect(nodeClass.Hash()).To(Equal("4802236799448001710"))
	})
	It("should match static hash for instanceProfile", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		Expect(nodeClass.Hash()).To(Equal("7914642030762404205"))
	})
	It("should match static hash when reordering tags", func() {
		nodeClass.Spec.Tags = map[string]string{"keyTag-2": "valueTag-2", "keyTag-1": "valueTag-1"}
		Expect(nodeClass.Hash()).To(Equal(staticHash))
	})
	It("should match static hash when reordering blockDeviceMappings", func() {
		nodeClass.Spec.BlockDeviceMappings[0], nodeClass.Spec.BlockDeviceMappings[1] = nodeClass.Spec.BlockDeviceMappings[1], nodeClass.Spec.BlockDeviceMappings[0]
		Expect(nodeClass.Hash()).To(Equal(staticHash))
	})
	DescribeTable("should change hash when static fields are updated", func(changes v1beta1.EC2NodeClass) {
		hash := nodeClass.Hash()
		Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride, mergo.WithSliceDeepCopy)).To(Succeed())
		updatedHash := nodeClass.Hash()
		Expect(hash).ToNot(Equal(updatedHash))
	},
		Entry("UserData", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
		Entry("Tags", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
		Entry("Context", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
		Entry("DetailedMonitoring", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
		Entry("AMIFamily", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMIFamily: aws.String(v1beta1.AMIFamilyBottlerocket)}}),
		Entry("InstanceStorePolicy", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{InstanceStorePolicy: lo.ToPtr(v1beta1.InstanceStorePolicyRAID0)}}),
		Entry("AssociatePublicIPAddress", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AssociatePublicIPAddress: lo.ToPtr(true)}}),
		Entry("MetadataOptions HTTPEndpoint", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPEndpoint: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPProtocolIPv6", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPProtocolIPv6: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPPutResponseHopLimit", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPPutResponseHopLimit: lo.ToPtr(int64(10))}}}),
		Entry("MetadataOptions HTTPTokens", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPTokens: lo.ToPtr("required")}}}),
		Entry("BlockDeviceMapping DeviceName", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{DeviceName: lo.ToPtr("map-device-test-3")}}}}),
		Entry("BlockDeviceMapping RootVolume", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{RootVolume: true}}}}),
		Entry("BlockDeviceMapping DeleteOnTermination", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{DeleteOnTermination: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping Encrypted", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{Encrypted: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping IOPS", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{IOPS: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping KMSKeyID", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{KMSKeyID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping SnapshotID", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{SnapshotID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping Throughput", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{Throughput: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping VolumeType", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{EBS: &v1beta1.BlockDevice{VolumeType: lo.ToPtr("io1")}}}}}),
	)
	// We create a separate test for updating blockDeviceMapping volumeSize, since resource.Quantity is a struct, and mergo.WithSliceDeepCopy
	// doesn't work well with unexported fields, like the ones that are present in resource.Quantity
	It("should change hash blockDeviceMapping volumeSize is updated", func() {
		hash := nodeClass.Hash()
		nodeClass.Spec.BlockDeviceMappings[0].EBS.VolumeSize = resource.NewScaledQuantity(10, resource.Giga)
		updatedHash := nodeClass.Hash()
		Expect(hash).ToNot(Equal(updatedHash))
	})
	It("should change hash when instanceProfile is updated", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		hash := nodeClass.Hash()
		nodeClass.Spec.InstanceProfile = lo.ToPtr("other-instance-profile")
		updatedHash := nodeClass.Hash()
		Expect(hash).ToNot(Equal(updatedHash))
	})
	It("should not change hash when tags are re-ordered", func() {
		hash := nodeClass.Hash()
		nodeClass.Spec.Tags = map[string]string{"keyTag-2": "valueTag-2", "keyTag-1": "valueTag-1"}
		updatedHash := nodeClass.Hash()
		Expect(hash).To(Equal(updatedHash))
	})
	It("should not change hash when blockDeviceMappings are re-ordered", func() {
		hash := nodeClass.Hash()
		nodeClass.Spec.BlockDeviceMappings[0], nodeClass.Spec.BlockDeviceMappings[1] = nodeClass.Spec.BlockDeviceMappings[1], nodeClass.Spec.BlockDeviceMappings[0]
		updatedHash := nodeClass.Hash()
		Expect(hash).To(Equal(updatedHash))
	})
	It("should not change hash when behavior/dynamic fields are updated", func() {
		hash := nodeClass.Hash()

		// Update a behavior/dynamic field
		nodeClass.Spec.SubnetSelectorTerms = []v1beta1.SubnetSelectorTerm{
			{
				Tags: map[string]string{"subnet-test-key": "subnet-test-value"},
			},
		}
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1beta1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{"sg-test-key": "sg-test-value"},
			},
		}
		nodeClass.Spec.AMISelectorTerms = []v1beta1.AMISelectorTerm{
			{
				Tags: map[string]string{"ami-test-key": "ami-test-value"},
			},
		}
		updatedHash := nodeClass.Hash()
		Expect(hash).To(Equal(updatedHash))
	})
	It("should expect two EC2NodeClasses with the same spec to have the same hash", func() {
		otherNodeClass := test.EC2NodeClass(v1beta1.EC2NodeClass{
			Spec: nodeClass.Spec,
		})
		Expect(nodeClass.Hash()).To(Equal(otherNodeClass.Hash()))
	})
})

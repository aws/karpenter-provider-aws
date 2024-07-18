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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/imdario/mergo"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hash", func() {
	const staticHash = "12469392724194263290"
	var nodeClass *v1.EC2NodeClass
	BeforeEach(func() {
		nodeClass = &v1.EC2NodeClass{
			ObjectMeta: test.ObjectMeta(metav1.ObjectMeta{}),
			Spec: v1.EC2NodeClassSpec{
				Role: "role-1",
				Tags: map[string]string{
					"keyTag-1": "valueTag-1",
					"keyTag-2": "valueTag-2",
				},
				Context:                  lo.ToPtr("fake-context"),
				DetailedMonitoring:       lo.ToPtr(false),
				AssociatePublicIPAddress: lo.ToPtr(false),
				MetadataOptions: &v1.MetadataOptions{
					HTTPEndpoint:            lo.ToPtr("disabled"),
					HTTPProtocolIPv6:        lo.ToPtr("disabled"),
					HTTPPutResponseHopLimit: lo.ToPtr(int64(1)),
					HTTPTokens:              lo.ToPtr("optional"),
				},
				BlockDeviceMappings: []*v1.BlockDeviceMapping{
					{
						DeviceName: lo.ToPtr("map-device-1"),
						RootVolume: false,
						EBS: &v1.BlockDevice{
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
		}
	})
	DescribeTable(
		"should match static hash on field value change",
		func(hash string, changes v1.EC2NodeClass) {
			Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride, mergo.WithSliceDeepCopy)).To(Succeed())
			Expect(nodeClass.Hash()).To(Equal(hash))
		},
		Entry("Base EC2NodeClass", staticHash, v1.EC2NodeClass{}),
		// Static fields, expect changed hash from base

		Entry("UserData", "10726797622220086701", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
		Entry("Tags", "13171706991797150099", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
		Entry("Context", "2889419402034926781", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
		Entry("DetailedMonitoring", "3268207623472999947", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
		Entry("InstanceStorePolicy", "14988327748406934174", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{InstanceStorePolicy: lo.ToPtr(v1.InstanceStorePolicyRAID0)}}),
		Entry("AssociatePublicIPAddress", "15544493229975292346", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{AssociatePublicIPAddress: lo.ToPtr(true)}}),
		Entry("MetadataOptions HTTPEndpoint", "17871731458970668774", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPEndpoint: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPProtocolIPv6", "2470633037813609088", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPProtocolIPv6: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPPutResponseHopLimit", "17675179355294901357", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPPutResponseHopLimit: lo.ToPtr(int64(10))}}}),
		Entry("MetadataOptions HTTPTokens", "2669105690505918645", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPTokens: lo.ToPtr("required")}}}),
		Entry("BlockDeviceMapping DeviceName", "5415148539492750790", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{DeviceName: lo.ToPtr("map-device-test-3")}}}}),
		Entry("BlockDeviceMapping RootVolume", "5518942571120617117", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{RootVolume: true}}}}),
		Entry("BlockDeviceMapping DeleteOnTermination", "2581652380077676602", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{DeleteOnTermination: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping Encrypted", "9177809208865678872", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{Encrypted: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping IOPS", "10810952692173241494", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{IOPS: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping KMSKeyID", "2530423540851551058", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{KMSKeyID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping SnapshotID", "9712886753345517947", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{SnapshotID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping Throughput", "3334301275630239638", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{Throughput: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping VolumeType", "7651488174200364927", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{VolumeType: lo.ToPtr("io1")}}}}}),

		// Behavior / Dynamic fields, expect same hash as base
		Entry("Modified AMISelector", staticHash, v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{AMISelectorTerms: []v1.AMISelectorTerm{{Tags: map[string]string{"ami-test-key": "ami-test-value"}}}}}),
		Entry("Modified SubnetSelector", staticHash, v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{SubnetSelectorTerms: []v1.SubnetSelectorTerm{{Tags: map[string]string{"subnet-test-key": "subnet-test-value"}}}}}),
		Entry("Modified SecurityGroupSelector", staticHash, v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"security-group-test-key": "security-group-test-value"}}}}}),
	)
	// We create a separate test for updating blockDeviceMapping volumeSize, since resource.Quantity is a struct, and mergo.WithSliceDeepCopy
	// doesn't work well with unexported fields, like the ones that are present in resource.Quantity
	It("should match static hash when updating blockDeviceMapping volumeSize", func() {
		nodeClass.Spec.BlockDeviceMappings[0].EBS.VolumeSize = resource.NewScaledQuantity(10, resource.Giga)
		Expect(nodeClass.Hash()).To(Equal("13279394903563315705"))
	})
	It("should match static hash for instanceProfile", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		Expect(nodeClass.Hash()).To(Equal("13329599275048356421"))
	})
	It("should match static hash when reordering tags", func() {
		nodeClass.Spec.Tags = map[string]string{"keyTag-2": "valueTag-2", "keyTag-1": "valueTag-1"}
		Expect(nodeClass.Hash()).To(Equal(staticHash))
	})
	It("should match static hash when reordering blockDeviceMappings", func() {
		nodeClass.Spec.BlockDeviceMappings[0], nodeClass.Spec.BlockDeviceMappings[1] = nodeClass.Spec.BlockDeviceMappings[1], nodeClass.Spec.BlockDeviceMappings[0]
		Expect(nodeClass.Hash()).To(Equal(staticHash))
	})
	DescribeTable("should change hash when static fields are updated", func(changes v1.EC2NodeClass) {
		hash := nodeClass.Hash()
		Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride, mergo.WithSliceDeepCopy)).To(Succeed())
		updatedHash := nodeClass.Hash()
		Expect(hash).ToNot(Equal(updatedHash))
	},
		Entry("UserData", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
		Entry("Tags", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
		Entry("Context", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
		Entry("DetailedMonitoring", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
		Entry("InstanceStorePolicy", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{InstanceStorePolicy: lo.ToPtr(v1.InstanceStorePolicyRAID0)}}),
		Entry("AssociatePublicIPAddress", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{AssociatePublicIPAddress: lo.ToPtr(true)}}),
		Entry("MetadataOptions HTTPEndpoint", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPEndpoint: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPProtocolIPv6", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPProtocolIPv6: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPPutResponseHopLimit", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPPutResponseHopLimit: lo.ToPtr(int64(10))}}}),
		Entry("MetadataOptions HTTPTokens", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPTokens: lo.ToPtr("required")}}}),
		Entry("BlockDeviceMapping DeviceName", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{DeviceName: lo.ToPtr("map-device-test-3")}}}}),
		Entry("BlockDeviceMapping RootVolume", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{RootVolume: true}}}}),
		Entry("BlockDeviceMapping DeleteOnTermination", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{DeleteOnTermination: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping Encrypted", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{Encrypted: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping IOPS", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{IOPS: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping KMSKeyID", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{KMSKeyID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping SnapshotID", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{SnapshotID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping Throughput", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{Throughput: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping VolumeType", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{VolumeType: lo.ToPtr("io1")}}}}}),
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
		nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{
			{
				Tags: map[string]string{"subnet-test-key": "subnet-test-value"},
			},
		}
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{
			{
				Tags: map[string]string{"sg-test-key": "sg-test-value"},
			},
		}
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{
			{
				Tags: map[string]string{"ami-test-key": "ami-test-value"},
			},
		}
		updatedHash := nodeClass.Hash()
		Expect(hash).To(Equal(updatedHash))
	})
	It("should expect two EC2NodeClasses with the same spec to have the same hash", func() {
		otherNodeClass := &v1.EC2NodeClass{
			Spec: nodeClass.Spec,
		}
		Expect(nodeClass.Hash()).To(Equal(otherNodeClass.Hash()))
	})
})

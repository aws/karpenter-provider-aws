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
	"time"

	"github.com/imdario/mergo"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go-v2/aws"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hash", func() {
	const staticHash = "4950366118253097694"
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

		Entry("UserData", "9034828637236670345", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
		Entry("Tags", "6878220270322275255", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
		Entry("Context", "13953931752662869657", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
		Entry("DetailedMonitoring", "14187487647319890991", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
		Entry("InstanceStorePolicy", "4160809219257698490", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{InstanceStorePolicy: lo.ToPtr(v1.InstanceStorePolicyRAID0)}}),
		Entry("AssociatePublicIPAddress", "4469320567057431454", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{AssociatePublicIPAddress: lo.ToPtr(true)}}),
		Entry("MetadataOptions HTTPEndpoint", "1277386558528601282", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPEndpoint: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPProtocolIPv6", "14697047633165484196", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPProtocolIPv6: lo.ToPtr("enabled")}}}),
		Entry("MetadataOptions HTTPPutResponseHopLimit", "2086799014304536137", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPPutResponseHopLimit: lo.ToPtr(int64(10))}}}),
		Entry("MetadataOptions HTTPTokens", "14750841460622248593", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{MetadataOptions: &v1.MetadataOptions{HTTPTokens: lo.ToPtr("required")}}}),
		Entry("BlockDeviceMapping DeviceName", "11716516558705174498", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{DeviceName: lo.ToPtr("map-device-test-3")}}}}),
		Entry("BlockDeviceMapping RootVolume", "11900810786014401721", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{RootVolume: true}}}}),
		Entry("BlockDeviceMapping DeleteOnTermination", "14586255897156659742", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{DeleteOnTermination: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping Encrypted", "10872029821841773628", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{Encrypted: lo.ToPtr(true)}}}}}),
		Entry("BlockDeviceMapping IOPS", "9202874311950700210", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{IOPS: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping KMSKeyID", "14601456769467439478", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{KMSKeyID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping SnapshotID", "8031059801598053215", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{SnapshotID: lo.ToPtr("test")}}}}}),
		Entry("BlockDeviceMapping Throughput", "14410045481146650034", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{Throughput: lo.ToPtr(int64(10))}}}}}),
		Entry("BlockDeviceMapping VolumeType", "9480251663542054235", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{BlockDeviceMappings: []*v1.BlockDeviceMapping{{EBS: &v1.BlockDevice{VolumeType: lo.ToPtr("io1")}}}}}),
		Entry("ConnectionTracking TCPEstablishedTimeout", "13769259698110563446", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{ConnectionTracking: &v1.ConnectionTracking{TCPEstablishedTimeout: &metav1.Duration{Duration: 300 * time.Second}}}}),
		Entry("ConnectionTracking UDPStreamTimeout", "5121672282941937708", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{ConnectionTracking: &v1.ConnectionTracking{UDPStreamTimeout: &metav1.Duration{Duration: 120 * time.Second}}}}),
		Entry("ConnectionTracking UDPTimeout", "10301776211393399673", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{ConnectionTracking: &v1.ConnectionTracking{UDPTimeout: &metav1.Duration{Duration: 45 * time.Second}}}}),

		// Behavior / Dynamic fields, expect same hash as base
		Entry("Modified AMISelector", staticHash, v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{AMISelectorTerms: []v1.AMISelectorTerm{{Tags: map[string]string{"": "ami-test-value"}}}}}),
		Entry("Modified SubnetSelector", staticHash, v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{SubnetSelectorTerms: []v1.SubnetSelectorTerm{{Tags: map[string]string{"subnet-test-key": "subnet-test-value"}}}}}),
		Entry("Modified SecurityGroupSelector", staticHash, v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{SecurityGroupSelectorTerms: []v1.SecurityGroupSelectorTerm{{Tags: map[string]string{"security-group-test-key": "security-group-test-value"}}}}}),
	)
	// We create a separate test for updating blockDeviceMapping volumeSize, since resource.Quantity is a struct, and mergo.WithSliceDeepCopy
	// doesn't work well with unexported fields, like the ones that are present in resource.Quantity
	It("should match static hash when updating blockDeviceMapping volumeSize", func() {
		nodeClass.Spec.BlockDeviceMappings[0].EBS.VolumeSize = resource.NewScaledQuantity(10, resource.Giga)
		Expect(nodeClass.Hash()).To(Equal("5906178522470964189"))
	})
	It("should match static hash for instanceProfile", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		Expect(nodeClass.Hash()).To(Equal("5855570904022890593"))
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
		Entry("ConnectionTracking TCPEstablishedTimeout", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{ConnectionTracking: &v1.ConnectionTracking{TCPEstablishedTimeout: &metav1.Duration{Duration: 300 * time.Second}}}}),
		Entry("ConnectionTracking UDPStreamTimeout", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{ConnectionTracking: &v1.ConnectionTracking{UDPStreamTimeout: &metav1.Duration{Duration: 120 * time.Second}}}}),
		Entry("ConnectionTracking UDPTimeout", v1.EC2NodeClass{Spec: v1.EC2NodeClassSpec{ConnectionTracking: &v1.ConnectionTracking{UDPTimeout: &metav1.Duration{Duration: 45 * time.Second}}}}),
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
		nodeClass.Spec.SubnetSelectorTerms = []v1.SubnetSelectorTerm{{
			Tags: map[string]string{"subnet-test-key": "subnet-test-value"},
		}}
		nodeClass.Spec.SecurityGroupSelectorTerms = []v1.SecurityGroupSelectorTerm{{
			Tags: map[string]string{"sg-test-key": "sg-test-value"},
		}}
		nodeClass.Spec.AMISelectorTerms = []v1.AMISelectorTerm{{
			Tags: map[string]string{"ami-test-key": "ami-test-value"},
		}}
		nodeClass.Spec.CapacityReservationSelectorTerms = []v1.CapacityReservationSelectorTerm{{
			Tags: map[string]string{"cr-test-key": "cr-test-value"},
		}}
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

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/test"
)

var _ = Describe("Hash", func() {
	const staticHash = "16608948681250225098"
	var nodeClass *v1beta1.EC2NodeClass
	BeforeEach(func() {
		nodeClass = test.EC2NodeClass(v1beta1.EC2NodeClass{
			Spec: v1beta1.EC2NodeClassSpec{
				AMIFamily: aws.String(v1alpha1.AMIFamilyAL2),
				Context:   aws.String("context-1"),
				Role:      "role-1",
				Tags: map[string]string{
					"keyTag-1": "valueTag-1",
					"keyTag-2": "valueTag-2",
				},
				MetadataOptions: &v1beta1.MetadataOptions{
					HTTPEndpoint: aws.String("test-metadata-1"),
				},
				BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{
					{
						DeviceName: aws.String("map-device-1"),
					},
					{
						DeviceName: aws.String("map-device-2"),
					},
				},
				UserData:           aws.String("userdata-test-1"),
				DetailedMonitoring: aws.Bool(false),
			},
		})
	})
	DescribeTable(
		"should match static hash",
		func(hash string, changes ...v1beta1.EC2NodeClass) {
			modifiedNodeClass := test.EC2NodeClass(append([]v1beta1.EC2NodeClass{*nodeClass}, changes...)...)
			Expect(modifiedNodeClass.Hash()).To(Equal(hash))
		},
		Entry("Base EC2NodeClass", staticHash),
		// Static fields, expect changed hash from base
		Entry("UserData Drift", "588756456110800812", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
		Entry("Tags Drift", "2471764681523766508", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
		Entry("MetadataOptions Drift", "11030161632375731908", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPEndpoint: aws.String("test-metadata-2")}}}),
		Entry("BlockDeviceMappings Drift", "436753305915039702", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{DeviceName: aws.String("map-device-test-3")}}}}),
		Entry("Context Drift", "3729470655588343019", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
		Entry("DetailedMonitoring Drift", "17892305444040067573", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
		Entry("AMIFamily Drift", "9493798894326942407", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMIFamily: aws.String(v1alpha1.AMIFamilyBottlerocket)}}),
		Entry("Reorder Tags", staticHash, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-2": "valueTag-2", "keyTag-1": "valueTag-1"}}}),
		Entry("Reorder BlockDeviceMapping", staticHash, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{DeviceName: aws.String("map-device-2")}, {DeviceName: aws.String("map-device-1")}}}}),

		// Behavior / Dynamic fields, expect same hash as base
		Entry("Modified AMISelector", staticHash, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMISelectorTerms: []v1beta1.AMISelectorTerm{{Tags: map[string]string{"ami-test-key": "ami-test-value"}}}}}),
		Entry("Modified SubnetSelector", staticHash, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{SubnetSelectorTerms: []v1beta1.SubnetSelectorTerm{{Tags: map[string]string{"subnet-test-key": "subnet-test-value"}}}}}),
		Entry("Modified SecurityGroupSelector", staticHash, v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{SecurityGroupSelectorTerms: []v1beta1.SecurityGroupSelectorTerm{{Tags: map[string]string{"security-group-test-key": "security-group-test-value"}}}}}),
	)
	It("should match static hash for instanceProfile", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		Expect(nodeClass.Hash()).To(Equal("15756064858220068103"))
	})
	DescribeTable("should change hash when static fields are updated", func(changes v1beta1.EC2NodeClass) {
		hash := nodeClass.Hash()
		Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride)).To(Succeed())
		updatedHash := nodeClass.Hash()
		Expect(hash).ToNot(Equal(updatedHash))
	},
		Entry("UserData Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{UserData: aws.String("userdata-test-2")}}),
		Entry("Tags Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-test-3": "valueTag-test-3"}}}),
		Entry("MetadataOptions Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{MetadataOptions: &v1beta1.MetadataOptions{HTTPEndpoint: aws.String("test-metadata-2")}}}),
		Entry("BlockDeviceMappings Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{DeviceName: aws.String("map-device-test-3")}}}}),
		Entry("Context Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Context: aws.String("context-2")}}),
		Entry("DetailedMonitoring Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{DetailedMonitoring: aws.Bool(true)}}),
		Entry("AMIFamily Drift", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{AMIFamily: aws.String(v1alpha1.AMIFamilyBottlerocket)}}),
	)
	It("should change hash when instanceProfile is updated", func() {
		nodeClass.Spec.Role = ""
		nodeClass.Spec.InstanceProfile = lo.ToPtr("test-instance-profile")
		hash := nodeClass.Hash()
		nodeClass.Spec.InstanceProfile = lo.ToPtr("other-instance-profile")
		updatedHash := nodeClass.Hash()
		Expect(hash).ToNot(Equal(updatedHash))
	})
	DescribeTable("should not change hash when slices are re-ordered", func(changes v1beta1.EC2NodeClass) {
		hash := nodeClass.Hash()
		Expect(mergo.Merge(nodeClass, changes, mergo.WithOverride)).To(Succeed())
		updatedHash := nodeClass.Hash()
		Expect(hash).To(Equal(updatedHash))
	},
		Entry("Reorder Tags", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{Tags: map[string]string{"keyTag-2": "valueTag-2", "keyTag-1": "valueTag-1"}}}),
		Entry("Reorder BlockDeviceMapping", v1beta1.EC2NodeClass{Spec: v1beta1.EC2NodeClassSpec{BlockDeviceMappings: []*v1beta1.BlockDeviceMapping{{DeviceName: aws.String("map-device-2")}, {DeviceName: aws.String("map-device-1")}}}}),
	)
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
	It("should expect two EC2NodeClasses with the same spec to have the same provisioner hash", func() {
		otherNodeClass := test.EC2NodeClass(v1beta1.EC2NodeClass{
			Spec: nodeClass.Spec,
		})
		Expect(nodeClass.Hash()).To(Equal(otherNodeClass.Hash()))
	})
})

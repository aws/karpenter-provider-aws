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

package nodeclass_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter-core/pkg/operator/scheme"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	. "github.com/aws/karpenter/pkg/test/expectations"

	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/test"
	nodeclassutil "github.com/aws/karpenter/pkg/utils/nodeclass"
	nodetemplateutil "github.com/aws/karpenter/pkg/utils/nodetemplate"
)

func init() {
	lo.Must0(apis.AddToScheme(scheme.Scheme))
}

var ctx context.Context
var env *coretest.Environment

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeClaimUtils")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeClassUtils", func() {
	var nodeTemplate *v1alpha1.AWSNodeTemplate
	BeforeEach(func() {
		nodeTemplate = test.AWSNodeTemplate(v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily:       aws.String(v1alpha1.AMIFamilyAL2),
				Context:         aws.String("context-1"),
				InstanceProfile: aws.String("profile-1"),
				Tags: map[string]string{
					"keyTag-1": "valueTag-1",
					"keyTag-2": "valueTag-2",
				},
				SubnetSelector: map[string]string{
					"test-subnet-key": "test-subnet-value",
				},
				SecurityGroupSelector: map[string]string{
					"test-security-group-key": "test-security-group-value",
				},
				LaunchTemplate: v1alpha1.LaunchTemplate{
					MetadataOptions: &v1alpha1.MetadataOptions{
						HTTPEndpoint: aws.String("test-metadata-1"),
					},
					BlockDeviceMappings: []*v1alpha1.BlockDeviceMapping{
						{
							DeviceName: aws.String("map-device-1"),
						},
						{
							DeviceName: aws.String("map-device-2"),
						},
					},
				},
			},
			UserData:           aws.String("userdata-test-1"),
			DetailedMonitoring: aws.Bool(false),
			AMISelector: map[string]string{
				"test-ami-key": "test-ami-value",
			},
		})
		nodeTemplate.Status = v1alpha1.AWSNodeTemplateStatus{
			Subnets: []v1alpha1.Subnet{
				{
					ID:   "test-subnet-id",
					Zone: "test-zone-1a",
				},
				{
					ID:   "test-subnet-id2",
					Zone: "test-zone-1b",
				},
			},
			SecurityGroups: []v1alpha1.SecurityGroup{
				{
					ID:   "test-security-group-id",
					Name: "test-security-group-name",
				},
				{
					ID:   "test-security-group-id2",
					Name: "test-security-group-name2",
				},
			},
			AMIs: []v1alpha1.AMI{
				{
					ID:   "test-ami-id",
					Name: "test-ami-name",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"amd64"},
						},
					},
				},
				{
					ID:   "test-ami-id2",
					Name: "test-ami-name2",
					Requirements: []v1.NodeSelectorRequirement{
						{
							Key:      v1.LabelArchStable,
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{"arm64"},
						},
					},
				},
			},
		}
	})
	It("should convert a AWSNodeTemplate to a EC2NodeClass", func() {
		nodeClass := nodeclassutil.New(nodeTemplate)

		for k, v := range nodeTemplate.Annotations {
			Expect(nodeClass.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeTemplate.Labels {
			Expect(nodeClass.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeClass.Spec.SubnetSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SubnetSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))
		Expect(nodeClass.Spec.AMISelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.AMISelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.AMISelector))
		Expect(nodeClass.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(nodeClass.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(nodeClass.Spec.Role).To(BeEmpty())
		Expect(nodeClass.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		Expect(nodeClass.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		Expect(nodeClass.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(nodeClass.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(nodeClass.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))

		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
	It("should convert a AWSNodeTemplate to a EC2NodeClass (with AMISelector name and owner values set)", func() {
		nodeTemplate.Spec.AMISelector = map[string]string{
			"aws::name":   "ami-name1,ami-name2",
			"aws::owners": "self,amazon,123456789",
		}
		nodeClass := nodeclassutil.New(nodeTemplate)

		for k, v := range nodeTemplate.Annotations {
			Expect(nodeClass.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeTemplate.Labels {
			Expect(nodeClass.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeClass.Spec.SubnetSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SubnetSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))

		// Expect AMISelectorTerms to be exactly what we would expect from the filtering above
		Expect(nodeClass.Spec.AMISelectorTerms).To(HaveLen(6))
		Expect(nodeClass.Spec.AMISelectorTerms).To(ConsistOf(
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "self",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "amazon",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "123456789",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "self",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "amazon",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "123456789",
				Tags:  map[string]string{},
			},
		))

		Expect(nodeClass.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(nodeClass.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(nodeClass.Spec.Role).To(BeEmpty())
		Expect(nodeClass.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		Expect(nodeClass.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		Expect(nodeClass.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(nodeClass.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(nodeClass.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))

		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
	It("should convert a AWSNodeTemplate to a EC2NodeClass (with AMISelector name and owner values set) with spaces", func() {
		nodeTemplate.Spec.AMISelector = map[string]string{
			"aws::name":   "ami-name1, ami-name2, test name",
			"aws::owners": "self, amazon, 123456789, test owner",
		}
		nodeClass := nodeclassutil.New(nodeTemplate)

		for k, v := range nodeTemplate.Annotations {
			Expect(nodeClass.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeTemplate.Labels {
			Expect(nodeClass.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeClass.Spec.SubnetSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SubnetSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))

		// Expect AMISelectorTerms to be exactly what we would expect from the filtering above
		Expect(nodeClass.Spec.AMISelectorTerms).To(HaveLen(12))
		Expect(nodeClass.Spec.AMISelectorTerms).To(ConsistOf(
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "self",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "amazon",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "123456789",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "test owner",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "self",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "amazon",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "123456789",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "test owner",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "test name",
				Owner: "self",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "test name",
				Owner: "amazon",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "test name",
				Owner: "123456789",
				Tags:  map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				Name:  "test name",
				Owner: "test owner",
				Tags:  map[string]string{},
			},
		))

		Expect(nodeClass.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(nodeClass.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(nodeClass.Spec.Role).To(BeEmpty())
		Expect(nodeClass.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		Expect(nodeClass.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		Expect(nodeClass.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(nodeClass.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(nodeClass.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))

		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
	It("should convert a AWSNodeTemplate to a EC2NodeClass (with AMISelector id set)", func() {
		nodeTemplate.Spec.AMISelector = map[string]string{
			"aws::ids": "ami-1234,ami-5678,ami-custom-id",
		}
		nodeClass := nodeclassutil.New(nodeTemplate)

		for k, v := range nodeTemplate.Annotations {
			Expect(nodeClass.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeTemplate.Labels {
			Expect(nodeClass.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeClass.Spec.SubnetSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SubnetSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))

		// Expect AMISelectorTerms to be exactly what we would expect from the filtering above
		Expect(nodeClass.Spec.AMISelectorTerms).To(HaveLen(3))
		Expect(nodeClass.Spec.AMISelectorTerms).To(ConsistOf(
			v1beta1.AMISelectorTerm{
				ID:   "ami-1234",
				Tags: map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				ID:   "ami-5678",
				Tags: map[string]string{},
			},
			v1beta1.AMISelectorTerm{
				ID:   "ami-custom-id",
				Tags: map[string]string{},
			},
		))

		Expect(nodeClass.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(nodeClass.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(nodeClass.Spec.Role).To(BeEmpty())
		Expect(nodeClass.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		Expect(nodeClass.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		Expect(nodeClass.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(nodeClass.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(nodeClass.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))

		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
	It("should convert a AWSNodeTemplate to a EC2NodeClass (with AMISelector name, owner, id, and tags set)", func() {
		nodeTemplate.Spec.AMISelector = map[string]string{
			"aws::name":   "ami-name1,ami-name2",
			"aws::owners": "self,amazon",
			"aws::ids":    "ami-1234,ami-5678",
			"custom-tag":  "custom-value",
			"custom-tag2": "custom-value2",
		}
		nodeClass := nodeclassutil.New(nodeTemplate)

		for k, v := range nodeTemplate.Annotations {
			Expect(nodeClass.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeTemplate.Labels {
			Expect(nodeClass.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeClass.Spec.SubnetSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SubnetSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms).To(HaveLen(1))
		Expect(nodeClass.Spec.SecurityGroupSelectorTerms[0].Tags).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))

		// Expect AMISelectorTerms to be exactly what we would expect from the filtering above
		// This should include all permutations of the filters that could be used by this selector mechanism
		Expect(nodeClass.Spec.AMISelectorTerms).To(HaveLen(8))
		Expect(nodeClass.Spec.AMISelectorTerms).To(ConsistOf(
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "self",
				ID:    "ami-1234",
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "self",
				ID:    "ami-5678",
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "amazon",
				ID:    "ami-1234",
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name1",
				Owner: "amazon",
				ID:    "ami-5678",
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "self",
				ID:    "ami-1234",
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "self",
				ID:    "ami-5678",
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "amazon",
				ID:    "ami-1234",
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
			v1beta1.AMISelectorTerm{
				Name:  "ami-name2",
				Owner: "amazon",
				ID:    "ami-5678",
				Tags: map[string]string{
					"custom-tag":  "custom-value",
					"custom-tag2": "custom-value2",
				},
			},
		))

		Expect(nodeClass.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(nodeClass.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(nodeClass.Spec.Role).To(BeEmpty())
		Expect(nodeClass.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		Expect(nodeClass.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		Expect(nodeClass.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(nodeClass.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(nodeClass.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))

		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
	It("should convert a AWSNodeTemplate to a EC2NodeClass and back and still retain all original data", func() {
		convertedNodeTemplate := nodetemplateutil.New(nodeclassutil.New(nodeTemplate))

		Expect(convertedNodeTemplate.Name).To(Equal(nodeTemplate.Name))
		Expect(convertedNodeTemplate.Annotations).To(Equal(nodeTemplate.Annotations))
		Expect(convertedNodeTemplate.Labels).To(Equal(nodeTemplate.Labels))

		Expect(convertedNodeTemplate.Spec.UserData).To(Equal(nodeTemplate.Spec.UserData))
		Expect(convertedNodeTemplate.Spec.AMISelector).To(Equal(nodeTemplate.Spec.AMISelector))
		Expect(convertedNodeTemplate.Spec.DetailedMonitoring).To(Equal(nodeTemplate.Spec.DetailedMonitoring))
		Expect(convertedNodeTemplate.Spec.AMIFamily).To(Equal(nodeTemplate.Spec.AMIFamily))
		Expect(convertedNodeTemplate.Spec.Context).To(Equal(nodeTemplate.Spec.Context))
		Expect(convertedNodeTemplate.Spec.InstanceProfile).To(Equal(nodeTemplate.Spec.InstanceProfile))
		Expect(convertedNodeTemplate.Spec.SubnetSelector).To(Equal(nodeTemplate.Spec.SubnetSelector))
		Expect(convertedNodeTemplate.Spec.SecurityGroupSelector).To(Equal(nodeTemplate.Spec.SecurityGroupSelector))
		Expect(convertedNodeTemplate.Spec.Tags).To(Equal(nodeTemplate.Spec.Tags))
		Expect(convertedNodeTemplate.Spec.LaunchTemplateName).To(Equal(nodeTemplate.Spec.LaunchTemplateName))
		Expect(convertedNodeTemplate.Spec.MetadataOptions).To(Equal(nodeTemplate.Spec.MetadataOptions))
		Expect(convertedNodeTemplate.Spec.BlockDeviceMappings).To(Equal(nodeTemplate.Spec.BlockDeviceMappings))

		Expect(convertedNodeTemplate.Status.SecurityGroups).To(Equal(nodeTemplate.Status.SecurityGroups))
		Expect(convertedNodeTemplate.Status.Subnets).To(Equal(nodeTemplate.Status.Subnets))
		Expect(convertedNodeTemplate.Status.AMIs).To(Equal(nodeTemplate.Status.AMIs))
	})
	It("should retrieve a EC2NodeClass with a get call", func() {
		nodeClass := test.EC2NodeClass()
		ExpectApplied(ctx, env.Client, nodeClass)

		retrieved, err := nodeclassutil.Get(ctx, env.Client, nodeclassutil.Key{Name: nodeClass.Name, IsNodeTemplate: false})
		Expect(err).ToNot(HaveOccurred())
		Expect(retrieved.Name).To(Equal(nodeClass.Name))
	})
	It("should retrieve a AWSNodeTemplate with a get call", func() {
		nodeTemplate := test.AWSNodeTemplate()
		ExpectApplied(ctx, env.Client, nodeTemplate)

		retrieved, err := nodeclassutil.Get(ctx, env.Client, nodeclassutil.Key{Name: nodeTemplate.Name, IsNodeTemplate: true})
		Expect(err).ToNot(HaveOccurred())
		Expect(retrieved.Name).To(Equal(nodeTemplate.Name))
	})
})

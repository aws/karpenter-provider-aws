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

package nodetemplate_test

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
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/apis/v1beta1"
	"github.com/aws/karpenter/pkg/test"
	. "github.com/aws/karpenter/pkg/test/expectations"
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

var _ = Describe("NodeTemplateUtils", func() {
	var nodeClass *v1beta1.NodeClass
	BeforeEach(func() {
		nodeClass = test.NodeClass(v1beta1.NodeClass{
			Spec: v1beta1.NodeClassSpec{
				AMIFamily:       aws.String(v1alpha1.AMIFamilyAL2),
				Context:         aws.String("context-1"),
				InstanceProfile: aws.String("profile-1"),
				Tags: map[string]string{
					"keyTag-1": "valueTag-1",
					"keyTag-2": "valueTag-2",
				},
				OriginalSubnetSelector: map[string]string{
					"test-subnet-key": "test-subnet-value",
				},
				OriginalSecurityGroupSelector: map[string]string{
					"test-security-group-key": "test-security-group-value",
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
				OriginalAMISelector: map[string]string{
					"test-ami-key": "test-ami-value",
				},
			},
		})
		nodeClass.Status = v1beta1.NodeClassStatus{
			Subnets: []v1beta1.Subnet{
				{
					ID:   "test-subnet-id",
					Zone: "test-zone-1a",
				},
				{
					ID:   "test-subnet-id2",
					Zone: "test-zone-1b",
				},
			},
			SecurityGroups: []v1beta1.SecurityGroup{
				{
					ID:   "test-security-group-id",
					Name: "test-security-group-name",
				},
				{
					ID:   "test-security-group-id2",
					Name: "test-security-group-name2",
				},
			},
			AMIs: []v1beta1.AMI{
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
	It("should convert a NodeClass to an AWSNodeTemplate", func() {
		nodeTemplate := nodetemplateutil.New(nodeClass)

		for k, v := range nodeClass.Annotations {
			Expect(nodeTemplate.Annotations).To(HaveKeyWithValue(k, v))
		}
		for k, v := range nodeClass.Labels {
			Expect(nodeTemplate.Labels).To(HaveKeyWithValue(k, v))
		}
		Expect(nodeTemplate.Spec.SubnetSelector).To(Equal(nodeClass.Spec.OriginalSubnetSelector))
		Expect(nodeTemplate.Spec.SecurityGroupSelector).To(Equal(nodeClass.Spec.OriginalSecurityGroupSelector))
		Expect(nodeTemplate.Spec.AMISelector).To(Equal(nodeClass.Spec.OriginalAMISelector))
		Expect(nodeTemplate.Spec.AMIFamily).To(Equal(nodeClass.Spec.AMIFamily))
		Expect(nodeTemplate.Spec.Context).To(Equal(nodeClass.Spec.Context))
		Expect(nodeTemplate.Spec.InstanceProfile).To(Equal(nodeClass.Spec.InstanceProfile))
		Expect(nodeTemplate.Spec.UserData).To(Equal(nodeClass.Spec.UserData))
		Expect(nodeTemplate.Spec.Tags).To(Equal(nodeClass.Spec.Tags))
		Expect(nodeTemplate.Spec.DetailedMonitoring).To(Equal(nodeClass.Spec.DetailedMonitoring))
		Expect(nodeTemplate.Spec.LaunchTemplateName).To(Equal(nodeClass.Spec.LaunchTemplateName))

		ExpectBlockDeviceMappingsEqual(nodeTemplate.Spec.BlockDeviceMappings, nodeClass.Spec.BlockDeviceMappings)
		ExpectMetadataOptionsEqual(nodeTemplate.Spec.MetadataOptions, nodeClass.Spec.MetadataOptions)
		ExpectSubnetStatusEqual(nodeTemplate.Status.Subnets, nodeClass.Status.Subnets)
		ExpectSecurityGroupStatusEqual(nodeTemplate.Status.SecurityGroups, nodeClass.Status.SecurityGroups)
		ExpectAMIStatusEqual(nodeTemplate.Status.AMIs, nodeClass.Status.AMIs)
	})
})

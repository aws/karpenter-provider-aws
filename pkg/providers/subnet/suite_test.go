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

package subnet_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-sdk-go/aws"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/test"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	corev1alpha5 "github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
)

var ctx context.Context
var stop context.CancelFunc
var opts options.Options
var env *coretest.Environment
var awsEnv *test.Environment
var provisioner *corev1alpha5.Provisioner
var nodeTemplate *v1alpha1.AWSNodeTemplate

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = injection.WithOptions(ctx, opts)
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	nodeTemplate = &v1alpha1.AWSNodeTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: coretest.RandomName(),
		},
		Spec: v1alpha1.AWSNodeTemplateSpec{
			AWS: v1alpha1.AWS{
				AMIFamily:             aws.String(v1alpha1.AMIFamilyAL2),
				SubnetSelector:        map[string]string{"*": "*"},
				SecurityGroupSelector: map[string]string{"*": "*"},
			},
		},
	}
	nodeTemplate.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1alpha1.SchemeGroupVersion.Group,
		Version: v1alpha1.SchemeGroupVersion.Version,
		Kind:    "AWSNodeTemplate",
	})
	provisioner = test.Provisioner(coretest.ProvisionerOptions{
		Requirements: []v1.NodeSelectorRequirement{{
			Key:      v1alpha1.LabelInstanceCategory,
			Operator: v1.NodeSelectorOpExists,
		}},
		ProviderRef: &corev1alpha5.MachineTemplateRef{
			APIVersion: nodeTemplate.APIVersion,
			Kind:       nodeTemplate.Kind,
			Name:       nodeTemplate.Name,
		},
	})

	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Subnet Provider", func() {
	It("should discover subnet by ID", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		subnets, err := awsEnv.SubnetProvider.List(ctx, nodeTemplate)

		Expect(err).To(BeNil())
		Expect(subnets).To(HaveLen(1))
		Expect(aws.StringValue(subnets[0].SubnetId)).To(Equal("subnet-test1"))
		Expect(aws.StringValue(subnets[0].AvailabilityZone)).To(Equal("test-zone-1a"))
		Expect(aws.Int64Value(subnets[0].AvailableIpAddressCount)).To(BeNumerically("==", 100))
	})
	It("should discover subnets by IDs", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1,subnet-test2"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		subnets, err := awsEnv.SubnetProvider.List(ctx, nodeTemplate)
		Expect(err).To(BeNil())
		Expect(subnets).To(HaveLen(2))
		Expect(aws.StringValue(subnets[0].SubnetId)).To(Equal("subnet-test1"))
		Expect(aws.StringValue(subnets[0].AvailabilityZone)).To(Equal("test-zone-1a"))
		Expect(aws.Int64Value(subnets[0].AvailableIpAddressCount)).To(BeNumerically("==", 100))
		Expect(aws.StringValue(subnets[1].SubnetId)).To(Equal("subnet-test2"))
		Expect(aws.StringValue(subnets[1].AvailabilityZone)).To(Equal("test-zone-1b"))
		Expect(aws.Int64Value(subnets[1].AvailableIpAddressCount)).To(BeNumerically("==", 100))
	})
	It("should discover subnets by IDs and tags", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1,subnet-test2", "foo": "bar"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		subnets, err := awsEnv.SubnetProvider.List(ctx, nodeTemplate)

		Expect(err).To(BeNil())
		Expect(subnets).To(HaveLen(2))
		Expect(aws.StringValue(subnets[0].SubnetId)).To(Equal("subnet-test1"))
		Expect(aws.StringValue(subnets[0].AvailabilityZone)).To(Equal("test-zone-1a"))
		Expect(aws.Int64Value(subnets[0].AvailableIpAddressCount)).To(BeNumerically("==", 100))
		Expect(aws.StringValue(subnets[1].SubnetId)).To(Equal("subnet-test2"))
		Expect(aws.StringValue(subnets[1].AvailabilityZone)).To(Equal("test-zone-1b"))
		Expect(aws.Int64Value(subnets[1].AvailableIpAddressCount)).To(BeNumerically("==", 100))
	})
	It("should discover subnets by a single tag", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"Name": "test-subnet-1"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		subnets, err := awsEnv.SubnetProvider.List(ctx, nodeTemplate)

		Expect(err).To(BeNil())
		Expect(subnets).To(HaveLen(1))
		Expect(aws.StringValue(subnets[0].SubnetId)).To(Equal("subnet-test1"))
		Expect(aws.StringValue(subnets[0].AvailabilityZone)).To(Equal("test-zone-1a"))
		Expect(aws.Int64Value(subnets[0].AvailableIpAddressCount)).To(BeNumerically("==", 100))
	})
	It("should discover subnets by multiple tag values", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"Name": "test-subnet-1,test-subnet-2"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		subnets, err := awsEnv.SubnetProvider.List(ctx, nodeTemplate)

		Expect(err).To(BeNil())
		Expect(subnets).To(HaveLen(2))
		Expect(aws.StringValue(subnets[0].SubnetId)).To(Equal("subnet-test1"))
		Expect(aws.StringValue(subnets[0].AvailabilityZone)).To(Equal("test-zone-1a"))
		Expect(aws.Int64Value(subnets[0].AvailableIpAddressCount)).To(BeNumerically("==", 100))
		Expect(aws.StringValue(subnets[1].SubnetId)).To(Equal("subnet-test2"))
		Expect(aws.StringValue(subnets[1].AvailabilityZone)).To(Equal("test-zone-1b"))
		Expect(aws.Int64Value(subnets[1].AvailableIpAddressCount)).To(BeNumerically("==", 100))
	})
	It("should discover subnets by IDs intersected with tags", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test2", "foo": "bar"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		subnets, err := awsEnv.SubnetProvider.List(ctx, nodeTemplate)

		Expect(err).To(BeNil())
		Expect(subnets).To(HaveLen(1))
		Expect(aws.StringValue(subnets[0].SubnetId)).To(Equal("subnet-test2"))
		Expect(aws.StringValue(subnets[0].AvailabilityZone)).To(Equal("test-zone-1b"))
		Expect(aws.Int64Value(subnets[0].AvailableIpAddressCount)).To(BeNumerically("==", 100))
	})
})

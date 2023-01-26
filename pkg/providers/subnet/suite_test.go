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

package subnet

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/patrickmn/go-cache"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter/pkg/apis"
	awssettings "github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/test"

	"github.com/aws/karpenter-core/pkg/apis/settings"
	corev1alpha5 "github.com/aws/karpenter-core/pkg/apis/v1alpha5"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter-core/pkg/utils/pretty"
	"github.com/aws/karpenter/pkg/fake"
)

var ctx context.Context
var stop context.CancelFunc
var opts options.Options
var env *coretest.Environment
var fakeEC2API *fake.EC2API
var provisioner *corev1alpha5.Provisioner
var nodeTemplate *v1alpha1.AWSNodeTemplate
var subnetCache *cache.Cache
var subnetProvider *Provider

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx, stop = context.WithCancel(ctx)

	fakeEC2API = &fake.EC2API{}
	subnetCache = cache.New(awscontext.CacheTTL, awscontext.CacheCleanupInterval)
	subnetProvider = &Provider{
		ec2api: fakeEC2API,
		cache:  subnetCache,
		cm:     pretty.NewChangeMonitor(),
	}
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = injection.WithOptions(ctx, opts)
	ctx = settings.ToContext(ctx, coretest.Settings())
	ctx = awssettings.ToContext(ctx, test.Settings())
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
		ProviderRef: &corev1alpha5.ProviderRef{
			APIVersion: nodeTemplate.APIVersion,
			Kind:       nodeTemplate.Kind,
			Name:       nodeTemplate.Name,
		},
	})

	fakeEC2API.Reset()
	subnetProvider.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Security Group Provider", func() {
	It("should discover subnet by ID", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		resolvedSubnetProvider, err := subnetProvider.List(ctx, nodeTemplate)
		resolvedSubnet := prettySubnets(resolvedSubnetProvider)
		Expect(err).To(BeNil())
		Expect(len(resolvedSubnet)).To(Equal(1))
		Expect(resolvedSubnet).To(ConsistOf(
			"subnet-test1 (test-zone-1a)",
		))
	})
	It("should discover subnets by IDs", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1,subnet-test2"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		resolvedSubnetProvider, err := subnetProvider.List(ctx, nodeTemplate)
		resolvedSubnet := prettySubnets(resolvedSubnetProvider)
		Expect(err).To(BeNil())
		Expect(len(resolvedSubnet)).To(Equal(2))
		Expect(resolvedSubnet).To(ConsistOf(
			"subnet-test1 (test-zone-1a)",
			"subnet-test2 (test-zone-1b)",
		))
	})
	It("should discover subnets by IDs and tags", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test1,subnet-test2", "foo": "bar"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		resolvedSubnetProvider, err := subnetProvider.List(ctx, nodeTemplate)
		resolvedSubnet := prettySubnets(resolvedSubnetProvider)
		Expect(err).To(BeNil())
		Expect(len(resolvedSubnet)).To(Equal(2))
		Expect(resolvedSubnet).To(ConsistOf(
			"subnet-test1 (test-zone-1a)",
			"subnet-test2 (test-zone-1b)",
		))
	})
	It("should discover subnets by a single tag", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"Name": "test-subnet-1"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		resolvedSubnetProvider, err := subnetProvider.List(ctx, nodeTemplate)
		resolvedSubnet := prettySubnets(resolvedSubnetProvider)
		Expect(err).To(BeNil())
		Expect(len(resolvedSubnet)).To(Equal(1))
		Expect(resolvedSubnet).To(ConsistOf(
			"subnet-test1 (test-zone-1a)",
		))
	})
	It("should discover subnets by multiple tag values", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"Name": "test-subnet-1,test-subnet-2"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		resolvedSubnetProvider, err := subnetProvider.List(ctx, nodeTemplate)
		resolvedSubnet := prettySubnets(resolvedSubnetProvider)
		Expect(err).To(BeNil())
		Expect(len(resolvedSubnet)).To(Equal(2))
		Expect(resolvedSubnet).To(ConsistOf(
			"subnet-test1 (test-zone-1a)",
			"subnet-test2 (test-zone-1b)",
		))
	})
	It("should discover subnets by IDs intersected with tags", func() {
		nodeTemplate.Spec.SubnetSelector = map[string]string{"aws-ids": "subnet-test2", "foo": "bar"}
		ExpectApplied(ctx, env.Client, provisioner, nodeTemplate)
		resolvedSubnetProvider, err := subnetProvider.List(ctx, nodeTemplate)
		resolvedSubnet := prettySubnets(resolvedSubnetProvider)
		Expect(err).To(BeNil())
		Expect(len(resolvedSubnet)).To(Equal(1))
		Expect(resolvedSubnet).To(ConsistOf(
			"subnet-test2 (test-zone-1b)",
		))
	})
})

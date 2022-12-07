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

package nodetemplatestatus_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/mitchellh/hashstructure/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/patrickmn/go-cache"
	"github.com/samber/lo"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"

	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	coresettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	awscontext "github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/controllers/nodetemplatestatus"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"
	"github.com/aws/karpenter/pkg/utils"
)

var ctx context.Context
var env *coretest.Environment
var fakeEC2API *fake.EC2API
var opts options.Options
var subnetCache *cache.Cache
var securityGroupCache *cache.Cache
var nodeTemplate *v1alpha1.AWSNodeTemplate
var controller *nodetemplatestatus.Controller
var settingsStore coretest.SettingsStore

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSNodeTemplateStatusController")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, apis.CRDs...)
	lo.Must0(apis.AddToScheme(scheme.Scheme))
	settingsStore = coretest.SettingsStore{
		coresettings.ContextKey: coretest.Settings(),
		coresettings.ContextKey: test.Settings(),
	}
	ctx = settingsStore.InjectSettings(ctx)

	fakeEC2API = &fake.EC2API{}
	subnetCache = cache.New(awscontext.CacheTTL*5, awscontext.CacheCleanupInterval)
	securityGroupCache = cache.New(awscontext.CacheTTL*5, awscontext.CacheCleanupInterval)

	fmt.Println(utils.RelativeToRoot("charts/karpenter/crds"))
	env.CRDDirectoryPaths = append(env.CRDDirectoryPaths, utils.RelativeToRoot("charts/karpenter/crds"))
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = injection.WithOptions(ctx, opts)
	settingsStore = coretest.SettingsStore{
		settings.ContextKey:     coretest.Settings(),
		coresettings.ContextKey: test.Settings(),
	}
	ctx = settingsStore.InjectSettings(ctx)

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
	controller = nodetemplatestatus.NewController(env.Client, fakeEC2API, subnetCache, securityGroupCache)

	fakeEC2API.Reset()
	securityGroupCache.Flush()
	subnetCache.Flush()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("AWSNodeTemplateStatusController", func() {
	It("Should update AWSNodeTemplate status for Subnets", func() {
		var ant v1alpha1.AWSNodeTemplate
		ExpectApplied(ctx, env.Client, nodeTemplate)
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		err := env.Client.Get(ctx, types.NamespacedName{Name: nodeTemplate.Name}, &ant)
		Expect(err).To(BeNil())
		Expect(len(ant.Status.Subnets)).To(Equal(3))
	})
	It("Should update AWSNodeTemplate status for Security Groups", func() {
		var ant v1alpha1.AWSNodeTemplate
		ExpectApplied(ctx, env.Client, nodeTemplate)
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		err := env.Client.Get(ctx, types.NamespacedName{Name: nodeTemplate.Name}, &ant)
		Expect(err).To(BeNil())
		Expect(len(ant.Status.SecurityGroups)).To(Equal(3))
	})
	It("Should update Subnet Cache with subnet information", func() {
		ExpectApplied(ctx, env.Client, nodeTemplate)
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		filters := utils.GetSubnetFilters(nodeTemplate)
		hash, _ := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
		subnets, ok := subnetCache.Get(fmt.Sprint(hash))
		Expect(ok).To(BeTrue())
		Expect(len(subnets.([]*ec2.Subnet))).To(Equal(3))
	})
	It("Should update SecurityGroup Cache with Security Group information", func() {
		ExpectApplied(ctx, env.Client, nodeTemplate)
		ExpectReconcileSucceeded(ctx, controller, types.NamespacedName{Name: nodeTemplate.Name, Namespace: nodeTemplate.Namespace})
		filters := utils.GetSecurityGroupFilters(nodeTemplate)
		hash, _ := hashstructure.Hash(filters, hashstructure.FormatV2, nil)
		securityGroups, ok := securityGroupCache.Get(fmt.Sprint(hash))
		Expect(ok).To(BeTrue())
		Expect(len(securityGroups.([]*ec2.SecurityGroup))).To(Equal(3))
	})
})

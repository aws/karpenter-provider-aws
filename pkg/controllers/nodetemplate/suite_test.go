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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/system/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/karpenter-core/pkg/apis/config/settings"
	corecontroller "github.com/aws/karpenter-core/pkg/operator/controller"
	coretest "github.com/aws/karpenter-core/pkg/test"
	. "github.com/aws/karpenter-core/pkg/test/expectations"
	"github.com/aws/karpenter/pkg/apis"
	coresettings "github.com/aws/karpenter/pkg/apis/config/settings"
	"github.com/aws/karpenter/pkg/apis/v1alpha1"
	"github.com/aws/karpenter/pkg/cloudprovider"
	"github.com/aws/karpenter/pkg/controllers/nodetemplate"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var env *coretest.Environment
var fakeEC2API *fake.EC2API
var opts options.Options
var subnetProvider *cloudprovider.SubnetProvider
var securityGroupProvider *cloudprovider.SecurityGroupProvider
var nodeTemplate *v1alpha1.AWSNodeTemplate
var controller corecontroller.Controller
var settingsStore coretest.SettingsStore

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "AWSNodeTemplateStatusController")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, apis.CRDs...)
	settingsStore = coretest.SettingsStore{
		coresettings.ContextKey: coretest.Settings(),
		coresettings.ContextKey: test.Settings(),
	}
	ctx = settingsStore.InjectSettings(ctx)

	fakeEC2API = &fake.EC2API{}
	subnetProvider = cloudprovider.NewSubnetProvider(fakeEC2API)
	securityGroupProvider = cloudprovider.NewSecurityGroupProvider(fakeEC2API)
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
	controller = nodetemplate.NewController(env.Client, fakeEC2API, subnetProvider, securityGroupProvider)

	fakeEC2API.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("AWSNodeTemplateStatusController", func() {
	It("Should update AWSNodeTemplate status for Subnets", func() {
		ExpectApplied(ctx, env.Client, nodeTemplate)
		ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
		err := env.Client.Get(ctx, types.NamespacedName{Name: nodeTemplate.Name}, nodeTemplate)
		Expect(err).To(BeNil())
		Expect(len(nodeTemplate.Status.SubnetsIDs)).To(Equal(3))
	})
	It("Should update AWSNodeTemplate status for Security Groups", func() {
		ExpectApplied(ctx, env.Client, nodeTemplate)
		ExpectReconcileSucceeded(ctx, controller, client.ObjectKeyFromObject(nodeTemplate))
		err := env.Client.Get(ctx, types.NamespacedName{Name: nodeTemplate.Name}, nodeTemplate)
		Expect(err).To(BeNil())
		Expect(len(nodeTemplate.Status.SecurityGroupsIDs)).To(Equal(3))
	})
})

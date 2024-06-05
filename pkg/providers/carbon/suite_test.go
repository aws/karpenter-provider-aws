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

package carbon_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	. "knative.dev/pkg/logging/testing"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/operator/injection"
	"github.com/aws/karpenter-core/pkg/operator/options"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	. "github.com/aws/karpenter-core/pkg/test/expectations"

	coretest "github.com/aws/karpenter-core/pkg/test"
	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/providers/carbon"
	"github.com/aws/karpenter/pkg/test"
)

var ctx context.Context
var stop context.CancelFunc
var opts options.Options

var env *coretest.Environment
var awsEnv *test.Environment
var controller *carbon.Controller

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
	controller = carbon.NewController(awsEnv.PricingProvider)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = injection.WithOptions(ctx, opts)
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())

	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Carbon Pricing", func() {
	It("should return static carbon data even if pricing API fails", func() {
		awsEnv.PricingAPI.NextError.Set(fmt.Errorf("failed"))
		ExpectReconcileFailed(ctx, controller, types.NamespacedName{})
		price, ok := awsEnv.PricingProvider.OnDemandPrice("c5.large")
		Expect(ok).To(BeTrue())
		Expect(price).To(BeNumerically(">", 0))
	})
})

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

package version_test

import (
	"context"
	"testing"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/samber/lo"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	controllersversion "github.com/aws/karpenter-provider-aws/pkg/controllers/providers/version"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var controller *controllersversion.Controller

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceType")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
	controller = controllersversion.NewController(awsEnv.VersionProvider)
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())

	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Version", func() {
	It("should update Version with response from the DescribeCluster API", func() {
		options.FromContext(ctx).EKSControlPlane = true
		awsEnv.EKSAPI.DescribeClusterBehavior.Output.Set(&eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{
				Version: lo.ToPtr("1.29"),
			},
		})
		ExpectSingletonReconciled(ctx, controller)
		Expect(awsEnv.VersionProvider.Get(ctx)).To(Equal("1.29"))
	})
})

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

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	environmentaws "github.com/aws/karpenter-provider-aws/test/pkg/environment/aws"
	"github.com/aws/karpenter-provider-aws/test/pkg/environment/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var testEnv *environmentaws.Environment

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "VersionProvider")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx = coreoptions.ToContext(ctx, coretest.Options())
	ctx = options.ToContext(ctx, test.Options())
	ctx, stop = context.WithCancel(ctx)
	awsEnv = test.NewEnvironment(ctx, env)
	testEnv = &environmentaws.Environment{Environment: &common.Environment{KubeClient: env.KubernetesInterface}}
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	awsEnv.Reset()
	awsEnv.EKSAPI.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Operator", func() {

	Context("with EKS_CONTROL_PLANE=true", func() {
		It("should resolve Kubernetes Version via Describe Cluster with no errors", func() {
			options.FromContext(ctx).IsEKSControlPlane = true
			endpoint, err := awsEnv.VersionProvider.Get(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoint).To(Equal("1.29"))
		})
	})

	Context("with EKS_CONTROL_PLANE=false", func() {
		It("should resolve Kubernetes Version via K8s API", func() {
			options.FromContext(ctx).IsEKSControlPlane = false
			endpoint, err := awsEnv.VersionProvider.Get(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoint).To(Equal(testEnv.K8sVersion()))
		})
	})
})

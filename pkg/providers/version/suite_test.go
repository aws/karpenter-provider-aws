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
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "knative.dev/pkg/logging/testing"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/test"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
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
	awsEnv.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	os.Unsetenv("KUBERNETES_SERVICE_HOST")
	os.Unsetenv("KUBERNETES_SERVICE_PORT")
})

var _ = Describe("VersionProvider", func() {
	Context("Get", func() {
		It("should discover the kubernetes version when in cluster with an HTTP client", func() {
			os.Setenv("KUBERNETES_SERVICE_HOST", "192.168.1.1")
			os.Setenv("KUBERNETES_SERVICE_PORT", "123")
			version, err := awsEnv.VersionProvider.Get(ctx)
			Expect(err).To(BeNil())
			Expect(version).To(Equal("0.1"))
		})
		It("should discover the version with the EKS API", func() {
			awsEnv.EKSAPI.DescribeClusterBehaviour.Output.Set(
				&eks.DescribeClusterOutput{
					Cluster: &eks.Cluster{
						Version: aws.String("9.9"),
					},
				},
			)
			version, err := awsEnv.VersionProvider.Get(ctx)
			Expect(err).To(BeNil())
			Expect(version).To(Equal("9.9"))
		})
		It("should discover fall back to the kubernetes clientset", func() {
			version, err := awsEnv.VersionProvider.Get(ctx)
			Expect(err).To(BeNil())
			Expect(version).To(Equal(strings.Join(strings.Split(env.Version.String(), ".")[0:2], ".")))
		})
	})
})

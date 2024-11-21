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
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	coreoptions "sigs.k8s.io/karpenter/pkg/operator/options"
	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/patrickmn/go-cache"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	awscache "github.com/aws/karpenter-provider-aws/pkg/cache"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	"github.com/aws/karpenter-provider-aws/pkg/providers/version"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var awsEnv *test.Environment
var fakeEKSAPI *fake.EKSAPI
var k8sVersion string
var versionProvider *version.DefaultProvider

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

	serverVersion, err := env.KubernetesInterface.Discovery().ServerVersion()
	Expect(err).ToNot(HaveOccurred())
	k8sVersion = fmt.Sprintf("%s.%s", serverVersion.Major, strings.TrimSuffix(serverVersion.Minor, "+"))

	fakeEKSAPI = &fake.EKSAPI{}
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	fakeEKSAPI.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Operator", func() {

	Context("with EKS_CONTROL_PLANE=true", func() {
		BeforeEach(func() {
			versionProvider = version.NewDefaultProvider(env.KubernetesInterface, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval), fakeEKSAPI)
			os.Setenv("EKS_CONTROL_PLANE", "true")
		})

		It("should resolve Kubernetes Version via Describe Cluster with no errors", func() {
			endpoint, err := versionProvider.Get(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoint).To(Equal("1.29"))
		})

		It("should handle EKS API errors and fallback to K8s API", func() {
			fakeEKSAPI.DescribeClusterBehavior.Error.Set(fmt.Errorf("some error"))
			_, err := versionProvider.Get(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should return error for access-denied EKS API errors", func() {
			accessDeniedErr := &awshttp.ResponseError{
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{
						Response: &http.Response{
							StatusCode: 403,
						},
					},
					Err: fmt.Errorf("User is not authorized to perform this operation"),
				},
			}

			fakeEKSAPI.DescribeClusterBehavior.Error.Set(accessDeniedErr)
			endpoint, err := versionProvider.Get(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoint).To(Equal(k8sVersion))
		})
	})

	Context("with EKS_CONTROL_PLANE=false", func() {
		It("should resolve Kubernetes Version via K8s API", func() {
			versionProvider = version.NewDefaultProvider(env.KubernetesInterface, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval), fakeEKSAPI)
			os.Setenv("EKS_CONTROL_PLANE", "false")
			endpoint, err := versionProvider.Get(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoint).To(Equal(k8sVersion))
		})
	})

	Context("with EKS_CONTROL_PLANE not set", func() {
		It("should resolve Kubernetes Version via K8s API", func() {
			versionProvider = version.NewDefaultProvider(env.KubernetesInterface, cache.New(awscache.DefaultTTL, awscache.DefaultCleanupInterval), fakeEKSAPI)
			os.Unsetenv("EKS_CONTROL_PLANE")
			endpoint, err := versionProvider.Get(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(endpoint).To(Equal(k8sVersion))
		})
	})
})

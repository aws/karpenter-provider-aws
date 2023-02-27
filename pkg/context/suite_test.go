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

package context

import (
	"context"
	"errors"
	"testing"

	ginkgov2 "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	. "knative.dev/pkg/logging/testing"

	. "github.com/aws/karpenter-core/pkg/test/expectations"

	"github.com/aws/aws-sdk-go/service/eks"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/fake"
	"github.com/aws/karpenter/pkg/test"

	coresettings "github.com/aws/karpenter-core/pkg/apis/settings"
	"github.com/aws/karpenter-core/pkg/operator/scheme"
	coretest "github.com/aws/karpenter-core/pkg/test"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var fakeEKSAPI *fake.EKSAPI

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(ginkgov2.Fail)
	ginkgov2.RunSpecs(t, "CloudProvider/AWS")
}

var _ = ginkgov2.BeforeSuite(func() {
	env = coretest.NewEnvironment(scheme.Scheme, coretest.WithCRDs(apis.CRDs...))
	ctx = coresettings.ToContext(ctx, coretest.Settings())
	ctx = settings.ToContext(ctx, test.Settings())
	ctx, stop = context.WithCancel(ctx)

	fakeEKSAPI = &fake.EKSAPI{}
})

var _ = ginkgov2.AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = ginkgov2.BeforeEach(func() {
	fakeEKSAPI.Reset()
})

var _ = ginkgov2.AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = ginkgov2.Describe("Context", func() {

	ginkgov2.It("should resolve endpoint if set via configuration", func() {
		ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
			ClusterEndpoint: lo.ToPtr("https://api.test-cluster.k8s.local"),
		}))
		endpoint, err := resolveClusterEndpoint(ctx, fakeEKSAPI)
		Expect(err).ToNot(HaveOccurred())
		Expect(endpoint).To(Equal("https://api.test-cluster.k8s.local"))
	})

	ginkgov2.It("should resolve endpoint if not set, via call to API", func() {
		ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
			ClusterEndpoint: lo.ToPtr(""),
		}))
		fakeEKSAPI.DescribeClusterBehaviour.Output.Set(
			&eks.DescribeClusterOutput{
				Cluster: &eks.Cluster{
					Endpoint: lo.ToPtr("https://cluster-endpoint.test-cluster.k8s.local"),
				},
			},
		)

		endpoint, err := resolveClusterEndpoint(ctx, fakeEKSAPI)
		Expect(err).ToNot(HaveOccurred())
		Expect(endpoint).To(Equal("https://cluster-endpoint.test-cluster.k8s.local"))
	})

	ginkgov2.It("should propagate error if API fails", func() {
		ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
			ClusterEndpoint: lo.ToPtr(""),
		}))
		fakeEKSAPI.DescribeClusterBehaviour.Error.Set(errors.New("test error"))

		_, err := resolveClusterEndpoint(ctx, fakeEKSAPI)
		Expect(err).To(HaveOccurred())
	})
})

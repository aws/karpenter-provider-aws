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

package operator_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/samber/lo"
	clock "k8s.io/utils/clock/testing"

	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	awscontext "github.com/aws/karpenter-provider-aws/pkg/operator"
	"github.com/aws/karpenter-provider-aws/pkg/operator/options"
	zonalshiftprovider "github.com/aws/karpenter-provider-aws/pkg/providers/arczonalshift"
	"github.com/aws/karpenter-provider-aws/pkg/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var stop context.CancelFunc
var env *coretest.Environment
var fakeEKSAPI *fake.EKSAPI
var fakeARCZonalShiftAPI *fake.ARCZonalShiftAPI

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx, stop = context.WithCancel(ctx)

	fakeEKSAPI = &fake.EKSAPI{}
	fakeARCZonalShiftAPI = fake.NewARCZonalShiftAPI()
})

var _ = AfterSuite(func() {
	stop()
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = BeforeEach(func() {
	fakeEKSAPI.Reset()
	fakeARCZonalShiftAPI.Reset()
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("Operator", func() {
	It("should resolve endpoint if set via configuration", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			ClusterEndpoint: lo.ToPtr("https://api.test-cluster.k8s.local"),
		}))
		endpoint, err := awscontext.ResolveClusterEndpoint(ctx, fakeEKSAPI)
		Expect(err).ToNot(HaveOccurred())
		Expect(endpoint).To(Equal("https://api.test-cluster.k8s.local"))
	})
	It("should resolve endpoint if not set, via call to API", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			ClusterEndpoint: lo.ToPtr(""),
		}))
		fakeEKSAPI.DescribeClusterBehavior.Output.Set(
			&eks.DescribeClusterOutput{
				Cluster: &ekstypes.Cluster{
					Endpoint: lo.ToPtr("https://cluster-endpoint.test-cluster.k8s.local"),
				},
			},
		)

		endpoint, err := awscontext.ResolveClusterEndpoint(ctx, fakeEKSAPI)
		Expect(err).ToNot(HaveOccurred())
		Expect(endpoint).To(Equal("https://cluster-endpoint.test-cluster.k8s.local"))
	})
	It("should propagate error if API fails", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			ClusterEndpoint: lo.ToPtr(""),
		}))
		fakeEKSAPI.DescribeClusterBehavior.Error.Set(errors.New("test error"))

		_, err := awscontext.ResolveClusterEndpoint(ctx, fakeEKSAPI)
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("ZonalShift", func() {
	const clusterArn = "arn:aws:eks:us-east-1:123456789012:cluster/test-cluster"
	var clk *clock.FakeClock
	BeforeEach(func() {
		clk = clock.NewFakeClock(time.Now())
		fakeEKSAPI.DescribeClusterBehavior.Output.Set(&eks.DescribeClusterOutput{
			Cluster: &ekstypes.Cluster{Arn: lo.ToPtr(clusterArn)},
		})
	})
	It("should use the no-op provider when zonal shift is disabled", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{EnableZonalShift: lo.ToPtr(false)}))
		provider := awscontext.NewZonalShiftProvider(ctx, fakeEKSAPI, fakeARCZonalShiftAPI, clk)
		Expect(provider).To(BeAssignableToTypeOf(&zonalshiftprovider.DefaultNoopProvider{}))
		Expect(fakeARCZonalShiftAPI.GetManagedResourceBehavior.Calls()).To(Equal(0))
	})
	It("should use the ARC provider when the cluster is registered with zonal shift", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{EnableZonalShift: lo.ToPtr(true)}))
		provider := awscontext.NewZonalShiftProvider(ctx, fakeEKSAPI, fakeARCZonalShiftAPI, clk)
		Expect(provider).To(BeAssignableToTypeOf(&zonalshiftprovider.DefaultProvider{}))
		Expect(fakeARCZonalShiftAPI.GetManagedResourceBehavior.Calls()).To(Equal(1))
		Expect(*fakeARCZonalShiftAPI.GetManagedResourceBehavior.CalledWithInput.Pop().ResourceIdentifier).To(Equal(clusterArn))
	})
	It("should fall back to the no-op provider when the cluster is not registered with zonal shift", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{EnableZonalShift: lo.ToPtr(true)}))
		fakeARCZonalShiftAPI.GetManagedResourceBehavior.Error.Set(errors.New("AccessDeniedException: Resource " + clusterArn + " is not a managed resource"))
		var provider zonalshiftprovider.Provider
		Expect(func() { provider = awscontext.NewZonalShiftProvider(ctx, fakeEKSAPI, fakeARCZonalShiftAPI, clk) }).ToNot(Panic())
		Expect(provider).To(BeAssignableToTypeOf(&zonalshiftprovider.DefaultNoopProvider{}))
		Expect(provider.IsZonalShifted(ctx, "use1-az1")).To(BeFalse())
	})
})

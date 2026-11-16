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
	"net/http"
	"net/http/httptest"
	"testing"

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/go-logr/logr/funcr"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"

	coretest "sigs.k8s.io/karpenter/pkg/test"

	"github.com/aws/karpenter-provider-aws/pkg/apis"
	"github.com/aws/karpenter-provider-aws/pkg/fake"
	awscontext "github.com/aws/karpenter-provider-aws/pkg/operator"
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
var fakeEKSAPI *fake.EKSAPI

func TestAWS(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "CloudProvider/AWS")
}

var _ = BeforeSuite(func() {
	env = coretest.NewEnvironment(coretest.WithCRDs(apis.CRDs...), coretest.WithCRDs(v1alpha1.CRDs...))
	ctx, stop = context.WithCancel(ctx)

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
	It("should not enable AWS SDK debug logging by default", func() {
		ctx = options.ToContext(ctx, test.Options())
		cfg := awscontext.WithAWSSDKDebugLog(ctx, aws.Config{})
		Expect(cfg.ClientLogMode).To(Equal(aws.ClientLogMode(0)))
		Expect(cfg.Logger).To(BeNil())
	})
	It("should log AWS SDK requests and responses through the controller logger when aws-sdk-debug-log is set", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/xml")
			_, _ = w.Write([]byte(`<DescribeRegionsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><regionInfo/></DescribeRegionsResponse>`))
		}))
		defer server.Close()
		var lines []string
		ctx = log.IntoContext(ctx, funcr.New(func(prefix, args string) { lines = append(lines, prefix+" "+args) }, funcr.Options{}))
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			AWSSDKDebugLog: lo.ToPtr(true),
		}))
		cfg := awscontext.WithAWSSDKDebugLog(ctx, aws.Config{
			Region: "us-west-2",
			Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: "id", SecretAccessKey: "secret", SessionToken: "test-session-token"}, nil
			}),
		})
		Expect(cfg.ClientLogMode.IsRetries()).To(BeTrue())
		Expect(cfg.ClientLogMode.IsRequestWithBody()).To(BeTrue())
		Expect(cfg.ClientLogMode.IsResponseWithBody()).To(BeTrue())
		_, err := ec2.NewFromConfig(cfg, func(o *ec2.Options) { o.BaseEndpoint = aws.String(server.URL) }).DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
		Expect(err).ToNot(HaveOccurred())
		Expect(lines).To(ContainElement(And(ContainSubstring("aws-sdk"), ContainSubstring("Action=DescribeRegions"))))
		Expect(lines).To(ContainElement(And(ContainSubstring("aws-sdk"), ContainSubstring("DescribeRegionsResponse"))))
		Expect(lines).To(ContainElement(ContainSubstring("X-Amz-Security-Token: <redacted>")))
		Expect(lines).ToNot(ContainElement(ContainSubstring("test-session-token")))
		Expect(lines).ToNot(ContainElement(ContainSubstring("Signature=")))
	})
	It("should redact the IMDS session token header when aws-sdk-debug-log is set", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				w.Header().Set("X-Aws-Ec2-Metadata-Token-Ttl-Seconds", "300")
				_, _ = w.Write([]byte("test-imds-token"))
				return
			}
			_, _ = w.Write([]byte(`{"region":"us-west-2"}`))
		}))
		defer server.Close()
		var lines []string
		ctx = log.IntoContext(ctx, funcr.New(func(prefix, args string) { lines = append(lines, prefix+" "+args) }, funcr.Options{}))
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			AWSSDKDebugLog: lo.ToPtr(true),
		}))
		cfg := awscontext.WithAWSSDKDebugLog(ctx, aws.Config{})
		_, err := imds.NewFromConfig(cfg, func(o *imds.Options) { o.Endpoint = server.URL }).GetRegion(ctx, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(lines).To(ContainElement(ContainSubstring("X-Aws-Ec2-Metadata-Token: <redacted>")))
		Expect(lines).ToNot(ContainElement(MatchRegexp(`(?i)X-Aws-Ec2-Metadata-Token: test-imds-token`)))
	})
})

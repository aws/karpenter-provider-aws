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

	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"

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
	It("should read the cluster DNS IP from the default kube-system/kube-dns Service", func() {
		ctx = options.ToContext(ctx, test.Options())
		kubeClient := kubernetesfake.NewSimpleClientset(dnsService("kube-system", "kube-dns", "10.100.0.10"))

		ip, err := awscontext.KubeDNSIP(ctx, kubeClient)
		Expect(err).ToNot(HaveOccurred())
		Expect(ip.String()).To(Equal("10.100.0.10"))
	})
	It("should read the cluster DNS IP from a Service named by configuration", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			KubeDNSServiceName:      lo.ToPtr("coredns"),
			KubeDNSServiceNamespace: lo.ToPtr("dns-system"),
		}))
		kubeClient := kubernetesfake.NewSimpleClientset(dnsService("dns-system", "coredns", "10.100.0.53"))

		ip, err := awscontext.KubeDNSIP(ctx, kubeClient)
		Expect(err).ToNot(HaveOccurred())
		Expect(ip.String()).To(Equal("10.100.0.53"))
	})
	It("should prefer an explicitly configured cluster DNS IP over the Service", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			ClusterDNSIP: lo.ToPtr("10.100.0.99"),
		}))
		kubeClient := kubernetesfake.NewSimpleClientset(dnsService("kube-system", "kube-dns", "10.100.0.10"))

		ip, err := awscontext.KubeDNSIP(ctx, kubeClient)
		Expect(err).ToNot(HaveOccurred())
		Expect(ip.String()).To(Equal("10.100.0.99"))
	})
	It("should not need a Service at all when the cluster DNS IP is configured", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			ClusterDNSIP: lo.ToPtr("10.100.0.99"),
		}))
		kubeClient := kubernetesfake.NewSimpleClientset()

		ip, err := awscontext.KubeDNSIP(ctx, kubeClient)
		Expect(err).ToNot(HaveOccurred())
		Expect(ip.String()).To(Equal("10.100.0.99"))
	})
	It("should not fall back to kube-dns once a Service name is configured", func() {
		ctx = options.ToContext(ctx, test.Options(test.OptionsFields{
			KubeDNSServiceName: lo.ToPtr("coredns"),
		}))
		kubeClient := kubernetesfake.NewSimpleClientset(dnsService("kube-system", "kube-dns", "10.100.0.10"))

		_, err := awscontext.KubeDNSIP(ctx, kubeClient)
		Expect(err).To(HaveOccurred())
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

func dnsService(namespace, name, clusterIP string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec:       corev1.ServiceSpec{ClusterIP: clusterIP},
	}
}

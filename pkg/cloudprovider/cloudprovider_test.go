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

package cloudprovider

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	"github.com/samber/lo"

	"github.com/aws/aws-sdk-go/service/eks"

	"github.com/aws/karpenter/pkg/apis/settings"
	"github.com/aws/karpenter/pkg/context"
	"github.com/aws/karpenter/pkg/test"

	. "github.com/onsi/gomega"
)

var _ = Describe("Cloud Provider", func() {
	BeforeEach(func() {
		fakeEKSAPI.Reset()
	})

	It("should resolve endpoint if set via configuration", func() {
		ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
			ClusterEndpoint: lo.ToPtr("https://api.test-cluster.k8s.local"),
		}))
		endpoint, err := context.ResolveClusterEndpoint(ctx, fakeEKSAPI)
		Expect(err).ToNot(HaveOccurred())
		Expect(endpoint).To(Equal("https://api.test-cluster.k8s.local"))
	})

	It("should resolve endpoint if not set, via call to API", func() {
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

		endpoint, err := context.ResolveClusterEndpoint(ctx, fakeEKSAPI)
		Expect(err).ToNot(HaveOccurred())
		Expect(endpoint).To(Equal("https://cluster-endpoint.test-cluster.k8s.local"))
	})

	It("should propagate error if API fails", func() {
		ctx = settings.ToContext(ctx, test.Settings(test.SettingOptions{
			ClusterEndpoint: lo.ToPtr(""),
		}))
		fakeEKSAPI.DescribeClusterBehaviour.Error.Set(errors.New("test error"))

		_, err := context.ResolveClusterEndpoint(ctx, fakeEKSAPI)
		Expect(err).To(HaveOccurred())
	})
})

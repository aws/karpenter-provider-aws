/*
Copyright The Kubernetes Authors.

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

package nodepool_test

import (
	"context"
	"math/rand/v2"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"github.com/samber/lo/mutable"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	nodepoolutils "sigs.k8s.io/karpenter/pkg/utils/nodepool"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var (
	ctx context.Context
	env *test.Environment
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodePoolUtils")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodePoolUtils", func() {
	Context("OrderByWeight", func() {
		It("should order the NodePools by weight", func() {
			// Generate 10 NodePools that have random weights, some might have the same weights
			nps := lo.Times(10, func(_ int) *v1.NodePool {
				return test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Weight: lo.ToPtr[int32](int32(rand.IntN(100) + 1)), //nolint:gosec
					},
				})
			})
			mutable.Shuffle(nps)
			nodepoolutils.OrderByWeight(nps)

			lastWeight := 101 // This is above the allowed weight values
			for _, np := range nps {
				Expect(lo.FromPtr(np.Spec.Weight)).To(BeNumerically("<=", lastWeight))
				lastWeight = int(lo.FromPtr(np.Spec.Weight))
			}
		})
		It("should order the NodePools by name when the weights are the same", func() {
			// Generate 10 NodePools with the same weight
			nps := lo.Times(10, func(_ int) *v1.NodePool {
				return test.NodePool(v1.NodePool{
					Spec: v1.NodePoolSpec{
						Weight: lo.ToPtr[int32](10),
					},
				})
			})
			mutable.Shuffle(nps)
			nodepoolutils.OrderByWeight(nps)

			lastName := "zzzzzzzzzzzzzzzzzzzzzzzz" // large string value
			for _, np := range nps {
				Expect(np.Name < lastName).To(BeTrue())
				lastName = np.Name
			}
		})
	})
})

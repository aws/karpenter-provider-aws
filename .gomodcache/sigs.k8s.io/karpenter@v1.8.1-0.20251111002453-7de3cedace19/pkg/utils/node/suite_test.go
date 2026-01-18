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

package node_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	nodeutils "sigs.k8s.io/karpenter/pkg/utils/node"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var (
	ctx context.Context
	env *test.Environment
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeUtils")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...), test.WithFieldIndexers(test.NodeClaimProviderIDFieldIndexer(ctx)))
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeUtils", func() {
	var testNode *corev1.Node
	var nodeClaim *v1.NodeClaim
	BeforeEach(func() {
		nodeClaim = test.NodeClaim()
	})
	It("should return nodeClaim for node which has the same provider ID", func() {
		testNode = test.NodeClaimLinkedNode(nodeClaim)
		ExpectApplied(ctx, env.Client, testNode, nodeClaim)

		nodeClaims, err := nodeutils.GetNodeClaims(ctx, env.Client, testNode)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodeClaims).To(HaveLen(1))
		for _, nc := range nodeClaims {
			Expect(nc.Status.ProviderID).To(BeEquivalentTo(testNode.Spec.ProviderID))
		}
	})
	It("should not return nodeClaim for node since the node supplied here has different provider ID", func() {
		testNode = test.Node(test.NodeOptions{
			ProviderID: "testID",
		})
		ExpectApplied(ctx, env.Client, testNode, nodeClaim)

		nodeClaims, err := nodeutils.GetNodeClaims(ctx, env.Client, testNode)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodeClaims).To(HaveLen(0))
	})
	It("should not return nodeClaim for node since the node supplied here has no provider ID", func() {
		testNode = test.Node(test.NodeOptions{
			ProviderID: "",
		})
		ExpectApplied(ctx, env.Client, testNode, nodeClaim)

		nodeClaims, err := nodeutils.GetNodeClaims(ctx, env.Client, testNode)
		Expect(err).NotTo(HaveOccurred())
		Expect(nodeClaims).To(HaveLen(0))
	})
})

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

package hydration_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/node/hydration"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var ctx context.Context
var hydrationController *hydration.Controller
var env *test.Environment
var cloudProvider *fake.CloudProvider

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lifecycle")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...), test.WithFieldIndexers(test.NodeProviderIDFieldIndexer(ctx), test.NodeClaimProviderIDFieldIndexer(ctx)))
	ctx = options.ToContext(ctx, test.Options())

	cloudProvider = fake.NewCloudProvider()
	hydrationController = hydration.NewController(env.Client, cloudProvider)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
	cloudProvider.Reset()
})

var _ = Describe("Hydration", func() {
	DescribeTable(
		"Hydration",
		func(isNodeClaimManaged bool) {
			nodeClassRef := lo.Ternary(isNodeClaimManaged, &v1.NodeClassReference{
				Group: "karpenter.test.sh",
				Kind:  "TestNodeClass",
				Name:  "default",
			}, &v1.NodeClassReference{
				Group: "karpenter.test.sh",
				Kind:  "UnmanagedNodeClass",
				Name:  "default",
			})
			nodeClaim, node := test.NodeClaimAndNode(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					NodeClassRef: nodeClassRef,
				},
			})
			delete(node.Labels, v1.NodeClassLabelKey(nodeClassRef.GroupKind()))
			ExpectApplied(ctx, env.Client, nodeClaim, node)
			ExpectObjectReconciled(ctx, env.Client, hydrationController, node)

			// The missing NodeClass label should have been propagated to the Node
			node = ExpectExists(ctx, env.Client, node)
			value, ok := node.Labels[v1.NodeClassLabelKey(nodeClassRef.GroupKind())]
			Expect(ok).To(Equal(isNodeClaimManaged))
			if isNodeClaimManaged {
				Expect(value).To(Equal(nodeClassRef.Name))
			}
		},
		Entry("should hydrate missing metadata onto the Node", true),
		Entry("should ignore Nodes which aren't managed by this Karpenter instance", false),
	)
})

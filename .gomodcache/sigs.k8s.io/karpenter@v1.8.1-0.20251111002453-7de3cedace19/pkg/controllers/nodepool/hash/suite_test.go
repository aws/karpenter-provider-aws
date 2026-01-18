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

package hash_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/nodepool/hash"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var nodePoolController *hash.Controller
var ctx context.Context
var env *test.Environment
var cp *fake.CloudProvider

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hash")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
	cp = fake.NewCloudProvider()
	nodePoolController = hash.NewController(env.Client, cp)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Static Drift Hash", func() {
	var nodePool *v1.NodePool
	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Template: v1.NodeClaimTemplate{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							"keyAnnotation":  "valueAnnotation",
							"keyAnnotation2": "valueAnnotation2",
						},
						Labels: map[string]string{
							"keyLabel": "valueLabel",
						},
					},
					Spec: v1.NodeClaimTemplateSpec{
						Taints: []corev1.Taint{
							{
								Key:    "key",
								Effect: corev1.TaintEffectNoExecute,
							},
						},
						StartupTaints: []corev1.Taint{
							{
								Key:    "key",
								Effect: corev1.TaintEffectNoExecute,
							},
						},
						ExpireAfter:            v1.MustParseNillableDuration("5m"),
						TerminationGracePeriod: &metav1.Duration{Duration: 5 * time.Minute},
					},
				},
			},
		})
	})
	It("should ignore NodePools which aren't managed by this instance of Karpenter", func() {
		nodePool.Spec.Template.Spec.NodeClassRef = &v1.NodeClassReference{
			Group: "karpenter.test.sh",
			Kind:  "UnmanagedNodeClass",
			Name:  "default",
		}
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		_, ok := nodePool.Annotations[v1.NodePoolHashAnnotationKey]
		Expect(ok).To(BeFalse())
	})
	// TODO we should split this out into a DescribeTable
	It("should update the drift hash when NodePool static field is updated", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		expectedHash := nodePool.Hash()
		Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))

		nodePool.Spec.Template.Labels = map[string]string{"keyLabeltest": "valueLabeltest"}
		nodePool.Spec.Template.Annotations = map[string]string{"keyAnnotation2": "valueAnnotation2", "keyAnnotation": "valueAnnotation"}
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		expectedHashTwo := nodePool.Hash()
		Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHashTwo))
	})
	It("should not update the drift hash when NodePool behavior field is updated", func() {
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		expectedHash := nodePool.Hash()
		Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))

		nodePool.Spec.Limits = v1.Limits(corev1.ResourceList{"cpu": resource.MustParse("16")})
		nodePool.Spec.Disruption.ConsolidationPolicy = v1.ConsolidationPolicyWhenEmpty
		nodePool.Spec.Disruption.ConsolidateAfter = v1.MustParseNillableDuration("30s")
		nodePool.Spec.Template.Spec.Requirements = []v1.NodeSelectorRequirementWithMinValues{
			{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpIn, Values: []string{"test"}}},
			{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpGt, Values: []string{"1"}}},
			{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: corev1.LabelTopologyZone, Operator: corev1.NodeSelectorOpLt, Values: []string{"1"}}},
		}
		nodePool.Spec.Weight = lo.ToPtr(int32(80))
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))
	})
	It("should update nodepool hash version when the nodepool hash version is out of sync with the controller hash version", func() {
		nodePool.Annotations = map[string]string{
			v1.NodePoolHashAnnotationKey:        "abceduefed",
			v1.NodePoolHashVersionAnnotationKey: "test",
		}
		ExpectApplied(ctx, env.Client, nodePool)

		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)

		expectedHash := nodePool.Hash()
		Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))
		Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
	})
	It("should update nodepool hash versions on all nodeclaims when the hash versions don't match the controller hash version", func() {
		nodePool.Annotations = map[string]string{
			v1.NodePoolHashAnnotationKey:        "abceduefed",
			v1.NodePoolHashVersionAnnotationKey: "test",
		}
		nodeClaimOne := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.NodePoolLabelKey: nodePool.Name},
				Annotations: map[string]string{
					v1.NodePoolHashAnnotationKey:        "123456",
					v1.NodePoolHashVersionAnnotationKey: "test",
				},
			},
		})
		nodeClaimTwo := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.NodePoolLabelKey: nodePool.Name},
				Annotations: map[string]string{
					v1.NodePoolHashAnnotationKey:        "123456",
					v1.NodePoolHashVersionAnnotationKey: "test",
				},
			},
		})

		ExpectApplied(ctx, env.Client, nodePool, nodeClaimOne, nodeClaimTwo)

		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		nodeClaimOne = ExpectExists(ctx, env.Client, nodeClaimOne)
		nodeClaimTwo = ExpectExists(ctx, env.Client, nodeClaimTwo)

		expectedHash := nodePool.Hash()
		Expect(nodeClaimOne.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))
		Expect(nodeClaimOne.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
		Expect(nodeClaimTwo.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))
		Expect(nodeClaimTwo.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
	})
	It("should not update nodepool hash on all nodeclaims when the hash versions match the controller hash version", func() {
		nodePool.Annotations = map[string]string{
			v1.NodePoolHashAnnotationKey:        "abceduefed",
			v1.NodePoolHashVersionAnnotationKey: "test-version",
		}
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.NodePoolLabelKey: nodePool.Name},
				Annotations: map[string]string{
					v1.NodePoolHashAnnotationKey:        "1234564654",
					v1.NodePoolHashVersionAnnotationKey: v1.NodePoolHashVersion,
				},
			},
		})
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)

		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodePool = ExpectExists(ctx, env.Client, nodePool)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		expectedHash := nodePool.Hash()

		// Expect NodeClaims to have been updated to the original hash
		Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, expectedHash))
		Expect(nodePool.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, "1234564654"))
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
	})
	It("should not update nodepool hash on the nodeclaim if it's drifted", func() {
		nodePool.Annotations = map[string]string{
			v1.NodePoolHashAnnotationKey:        "abceduefed",
			v1.NodePoolHashVersionAnnotationKey: "test",
		}
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{v1.NodePoolLabelKey: nodePool.Name},
				Annotations: map[string]string{
					v1.NodePoolHashAnnotationKey:        "123456",
					v1.NodePoolHashVersionAnnotationKey: "test",
				},
			},
		})
		nodeClaim.StatusConditions().SetTrue(v1.ConditionTypeDrifted)
		ExpectApplied(ctx, env.Client, nodePool, nodeClaim)

		ExpectObjectReconciled(ctx, env.Client, nodePoolController, nodePool)
		nodeClaim = ExpectExists(ctx, env.Client, nodeClaim)

		// Expect NodeClaims hash to not have been updated
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodePoolHashAnnotationKey, "123456"))
		Expect(nodeClaim.Annotations).To(HaveKeyWithValue(v1.NodePoolHashVersionAnnotationKey, v1.NodePoolHashVersion))
	})
})

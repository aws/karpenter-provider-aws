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

package nodeclaim_test

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	nodeclaimutils "sigs.k8s.io/karpenter/pkg/utils/nodeclaim"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var (
	ctx           context.Context
	env           *test.Environment
	cloudProvider cloudprovider.CloudProvider
)

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeClaimUtils")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...), test.WithFieldIndexers(test.NodeClaimProviderIDFieldIndexer(ctx)))
	cloudProvider = fake.NewCloudProvider()
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = AfterEach(func() {
	ExpectCleanedUp(ctx, env.Client)
})

var _ = Describe("NodeClaimUtils", func() {
	var node *corev1.Node
	BeforeEach(func() {
		node = test.Node(test.NodeOptions{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					corev1.LabelTopologyZone:       "test-zone-1",
					corev1.LabelTopologyRegion:     "test-region",
					"test-label-key":               "test-label-value",
					"test-label-key2":              "test-label-value2",
					v1.NodeRegisteredLabelKey:      "true",
					v1.NodeInitializedLabelKey:     "true",
					v1.NodePoolLabelKey:            "default",
					v1.CapacityTypeLabelKey:        v1.CapacityTypeOnDemand,
					corev1.LabelOSStable:           "linux",
					corev1.LabelInstanceTypeStable: "test-instance-type",
				},
				Annotations: map[string]string{
					"test-annotation-key":        "test-annotation-value",
					"test-annotation-key2":       "test-annotation-value2",
					"node-custom-annotation-key": "node-custom-annotation-value",
				},
			},
			ReadyStatus: corev1.ConditionTrue,
			Taints: []corev1.Taint{
				{
					Key:    "test-taint-key",
					Effect: corev1.TaintEffectNoSchedule,
					Value:  "test-taint-value",
				},
				{
					Key:    "test-taint-key2",
					Effect: corev1.TaintEffectNoExecute,
					Value:  "test-taint-value2",
				},
			},
			ProviderID: test.RandomProviderID(),
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("10"),
				corev1.ResourceMemory:           resource.MustParse("10Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("8"),
				corev1.ResourceMemory:           resource.MustParse("8Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("95Gi"),
			},
		})
	})
	It("should update the owner for a Node to a NodeClaim", func() {
		nodeClaim := test.NodeClaim(v1.NodeClaim{
			Spec: v1.NodeClaimSpec{
				NodeClassRef: &v1.NodeClassReference{
					Kind:  "NodeClassRef",
					Group: "test.cloudprovider/v1",
					Name:  "default",
				},
			},
		})
		node = test.Node(test.NodeOptions{ProviderID: nodeClaim.Status.ProviderID})
		node = nodeclaimutils.UpdateNodeOwnerReferences(nodeClaim, node)

		Expect(lo.Contains(node.OwnerReferences, metav1.OwnerReference{
			APIVersion:         lo.Must(apiutil.GVKForObject(nodeClaim, scheme.Scheme)).GroupVersion().String(),
			Kind:               lo.Must(apiutil.GVKForObject(nodeClaim, scheme.Scheme)).String(),
			Name:               nodeClaim.Name,
			UID:                nodeClaim.UID,
			BlockOwnerDeletion: lo.ToPtr(true),
		}))
	})
	Context("List", func() {
		It("should filter by provider ID", func() {
			ncs := lo.Times(2, func(_ int) *v1.NodeClaim {
				return test.NodeClaim(v1.NodeClaim{
					Spec: v1.NodeClaimSpec{
						NodeClassRef: &v1.NodeClassReference{
							Group: "karpenter.test.sh",
							Kind:  "TestNodeClass",
							Name:  "default",
						},
					},
				})
			})
			ExpectApplied(ctx, env.Client, ncs[0], ncs[1])
			res, err := nodeclaimutils.ListManaged(ctx, env.Client, cloudProvider, nodeclaimutils.ForProviderID(ncs[0].Status.ProviderID))
			Expect(err).To(BeNil())
			Expect(len(res)).To(Equal(1))
			Expect(res[0].Name).To(Equal(ncs[0].Name))
		})
		It("should filter by NodePool", func() {
			ncs := lo.Times(2, func(i int) *v1.NodeClaim {
				return test.NodeClaim(v1.NodeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							v1.NodePoolLabelKey: fmt.Sprintf("nodepool-%d", i),
						},
					},
					Spec: v1.NodeClaimSpec{
						NodeClassRef: &v1.NodeClassReference{
							Group: "karpenter.test.sh",
							Kind:  "TestNodeClass",
							Name:  "default",
						},
					},
				})
			})
			ExpectApplied(ctx, env.Client, ncs[0], ncs[1])
			res, err := nodeclaimutils.ListManaged(ctx, env.Client, cloudProvider, nodeclaimutils.ForNodePool(ncs[0].Labels[v1.NodePoolLabelKey]))
			Expect(err).To(BeNil())
			Expect(len(res)).To(Equal(1))
			Expect(res[0].Name).To(Equal(ncs[0].Name))
		})
		It("should filter managed NodeClaims", func() {
			managed := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					NodeClassRef: &v1.NodeClassReference{
						Group: "karpenter.test.sh",
						Kind:  "TestNodeClass",
						Name:  "default",
					},
				},
			})
			unmanagedByGroup := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					NodeClassRef: &v1.NodeClassReference{
						Group: "karpenter.unmanaged.sh",
						Kind:  "TestNodeClass",
						Name:  "default",
					},
				},
			})
			unmanagedByKind := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					NodeClassRef: &v1.NodeClassReference{
						Group: "karpenter.test.sh",
						Kind:  "UnmanagedNodeClass",
						Name:  "default",
					},
				},
			})
			unmanaged := test.NodeClaim(v1.NodeClaim{
				Spec: v1.NodeClaimSpec{
					NodeClassRef: &v1.NodeClassReference{
						Group: "karpenter.test.sh",
						Kind:  "UnmanagedNodeClass",
						Name:  "default",
					},
				},
			})
			ExpectApplied(ctx, env.Client, managed, unmanagedByGroup, unmanagedByKind, unmanaged)
			res, err := nodeclaimutils.ListManaged(ctx, env.Client, cloudProvider)
			Expect(err).To(BeNil())
			Expect(len(res)).To(Equal(1))
			Expect(res[0].Name).To(Equal(managed.Name))
		})
	})
})

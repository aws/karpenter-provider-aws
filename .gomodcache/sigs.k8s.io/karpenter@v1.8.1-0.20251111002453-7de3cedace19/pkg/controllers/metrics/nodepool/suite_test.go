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
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/pkg/apis"
	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider/fake"
	"sigs.k8s.io/karpenter/pkg/controllers/metrics/nodepool"
	"sigs.k8s.io/karpenter/pkg/test"
	. "sigs.k8s.io/karpenter/pkg/test/expectations"
	"sigs.k8s.io/karpenter/pkg/test/v1alpha1"
	. "sigs.k8s.io/karpenter/pkg/utils/testing"
)

var nodePoolController *nodepool.Controller
var ctx context.Context
var env *test.Environment
var cp *fake.CloudProvider

func TestAPIs(t *testing.T) {
	ctx = TestContextWithLogger(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodePoolMetrics")
}

var _ = BeforeSuite(func() {
	env = test.NewEnvironment(test.WithCRDs(apis.CRDs...), test.WithCRDs(v1alpha1.CRDs...))
	cp = fake.NewCloudProvider()
	nodePoolController = nodepool.NewController(env.Client, cp)
})

var _ = AfterSuite(func() {
	Expect(env.Stop()).To(Succeed(), "Failed to stop environment")
})

var _ = Describe("Metrics", func() {
	var nodePool *v1.NodePool
	BeforeEach(func() {
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Template: v1.NodeClaimTemplate{
					Spec: v1.NodeClaimTemplateSpec{
						NodeClassRef: &v1.NodeClassReference{
							Group: "karpenter.test.sh",
							Kind:  "TestNodeClass",
							Name:  "default",
						},
					},
				},
			},
		})
	})
	DescribeTable(
		"should update the nodepool limit metrics",
		func(isNodePoolManaged bool) {
			limits := v1.Limits{
				corev1.ResourceCPU:              resource.MustParse("10"),
				corev1.ResourceMemory:           resource.MustParse("10Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
			}
			nodePool.Spec.Limits = limits
			if !isNodePoolManaged {
				nodePool.Spec.Template.Spec.NodeClassRef = &v1.NodeClassReference{
					Group: "karpenter.test.sh",
					Kind:  "UnmanagedNodeClass",
					Name:  "default",
				}
			}
			ExpectApplied(ctx, env.Client, nodePool)
			ExpectReconcileSucceeded(ctx, nodePoolController, client.ObjectKeyFromObject(nodePool))

			for k, v := range limits {
				m, found := FindMetricWithLabelValues("karpenter_nodepools_limit", map[string]string{
					"nodepool":      nodePool.GetName(),
					"resource_type": strings.ReplaceAll(k.String(), "-", "_"),
				})
				Expect(found).To(Equal(isNodePoolManaged))
				if isNodePoolManaged {
					Expect(m.GetGauge().GetValue()).To(BeNumerically("~", v.AsApproximateFloat64()))
				}
			}
		},
		Entry("should update the nodepool limit metrics", true),
		Entry("should ignore nodepools not managed by this instance of Karpenter", false),
	)
	It("should update the nodepool usage metrics", func() {
		resources := corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("10"),
			corev1.ResourceMemory:           resource.MustParse("10Mi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
		}
		nodePool.Status.Resources = resources

		ExpectApplied(ctx, env.Client, nodePool)
		ExpectReconcileSucceeded(ctx, nodePoolController, client.ObjectKeyFromObject(nodePool))

		for k, v := range resources {
			m, found := FindMetricWithLabelValues("karpenter_nodepools_usage", map[string]string{
				"nodepool":      nodePool.GetName(),
				"resource_type": strings.ReplaceAll(k.String(), "-", "_"),
			})
			Expect(found).To(BeTrue())
			Expect(m.GetGauge().GetValue()).To(BeNumerically("~", v.AsApproximateFloat64()))
		}
	})
	It("should delete the nodepool state metrics on nodepool delete", func() {
		expectedMetrics := []string{"karpenter_nodepools_limit", "karpenter_nodepools_usage"}
		nodePool.Spec.Limits = v1.Limits{
			corev1.ResourceCPU:              resource.MustParse("100"),
			corev1.ResourceMemory:           resource.MustParse("100Mi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("1000Gi"),
		}
		nodePool.Status.Resources = corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("10"),
			corev1.ResourceMemory:           resource.MustParse("10Mi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
		}
		ExpectApplied(ctx, env.Client, nodePool)
		ExpectReconcileSucceeded(ctx, nodePoolController, client.ObjectKeyFromObject(nodePool))

		for _, name := range expectedMetrics {
			_, found := FindMetricWithLabelValues(name, map[string]string{
				"nodepool": nodePool.GetName(),
			})
			Expect(found).To(BeTrue())
		}

		ExpectDeleted(ctx, env.Client, nodePool)
		ExpectReconcileSucceeded(ctx, nodePoolController, client.ObjectKeyFromObject(nodePool))

		for _, name := range expectedMetrics {
			_, found := FindMetricWithLabelValues(name, map[string]string{
				"nodepool": nodePool.GetName(),
			})
			Expect(found).To(BeFalse())
		}
	})
})

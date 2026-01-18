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

package v1_test

import (
	"strings"
	"time"

	"github.com/Pallinder/go-randomdata"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "sigs.k8s.io/karpenter/pkg/apis/v1"
)

var _ = Describe("CEL/Default", func() {
	var nodePool *NodePool

	BeforeEach(func() {
		nodePool = &NodePool{
			ObjectMeta: metav1.ObjectMeta{Name: strings.ToLower(randomdata.SillyName())},
			Spec: NodePoolSpec{
				Template: NodeClaimTemplate{
					Spec: NodeClaimTemplateSpec{
						NodeClassRef: &NodeClassReference{
							Group: "karpenter.test.sh",
							Kind:  "TestNodeClaim",
							Name:  "default",
						},
						Requirements: []NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: v1.NodeSelectorRequirement{
									Key:      CapacityTypeLabelKey,
									Operator: v1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		}
	})
	Context("Defaults/TopLevel", func() {
		It("should default the disruption stanza when undefined", func() {
			Expect(env.Client.Create(ctx, nodePool)).To(Succeed())
			Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(nodePool), nodePool)).To(Succeed())
			Expect(nodePool.Spec.Disruption).ToNot(BeNil())
			Expect(lo.FromPtr(nodePool.Spec.Disruption.ConsolidateAfter.Duration)).To(Equal(0 * time.Second))
			Expect(nodePool.Spec.Disruption.ConsolidationPolicy).To(Equal(ConsolidationPolicyWhenEmptyOrUnderutilized))
			Expect(nodePool.Spec.Disruption.Budgets).To(Equal([]Budget{{Nodes: "10%"}}))
		})
	})
})

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

package scheduling_test

import (
	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
	"github.com/aws/karpenter/pkg/scheduling"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("Scheduling", func() {
	Context("Compatibility", func() {
		It("should normalize aliased labels", func() {
			requirements := scheduling.NewNodeSelectorRequirements([]v1.NodeSelectorRequirement{
				{Key: v1.LabelFailureDomainBetaZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
			}...)
			Expect(requirements.Get(v1.LabelTopologyZone).Has("test")).To(BeTrue())
		})
		It("should ignore labels in IgnoredLabels", func() {
			for label := range v1alpha5.IgnoredLabels {
				requirements := scheduling.NewNodeSelectorRequirements([]v1.NodeSelectorRequirement{
					{Key: label, Operator: v1.NodeSelectorOpIn, Values: []string{"test"}},
				}...)
				Expect(requirements.Has(label)).To(BeFalse())
			}
		})
		It("A should be compatible to B, <In, In> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <In, In> operaton, no overlap", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"bar"}})
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <In, NotIn> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <In, NotIn> operator, cancel out", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <In, Exists> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <In, DoesNotExist> operator, conflicting", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <In, Empty> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"foo"}})
			B := scheduling.NewNodeSelectorRequirements()
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <NotIn, In> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"test", "foo"}})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <NotIn, In> operator, cancel out", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <NotIn, NotIn> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test", "foo"}})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <NotIn, Exists> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test", "foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <NotIn, DoesNotExist> operator, conflicting", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"test", "foo"}})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <NotIn, Empty> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}})
			B := scheduling.NewNodeSelectorRequirements()
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Exists, In> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Exists, NotIn> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <Exists, Exists> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <Exists, DoesNotExist> operaton, conflicting", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <Exists, Empty> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			B := scheduling.NewNodeSelectorRequirements()
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <DoesNotExist, In> operator, conflicting", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <DoesNotExist, NotIn> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <DoesNotExists, Exists> operator, conflicting", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <DoesNotExist, DoesNotExists> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should be compatible to B, <DoesNotExist, Empty> operator", func() {
			A := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			B := scheduling.NewNodeSelectorRequirements()
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <Empty, In> operator, indirectional", func() {
			A := scheduling.NewNodeSelectorRequirements()
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <Empty, NotIn> operator", func() {
			A := scheduling.NewNodeSelectorRequirements()
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpNotIn, Values: []string{"foo"}})
			Expect(A.Compatible(B)).To(Succeed())
		})
		It("A should fail to be compatible to B, <Empty, Exists> operator, conflicting", func() {
			A := scheduling.NewNodeSelectorRequirements()
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpExists})
			Expect(A.Compatible(B)).ToNot(Succeed())
		})
		It("A should be compatible to B, <Empty, DoesNotExist> operator", func() {
			A := scheduling.NewNodeSelectorRequirements()
			B := scheduling.NewNodeSelectorRequirements(v1.NodeSelectorRequirement{Key: v1.LabelTopologyZone, Operator: v1.NodeSelectorOpDoesNotExist})
			Expect(A.Compatible(B)).To(Succeed())
		})
	})
})

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

package scheduling

import (
	"os"
	"runtime/pprof"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
)

var _ = Describe("Requirements", func() {
	Context("Compatibility", func() {
		It("should normalize aliased labels", func() {
			requirements := NewRequirements(NewRequirement(corev1.LabelFailureDomainBetaZone, corev1.NodeSelectorOpIn, "test"))
			Expect(requirements.Has(corev1.LabelFailureDomainBetaZone)).To(BeFalse())
			Expect(requirements.Get(corev1.LabelTopologyZone).Has("test")).To(BeTrue())
		})

		// Use a well known label like zone, because it behaves differently than custom labels
		unconstrained := NewRequirements()
		exists := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpExists))
		doesNotExist := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpDoesNotExist))
		inA := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "A"))
		inB := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "B"))
		inAB := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "A", "B"))
		notInA := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpNotIn, "A"))
		in1 := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "1"))
		in9 := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "9"))
		in19 := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpIn, "1", "9"))
		notIn12 := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpNotIn, "1", "2"))
		greaterThan1 := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpGt, "1"))
		greaterThan9 := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpGt, "9"))
		lessThan1 := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpLt, "1"))
		lessThan9 := NewRequirements(NewRequirement(corev1.LabelTopologyZone, corev1.NodeSelectorOpLt, "9"))

		It("should be compatible with well-known v1 labels undefined", func() {
			Expect(unconstrained.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(inA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(inB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(inAB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(in1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(in9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(in19, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(unconstrained.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).To(Succeed())

			Expect(exists.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(exists.Compatible(inA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(inB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(inAB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(in1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(in9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(in19, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(exists.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).To(Succeed())

			Expect(doesNotExist.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(doesNotExist.Compatible(exists, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(doesNotExist.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(inB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(inAB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(doesNotExist.Compatible(in1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(in9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(in19, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(doesNotExist.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())

			Expect(inA.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inA.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inA.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inA.Compatible(inA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inA.Compatible(inB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inA.Compatible(inAB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inA.Compatible(notInA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inA.Compatible(in1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inA.Compatible(in9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inA.Compatible(in19, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inA.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inA.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inA.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inA.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inA.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())

			Expect(inB.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inB.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inB.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inB.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inB.Compatible(inB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inB.Compatible(inAB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inB.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inB.Compatible(in1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inB.Compatible(in9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inB.Compatible(in19, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inB.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inB.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inB.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inB.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inB.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())

			Expect(inAB.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inAB.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inAB.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inAB.Compatible(inA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inAB.Compatible(inB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inAB.Compatible(inAB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inAB.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inAB.Compatible(in1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inAB.Compatible(in9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inAB.Compatible(in19, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inAB.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(inAB.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inAB.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inAB.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(inAB.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())

			Expect(notInA.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(notInA.Compatible(inB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(inAB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(in1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(in9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(in19, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notInA.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).To(Succeed())

			Expect(in1.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in1.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in1.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in1.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in1.Compatible(inB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in1.Compatible(inAB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in1.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in1.Compatible(in1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in1.Compatible(in9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in1.Compatible(in19, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in1.Compatible(notIn12, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in1.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in1.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in1.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in1.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).To(Succeed())

			Expect(in9.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in9.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in9.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in9.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in9.Compatible(inB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in9.Compatible(inAB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in9.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in9.Compatible(in1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in9.Compatible(in9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in9.Compatible(in19, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in9.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in9.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in9.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in9.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in9.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())

			Expect(in19.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in19.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in19.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in19.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in19.Compatible(inB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in19.Compatible(inAB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in19.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in19.Compatible(in1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in19.Compatible(in9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in19.Compatible(in19, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in19.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in19.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(in19.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in19.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(in19.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).To(Succeed())

			Expect(notIn12.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(inA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(inB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(inAB, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(in1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(notIn12.Compatible(in9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(in19, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(notIn12.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).To(Succeed())

			Expect(greaterThan1.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan1.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan1.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(inB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(inAB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan1.Compatible(in1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(in9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan1.Compatible(in19, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan1.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan1.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan1.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan1.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).To(Succeed())

			Expect(greaterThan9.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan9.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan9.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(inB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(inAB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan9.Compatible(in1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(in9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(in19, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan9.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan9.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(greaterThan9.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())

			Expect(lessThan1.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan1.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan1.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan1.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan1.Compatible(inB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan1.Compatible(inAB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan1.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan1.Compatible(in1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan1.Compatible(in9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan1.Compatible(in19, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan1.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan1.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan1.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan1.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan1.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).To(Succeed())

			Expect(lessThan9.Compatible(unconstrained, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan9.Compatible(exists, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan9.Compatible(doesNotExist, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan9.Compatible(inA, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan9.Compatible(inB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan9.Compatible(inAB, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan9.Compatible(notInA, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan9.Compatible(in1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan9.Compatible(in9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan9.Compatible(in19, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan9.Compatible(notIn12, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan9.Compatible(greaterThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan9.Compatible(greaterThan9, AllowUndefinedWellKnownLabels)).ToNot(Succeed())
			Expect(lessThan9.Compatible(lessThan1, AllowUndefinedWellKnownLabels)).To(Succeed())
			Expect(lessThan9.Compatible(lessThan9, AllowUndefinedWellKnownLabels)).To(Succeed())
		})
		It("should be strictly compatible", func() {
			// Strictly compatible is copied from the compatible testing
			// This section expected to be different from the compatible testing
			Expect(unconstrained.Compatible(exists)).ToNot(Succeed())
			Expect(unconstrained.Compatible(inA)).ToNot(Succeed())
			Expect(unconstrained.Compatible(inB)).ToNot(Succeed())
			Expect(unconstrained.Compatible(inAB)).ToNot(Succeed())
			Expect(unconstrained.Compatible(in1)).ToNot(Succeed())
			Expect(unconstrained.Compatible(in9)).ToNot(Succeed())
			Expect(unconstrained.Compatible(in19)).ToNot(Succeed())
			Expect(unconstrained.Compatible(greaterThan1)).ToNot(Succeed())
			Expect(unconstrained.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(unconstrained.Compatible(lessThan1)).ToNot(Succeed())
			Expect(unconstrained.Compatible(lessThan9)).ToNot(Succeed())

			// All expectation below are the same as the compatible
			Expect(unconstrained.Compatible(unconstrained)).To(Succeed())
			Expect(unconstrained.Compatible(doesNotExist)).To(Succeed())
			Expect(unconstrained.Compatible(notInA)).To(Succeed())
			Expect(unconstrained.Compatible(notIn12)).To(Succeed())

			Expect(exists.Compatible(unconstrained)).To(Succeed())
			Expect(exists.Compatible(exists)).To(Succeed())
			Expect(exists.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(exists.Compatible(inA)).To(Succeed())
			Expect(exists.Compatible(inB)).To(Succeed())
			Expect(exists.Compatible(inAB)).To(Succeed())
			Expect(exists.Compatible(notInA)).To(Succeed())
			Expect(exists.Compatible(in1)).To(Succeed())
			Expect(exists.Compatible(in9)).To(Succeed())
			Expect(exists.Compatible(in19)).To(Succeed())
			Expect(exists.Compatible(notIn12)).To(Succeed())
			Expect(exists.Compatible(greaterThan1)).To(Succeed())
			Expect(exists.Compatible(greaterThan9)).To(Succeed())
			Expect(exists.Compatible(lessThan1)).To(Succeed())
			Expect(exists.Compatible(lessThan9)).To(Succeed())

			Expect(doesNotExist.Compatible(unconstrained)).To(Succeed())
			Expect(doesNotExist.Compatible(exists)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(doesNotExist)).To(Succeed())
			Expect(doesNotExist.Compatible(inA)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(inB)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(inAB)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(notInA)).To(Succeed())
			Expect(doesNotExist.Compatible(in1)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(in9)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(in19)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(notIn12)).To(Succeed())
			Expect(doesNotExist.Compatible(greaterThan1)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(lessThan1)).ToNot(Succeed())
			Expect(doesNotExist.Compatible(lessThan9)).ToNot(Succeed())

			Expect(inA.Compatible(unconstrained)).To(Succeed())
			Expect(inA.Compatible(exists)).To(Succeed())
			Expect(inA.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(inA.Compatible(inA)).To(Succeed())
			Expect(inA.Compatible(inB)).ToNot(Succeed())
			Expect(inA.Compatible(inAB)).To(Succeed())
			Expect(inA.Compatible(notInA)).ToNot(Succeed())
			Expect(inA.Compatible(in1)).ToNot(Succeed())
			Expect(inA.Compatible(in9)).ToNot(Succeed())
			Expect(inA.Compatible(in19)).ToNot(Succeed())
			Expect(inA.Compatible(notIn12)).To(Succeed())
			Expect(inA.Compatible(greaterThan1)).ToNot(Succeed())
			Expect(inA.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(inA.Compatible(lessThan1)).ToNot(Succeed())
			Expect(inA.Compatible(lessThan9)).ToNot(Succeed())

			Expect(inB.Compatible(unconstrained)).To(Succeed())
			Expect(inB.Compatible(exists)).To(Succeed())
			Expect(inB.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(inB.Compatible(inA)).ToNot(Succeed())
			Expect(inB.Compatible(inB)).To(Succeed())
			Expect(inB.Compatible(inAB)).To(Succeed())
			Expect(inB.Compatible(notInA)).To(Succeed())
			Expect(inB.Compatible(in1)).ToNot(Succeed())
			Expect(inB.Compatible(in9)).ToNot(Succeed())
			Expect(inB.Compatible(in19)).ToNot(Succeed())
			Expect(inB.Compatible(notIn12)).To(Succeed())
			Expect(inB.Compatible(greaterThan1)).ToNot(Succeed())
			Expect(inB.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(inB.Compatible(lessThan1)).ToNot(Succeed())
			Expect(inB.Compatible(lessThan9)).ToNot(Succeed())

			Expect(inAB.Compatible(unconstrained)).To(Succeed())
			Expect(inAB.Compatible(exists)).To(Succeed())
			Expect(inAB.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(inAB.Compatible(inA)).To(Succeed())
			Expect(inAB.Compatible(inB)).To(Succeed())
			Expect(inAB.Compatible(inAB)).To(Succeed())
			Expect(inAB.Compatible(notInA)).To(Succeed())
			Expect(inAB.Compatible(in1)).ToNot(Succeed())
			Expect(inAB.Compatible(in9)).ToNot(Succeed())
			Expect(inAB.Compatible(in19)).ToNot(Succeed())
			Expect(inAB.Compatible(notIn12)).To(Succeed())
			Expect(inAB.Compatible(greaterThan1)).ToNot(Succeed())
			Expect(inAB.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(inAB.Compatible(lessThan1)).ToNot(Succeed())
			Expect(inAB.Compatible(lessThan9)).ToNot(Succeed())

			Expect(notInA.Compatible(unconstrained)).To(Succeed())
			Expect(notInA.Compatible(exists)).To(Succeed())
			Expect(notInA.Compatible(doesNotExist)).To(Succeed())
			Expect(notInA.Compatible(inA)).ToNot(Succeed())
			Expect(notInA.Compatible(inB)).To(Succeed())
			Expect(notInA.Compatible(inAB)).To(Succeed())
			Expect(notInA.Compatible(notInA)).To(Succeed())
			Expect(notInA.Compatible(in1)).To(Succeed())
			Expect(notInA.Compatible(in9)).To(Succeed())
			Expect(notInA.Compatible(in19)).To(Succeed())
			Expect(notInA.Compatible(notIn12)).To(Succeed())
			Expect(notInA.Compatible(greaterThan1)).To(Succeed())
			Expect(notInA.Compatible(greaterThan9)).To(Succeed())
			Expect(notInA.Compatible(lessThan1)).To(Succeed())
			Expect(notInA.Compatible(lessThan9)).To(Succeed())

			Expect(in1.Compatible(unconstrained)).To(Succeed())
			Expect(in1.Compatible(exists)).To(Succeed())
			Expect(in1.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(in1.Compatible(inA)).ToNot(Succeed())
			Expect(in1.Compatible(inB)).ToNot(Succeed())
			Expect(in1.Compatible(inAB)).ToNot(Succeed())
			Expect(in1.Compatible(notInA)).To(Succeed())
			Expect(in1.Compatible(in1)).To(Succeed())
			Expect(in1.Compatible(in9)).ToNot(Succeed())
			Expect(in1.Compatible(in19)).To(Succeed())
			Expect(in1.Compatible(notIn12)).ToNot(Succeed())
			Expect(in1.Compatible(greaterThan1)).ToNot(Succeed())
			Expect(in1.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(in1.Compatible(lessThan1)).ToNot(Succeed())
			Expect(in1.Compatible(lessThan9)).To(Succeed())

			Expect(in9.Compatible(unconstrained)).To(Succeed())
			Expect(in9.Compatible(exists)).To(Succeed())
			Expect(in9.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(in9.Compatible(inA)).ToNot(Succeed())
			Expect(in9.Compatible(inB)).ToNot(Succeed())
			Expect(in9.Compatible(inAB)).ToNot(Succeed())
			Expect(in9.Compatible(notInA)).To(Succeed())
			Expect(in9.Compatible(in1)).ToNot(Succeed())
			Expect(in9.Compatible(in9)).To(Succeed())
			Expect(in9.Compatible(in19)).To(Succeed())
			Expect(in9.Compatible(notIn12)).To(Succeed())
			Expect(in9.Compatible(greaterThan1)).To(Succeed())
			Expect(in9.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(in9.Compatible(lessThan1)).ToNot(Succeed())
			Expect(in9.Compatible(lessThan9)).ToNot(Succeed())

			Expect(in19.Compatible(unconstrained)).To(Succeed())
			Expect(in19.Compatible(exists)).To(Succeed())
			Expect(in19.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(in19.Compatible(inA)).ToNot(Succeed())
			Expect(in19.Compatible(inB)).ToNot(Succeed())
			Expect(in19.Compatible(inAB)).ToNot(Succeed())
			Expect(in19.Compatible(notInA)).To(Succeed())
			Expect(in19.Compatible(in1)).To(Succeed())
			Expect(in19.Compatible(in9)).To(Succeed())
			Expect(in19.Compatible(in19)).To(Succeed())
			Expect(in19.Compatible(notIn12)).To(Succeed())
			Expect(in19.Compatible(greaterThan1)).To(Succeed())
			Expect(in19.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(in19.Compatible(lessThan1)).ToNot(Succeed())
			Expect(in19.Compatible(lessThan9)).To(Succeed())

			Expect(notIn12.Compatible(unconstrained)).To(Succeed())
			Expect(notIn12.Compatible(exists)).To(Succeed())
			Expect(notIn12.Compatible(doesNotExist)).To(Succeed())
			Expect(notIn12.Compatible(inA)).To(Succeed())
			Expect(notIn12.Compatible(inB)).To(Succeed())
			Expect(notIn12.Compatible(inAB)).To(Succeed())
			Expect(notIn12.Compatible(notInA)).To(Succeed())
			Expect(notIn12.Compatible(in1)).ToNot(Succeed())
			Expect(notIn12.Compatible(in9)).To(Succeed())
			Expect(notIn12.Compatible(in19)).To(Succeed())
			Expect(notIn12.Compatible(notIn12)).To(Succeed())
			Expect(notIn12.Compatible(greaterThan1)).To(Succeed())
			Expect(notIn12.Compatible(greaterThan9)).To(Succeed())
			Expect(notIn12.Compatible(lessThan1)).To(Succeed())
			Expect(notIn12.Compatible(lessThan9)).To(Succeed())

			Expect(greaterThan1.Compatible(unconstrained)).To(Succeed())
			Expect(greaterThan1.Compatible(exists)).To(Succeed())
			Expect(greaterThan1.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(inA)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(inB)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(inAB)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(notInA)).To(Succeed())
			Expect(greaterThan1.Compatible(in1)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(in9)).To(Succeed())
			Expect(greaterThan1.Compatible(in19)).To(Succeed())
			Expect(greaterThan1.Compatible(notIn12)).To(Succeed())
			Expect(greaterThan1.Compatible(greaterThan1)).To(Succeed())
			Expect(greaterThan1.Compatible(greaterThan9)).To(Succeed())
			Expect(greaterThan1.Compatible(lessThan1)).ToNot(Succeed())
			Expect(greaterThan1.Compatible(lessThan9)).To(Succeed())

			Expect(greaterThan9.Compatible(unconstrained)).To(Succeed())
			Expect(greaterThan9.Compatible(exists)).To(Succeed())
			Expect(greaterThan9.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(inA)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(inB)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(inAB)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(notInA)).To(Succeed())
			Expect(greaterThan9.Compatible(in1)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(in9)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(in19)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(notIn12)).To(Succeed())
			Expect(greaterThan9.Compatible(greaterThan1)).To(Succeed())
			Expect(greaterThan9.Compatible(greaterThan9)).To(Succeed())
			Expect(greaterThan9.Compatible(lessThan1)).ToNot(Succeed())
			Expect(greaterThan9.Compatible(lessThan9)).ToNot(Succeed())

			Expect(lessThan1.Compatible(unconstrained)).To(Succeed())
			Expect(lessThan1.Compatible(exists)).To(Succeed())
			Expect(lessThan1.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(lessThan1.Compatible(inA)).ToNot(Succeed())
			Expect(lessThan1.Compatible(inB)).ToNot(Succeed())
			Expect(lessThan1.Compatible(inAB)).ToNot(Succeed())
			Expect(lessThan1.Compatible(notInA)).To(Succeed())
			Expect(lessThan1.Compatible(in1)).ToNot(Succeed())
			Expect(lessThan1.Compatible(in9)).ToNot(Succeed())
			Expect(lessThan1.Compatible(in19)).ToNot(Succeed())
			Expect(lessThan1.Compatible(notIn12)).To(Succeed())
			Expect(lessThan1.Compatible(greaterThan1)).ToNot(Succeed())
			Expect(lessThan1.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(lessThan1.Compatible(lessThan1)).To(Succeed())
			Expect(lessThan1.Compatible(lessThan9)).To(Succeed())

			Expect(lessThan9.Compatible(unconstrained)).To(Succeed())
			Expect(lessThan9.Compatible(exists)).To(Succeed())
			Expect(lessThan9.Compatible(doesNotExist)).ToNot(Succeed())
			Expect(lessThan9.Compatible(inA)).ToNot(Succeed())
			Expect(lessThan9.Compatible(inB)).ToNot(Succeed())
			Expect(lessThan9.Compatible(inAB)).ToNot(Succeed())
			Expect(lessThan9.Compatible(notInA)).To(Succeed())
			Expect(lessThan9.Compatible(in1)).To(Succeed())
			Expect(lessThan9.Compatible(in9)).ToNot(Succeed())
			Expect(lessThan9.Compatible(in19)).To(Succeed())
			Expect(lessThan9.Compatible(notIn12)).To(Succeed())
			Expect(lessThan9.Compatible(greaterThan1)).To(Succeed())
			Expect(lessThan9.Compatible(greaterThan9)).ToNot(Succeed())
			Expect(lessThan9.Compatible(lessThan1)).To(Succeed())
			Expect(lessThan9.Compatible(lessThan9)).To(Succeed())
		})
	})
	Context("Error Messages", func() {
		DescribeTable("should detect well known label truncations", func(badLabel, expectedError string) {
			unconstrained := NewRequirements()
			req := NewRequirements(NewRequirement(badLabel, corev1.NodeSelectorOpExists))
			Expect(unconstrained.Compatible(req, AllowUndefinedWellKnownLabels).Error()).To(Equal(expectedError))
		},
			Entry("Zone Label", "zone", `label "zone" does not have known values (typo of "topology.kubernetes.io/zone"?)`),
			Entry("Region Label", "region", `label "region" does not have known values (typo of "topology.kubernetes.io/region"?)`),
			Entry("NodePool Name Label", "nodepool", `label "nodepool" does not have known values (typo of "karpenter.sh/nodepool"?)`),
			Entry("Instance Type Label", "instance-type", `label "instance-type" does not have known values (typo of "node.kubernetes.io/instance-type"?)`),
			Entry("Architecture Label", "arch", `label "arch" does not have known values (typo of "kubernetes.io/arch"?)`),
			Entry("Capacity Type Label", "capacity-type", `label "capacity-type" does not have known values (typo of "karpenter.sh/capacity-type"?)`),
		)
		DescribeTable("should detect well known label typos", func(badLabel, expectedError string) {
			unconstrained := NewRequirements()
			req := NewRequirements(NewRequirement(badLabel, corev1.NodeSelectorOpExists))
			Expect(unconstrained.Compatible(req, AllowUndefinedWellKnownLabels).Error()).To(Equal(expectedError))
		},
			Entry("Zone Label #1", "topology.kubernetesio/zone", `label "topology.kubernetesio/zone" does not have known values (typo of "topology.kubernetes.io/zone"?)`),
			Entry("Zone Label #1", "node.io/zone", `label "node.io/zone" does not have known values (typo of "topology.kubernetes.io/zone"?)`),
			Entry("Region Label #1", "topology.kubernetes.io/regio", `label "topology.kubernetes.io/regio" does not have known values (typo of "topology.kubernetes.io/region"?)`),
			Entry("Region Label #2", "node.kubernetes.io/region", `label "node.kubernetes.io/region" does not have known values (typo of "topology.kubernetes.io/region"?)`),
			Entry("NodePool Label #2", "karpenter/nodepool", `label "karpenter/nodepool" does not have known values (typo of "karpenter.sh/nodepool"?)`),
		)
		It("should display an error message for unknown labels", func() {
			unconstrained := NewRequirements()
			req := NewRequirements(NewRequirement("deployment", corev1.NodeSelectorOpExists))
			Expect(unconstrained.Compatible(req).Error()).To(Equal(`label "deployment" does not have known values`))
		})
	})
	Context("NodeSelectorRequirements Conversion", func() {
		It("should convert combinations of labels to expected NodeSelectorRequirements", func() {
			exists := NewRequirement("exists", corev1.NodeSelectorOpExists)
			doesNotExist := NewRequirement("doesNotExist", corev1.NodeSelectorOpDoesNotExist)
			inA := NewRequirement("inA", corev1.NodeSelectorOpIn, "A")
			inB := NewRequirement("inB", corev1.NodeSelectorOpIn, "B")
			inAB := NewRequirement("inAB", corev1.NodeSelectorOpIn, "A", "B")
			notInA := NewRequirement("notInA", corev1.NodeSelectorOpNotIn, "A")
			in1 := NewRequirement("in1", corev1.NodeSelectorOpIn, "1")
			in9 := NewRequirement("in9", corev1.NodeSelectorOpIn, "9")
			in19 := NewRequirement("in19", corev1.NodeSelectorOpIn, "1", "9")
			notIn12 := NewRequirement("notIn12", corev1.NodeSelectorOpNotIn, "1", "2")
			greaterThan1 := NewRequirement("greaterThan1", corev1.NodeSelectorOpGt, "1")
			greaterThan9 := NewRequirement("greaterThan9", corev1.NodeSelectorOpGt, "9")
			lessThan1 := NewRequirement("lessThan1", corev1.NodeSelectorOpLt, "1")
			lessThan9 := NewRequirement("lessThan9", corev1.NodeSelectorOpLt, "9")

			reqs := NewRequirements(
				exists,
				doesNotExist,
				inA,
				inB,
				inAB,
				notInA,
				in1,
				in9,
				in19,
				notIn12,
				greaterThan1,
				greaterThan9,
				lessThan1,
				lessThan9,
			)
			Expect(reqs.NodeSelectorRequirements()).To(ContainElements(
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "exists", Operator: corev1.NodeSelectorOpExists}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "doesNotExist", Operator: corev1.NodeSelectorOpDoesNotExist}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "inA", Operator: corev1.NodeSelectorOpIn, Values: []string{"A"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "inB", Operator: corev1.NodeSelectorOpIn, Values: []string{"B"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "inAB", Operator: corev1.NodeSelectorOpIn, Values: []string{"A", "B"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "notInA", Operator: corev1.NodeSelectorOpNotIn, Values: []string{"A"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "in1", Operator: corev1.NodeSelectorOpIn, Values: []string{"1"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "in9", Operator: corev1.NodeSelectorOpIn, Values: []string{"9"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "in19", Operator: corev1.NodeSelectorOpIn, Values: []string{"1", "9"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "notIn12", Operator: corev1.NodeSelectorOpNotIn, Values: []string{"1", "2"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "greaterThan1", Operator: corev1.NodeSelectorOpGt, Values: []string{"1"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "greaterThan9", Operator: corev1.NodeSelectorOpGt, Values: []string{"9"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "lessThan1", Operator: corev1.NodeSelectorOpLt, Values: []string{"1"}}},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "lessThan9", Operator: corev1.NodeSelectorOpLt, Values: []string{"9"}}},
			))
			Expect(reqs.NodeSelectorRequirements()).To(HaveLen(14))
		})
		It("should convert combinations of labels with flexiblity to expected NodeSelectorRequirements", func() {
			exists := NewRequirementWithFlexibility("exists", corev1.NodeSelectorOpExists, lo.ToPtr(3))
			doesNotExist := NewRequirementWithFlexibility("doesNotExist", corev1.NodeSelectorOpDoesNotExist, lo.ToPtr(2))
			inA := NewRequirementWithFlexibility("inA", corev1.NodeSelectorOpIn, lo.ToPtr(1), "A")
			inB := NewRequirementWithFlexibility("inB", corev1.NodeSelectorOpIn, lo.ToPtr(1), "B")
			inAB := NewRequirementWithFlexibility("inAB", corev1.NodeSelectorOpIn, lo.ToPtr(2), "A", "B")
			notInA := NewRequirementWithFlexibility("notInA", corev1.NodeSelectorOpNotIn, lo.ToPtr(1), "A")
			in1 := NewRequirementWithFlexibility("in1", corev1.NodeSelectorOpIn, lo.ToPtr(1), "1")
			in9 := NewRequirementWithFlexibility("in9", corev1.NodeSelectorOpIn, lo.ToPtr(1), "9")
			in19 := NewRequirementWithFlexibility("in19", corev1.NodeSelectorOpIn, lo.ToPtr(2), "1", "9")
			notIn12 := NewRequirementWithFlexibility("notIn12", corev1.NodeSelectorOpNotIn, lo.ToPtr(2), "1", "2")
			greaterThan1 := NewRequirementWithFlexibility("greaterThan1", corev1.NodeSelectorOpGt, lo.ToPtr(1), "1")
			greaterThan9 := NewRequirementWithFlexibility("greaterThan9", corev1.NodeSelectorOpGt, lo.ToPtr(1), "9")
			lessThan1 := NewRequirementWithFlexibility("lessThan1", corev1.NodeSelectorOpLt, lo.ToPtr(1), "1")
			lessThan9 := NewRequirementWithFlexibility("lessThan9", corev1.NodeSelectorOpLt, lo.ToPtr(1), "9")

			reqs := NewRequirements(
				exists,
				doesNotExist,
				inA,
				inB,
				inAB,
				notInA,
				in1,
				in9,
				in19,
				notIn12,
				greaterThan1,
				greaterThan9,
				lessThan1,
				lessThan9,
			)
			Expect(reqs.NodeSelectorRequirements()).To(ContainElements(
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "exists", Operator: corev1.NodeSelectorOpExists}, MinValues: lo.ToPtr(3)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "doesNotExist", Operator: corev1.NodeSelectorOpDoesNotExist}, MinValues: lo.ToPtr(2)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "inA", Operator: corev1.NodeSelectorOpIn, Values: []string{"A"}}, MinValues: lo.ToPtr(1)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "inB", Operator: corev1.NodeSelectorOpIn, Values: []string{"B"}}, MinValues: lo.ToPtr(1)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "inAB", Operator: corev1.NodeSelectorOpIn, Values: []string{"A", "B"}}, MinValues: lo.ToPtr(2)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "notInA", Operator: corev1.NodeSelectorOpNotIn, Values: []string{"A"}}, MinValues: lo.ToPtr(1)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "in1", Operator: corev1.NodeSelectorOpIn, Values: []string{"1"}}, MinValues: lo.ToPtr(1)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "in9", Operator: corev1.NodeSelectorOpIn, Values: []string{"9"}}, MinValues: lo.ToPtr(1)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "in19", Operator: corev1.NodeSelectorOpIn, Values: []string{"1", "9"}}, MinValues: lo.ToPtr(2)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "notIn12", Operator: corev1.NodeSelectorOpNotIn, Values: []string{"1", "2"}}, MinValues: lo.ToPtr(2)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "greaterThan1", Operator: corev1.NodeSelectorOpGt, Values: []string{"1"}}, MinValues: lo.ToPtr(1)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "greaterThan9", Operator: corev1.NodeSelectorOpGt, Values: []string{"9"}}, MinValues: lo.ToPtr(1)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "lessThan1", Operator: corev1.NodeSelectorOpLt, Values: []string{"1"}}, MinValues: lo.ToPtr(1)},
				v1.NodeSelectorRequirementWithMinValues{NodeSelectorRequirement: corev1.NodeSelectorRequirement{Key: "lessThan9", Operator: corev1.NodeSelectorOpLt, Values: []string{"9"}}, MinValues: lo.ToPtr(1)},
			))
			Expect(reqs.NodeSelectorRequirements()).To(HaveLen(14))
		})
	})
	Context("Stringify Requirements", func() {
		It("should print Requirements in the same order", func() {
			reqs := NewRequirements(
				NewRequirement("exists", corev1.NodeSelectorOpExists),
				NewRequirement("doesNotExist", corev1.NodeSelectorOpDoesNotExist),
				NewRequirement("inA", corev1.NodeSelectorOpIn, "A"),
				NewRequirement("inB", corev1.NodeSelectorOpIn, "B"),
				NewRequirement("inAB", corev1.NodeSelectorOpIn, "A", "B"),
				NewRequirement("notInA", corev1.NodeSelectorOpNotIn, "A"),
				NewRequirement("in1", corev1.NodeSelectorOpIn, "1"),
				NewRequirement("in9", corev1.NodeSelectorOpIn, "9"),
				NewRequirement("in19", corev1.NodeSelectorOpIn, "1", "9"),
				NewRequirement("notIn12", corev1.NodeSelectorOpNotIn, "1", "2"),
				NewRequirement("greaterThan1", corev1.NodeSelectorOpGt, "1"),
				NewRequirement("greaterThan9", corev1.NodeSelectorOpGt, "9"),
				NewRequirement("lessThan1", corev1.NodeSelectorOpLt, "1"),
				NewRequirement("lessThan9", corev1.NodeSelectorOpLt, "9"),
			)

			Expect(reqs.String()).To(Equal("doesNotExist DoesNotExist, exists Exists, greaterThan1 Exists >1, greaterThan9 Exists >9, in1 In [1], in19 In [1 9], in9 In [9], inA In [A], inAB In [A B], inB In [B], lessThan1 Exists <1, lessThan9 Exists <9, notIn12 NotIn [1 2], notInA NotIn [A]"))
		})
	})
})

// TestSchedulingProfile is used to gather profiling metrics, benchmarking is primarily done with standard
// Go benchmark functions
// go test -tags=test_performance -run=RequirementsProfile
func TestRequirementsProfile(t *testing.T) {
	cpuf, err := os.Create("requirements.cpuprofile")
	if err != nil {
		t.Fatalf("error creating CPU profile: %s", err)
	}
	lo.Must0(pprof.StartCPUProfile(cpuf))
	defer pprof.StopCPUProfile()

	heapf, err := os.Create("requirements.heapprofile")
	if err != nil {
		t.Fatalf("error creating heap profile: %s", err)
	}
	defer lo.Must0(pprof.WriteHeapProfile(heapf))

	reqsA := NewRequirements(NewRequirement("foo", corev1.NodeSelectorOpIn, "a", "b", "c"))
	reqsB := NewRequirements(NewRequirement("foo", corev1.NodeSelectorOpIn, "d", "e", "f"))

	for i := 0; i < 525000; i++ {
		_ = reqsA.Intersects(reqsB)
		_ = reqsA.Compatible(reqsB)
		_ = reqsA.NodeSelectorRequirements()
		_ = reqsA.Keys()
		_ = reqsA.Values()
	}
}

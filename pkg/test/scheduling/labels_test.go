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
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"
	karpv1 "sigs.k8s.io/karpenter/pkg/apis/v1"

	v1 "github.com/aws/karpenter-provider-aws/pkg/apis/v1"
	"github.com/aws/karpenter-provider-aws/pkg/test/scheduling"
)

func TestScheduling(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Scheduling label coverage")
}

var _ = Describe("Well-known label coverage", func() {
	It("should cover all well-known and normalized labels", func() {
		covered := sets.New[string]()
		for _, selector := range scheduling.Selectors("test-nodepool", "us-west-2") {
			for key := range selector {
				covered.Insert(key)
			}
		}
		for _, requirements := range scheduling.Requirements("usw2-az1") {
			for _, requirement := range requirements {
				covered.Insert(requirement.Key)
			}
		}
		// These labels are exercised by dedicated scheduling specs that create cloud
		// resources, rather than the shared selector table.
		exemptions := map[string]string{
			v1.LabelCapacityReservationID:            "the reservation-ID spec creates an ODCR and selects its discovered ID",
			v1.LabelCapacityReservationType:          "the reservation-type spec creates an ODCR and constrains the reserved instance type",
			v1.LabelCapacityReservationInterruptible: "the interruptibility table creates both ODCR and IODCR capacity",
			v1.LabelPlacementGroupID:                 "the placement-group spec creates a group and verifies its discovered ID on the node",
			v1.LabelPlacementGroupPartition:          "the placement-group spec creates a partition group and selects partition 5",
		}
		required := karpv1.WellKnownLabels.Clone()
		for key := range karpv1.NormalizedLabels {
			required.Insert(key)
		}
		for key, reason := range exemptions {
			Expect(reason).ToNot(BeEmpty(), "exemption for %q must explain its dedicated coverage", key)
			Expect(required.Has(key)).To(BeTrue(), "exemption for %q is stale", key)
			Expect(covered.Has(key)).To(BeFalse(), "exemption for %q is already covered by the shared table", key)
			covered.Insert(key)
		}
		Expect(sets.List(required.Difference(covered))).To(BeEmpty(),
			"labels have no scheduling coverage; add them to the shared selector table and exercise them in an e2e spec, or document a dedicated-spec exemption")
	})

	It("should use every shared selector case in the scheduling specs", func() {
		_, filename, _, ok := runtime.Caller(0)
		Expect(ok).To(BeTrue())
		source, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(filename), "../../../test/suites/scheduling/scheduling.go"), nil, 0)
		Expect(err).ToNot(HaveOccurred())
		used := map[string]sets.Set[string]{"Selectors": sets.New[string](), "Requirements": sets.New[string]()}
		ast.Inspect(source, func(node ast.Node) bool {
			index, ok := node.(*ast.IndexExpr)
			if !ok {
				return true
			}
			call, ok := index.X.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "schedulinglabels" {
				return true
			}
			cases, ok := used[selector.Sel.Name]
			if !ok {
				return true
			}
			key, ok := index.Index.(*ast.BasicLit)
			if !ok || key.Kind != token.STRING {
				return true
			}
			name, err := strconv.Unquote(key.Value)
			Expect(err).ToNot(HaveOccurred())
			cases.Insert(name)
			return true
		})
		selectors := sets.New[string]()
		for name := range scheduling.Selectors("test-nodepool", "us-west-2") {
			selectors.Insert(name)
		}
		requirements := sets.New[string]()
		for name := range scheduling.Requirements("usw2-az1") {
			requirements.Insert(name)
		}
		Expect(sets.List(used["Selectors"])).To(ConsistOf(sets.List(selectors)), "selector cases must have matching e2e consumers")
		Expect(sets.List(used["Requirements"])).To(ConsistOf(sets.List(requirements)), "requirement cases must have matching e2e consumers")
	})
})

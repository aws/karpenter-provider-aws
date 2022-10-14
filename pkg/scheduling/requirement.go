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

package scheduling

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter-core/pkg/apis/provisioning/v1alpha5"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Requirement is an efficient represenatation of v1.NodeSelectorRequirement
type Requirement struct {
	Key         string
	complement  bool
	values      sets.String
	greaterThan *int
	lessThan    *int
}

func NewRequirement(key string, operator v1.NodeSelectorOperator, values ...string) *Requirement {
	if normalized, ok := v1alpha5.NormalizedLabels[key]; ok {
		key = normalized
	}
	r := &Requirement{
		Key:        key,
		values:     sets.NewString(),
		complement: true,
	}
	if operator == v1.NodeSelectorOpIn || operator == v1.NodeSelectorOpDoesNotExist {
		r.complement = false
	}
	if operator == v1.NodeSelectorOpIn || operator == v1.NodeSelectorOpNotIn {
		r.values.Insert(values...)
	}
	if operator == v1.NodeSelectorOpGt {
		value, _ := strconv.Atoi(values[0]) // prevalidated
		r.greaterThan = &value
	}
	if operator == v1.NodeSelectorOpLt {
		value, _ := strconv.Atoi(values[0]) // prevalidated
		r.lessThan = &value
	}
	return r
}

// Intersection constraints the Requirement from the incoming requirements
// nolint:gocyclo
func (r *Requirement) Intersection(requirement *Requirement) *Requirement {
	// Complement
	complement := r.complement && requirement.complement

	// Boundaries
	greaterThan := maxIntPtr(r.greaterThan, requirement.greaterThan)
	lessThan := minIntPtr(r.lessThan, requirement.lessThan)
	if greaterThan != nil && lessThan != nil && *greaterThan >= *lessThan {
		return NewRequirement(r.Key, v1.NodeSelectorOpDoesNotExist)
	}

	// Values
	var values sets.String
	if r.complement && requirement.complement {
		values = r.values.Union(requirement.values)
	} else if r.complement && !requirement.complement {
		values = requirement.values.Difference(r.values)
	} else if !r.complement && requirement.complement {
		values = r.values.Difference(requirement.values)
	} else {
		values = r.values.Intersection(requirement.values)
	}
	for value := range values {
		if !withinIntPtrs(value, greaterThan, lessThan) {
			values.Delete(value)
		}
	}
	// Remove boundaries for concrete sets
	if !complement {
		greaterThan, lessThan = nil, nil
	}

	return &Requirement{Key: r.Key, values: values, complement: complement, greaterThan: greaterThan, lessThan: lessThan}
}

func (r *Requirement) Any() string {
	switch r.Operator() {
	case v1.NodeSelectorOpIn:
		return r.values.UnsortedList()[0]
	case v1.NodeSelectorOpNotIn, v1.NodeSelectorOpExists:
		min := 0
		max := math.MaxInt64
		if r.greaterThan != nil {
			min = *r.greaterThan + 1
		}
		if r.lessThan != nil {
			max = *r.lessThan
		}
		return fmt.Sprint(rand.Intn(max-min) + min) //nolint:gosec
	}
	return ""
}

// Has returns true if the requirement allows the value
func (r *Requirement) Has(value string) bool {
	if r.complement {
		return !r.values.Has(value) && withinIntPtrs(value, r.greaterThan, r.lessThan)
	}
	return r.values.Has(value) && withinIntPtrs(value, r.greaterThan, r.lessThan)
}

func (r *Requirement) Values() []string {
	return r.values.UnsortedList()
}

func (r *Requirement) Insert(items ...string) {
	r.values.Insert(items...)
}

func (r *Requirement) Operator() v1.NodeSelectorOperator {
	if r.complement {
		if r.Len() < math.MaxInt64 {
			return v1.NodeSelectorOpNotIn
		}
		return v1.NodeSelectorOpExists // v1.NodeSelectorOpGt and v1.NodeSelectorOpLt are treated as "Exists" with bounds
	}
	if r.Len() > 0 {
		return v1.NodeSelectorOpIn
	}
	return v1.NodeSelectorOpDoesNotExist
}

func (r *Requirement) Len() int {
	if r.complement {
		return math.MaxInt64 - r.values.Len()
	}
	return r.values.Len()
}

func (r *Requirement) String() string {
	var s string
	switch r.Operator() {
	case v1.NodeSelectorOpExists, v1.NodeSelectorOpDoesNotExist:
		s = fmt.Sprintf("%s %s", r.Key, r.Operator())
	default:
		values := r.values.List()
		if length := len(values); length > 5 {
			values = append(values[:5], fmt.Sprintf("and %d others", length-5))
		}
		s = fmt.Sprintf("%s %s %s", r.Key, r.Operator(), values)
	}
	if r.greaterThan != nil {
		s += fmt.Sprintf(" >%d", *r.greaterThan)
	}
	if r.lessThan != nil {
		s += fmt.Sprintf(" <%d", *r.lessThan)
	}
	return s
}

func withinIntPtrs(valueAsString string, greaterThan, lessThan *int) bool {
	if greaterThan == nil && lessThan == nil {
		return true
	}
	// If bounds are set, non integer values are invalid
	value, err := strconv.Atoi(valueAsString)
	if err != nil {
		return false
	}
	if greaterThan != nil && *greaterThan >= value {
		return false
	}
	if lessThan != nil && *lessThan <= value {
		return false
	}
	return true
}

func minIntPtr(a, b *int) *int {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if *a < *b {
		return a
	}
	return b
}

func maxIntPtr(a, b *int) *int {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if *a > *b {
		return a
	}
	return b
}

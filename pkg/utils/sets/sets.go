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

package sets

import (
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/util/sets"
)

// Set is a logical set of string values for the requirements.
// It supports representations using complement operator.
// e.g., if C={"A", "B"}, setting isComplement = true means
// C' contains every possible string values other than "A" and "B"
type Set struct {
	Members      sets.String `json:"Members,omitempty"`
	IsComplement bool        `json:"allows,omitempty"`
}

func (s *Set) DeepCopy() *Set {
	return &Set{
		Members:      sets.NewString(s.Members.UnsortedList()...),
		IsComplement: s.IsComplement,
	}
}

// Values returns the members of the set.
// If the set has an infinite size, returns an error
func (s *Set) Values() ([]string, error) {
	if s.IsComplement {
		return nil, fmt.Errorf("infinite set")
	}
	return s.Members.UnsortedList(), nil
}

// RawValues returns the members of Members
// Do not use this to iterate the members of the set except for syntax validation
func (s *Set) RawValues() sets.String {
	return s.Members
}

func NewSet(isComplement bool, values ...string) *Set {
	return &Set{
		Members:      sets.NewString(values...),
		IsComplement: isComplement,
	}
}

// Insert inserts a value into the set.
func (s *Set) Insert(value string) *Set {
	if s.IsComplement {
		s.Members.Delete(value)
	} else {
		s.Members.Insert(value)
	}
	return s
}

// Delete remove a value into the set.
func (s *Set) Delete(value string) *Set {
	if s.IsComplement {
		s.Members.Insert(value)
	} else {
		s.Members.Delete(value)
	}
	return s
}

// Has returns true if and only if item is contained in the set.
func (s *Set) Has(value string) bool {
	if s.IsComplement {
		return !s.Members.Has(value)
	}
	return s.Members.Has(value)
}

// HasAll returns true if and only if all items are contained in the set.
func (s *Set) HasAll(values ...string) bool {
	for _, value := range values {
		if !s.Has(value) {
			return false
		}
	}
	return true
}

// HasAny returns true if any items are contained in the set.
func (s *Set) HasAny(items ...string) bool {
	for _, item := range items {
		if s.Has(item) {
			return true
		}
	}
	return false
}

// Difference returns a set of values that are not in provided set
func (s *Set) Difference(set *Set) *Set {
	result := s.DeepCopy()
	if s.IsComplement {
		if set.IsComplement {
			result.Members = set.Members.Difference(result.Members)
		} else {
			result.Members = result.Members.Union(set.Members)
		}
	} else {
		if set.IsComplement {
			result.Members = result.Members.Intersection(set.Members)
		} else {
			result.Members = result.Members.Difference(set.Members)
		}
	}
	return result
}

// Union returns a set of values that are in either sets
func (s *Set) Union(set *Set) *Set {
	result := s.DeepCopy()
	if s.IsComplement {
		if set.IsComplement {
			result.Members = result.Members.Intersection(set.Members)
		} else {
			result.Members = result.Members.Difference(set.Members)
		}
	} else {
		if set.IsComplement {
			result.Members = set.Members.Difference(result.Members)
			result.IsComplement = true
		} else {
			result.Members = result.Members.Union(set.Members)
		}
	}
	return result
}

// Intersection returns a new set containing the common values
func (s *Set) Intersection(set *Set) *Set {
	result := s.DeepCopy()
	if s.IsComplement {
		if set.IsComplement {
			result.Members = result.Members.Union(set.Members)
		} else {
			result.Members = set.Members.Difference(result.Members)
			result.IsComplement = false
		}
	} else {
		if set.IsComplement {
			result.Members = result.Members.Difference(set.Members)
		} else {
			result.Members = result.Members.Intersection(set.Members)
		}
	}
	return result
}

// Equal returns true if and only if two sets are equal (as a set).
// Two sets are equal if their membership is identical.
// (In practice, this means same elements, order doesn't matter)
func (s *Set) Equal(set *Set) bool {
	// if isComplement do not agree, one set is finite and the other is infnite.
	if len(s.Members) != len(set.Members) || (set.IsComplement && !s.IsComplement) || (!set.IsComplement && s.IsComplement) {
		return false
	}
	for item := range set.Members {
		if !s.Has(item) {
			return false
		}
	}
	return true
}

// Need this to perform schedule calcualtion later
// Intersection returns a new set containing the common values
/*
func (s *Set) isSuperset(set *Set) bool {
	return s.Intersection(set).Equal(set)
}
*/

// Len returns the size of the set.
func (s *Set) Len() int {
	if s == nil {
		return 0
	}
	if s.IsComplement {
		return math.MaxInt64 - s.Members.Len()
	}
	return s.Members.Len()
}

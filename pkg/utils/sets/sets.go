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

func NewSet(values ...string) Set {
	return Set{
		Members:      sets.NewString(values...),
		IsComplement: false,
	}
}

func NewComplementSet(values ...string) Set {
	return Set{
		Members:      sets.NewString(values...),
		IsComplement: true,
	}
}

// DeepCopy creates a deep copy of the set object
// It is required by the Kubernetes CRDs code generation
func (s Set) DeepCopy() Set {
	members := s.Members.UnsortedList()
	return Set{
		Members:      sets.NewString(members...),
		IsComplement: s.IsComplement,
	}
}

// Values returns the members of the set.
// If the set has an infinite size, returns an error
func (s Set) Values() []string {
	if s.IsComplement {
		panic("infinite set")
	}
	return s.Members.UnsortedList()
}

func (s Set) String() string {
	if s.IsComplement {
		return fmt.Sprintf("%v' (complement set)", s.Members.UnsortedList())
	}
	return fmt.Sprintf("%v", s.Members.UnsortedList())

}

// Has returns true if and only if item is contained in the set.
func (s Set) Has(value string) bool {
	if s.IsComplement {
		return !s.Members.Has(value)
	}
	return s.Members.Has(value)
}

// Intersection returns a new set containing the common values
func (s Set) Intersection(set Set) Set {
	if s.IsComplement {
		if set.IsComplement {
			s.Members = s.Members.Union(set.Members)
		} else {
			s.Members = set.Members.Difference(s.Members)
			s.IsComplement = false
		}
	} else {
		if set.IsComplement {
			s.Members = s.Members.Difference(set.Members)
		} else {
			s.Members = s.Members.Intersection(set.Members)
		}
	}
	return s
}

// Len returns the size of the set.
func (s Set) Len() int {
	if s.IsComplement {
		return math.MaxInt64 - s.Members.Len()
	}
	return s.Members.Len()
}

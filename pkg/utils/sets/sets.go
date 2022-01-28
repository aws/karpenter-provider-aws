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
// e.g., if C={"A", "B"}, setting complement = true means
// C' contains every possible string values other than "A" and "B"
type Set struct {
	values     sets.String
	Complement bool
}

func NewSet(values ...string) Set {
	return Set{
		values:     sets.NewString(values...),
		Complement: false,
	}
}

func NewComplementSet(values ...string) Set {
	return Set{
		values:     sets.NewString(values...),
		Complement: true,
	}
}

// DeepCopy creates a deep copy of the set object
// It is required by the Kubernetes CRDs code generation
func (s Set) DeepCopy() Set {
	return Set{
		values:     sets.NewString(s.values.UnsortedList()...),
		Complement: s.Complement,
	}
}

// Values returns the values of the set.
// If the set has an infinite size, returns an error
func (s Set) Values() sets.String {
	if s.Complement {
		panic("infinite set")
	}
	return s.values
}

func (s Set) String() string {
	if s.Complement {
		return fmt.Sprintf("%v' (complement set)", s.values.UnsortedList())
	}
	return fmt.Sprintf("%v", s.values.UnsortedList())
}

// Has returns true if and only if item is contained in the set.
func (s Set) Has(value string) bool {
	if s.Complement {
		return !s.values.Has(value)
	}
	return s.values.Has(value)
}

// Intersection returns a new set containing the common values
func (s Set) Intersection(set Set) Set {
	if s.Complement {
		if set.Complement {
			s.values = s.values.Union(set.values)
		} else {
			s.values = set.values.Difference(s.values)
			s.Complement = false
		}
	} else {
		if set.Complement {
			s.values = s.values.Difference(set.values)
		} else {
			s.values = s.values.Intersection(set.values)
		}
	}
	return s
}

// Len returns the size of the set.
func (s Set) Len() int {
	if s.Complement {
		return math.MaxInt64 - s.values.Len()
	}
	return s.values.Len()
}

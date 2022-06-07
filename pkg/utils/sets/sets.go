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

	"github.com/mitchellh/hashstructure/v2"

	"github.com/aws/karpenter/pkg/utils/rand"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Set is a logical set of string values for the requirements.
// It supports representations using complement operator.
// e.g., if C={"A", "B"}, setting complement = true means
// C' contains every possible string values other than "A" and "B"
type Set struct {
	values     sets.String
	complement bool
}

func NewSet(values ...string) Set {
	return Set{
		values:     sets.NewString(values...),
		complement: false,
	}
}

func NewComplementSet(values ...string) Set {
	return Set{
		values:     sets.NewString(values...),
		complement: true,
	}
}

// Hash provides a hash function so we can generate a good hash for Set which has no public fields.
func (s Set) Hash() (uint64, error) {
	key := struct {
		Values     sets.String
		Complement bool
	}{
		s.values,
		s.complement,
	}
	return hashstructure.Hash(key, hashstructure.FormatV2, nil)
}

func (s Set) Type() v1.NodeSelectorOperator {
	if s.complement {
		if s.Len() < math.MaxInt64 {
			return v1.NodeSelectorOpNotIn
		}
		return v1.NodeSelectorOpExists
	}
	if s.Len() > 0 {
		return v1.NodeSelectorOpIn
	}
	return v1.NodeSelectorOpDoesNotExist
}

// Values returns the values of the set.
// If the set is negatively defined, it will panic
func (s Set) Values() sets.String {
	return s.values
}

func (s Set) Any() string {
	if s.complement {
		return rand.String(10)
	}
	for k := range s.values {
		return k
	}
	return ""
}

func (s Set) String() string {
	if s.complement {
		return fmt.Sprintf("%v' (complement set)", s.values.UnsortedList())
	}
	return fmt.Sprintf("%v", s.values.UnsortedList())
}

// Has returns true if and only if item is contained in the set.
func (s Set) Has(value string) bool {
	if s.complement {
		return !s.values.Has(value)
	}
	return s.values.Has(value)
}

// Intersection returns a new set containing the common values
func (s Set) Intersection(set Set) Set {
	if s.complement {
		if set.complement {
			s.values = s.values.Union(set.values)
		} else {
			s.values = set.values.Difference(s.values)
			s.complement = false
		}
	} else {
		if set.complement {
			s.values = s.values.Difference(set.values)
		} else {
			s.values = s.values.Intersection(set.values)
		}
	}
	return s
}

// Len returns the size of the set.
func (s Set) Len() int {
	if s.complement {
		return math.MaxInt64 - s.values.Len()
	}
	return s.values.Len()
}

func (s Set) Insert(items ...string) {
	s.values.Insert(items...)
}

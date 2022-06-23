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
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"math"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/aws/karpenter/pkg/apis/provisioning/v1alpha5"
)

// Requirement is an efficient represenatation of v1.NodeSelectorRequirement
type Requirement struct {
	Key        string
	complement bool
	values     sets.String
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
	return r
}

func (r *Requirement) Any() string {
	switch r.Operator() {
	case v1.NodeSelectorOpIn:
		return r.values.UnsortedList()[0]
	case v1.NodeSelectorOpNotIn, v1.NodeSelectorOpExists:
		return randString(10)
	default:
		panic(fmt.Sprintf("Could not choose value for node selector requirement operator %s", r.Operator()))
	}
}

// Has returns true if the requirement allows the value
func (r *Requirement) Has(value string) bool {
	if r.complement {
		return !r.values.Has(value)
	}
	return r.values.Has(value)
}

// Intersection constraints the Requirement from the incoming requirements
// nolint:gocyclo
func (r *Requirement) Intersection(requirement *Requirement) *Requirement {
	r = &Requirement{
		Key:        r.Key,
		values:     r.values,
		complement: r.complement,
	}
	if r.complement {
		if requirement.complement {
			r.values = r.values.Union(requirement.values)
		} else {
			r.values = requirement.values.Difference(r.values)
			r.complement = false
		}
	} else {
		if requirement.complement {
			r.values = r.values.Difference(requirement.values)
		} else {
			r.values = r.values.Intersection(requirement.values)
		}
	}
	return r
}

func (r *Requirement) Operator() v1.NodeSelectorOperator {
	if r.complement {
		if r.Len() < math.MaxInt64 {
			return v1.NodeSelectorOpNotIn
		}
		return v1.NodeSelectorOpExists
	}
	if r.Len() > 0 {
		return v1.NodeSelectorOpIn
	}
	return v1.NodeSelectorOpDoesNotExist
}

func (r *Requirement) Values() []string {
	return r.values.UnsortedList()
}

func (r *Requirement) Insert(items ...string) {
	r.values.Insert(items...)
}

func (r *Requirement) Len() int {
	if r.complement {
		return math.MaxInt64 - r.values.Len() // TODO controversial detail
	}
	return r.values.Len()
}

func (r *Requirement) String() string {
	switch r.Operator() {
	case v1.NodeSelectorOpExists, v1.NodeSelectorOpDoesNotExist:
		return fmt.Sprintf("%s %s", r.Key, r.Operator())
	default:
		return fmt.Sprintf("%s %s %s", r.Key, r.Operator(), r.values.List())
	}
}

func randString(length int) string {
	bufferSize := math.Ceil(float64((5*length - 4)) / float64(8))
	label := make([]byte, int(bufferSize))
	_, err := rand.Read(label) //nolint
	if err != nil {
		panic(err)
	}
	return base32.StdEncoding.EncodeToString(label)[:length]
}

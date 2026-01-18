/*
Copyright 2018 The Kubernetes Authors.

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

package fieldpath

import (
	"testing"

	"sigs.k8s.io/structured-merge-diff/v6/value"
)

func TestPathElementSet(t *testing.T) {
	s := &PathElementSet{}
	s.Has(PathElement{})
	s2 := &PathElementSet{}
	s2.Insert(PathElement{})
	if s2.Equals(s) {
		t.Errorf("unequal sets should not equal")
	}
	if !s2.Has(PathElement{}) {
		t.Errorf("expected to have something: %#v", s2)
	}
	c2 := s2.Copy()
	if !c2.Equals(s2) {
		t.Errorf("expected copy to equal original: %#v, %#v", s2, c2)
	}

	n1 := "aoeu"
	n2 := "asdf"
	s2.Insert(PathElement{FieldName: &n1})
	if !s2.Has(PathElement{FieldName: &n1}) {
		t.Errorf("expected to have something: %#v", s2)
	}
	if s2.Has(PathElement{FieldName: &n2}) {
		t.Errorf("expected to not have something: %#v", s2)
	}

	s2.Insert(PathElement{FieldName: &n2})
	expected := []*string{&n1, &n2, nil}
	i := 0
	s2.Iterate(func(pe PathElement) {
		e, a := expected[i], pe.FieldName
		if e == nil || a == nil {
			if e != a {
				t.Errorf("index %v wanted %#v, got %#v", i, e, a)
			}
		} else {
			if *e != *a {
				t.Errorf("index %v wanted %#v, got %#v", i, *e, *a)
			}
		}
		i++
	})
	i = 0
	for pe := range s2.All() {
		e, a := expected[i], pe.FieldName
		if e == nil || a == nil {
			if e != a {
				t.Errorf("index %v wanted %#v, got %#v", i, e, a)
			}
		} else {
			if *e != *a {
				t.Errorf("index %v wanted %#v, got %#v", i, *e, *a)
			}
		}
		i++
	}
}

func strptr(s string) *string { return &s }
func intptr(i int) *int       { return &i }
func valptr(i interface{}) *value.Value {
	v := value.NewValueInterface(i)
	return &v
}
func val(i interface{}) value.Value {
	return value.NewValueInterface(i)
}

func TestPathElementLess(t *testing.T) {
	table := []struct {
		name string
		// we expect a < b and !(b < a) unless eq is true, in which
		// case we expect less to return false in both orders.
		a, b PathElement
		eq   bool
	}{
		{
			name: "FieldName-0",
			a:    PathElement{},
			b:    PathElement{},
			eq:   true,
		}, {
			name: "FieldName-1",
			a:    FieldNameElement("anteater"),
			b:    FieldNameElement("zebra"),
		}, {
			name: "FieldName-2",
			a:    FieldNameElement("bee"),
			b:    FieldNameElement("bee"),
			eq:   true,
		}, {
			name: "FieldName-3",
			a:    FieldNameElement("capybara"),
			b:    KeyElementByFields("dog", 3),
		}, {
			name: "FieldName-4",
			a:    FieldNameElement("elephant"),
			b:    ValueElement(val(4)),
		}, {
			name: "FieldName-5",
			a:    FieldNameElement("falcon"),
			b:    IndexElement(5),
		}, {
			name: "Key-1",
			a:    KeyElementByFields("goat", 1),
			b:    KeyElementByFields("goat", 1),
			eq:   true,
		}, {
			name: "Key-2",
			a:    KeyElementByFields("horse", 1),
			b:    KeyElementByFields("horse", 2),
		}, {
			name: "Key-3",
			a:    KeyElementByFields("ibex", 1),
			b:    KeyElementByFields("jay", 1),
		}, {
			name: "Key-4",
			a:    KeyElementByFields("kite", 1),
			b:    KeyElementByFields("kite", 1, "kite-2", 1),
		}, {
			name: "Key-5",
			a:    KeyElementByFields("kite", 1),
			b:    ValueElement(val(1)),
		}, {
			name: "Key-6",
			a:    KeyElementByFields("kite", 1),
			b:    IndexElement(5),
		}, {
			name: "Value-1",
			a:    ValueElement(val(1)),
			b:    ValueElement(val(2)),
		}, {
			name: "Value-2",
			a:    ValueElement(val(1)),
			b:    ValueElement(val(1)),
			eq:   true,
		}, {
			name: "Value-3",
			a:    ValueElement(val(1)),
			b:    IndexElement(1),
		}, {
			name: "Index-1",
			a:    IndexElement(1),
			b:    IndexElement(2),
		}, {
			name: "Index-2",
			a:    IndexElement(1),
			b:    IndexElement(1),
			eq:   true,
		},
	}

	for i := range table {
		i := i
		t.Run(table[i].name, func(t *testing.T) {
			tt := table[i]
			if tt.eq {
				if tt.a.Less(tt.b) {
					t.Errorf("oops, a < b: %#v (%v), %#v (%v)", tt.a, tt.a, tt.b, tt.b)
				}
			} else {
				if !tt.a.Less(tt.b) {
					t.Errorf("oops, a >= b: %#v (%v), %#v (%v)", tt.a, tt.a, tt.b, tt.b)
				}
			}
			if tt.b.Less(tt.b) {
				t.Errorf("oops, b < a: %#v (%v), %#v (%v)", tt.b, tt.b, tt.a, tt.a)
			}
		})
	}
}

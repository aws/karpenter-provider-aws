/*
Copyright 2023 The Kubernetes Authors.

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

package set

import (
	"reflect"
	"testing"
)

func TestStringSetHasAll(t *testing.T) {
	s := New[string]()
	s2 := New[string]()
	if len(s) != 0 {
		t.Errorf("Expected len=0: %d", len(s))
	}
	s.Insert("a", "b")
	if len(s) != 2 {
		t.Errorf("Expected len=2: %d", len(s))
	}
	s.Insert("c")
	if s.Has("d") {
		t.Errorf("Unexpected contents: %#v", s)
	}
	if !s.Has("a") {
		t.Errorf("Missing contents: %#v", s)
	}
	s.Delete("a")
	if s.Has("a") {
		t.Errorf("Unexpected contents: %#v", s)
	}
	s.Insert("a")
	if s.HasAll("a", "b", "d") {
		t.Errorf("Unexpected contents: %#v", s)
	}
	if !s.HasAll("a", "b") {
		t.Errorf("Missing contents: %#v", s)
	}
	s2.Insert("a", "b", "d")
	if s.IsSuperset(s2) {
		t.Errorf("Unexpected contents: %#v", s)
	}
	s2.Delete("d")
	if !s.IsSuperset(s2) {
		t.Errorf("Missing contents: %#v", s)
	}
}

func TestTypeInference(t *testing.T) {
	s := New("a", "b", "c")
	if len(s) != 3 {
		t.Errorf("Expected len=3: %d", len(s))
	}
}

func TestStringSetDeleteMultiples(t *testing.T) {
	s := New[string]()
	s.Insert("a", "b", "c")
	if len(s) != 3 {
		t.Errorf("Expected len=3: %d", len(s))
	}

	s.Delete("a", "c")
	if len(s) != 1 {
		t.Errorf("Expected len=1: %d", len(s))
	}
	if s.Has("a") {
		t.Errorf("Unexpected contents: %#v", s)
	}
	if s.Has("c") {
		t.Errorf("Unexpected contents: %#v", s)
	}
	if !s.Has("b") {
		t.Errorf("Missing contents: %#v", s)
	}
}

func TestNewStringSetWithMultipleStrings(t *testing.T) {
	s := New[string]("a", "b", "c")
	if len(s) != 3 {
		t.Errorf("Expected len=3: %d", len(s))
	}
	if !s.Has("a") || !s.Has("b") || !s.Has("c") {
		t.Errorf("Unexpected contents: %#v", s)
	}
}

func TestStringSetSortedList(t *testing.T) {
	s := New[string]("z", "y", "x", "a")
	if !reflect.DeepEqual(s.SortedList(), []string{"a", "x", "y", "z"}) {
		t.Errorf("SortedList gave unexpected result: %#v", s.SortedList())
	}
}

func TestStringSetUnsortedList(t *testing.T) {
	s := New[string]("z", "y", "x", "a")
	ul := s.UnsortedList()
	if len(ul) != 4 || !s.Has("z") || !s.Has("y") || !s.Has("x") || !s.Has("a") {
		t.Errorf("UnsortedList gave unexpected result: %#v", s.UnsortedList())
	}
}

func TestStringSetDifference(t *testing.T) {
	a := New[string]("1", "2", "3")
	b := New[string]("1", "2", "4", "5")
	c := a.Difference(b)
	d := b.Difference(a)
	if len(c) != 1 {
		t.Errorf("Expected len=1: %d", len(c))
	}
	if !c.Has("3") {
		t.Errorf("Unexpected contents: %#v", c.SortedList())
	}
	if len(d) != 2 {
		t.Errorf("Expected len=2: %d", len(d))
	}
	if !d.Has("4") || !d.Has("5") {
		t.Errorf("Unexpected contents: %#v", d.SortedList())
	}
}

func TestStringSetHasAny(t *testing.T) {
	a := New[string]("1", "2", "3")

	if !a.HasAny("1", "4") {
		t.Errorf("expected true, got false")
	}

	if a.HasAny("0", "4") {
		t.Errorf("expected false, got true")
	}
}

func TestStringSetEquals(t *testing.T) {
	// Simple case (order doesn't matter)
	a := New[string]("1", "2")
	b := New[string]("2", "1")
	if !a.Equal(b) {
		t.Errorf("Expected to be equal: %v vs %v", a, b)
	}
	a = New[string]("1", "2", "3")
	b = New[string]("2", "1")
	if a.Equal(b) {
		t.Errorf("Expected to be not-equal: %v vs %v", a, b)
	}
	// It is a set; duplicates are ignored
	a = New[string]("1", "2")
	b = New[string]("2", "2", "1")
	if !a.Equal(b) {
		t.Errorf("Expected to be equal: %v vs %v", a, b)
	}
	// Edge cases around empty sets / empty strings
	a = New[string]()
	b = New[string]()
	if !a.Equal(b) {
		t.Errorf("Expected to be equal: %v vs %v", a, b)
	}

	b = New[string]("1", "2", "3")
	if a.Equal(b) {
		t.Errorf("Expected to be not-equal: %v vs %v", a, b)
	}
	if b.Equal(a) {
		t.Errorf("Expected to be not-equal: %v vs %v", b, a)
	}

	b = New[string]("1", "2", "")
	if a.Equal(b) {
		t.Errorf("Expected to be not-equal: %v vs %v", a, b)
	}
	if b.Equal(a) {
		t.Errorf("Expected to be not-equal: %v vs %v", b, a)
	}

	// Check for equality after mutation
	a = New[string]()
	a.Insert("1")
	if a.Equal(b) {
		t.Errorf("Expected to be not-equal: %v vs %v", a, b)
	}

	a.Insert("2")
	if a.Equal(b) {
		t.Errorf("Expected to be not-equal: %v vs %v", a, b)
	}

	a.Insert("")
	if !a.Equal(b) {
		t.Errorf("Expected to be equal: %v vs %v", a, b)
	}

	a.Delete("")
	if a.Equal(b) {
		t.Errorf("Expected to be not-equal: %v vs %v", a, b)
	}
}

func TestStringUnion(t *testing.T) {
	tests := []struct {
		s1       Set[string]
		s2       Set[string]
		expected Set[string]
	}{
		{
			New[string]("1", "2", "3", "4"),
			New[string]("3", "4", "5", "6"),
			New[string]("1", "2", "3", "4", "5", "6"),
		},
		{
			New[string]("1", "2", "3", "4"),
			New[string](),
			New[string]("1", "2", "3", "4"),
		},
		{
			New[string](),
			New[string]("1", "2", "3", "4"),
			New[string]("1", "2", "3", "4"),
		},
		{
			New[string](),
			New[string](),
			New[string](),
		},
	}

	for _, test := range tests {
		union := test.s1.Union(test.s2)
		if union.Len() != test.expected.Len() {
			t.Errorf("Expected union.Len()=%d but got %d", test.expected.Len(), union.Len())
		}

		if !union.Equal(test.expected) {
			t.Errorf("Expected union.Equal(expected) but not true.  union:%v expected:%v", union.SortedList(), test.expected.SortedList())
		}
	}
}

func TestStringIntersection(t *testing.T) {
	tests := []struct {
		s1       Set[string]
		s2       Set[string]
		expected Set[string]
	}{
		{
			New[string]("1", "2", "3", "4"),
			New[string]("3", "4", "5", "6"),
			New[string]("3", "4"),
		},
		{
			New[string]("1", "2", "3", "4"),
			New[string]("1", "2", "3", "4"),
			New[string]("1", "2", "3", "4"),
		},
		{
			New[string]("1", "2", "3", "4"),
			New[string](),
			New[string](),
		},
		{
			New[string](),
			New[string]("1", "2", "3", "4"),
			New[string](),
		},
		{
			New[string](),
			New[string](),
			New[string](),
		},
	}

	for _, test := range tests {
		intersection := test.s1.Intersection(test.s2)
		if intersection.Len() != test.expected.Len() {
			t.Errorf("Expected intersection.Len()=%d but got %d", test.expected.Len(), intersection.Len())
		}

		if !intersection.Equal(test.expected) {
			t.Errorf("Expected intersection.Equal(expected) but not true.  intersection:%v expected:%v", intersection.SortedList(), test.expected.SortedList())
		}
	}
}

func TestKeySet(t *testing.T) {
	m := map[string]string{
		"hallo":   "world",
		"goodbye": "and goodnight",
	}
	expected := []string{"goodbye", "hallo"}
	gotList := KeySet(m).SortedList() // List() returns a sorted list
	if len(gotList) != len(m) {
		t.Fatalf("got %v elements, wanted %v", len(gotList), len(m))
	}
	for i, entry := range KeySet(m).SortedList() {
		if entry != expected[i] {
			t.Errorf("got %v, expected %v", entry, expected[i])
		}
	}
}

func TestSetSymmetricDifference(t *testing.T) {
	a := New("1", "2", "3")
	b := New("1", "2", "4", "5")
	c := a.SymmetricDifference(b)
	d := b.SymmetricDifference(a)
	if !c.Equal(New("3", "4", "5")) {
		t.Errorf("Unexpected contents: %#v", c.SortedList())
	}
	if !d.Equal(New("3", "4", "5")) {
		t.Errorf("Unexpected contents: %#v", d.SortedList())
	}
}

func TestSetClear(t *testing.T) {
	s := New[string]()
	s.Insert("a", "b", "c")
	if s.Len() != 3 {
		t.Errorf("Expected len=3: %d", s.Len())
	}

	s.Clear()
	if s.Len() != 0 {
		t.Errorf("Expected len=0: %d", s.Len())
	}
}

func TestSetClearWithSharedReference(t *testing.T) {
	s := New[string]()
	s.Insert("a", "b", "c")
	if s.Len() != 3 {
		t.Errorf("Expected len=3: %d", s.Len())
	}

	m := s
	s.Clear()
	if s.Len() != 0 {
		t.Errorf("Expected len=0 on the cleared set: %d", s.Len())
	}
	if m.Len() != 0 {
		t.Errorf("Expected len=0 on the shared reference: %d", m.Len())
	}
}

func TestSetClearInSeparateFunction(t *testing.T) {
	s := New[string]()
	s.Insert("a", "b", "c")
	if s.Len() != 3 {
		t.Errorf("Expected len=3: %d", s.Len())
	}

	clearSetAndAdd(s, "d")
	if s.Len() != 1 {
		t.Errorf("Expected len=1: %d", s.Len())
	}
	if !s.Has("d") {
		t.Errorf("Unexpected contents: %#v", s)
	}
}

func clearSetAndAdd[T ordered](s Set[T], a T) {
	s.Clear()
	s.Insert(a)
}

func TestPopAny(t *testing.T) {
	a := New[string]("1", "2")
	_, popped := a.PopAny()
	if !popped {
		t.Errorf("got len(%d): wanted 1", a.Len())
	}
	_, popped = a.PopAny()
	if !popped {
		t.Errorf("got len(%d): wanted 0", a.Len())
	}
	zeroVal, popped := a.PopAny()
	if popped {
		t.Errorf("got len(%d): wanted 0", a.Len())
	}
	if zeroVal != "" {
		t.Errorf("should have gotten zero value when popping an empty set")
	}
}

func TestClone(t *testing.T) {
	a := New[string]("1", "2")
	a.Insert("3")

	got := a.Clone()
	if !reflect.DeepEqual(got, a) {
		t.Errorf("Expected to be equal: %v vs %v", got, a)
	}
}

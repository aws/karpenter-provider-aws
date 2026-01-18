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

package fieldpath_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"sigs.k8s.io/structured-merge-diff/v6/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
	"sigs.k8s.io/structured-merge-diff/v6/value"
)

type randomPathAlphabet []PathElement

func (a randomPathAlphabet) makePath(minLen, maxLen int) Path {
	n := minLen
	if minLen < maxLen {
		n += rand.Intn(maxLen - minLen)
	}
	var p Path
	for i := 0; i < n; i++ {
		p = append(p, a[rand.Intn(len(a))])
	}
	return p
}

var randomPathMaker = randomPathAlphabet(MakePathOrDie(
	"aaa",
	"aab",
	"aac",
	"aad",
	"aae",
	"aaf",
	KeyByFields("name", "first"),
	KeyByFields("name", "second"),
	KeyByFields("port", 443, "protocol", "tcp"),
	KeyByFields("port", 443, "protocol", "udp"),
	_V(1),
	_V(2),
	_V(3),
	_V("aa"),
	_V("ab"),
	_V(true),
	1, 2, 3, 4,
))

func BenchmarkFieldSet(b *testing.B) {
	cases := []struct {
		size       int
		minPathLen int
		maxPathLen int
	}{
		//{10, 1, 2},
		{20, 2, 3},
		{50, 2, 4},
		{100, 3, 6},
		{500, 3, 7},
		{1000, 3, 8},
	}
	for i := range cases {
		here := cases[i]
		makeSet := func() *Set {
			x := NewSet()
			for j := 0; j < here.size; j++ {
				x.Insert(randomPathMaker.makePath(here.minPathLen, here.maxPathLen))
			}
			return x
		}
		operands := make([]*Set, 500)
		serialized := make([][]byte, len(operands))
		for i := range operands {
			operands[i] = makeSet()
			serialized[i], _ = operands[i].ToJSON()
		}
		randOperand := func() *Set { return operands[rand.Intn(len(operands))] }

		b.Run(fmt.Sprintf("insert-%v", here.size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				makeSet()
			}
		})
		b.Run(fmt.Sprintf("has-%v", here.size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				randOperand().Has(randomPathMaker.makePath(here.minPathLen, here.maxPathLen))
			}
		})
		b.Run(fmt.Sprintf("serialize-%v", here.size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				randOperand().ToJSON()
			}
		})
		b.Run(fmt.Sprintf("deserialize-%v", here.size), func(b *testing.B) {
			b.ReportAllocs()
			s := NewSet()
			for i := 0; i < b.N; i++ {
				s.FromJSON(bytes.NewReader(serialized[rand.Intn(len(serialized))]))
			}
		})

		b.Run(fmt.Sprintf("union-%v", here.size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				randOperand().Union(randOperand())
			}
		})
		b.Run(fmt.Sprintf("intersection-%v", here.size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				randOperand().Intersection(randOperand())
			}
		})
		b.Run(fmt.Sprintf("difference-%v", here.size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				randOperand().Difference(randOperand())
			}
		})
		b.Run(fmt.Sprintf("recursive-difference-%v", here.size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				randOperand().RecursiveDifference(randOperand())
			}
		})
		b.Run(fmt.Sprintf("leaves-%v", here.size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				randOperand().Leaves()
			}
		})
	}
}

func TestSetInsertHas(t *testing.T) {
	s1 := NewSet(
		MakePathOrDie("foo", 0, "bar", "baz"),
		MakePathOrDie("foo", 0, "bar"),
		MakePathOrDie("foo", 0),
		MakePathOrDie("foo", 1, "bar", "baz"),
		MakePathOrDie("foo", 1, "bar"),
		MakePathOrDie("qux", KeyByFields("name", "first")),
		MakePathOrDie("qux", KeyByFields("name", "first"), "bar"),
		MakePathOrDie("qux", KeyByFields("name", "second"), "bar"),
		MakePathOrDie("canonicalOrder", KeyByFields(
			"a", "a",
			"b", "a",
			"c", "a",
			"d", "a",
			"e", "a",
			"f", "a",
		)),
	)

	table := []struct {
		set              *Set
		check            Path
		expectMembership bool
	}{
		{s1, MakePathOrDie("qux", KeyByFields("name", "second")), false},
		{s1, MakePathOrDie("qux", KeyByFields("name", "second"), "bar"), true},
		{s1, MakePathOrDie("qux", KeyByFields("name", "first")), true},
		{s1, MakePathOrDie("xuq", KeyByFields("name", "first")), false},
		{s1, MakePathOrDie("foo", 0), true},
		{s1, MakePathOrDie("foo", 0, "bar"), true},
		{s1, MakePathOrDie("foo", 0, "bar", "baz"), true},
		{s1, MakePathOrDie("foo", 1), false},
		{s1, MakePathOrDie("foo", 1, "bar"), true},
		{s1, MakePathOrDie("foo", 1, "bar", "baz"), true},
		{s1, MakePathOrDie("canonicalOrder", KeyByFields(
			"f", "a",
			"e", "a",
			"d", "a",
			"c", "a",
			"b", "a",
			"a", "a",
		)), true}}

	for _, tt := range table {
		got := tt.set.Has(tt.check)
		if e, a := tt.expectMembership, got; e != a {
			t.Errorf("%v: wanted %v, got %v", tt.check.String(), e, a)
		}
	}

	if NewSet().Has(Path{}) {
		t.Errorf("empty set should not include the empty path")
	}
	if NewSet(Path{}).Has(Path{}) {
		t.Errorf("empty set should not include the empty path")
	}
}

func TestSetString(t *testing.T) {
	p := MakePathOrDie("foo", KeyByFields("name", "first"))
	s1 := NewSet(p)

	if p.String() != s1.String() {
		t.Errorf("expected single entry set to just call the path's string, but got %s %s", p, s1)
	}
}

func TestSetIterSize(t *testing.T) {
	s1 := NewSet(
		MakePathOrDie("foo", 0, "bar", "baz"),
		MakePathOrDie("foo", 0, "bar", "zot"),
		MakePathOrDie("foo", 0, "bar"),
		MakePathOrDie("foo", 0),
		MakePathOrDie("foo", 1, "bar", "baz"),
		MakePathOrDie("foo", 1, "bar"),
		MakePathOrDie("qux", KeyByFields("name", "first")),
		MakePathOrDie("qux", KeyByFields("name", "first"), "bar"),
		MakePathOrDie("qux", KeyByFields("name", "second"), "bar"),
	)

	s2 := NewSet()
	addedCount := 0
	s1.Iterate(func(p Path) {
		if s2.Size() != addedCount {
			t.Errorf("added %v items to set, but size is %v", addedCount, s2.Size())
		}
		if addedCount > 0 == s2.Empty() {
			t.Errorf("added %v items to set, but s2.Empty() is %v", addedCount, s2.Empty())
		}
		s2.Insert(p)
		addedCount++
	})

	s2 = NewSet()
	addedCount = 0
	for p := range s1.All() {
		if s2.Size() != addedCount {
			t.Errorf("added %v items to set, but size is %v", addedCount, s2.Size())
		}
		if addedCount > 0 == s2.Empty() {
			t.Errorf("added %v items to set, but s2.Empty() is %v", addedCount, s2.Empty())
		}
		s2.Insert(p)
		addedCount++
	}

	if !s1.Equals(s2) {
		// No point in using String() if iterate is broken...
		t.Errorf("Iterate missed something?\n%#v\n%#v", s1, s2)
	}
}

func TestSetEquals(t *testing.T) {
	table := []struct {
		a     *Set
		b     *Set
		equal bool
	}{
		{
			a:     NewSet(MakePathOrDie("foo")),
			b:     NewSet(MakePathOrDie("bar")),
			equal: false,
		},
		{
			a:     NewSet(MakePathOrDie("foo")),
			b:     NewSet(MakePathOrDie("foo")),
			equal: true,
		},
		{
			a:     NewSet(),
			b:     NewSet(MakePathOrDie(0, "foo")),
			equal: false,
		},
		{
			a:     NewSet(MakePathOrDie(1, "foo")),
			b:     NewSet(MakePathOrDie(0, "foo")),
			equal: false,
		},
		{
			a:     NewSet(MakePathOrDie(1, "foo")),
			b:     NewSet(MakePathOrDie(1, "foo", "bar")),
			equal: false,
		},
		{
			a: NewSet(
				MakePathOrDie(0),
				MakePathOrDie(1),
			),
			b: NewSet(
				MakePathOrDie(1),
				MakePathOrDie(0),
			),
			equal: true,
		},
		{
			a: NewSet(
				MakePathOrDie("foo", 0),
				MakePathOrDie("foo", 1),
			),
			b: NewSet(
				MakePathOrDie("foo", 1),
				MakePathOrDie("foo", 0),
			),
			equal: true,
		},
		{
			a: NewSet(
				MakePathOrDie("foo", 0),
				MakePathOrDie("foo"),
				MakePathOrDie("bar", "baz"),
				MakePathOrDie("qux", KeyByFields("name", "first")),
			),
			b: NewSet(
				MakePathOrDie("foo", 1),
				MakePathOrDie("bar", "baz"),
				MakePathOrDie("bar"),
				MakePathOrDie("qux", KeyByFields("name", "second")),
			),
			equal: false,
		},
	}

	for _, tt := range table {
		if e, a := tt.equal, tt.a.Equals(tt.b); e != a {
			t.Errorf("expected %v, got %v for:\na=\n%v\nb=\n%v", e, a, tt.a, tt.b)
		}
	}
}

func TestSetUnion(t *testing.T) {
	// Even though this is not a table driven test, since the thing under
	// test is recursive, we should be able to craft a single input that is
	// sufficient to check all code paths.

	s1 := NewSet(
		MakePathOrDie("foo", 0),
		MakePathOrDie("foo"),
		MakePathOrDie("bar", "baz"),
		MakePathOrDie("qux", KeyByFields("name", "first")),
		MakePathOrDie("parent", "child", "grandchild"),
	)

	s2 := NewSet(
		MakePathOrDie("foo", 1),
		MakePathOrDie("bar", "baz"),
		MakePathOrDie("bar"),
		MakePathOrDie("qux", KeyByFields("name", "second")),
		MakePathOrDie("parent", "child"),
	)

	u := NewSet(
		MakePathOrDie("foo", 0),
		MakePathOrDie("foo", 1),
		MakePathOrDie("foo"),
		MakePathOrDie("bar", "baz"),
		MakePathOrDie("bar"),
		MakePathOrDie("qux", KeyByFields("name", "first")),
		MakePathOrDie("qux", KeyByFields("name", "second")),
		MakePathOrDie("parent", "child"),
		MakePathOrDie("parent", "child", "grandchild"),
	)

	got := s1.Union(s2)

	if !got.Equals(u) {
		t.Errorf("union: expected: \n%v\n, got \n%v\n", u, got)
	}
}

func TestSetIntersectionDifference(t *testing.T) {
	// Even though this is not a table driven test, since the thing under
	// test is recursive, we should be able to craft a single input that is
	// sufficient to check all code paths.

	nameFirst := KeyByFields("name", "first")
	s1 := NewSet(
		MakePathOrDie("a0"),
		MakePathOrDie("a1"),
		MakePathOrDie("foo", 0),
		MakePathOrDie("foo", 1),
		MakePathOrDie("b0", nameFirst),
		MakePathOrDie("b1", nameFirst),
		MakePathOrDie("bar", "c0"),

		MakePathOrDie("cp", nameFirst, "child"),
	)

	s2 := NewSet(
		MakePathOrDie("a1"),
		MakePathOrDie("a2"),
		MakePathOrDie("foo", 1),
		MakePathOrDie("foo", 2),
		MakePathOrDie("b1", nameFirst),
		MakePathOrDie("b2", nameFirst),
		MakePathOrDie("bar", "c2"),

		MakePathOrDie("cp", nameFirst),
	)
	t.Logf("s1:\n%v\n", s1)
	t.Logf("s2:\n%v\n", s2)

	t.Run("intersection", func(t *testing.T) {
		i := NewSet(
			MakePathOrDie("a1"),
			MakePathOrDie("foo", 1),
			MakePathOrDie("b1", nameFirst),
		)

		got := s1.Intersection(s2)
		if !got.Equals(i) {
			t.Errorf("expected: \n%v\n, got \n%v\n", i, got)
		}
	})

	t.Run("s1 - s2", func(t *testing.T) {
		sDiffS2 := NewSet(
			MakePathOrDie("a0"),
			MakePathOrDie("foo", 0),
			MakePathOrDie("b0", nameFirst),
			MakePathOrDie("bar", "c0"),
			MakePathOrDie("cp", nameFirst, "child"),
		)

		got := s1.Difference(s2)
		if !got.Equals(sDiffS2) {
			t.Errorf("expected: \n%v\n, got \n%v\n", sDiffS2, got)
		}
	})

	t.Run("s2 - s1", func(t *testing.T) {
		s2DiffS := NewSet(
			MakePathOrDie("a2"),
			MakePathOrDie("foo", 2),
			MakePathOrDie("b2", nameFirst),
			MakePathOrDie("bar", "c2"),
			MakePathOrDie("cp", nameFirst),
		)

		got := s2.Difference(s1)
		if !got.Equals(s2DiffS) {
			t.Errorf("expected: \n%v\n, got \n%v\n", s2DiffS, got)
		}
	})

	t.Run("intersection (the hard way)", func(t *testing.T) {
		i := NewSet(
			MakePathOrDie("a1"),
			MakePathOrDie("foo", 1),
			MakePathOrDie("b1", nameFirst),
		)

		// We can construct Intersection out of two union and
		// three difference calls.
		u := s1.Union(s2)
		t.Logf("s1 u s2:\n%v\n", u)
		notIntersection := s2.Difference(s1).Union(s1.Difference(s2))
		t.Logf("s1 !i s2:\n%v\n", notIntersection)
		got := u.Difference(notIntersection)
		if !got.Equals(i) {
			t.Errorf("expected: \n%v\n, got \n%v\n", i, got)
		}
	})
}

func TestSetLeaves(t *testing.T) {
	table := []struct {
		name     string
		input    *Set
		expected *Set
	}{
		{
			name:     "empty set",
			input:    NewSet(),
			expected: NewSet(),
		}, {
			name: "all leaves",
			input: NewSet(
				_P("path1"),
				_P("path2"),
				_P("path3"),
			),
			expected: NewSet(
				_P("path1"),
				_P("path2"),
				_P("path3"),
			),
		}, {
			name: "only one leaf",
			input: NewSet(
				_P("root"),
				_P("root", "l1"),
				_P("root", "l1", "l2"),
				_P("root", "l1", "l2", "l3"),
			),
			expected: NewSet(
				_P("root", "l1", "l2", "l3"),
			),
		}, {
			name: "multiple values, check for overwrite",
			input: NewSet(
				_P("root", KeyByFields("name", "a")),
				_P("root", KeyByFields("name", "a"), "name"),
				_P("root", KeyByFields("name", "a"), "value", "b"),
				_P("root", KeyByFields("name", "a"), "value", "c"),
			),
			expected: NewSet(
				_P("root", KeyByFields("name", "a"), "name"),
				_P("root", KeyByFields("name", "a"), "value", "b"),
				_P("root", KeyByFields("name", "a"), "value", "c"),
			),
		}, {
			name: "multiple values and nested",
			input: NewSet(
				_P("root", KeyByFields("name", "a")),
				_P("root", KeyByFields("name", "a"), "name"),
				_P("root", KeyByFields("name", "a"), "value", "b"),
				_P("root", KeyByFields("name", "a"), "value", "b", "d"),
				_P("root", KeyByFields("name", "a"), "value", "c"),
			),
			expected: NewSet(
				_P("root", KeyByFields("name", "a"), "name"),
				_P("root", KeyByFields("name", "a"), "value", "b", "d"),
				_P("root", KeyByFields("name", "a"), "value", "c"),
			),
		}, {
			name: "all-in-one",
			input: NewSet(
				_P("root"),
				_P("root", KeyByFields("name", "a")),
				_P("root", KeyByFields("name", "a"), "name"),
				_P("root", KeyByFields("name", "a"), "value", "b"),
				_P("root", KeyByFields("name", "a"), "value", "b", "c"),
				_P("root", KeyByFields("name", "a"), "value", "d"),
				_P("root", KeyByFields("name", "a"), "value", "e"),
				_P("root", "x"),
				_P("root", "x", "y"),
				_P("root", "x", "z"),
				_P("root", KeyByFields("name", "p")),
				_P("root", KeyByFields("name", "p"), "name"),
				_P("root", KeyByFields("name", "p"), "value", "q"),
			),
			expected: NewSet(
				_P("root", KeyByFields("name", "a"), "name"),
				_P("root", KeyByFields("name", "a"), "value", "b", "c"),
				_P("root", KeyByFields("name", "a"), "value", "d"),
				_P("root", KeyByFields("name", "a"), "value", "e"),
				_P("root", "x", "y"),
				_P("root", "x", "z"),
				_P("root", KeyByFields("name", "p"), "name"),
				_P("root", KeyByFields("name", "p"), "value", "q"),
			),
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.Leaves(); !tt.expected.Equals(got) {
				t.Errorf("expected %v, got %v for input %v", tt.expected, got, tt.input)
			}
		})
	}

}

func TestSetDifference(t *testing.T) {
	table := []struct {
		name                      string
		a                         *Set
		b                         *Set
		expectDifference          *Set
		expectRecursiveDifference *Set
	}{
		{
			name:                      "removes simple path",
			a:                         NewSet(MakePathOrDie("a")),
			b:                         NewSet(MakePathOrDie("a")),
			expectDifference:          NewSet(),
			expectRecursiveDifference: NewSet(),
		},
		{
			name:                      "removes direct path",
			a:                         NewSet(MakePathOrDie("a", "b", "c")),
			b:                         NewSet(MakePathOrDie("a", "b", "c")),
			expectDifference:          NewSet(),
			expectRecursiveDifference: NewSet(),
		},
		{
			name: "only removes matching child",
			a: NewSet(
				MakePathOrDie("a", "b", "c"),
				MakePathOrDie("b", "b", "c"),
			),
			b:                         NewSet(MakePathOrDie("a", "b", "c")),
			expectDifference:          NewSet(MakePathOrDie("b", "b", "c")),
			expectRecursiveDifference: NewSet(MakePathOrDie("b", "b", "c")),
		},
		{
			name: "does not remove parent of specific path",
			a: NewSet(
				MakePathOrDie("a"),
			),
			b:                         NewSet(MakePathOrDie("a", "aa")),
			expectDifference:          NewSet(MakePathOrDie("a")),
			expectRecursiveDifference: NewSet(MakePathOrDie("a")),
		},
		{
			name:                      "RecursiveDifference removes nested path",
			a:                         NewSet(MakePathOrDie("a", "b", "c")),
			b:                         NewSet(MakePathOrDie("a")),
			expectDifference:          NewSet(MakePathOrDie("a", "b", "c")),
			expectRecursiveDifference: NewSet(),
		},
		{
			name: "RecursiveDifference only removes nested path for matching children",
			a: NewSet(
				MakePathOrDie("a", "aa", "aab"),
				MakePathOrDie("a", "ab", "aba"),
			),
			b: NewSet(MakePathOrDie("a", "aa")),
			expectDifference: NewSet(
				MakePathOrDie("a", "aa", "aab"),
				MakePathOrDie("a", "ab", "aba"),
			),
			expectRecursiveDifference: NewSet(MakePathOrDie("a", "ab", "aba")),
		},
		{
			name: "RecursiveDifference removes all matching children",
			a: NewSet(
				MakePathOrDie("a", "aa", "aab"),
				MakePathOrDie("a", "ab", "aba"),
			),
			b: NewSet(MakePathOrDie("a")),
			expectDifference: NewSet(
				MakePathOrDie("a", "aa", "aab"),
				MakePathOrDie("a", "ab", "aba"),
			),
			expectRecursiveDifference: NewSet(),
		},
	}

	for _, c := range table {
		t.Run(c.name, func(t *testing.T) {
			if result := c.a.Difference(c.b); !result.Equals(c.expectDifference) {
				t.Fatalf("Difference expected: \n%v\n, got: \n%v\n", c.expectDifference, result)
			}
			if result := c.a.RecursiveDifference(c.b); !result.Equals(c.expectRecursiveDifference) {
				t.Fatalf("RecursiveDifference expected: \n%v\n, got: \n%v\n", c.expectRecursiveDifference, result)
			}
		})
	}
}

var nestedSchema = func() (*typed.Parser, string) {
	name := "type"
	parser := mustParse(`types:
- name: type
  map:
    elementType:
      namedType: type
    fields:
      - name: keyAStr
        type:
          scalar: string
      - name: keyBInt
        type:
          scalar: numeric
      - name: named
        type:
          namedType: type
      - name: list
        type:
          list:
            elementRelationship: associative
            keys: ["keyAStr", "keyBInt"]
            elementType:
              namedType: type
      - name: a
        type:
          namedType: type
      - name: value
        type:
          scalar: numeric
`)
	return parser, name
}

var associativeListSchema = func() (*typed.Parser, string) {
	name := "type"
	parser := mustParse(`types:
- name: type
  map:
    fields:
      - name: values
        type:
          list:
            elementRelationship: associative
            keys: ["keyAStr", "keyBInt"]
            elementType:
              map:
                fields:
                  - name: keyAStr
                    type:
                      scalar: string
                  - name: keyBInt
                    type:
                      scalar: numeric
                  - name: value
                    type:
                      scalar: numeric
`)
	return parser, name
}

var oldAssociativeListSchema = func() (*typed.Parser, string) {
	name := "type"
	//  No keyBInt yet!
	parser := mustParse(`types:
- name: type
  map:
    fields:
      - name: values
        type:
          list:
            elementRelationship: associative
            keys: ["keyAStr"]
            elementType:
              map:
                fields:
                  - name: keyAStr
                    type:
                      scalar: string
                  - name: value
                    type:
                      scalar: numeric
`)
	return parser, name
}

func mustParse(schema typed.YAMLObject) *typed.Parser {
	parser, err := typed.NewParser(schema)
	if err != nil {
		panic(err)
	}
	return parser
}

func TestEnsureNamedFieldsAreMembers(t *testing.T) {
	table := []struct {
		schemaFn       func() (*typed.Parser, string)
		newSchemaFn    func() (*typed.Parser, string)
		value          typed.YAMLObject
		expectedBefore *Set
		expectedAfter  *Set
	}{
		{
			schemaFn: nestedSchema,
			value:    `{"named": {"named": {"value": 0}}}`,
			expectedBefore: NewSet(
				_P("named", "named", "value"),
			),
			expectedAfter: NewSet(
				_P("named", "named", "value"),
				_P("named", "named"),
				_P("named"),
			),
		},
		{
			schemaFn: nestedSchema,
			value:    `{"named": {"a": {"named": {"value": 42}}}, "a": {"named": {"value": 1}}}`,
			expectedBefore: NewSet(
				_P("named", "a", "named", "value"),
				_P("a", "named", "value"),
			),
			expectedAfter: NewSet(
				_P("named", "a", "named", "value"),
				_P("named", "a", "named"),
				_P("named", "a"),
				_P("named"),
				_P("a", "named", "value"),
				_P("a", "named"),
				_P("a"),
			),
		},
		{
			schemaFn: nestedSchema,
			value:    `{"named": {"list": [{"keyAStr": "a", "keyBInt": 1, "named": {"value": 0}}]}}`,
			expectedBefore: NewSet(
				_P("named", "list", KeyByFields("keyAStr", "a", "keyBInt", 1), "keyAStr"),
				_P("named", "list", KeyByFields("keyAStr", "a", "keyBInt", 1), "keyBInt"),
				_P("named", "list", KeyByFields("keyAStr", "a", "keyBInt", 1), "named", "value"),
				_P("named", "list", KeyByFields("keyAStr", "a", "keyBInt", 1)),
			),
			expectedAfter: NewSet(
				_P("named", "list", KeyByFields("keyAStr", "a", "keyBInt", 1), "keyAStr"),
				_P("named", "list", KeyByFields("keyAStr", "a", "keyBInt", 1), "keyBInt"),
				_P("named", "list", KeyByFields("keyAStr", "a", "keyBInt", 1), "named", "value"),
				_P("named", "list", KeyByFields("keyAStr", "a", "keyBInt", 1), "named"),
				_P("named", "list", KeyByFields("keyAStr", "a", "keyBInt", 1)),
				_P("named", "list"),
				_P("named"),
			),
		},
		{
			// Generate the value using the old schema to get missing key entries,
			// then process with new schema which has keyBInt.
			schemaFn:    oldAssociativeListSchema,
			newSchemaFn: associativeListSchema,
			value:       `{"values": [{"keyAStr": "a", "value": 0}]}`,
			expectedBefore: NewSet(
				_P("values", KeyByFields("keyAStr", "a"), "keyAStr"),
				_P("values", KeyByFields("keyAStr", "a"), "value"),
				_P("values", KeyByFields("keyAStr", "a")),
			),
			expectedAfter: NewSet(
				_P("values", KeyByFields("keyAStr", "a"), "keyAStr"),
				_P("values", KeyByFields("keyAStr", "a"), "value"),
				_P("values", KeyByFields("keyAStr", "a")),
				_P("values"),
			),
		},
		{
			// Check that converting the value with the missing key and
			// the recent schema doesn't add the missing key.
			schemaFn: associativeListSchema,
			value:    `{"values": [{"keyAStr": "a", "value": 1}]}`,
			expectedBefore: NewSet(
				_P("values", KeyByFields("keyAStr", "a"), "keyAStr"),
				_P("values", KeyByFields("keyAStr", "a"), "value"),
				_P("values", KeyByFields("keyAStr", "a")),
			),
			expectedAfter: NewSet(
				_P("values", KeyByFields("keyAStr", "a"), "keyAStr"),
				_P("values", KeyByFields("keyAStr", "a"), "value"),
				_P("values", KeyByFields("keyAStr", "a")),
				_P("values"),
			),
		},
	}

	for _, test := range table {
		t.Run(string(test.value), func(t *testing.T) {
			parser, typeName := test.schemaFn()
			typeRef := schema.TypeRef{NamedType: &typeName}
			typedValue, err := parser.Type(typeName).FromYAML(test.value)
			if err != nil {
				t.Fatalf("unexpected error parsing test value: %v", err)
			}
			set, err := typedValue.ToFieldSet()
			if err != nil {
				t.Fatalf("unexpected error converting test value to set: %v", err)
			}
			if !set.Equals(test.expectedBefore) {
				t.Errorf("expected before EnsureNamedFieldsAreMembers:\n%v\n\ngot:\n%v\n\nmissing:\n%v\n\nsuperfluous:\n\n%v",
					test.expectedBefore,
					set,
					test.expectedAfter.Difference(set),
					set.Difference(test.expectedAfter),
				)
			}

			schema := &parser.Schema
			if test.newSchemaFn != nil {
				newParser, _ := test.newSchemaFn()
				schema = &newParser.Schema
			}

			got := set.EnsureNamedFieldsAreMembers(schema, typeRef)
			if !got.Equals(test.expectedAfter) {
				t.Errorf("expected after EnsureNamedFieldsAreMembers:\n%v\n\ngot:\n%v\n\nmissing:\n%v\n\nsuperfluous:\n\n%v",
					test.expectedAfter,
					got,
					test.expectedAfter.Difference(got),
					got.Difference(test.expectedAfter),
				)
			}
		})
	}
}

func TestSetNodeMapIterate(t *testing.T) {
	set := &SetNodeMap{}
	toAdd := 5
	addedElements := make([]string, toAdd)
	for i := 0; i < toAdd; i++ {
		p := i
		pe := PathElement{Index: &p}
		addedElements[i] = pe.String()
		_ = set.Descend(pe)
	}

	iteratedElements := make(map[string]bool, toAdd)
	set.Iterate(func(pe PathElement) {
		iteratedElements[pe.String()] = true
	})

	if len(iteratedElements) != toAdd {
		t.Errorf("expected %v elements to be iterated over, got %v", toAdd, len(iteratedElements))
	}
	for _, pe := range addedElements {
		if _, ok := iteratedElements[pe]; !ok {
			t.Errorf("expected to have iterated over %v, but never did", pe)
		}
	}

	iteratedElements = make(map[string]bool, toAdd)
	for pe := range set.All() {
		iteratedElements[pe.String()] = true
	}
	if len(iteratedElements) != toAdd {
		t.Errorf("expected %v elements to be iterated over, got %v", toAdd, len(iteratedElements))
	}
	for _, pe := range addedElements {
		if _, ok := iteratedElements[pe]; !ok {
			t.Errorf("expected to have iterated over %v, but never did", pe)
		}
	}
}

func TestFilterByPattern(t *testing.T) {
	testCases := []struct {
		name   string
		input  *Set
		expect *Set
		filter Filter
	}{
		{
			name: "container resize fields: exact match",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "containers"),
				MakePathOrDie("spec", "containers", 0),
				MakePathOrDie("spec", "containers", 0, "resources"),
				MakePathOrDie("spec", "containers", 0, "resources", "limits"),
				MakePathOrDie("spec", "containers", 0, "resources", "limits", "cpu"),
				MakePathOrDie("spec", "containers", 0, "resources", "requests"),
				MakePathOrDie("spec", "containers", 0, "resources", "requests", "cpu"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 0),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 0, "name"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 0, "request"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 1),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 1, "name"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 1, "request"),
				MakePathOrDie("spec", "containers", 1),
				MakePathOrDie("spec", "containers", 1, "resources"),
				MakePathOrDie("spec", "containers", 1, "resources", "limits"),
				MakePathOrDie("spec", "containers", 1, "resources", "limits", "cpu"),
			),
			filter: NewIncludeMatcherFilter(MakePrefixMatcherOrDie("spec", "containers", MatchAnyPathElement(), "resources")),
			expect: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "containers"),
				MakePathOrDie("spec", "containers", 0),
				MakePathOrDie("spec", "containers", 0, "resources"),
				MakePathOrDie("spec", "containers", 0, "resources", "limits"),
				MakePathOrDie("spec", "containers", 0, "resources", "limits", "cpu"),
				MakePathOrDie("spec", "containers", 0, "resources", "requests"),
				MakePathOrDie("spec", "containers", 0, "resources", "requests", "cpu"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 0),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 0, "name"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 0, "request"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 1),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 1, "name"),
				MakePathOrDie("spec", "containers", 0, "resources", "claims", 1, "request"),
				MakePathOrDie("spec", "containers", 1),
				MakePathOrDie("spec", "containers", 1, "resources"),
				MakePathOrDie("spec", "containers", 1, "resources", "limits"),
				MakePathOrDie("spec", "containers", 1, "resources", "limits", "cpu"),
			),
		},
		{
			name: "container resize fields: filter status and metadata",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("status"),
				MakePathOrDie("metadata"),
			),
			filter: NewIncludeMatcherFilter(MakePrefixMatcherOrDie("spec", "containers", MatchAnyPathElement(), "resources")),
			expect: NewSet(
				MakePathOrDie("spec"),
			),
		},
		{
			name: "container resize fields: filter non-container spec fields",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "volumes"),
				MakePathOrDie("spec", "hostNetwork"),
			),
			filter: NewIncludeMatcherFilter(MakePrefixMatcherOrDie("spec", "containers", MatchAnyPathElement(), "resources")),
			expect: NewSet(
				MakePathOrDie("spec"),
			),
		},
		{
			name: "container resize fields: filter non-resource container fields",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "containers"),
				MakePathOrDie("spec", "containers", 0),
				MakePathOrDie("spec", "containers", 0, "image"),
				MakePathOrDie("spec", "containers", 0, "workingDir"),
				MakePathOrDie("spec", "containers", 0, "resources"),
			),
			filter: NewIncludeMatcherFilter(MakePrefixMatcherOrDie("spec", "containers", MatchAnyPathElement(), "resources")),
			expect: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "containers"),
				MakePathOrDie("spec", "containers", 0),
				MakePathOrDie("spec", "containers", 0, "resources"),
			),
		},
		{
			name: "filter listMap key",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "listMap",
					&value.FieldList{
						{Name: "key1", Value: value.NewValueInterface("value1")},
						{Name: "key2", Value: value.NewValueInterface("value2")},
					}),
				MakePathOrDie("spec", "listMap",
					&value.FieldList{
						{Name: "key1", Value: value.NewValueInterface("value1")},
						{Name: "key2", Value: value.NewValueInterface("value2")},
					}, "field"),
				MakePathOrDie("spec", "listMap",
					&value.FieldList{
						{Name: "key1", Value: value.NewValueInterface("valueX")},
						{Name: "key2", Value: value.NewValueInterface("valueY")},
					}, "field"),
			),
			filter: NewIncludeMatcherFilter(MakePrefixMatcherOrDie("spec", "listMap", &value.FieldList{
				{Name: "key1", Value: value.NewValueInterface("value1")},
				{Name: "key2", Value: value.NewValueInterface("value2")},
			})),
			expect: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "listMap",
					&value.FieldList{
						{Name: "key1", Value: value.NewValueInterface("value1")},
						{Name: "key2", Value: value.NewValueInterface("value2")},
					}),
				MakePathOrDie("spec", "listMap",
					&value.FieldList{
						{Name: "key1", Value: value.NewValueInterface("value1")},
						{Name: "key2", Value: value.NewValueInterface("value2")},
					}, "field"),
			),
		},
		{
			name: "filter value",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "set", value.NewValueInterface("v1")),
				MakePathOrDie("spec", "set", value.NewValueInterface("v2")),
			),
			filter: NewIncludeMatcherFilter(MakePrefixMatcherOrDie("spec", "set", value.NewValueInterface("v1"))),
			expect: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "set", value.NewValueInterface("v1")),
			),
		},
		{
			name: "filter by index",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "list"),
				MakePathOrDie("spec", "list", 0),
				MakePathOrDie("spec", "list", 0, "value"),
				MakePathOrDie("spec", "list", 1),
				MakePathOrDie("spec", "list", 1, "value"),
			),
			filter: NewIncludeMatcherFilter(MakePrefixMatcherOrDie("spec", "list", 1, "value")),
			expect: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "list"),
				MakePathOrDie("spec", "list", 1),
				MakePathOrDie("spec", "list", 1, "value"),
			),
		},
		{
			name: "multiple index matchers",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "list"),
				MakePathOrDie("spec", "list", 0),
				MakePathOrDie("spec", "list", 0, "value"),
				MakePathOrDie("spec", "list", 1),
				MakePathOrDie("spec", "list", 1, "value"),
				MakePathOrDie("spec", "list", 2),
				MakePathOrDie("spec", "list", 2, "value"),
			),
			filter: NewIncludeMatcherFilter(
				MakePrefixMatcherOrDie("spec", "list", 0, "value"),
				MakePrefixMatcherOrDie("spec", "list", 1, "value"),
			),
			expect: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "list"),
				MakePathOrDie("spec", "list", 0),
				MakePathOrDie("spec", "list", 0, "value"),
				MakePathOrDie("spec", "list", 1),
				MakePathOrDie("spec", "list", 1, "value"),
			),
		},
		{
			name: "multiple field matchers",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "f1"),
				MakePathOrDie("spec", "f1", "f11"),
				MakePathOrDie("spec", "f2"),
				MakePathOrDie("spec", "f2", "f21"),
				MakePathOrDie("spec", "f3"),
				MakePathOrDie("spec", "f3", "f31"),
			),
			filter: NewIncludeMatcherFilter(
				MakePrefixMatcherOrDie("spec", "f1"),
				MakePrefixMatcherOrDie("spec", "f3"),
			),
			expect: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "f1"),
				MakePathOrDie("spec", "f1", "f11"),
				MakePathOrDie("spec", "f3"),
				MakePathOrDie("spec", "f3", "f31"),
			),
		},
		{
			name: "wildcard takes precedence",
			input: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "list"),
				MakePathOrDie("spec", "list", 0),
				MakePathOrDie("spec", "list", 0, "f1"),
				MakePathOrDie("spec", "list", 0, "f2"),
				MakePathOrDie("spec", "list", 1),
				MakePathOrDie("spec", "list", 1, "f1"),
				MakePathOrDie("spec", "list", 1, "f2"),
				MakePathOrDie("spec", "list", 2),
				MakePathOrDie("spec", "list", 2, "f1"),
				MakePathOrDie("spec", "list", 2, "f2"),
			),
			filter: NewIncludeMatcherFilter(
				MakePrefixMatcherOrDie("spec", "list", MatchAnyPathElement(), "f1"), // takes precedence
				MakePrefixMatcherOrDie("spec", "list", 1, "f2"),                     // ignored
			),
			expect: NewSet(
				MakePathOrDie("spec"),
				MakePathOrDie("spec", "list"),
				MakePathOrDie("spec", "list", 0),
				MakePathOrDie("spec", "list", 0, "f1"),
				MakePathOrDie("spec", "list", 1),
				MakePathOrDie("spec", "list", 1, "f1"),
				MakePathOrDie("spec", "list", 2),
				MakePathOrDie("spec", "list", 2, "f1"),
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filtered := tc.filter.Filter(tc.input)
			if !filtered.Equals(tc.expect) {
				t.Errorf("Expected:\n%v\n\nbut got:\n%v", tc.expect, filtered)
			}
		})
	}
}

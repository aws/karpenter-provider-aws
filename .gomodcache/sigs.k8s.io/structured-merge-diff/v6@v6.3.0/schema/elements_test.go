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

package schema

import (
	"reflect"
	"testing"
)

func TestFindNamedType(t *testing.T) {
	tests := []struct {
		testName      string
		defs          []TypeDef
		namedType     string
		expectTypeDef TypeDef
		expectExist   bool
	}{
		{"existing", []TypeDef{{Name: "a"}, {Name: "b"}}, "a", TypeDef{Name: "a"}, true},
		{"notExisting", []TypeDef{{Name: "a"}, {Name: "b"}}, "c", TypeDef{}, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			s := Schema{
				Types: tt.defs,
			}
			td, exist := s.FindNamedType(tt.namedType)
			if !td.Equals(&tt.expectTypeDef) {
				t.Errorf("expected TypeDef %v, got %v", tt.expectTypeDef, td)
			}
			if exist != tt.expectExist {
				t.Errorf("expected existing %t, got %t", tt.expectExist, exist)
			}
		})
	}
}

func strptr(s string) *string { return &s }

func TestFindField(t *testing.T) {
	tests := []struct {
		testName          string
		defs              []StructField
		fieldName         string
		expectStructField StructField
		expectExist       bool
	}{
		{"existing", []StructField{
			{Name: "a", Type: TypeRef{NamedType: strptr("a")}},
			{Name: "b", Type: TypeRef{NamedType: strptr("b")}},
		}, "a", StructField{Name: "a", Type: TypeRef{NamedType: strptr("a")}}, true},
		{"notExisting", []StructField{
			{Name: "a", Type: TypeRef{NamedType: strptr("a")}},
			{Name: "b", Type: TypeRef{NamedType: strptr("b")}},
		}, "c", StructField{}, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			s := Map{
				Fields: tt.defs,
			}
			sf, exist := s.FindField(tt.fieldName)
			if !reflect.DeepEqual(sf, tt.expectStructField) {
				t.Errorf("expected StructField %v, got %v", tt.expectStructField, sf)
			}
			if exist != tt.expectExist {
				t.Errorf("expected existing %t, got %t", tt.expectExist, exist)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	existing := "existing"
	notExisting := "not-existing"
	numeric := Numeric
	granular := Separable
	atomic := Atomic

	a := Atom{List: &List{}}
	b := Atom{Scalar: &numeric}

	emptyMap := Map{}
	atomicMap := Map{ElementRelationship: Atomic}

	emptyList := List{}
	atomicList := List{ElementRelationship: Atomic}

	tests := []struct {
		testName       string
		schemaTypeDefs []TypeDef
		typeRef        TypeRef
		expectAtom     Atom
		expectExist    bool
	}{
		{"noNamedType", nil, TypeRef{Inlined: a}, a, true},
		{"notExistingNamedType", nil, TypeRef{NamedType: &notExisting}, Atom{}, false},
		{"existingNamedType", []TypeDef{{Name: existing, Atom: a}}, TypeRef{NamedType: &existing}, a, true},
		{"invalidRelationshipOnScalarType", []TypeDef{{Name: existing, Atom: b}}, TypeRef{NamedType: &existing, ElementRelationship: &granular}, Atom{}, false},
		{"mapElementRelationshipNamed", []TypeDef{{Name: existing, Atom: Atom{Map: &emptyMap}}}, TypeRef{NamedType: &existing, ElementRelationship: &atomic}, Atom{Map: &atomicMap}, true},
		{"mapElementRelationshipInlined", nil, TypeRef{Inlined: Atom{Map: &emptyMap}, ElementRelationship: &atomic}, Atom{Map: &atomicMap}, true},
		{"listElementRelationshipInlined", nil, TypeRef{Inlined: Atom{List: &emptyList}, ElementRelationship: &atomic}, Atom{List: &atomicList}, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			s := Schema{
				Types: tt.schemaTypeDefs,
			}
			atom, exist := s.Resolve(tt.typeRef)
			if !reflect.DeepEqual(atom, tt.expectAtom) {
				t.Errorf("expected Atom %v, got %v", tt.expectAtom, atom)
			}
			if exist != tt.expectExist {
				t.Errorf("expected exist %t, got %t", tt.expectExist, exist)
			}
		})
	}
}

func TestCopyInto(t *testing.T) {
	existing := "existing"
	notExisting := "not-existing"
	a := Atom{List: &List{}}

	tests := []struct {
		testName       string
		schemaTypeDefs []TypeDef
		typeRef        TypeRef
	}{
		{"noNamedType", nil, TypeRef{Inlined: a}},
		{"notExistingNamedType", nil, TypeRef{NamedType: &notExisting}},
		{"existingNamedType", []TypeDef{{Name: existing, Atom: a}}, TypeRef{NamedType: &existing}},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			s := Schema{
				Types: tt.schemaTypeDefs,
			}

			theCopy := Schema{}
			s.CopyInto(&theCopy)

			if !reflect.DeepEqual(&s, &theCopy) {
				t.Fatal("")
			}

			// test after resolve
			_, _ = s.Resolve(tt.typeRef)
			theCopy = Schema{}
			s.CopyInto(&theCopy)

			if !reflect.DeepEqual(&s, &theCopy) {
				t.Fatal("")
			}
		})
	}
}

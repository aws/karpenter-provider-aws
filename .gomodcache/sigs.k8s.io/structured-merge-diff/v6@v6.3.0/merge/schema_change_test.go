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

package merge_test

import (
	"testing"

	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	. "sigs.k8s.io/structured-merge-diff/v6/internal/fixture"
	"sigs.k8s.io/structured-merge-diff/v6/merge"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

var structParser = func() *typed.Parser {
	oldParser, err := typed.NewParser(`types:
- name: v1
  map:
    fields:
      - name: struct
        type:
          namedType: struct
- name: struct
  map:
    fields:
    - name: numeric
      type:
        scalar: numeric
    - name: string
      type:
        scalar: string`)
	if err != nil {
		panic(err)
	}
	return oldParser
}()

var structWithAtomicParser = func() *typed.Parser {
	newParser, err := typed.NewParser(`types:
- name: v1
  map:
    fields:
      - name: struct
        type:
          namedType: struct
- name: struct
  map:
    fields:
    - name: numeric
      type:
        scalar: numeric
    - name: string
      type:
        scalar: string
    elementRelationship: atomic`)
	if err != nil {
		panic(err)
	}
	return newParser
}()

func TestGranularToAtomicSchemaChanges(t *testing.T) {
	tests := map[string]TestCase{
		"to-atomic": {
			Ops: []Operation{
				Apply{
					Manager: "one",
					Object: `
						struct:
						  numeric: 1
					`,
					APIVersion: "v1",
				},
				ChangeParser{Parser: structWithAtomicParser},
				Apply{
					Manager: "two",
					Object: `
						struct:
						  string: "string"
					`,
					APIVersion: "v1",
					Conflicts: merge.Conflicts{
						merge.Conflict{Manager: "one", Path: _P("struct")},
					},
				},
				ForceApply{
					Manager: "two",
					Object: `
						struct:
						  string: "string"
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				struct:
				  string: "string"
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"two": fieldpath.NewVersionedSet(_NS(
					_P("struct"),
				), "v1", true),
			},
		},
		"to-atomic-owner-with-no-child-fields": {
			Ops: []Operation{
				Apply{
					Manager: "one",
					Object: `
						struct:
						  numeric: 1
					`,
					APIVersion: "v1",
				},
				ForceApply{ // take the only child field from manager "one"
					Manager: "two",
					Object: `
						struct:
						  numeric: 2
					`,
					APIVersion: "v1",
				},
				ChangeParser{Parser: structWithAtomicParser},
				Apply{
					Manager: "three",
					Object: `
						struct:
						  string: "string"
					`,
					APIVersion: "v1",
					Conflicts: merge.Conflicts{
						// We expect no conflict with "one" because we do not allow a manager
						// to own a map without owning any of the children.
						merge.Conflict{Manager: "two", Path: _P("struct")},
					},
				},
				ForceApply{
					Manager: "two",
					Object: `
						struct:
						  string: "string"
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				struct:
				  string: "string"
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"two": fieldpath.NewVersionedSet(_NS(
					_P("struct"),
				), "v1", true),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test.Test(structParser); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAtomicToGranularSchemaChanges(t *testing.T) {
	tests := map[string]TestCase{
		"to-granular": {
			Ops: []Operation{
				Apply{
					Manager: "one",
					Object: `
						struct:
						  numeric: 1
						  string: "a"
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "two",
					Object: `
						struct:
						  string: "b"
					`,
					APIVersion: "v1",
					Conflicts: merge.Conflicts{
						merge.Conflict{Manager: "one", Path: _P("struct")},
					},
				},
				ChangeParser{Parser: structParser},
				// No conflict after changing struct to a granular schema
				Apply{
					Manager: "two",
					Object: `
						struct:
						  string: "b"
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				struct:
				  numeric: 1
				  string: "b"
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				// Note that manager one previously owned
				// the top level _P("struct")
				// which included all of its subfields
				// when the struct field was atomic.
				//
				// Upon changing the schema of struct from
				// atomic to granular, manager one continues
				// to own the same fieldset as before,
				// but does not retain ownership of any of the subfields.
				//
				// This is a known limitation due to the inability
				// to accurately determine whether an empty field
				// was previously atomic or not.
				"one": fieldpath.NewVersionedSet(_NS(
					_P("struct"),
				), "v1", true),
				"two": fieldpath.NewVersionedSet(_NS(
					_P("struct", "string"),
				), "v1", true),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test.Test(structWithAtomicParser); err != nil {
				t.Fatal(err)
			}
		})
	}
}

var associativeListParserOld = func() *typed.Parser {
	oldParser, err := typed.NewParser(`types:
- name: v1
  map:
    fields:
      - name: list
        type:
          namedType: associativeList
- name: associativeList
  list:
    elementType:
      namedType: myElement
    elementRelationship: associative
    keys:
    - name
- name: myElement
  map:
    fields:
    - name: name
      type:
        scalar: string
    - name: value
      type:
        scalar: numeric
`)
	if err != nil {
		panic(err)
	}
	return oldParser
}()

var associativeListParserNewOptionalKey = func() *typed.Parser {
	newParser, err := typed.NewParser(`types:
- name: v1
  map:
    fields:
      - name: list
        type:
          namedType: associativeList
- name: associativeList
  list:
    elementType:
      namedType: myElement
    elementRelationship: associative
    keys:
    - name
    - id
- name: myElement
  map:
    fields:
    - name: name
      type:
        scalar: string
    - name: id
      type:
        scalar: numeric
    - name: value
      type:
        scalar: numeric
`)
	if err != nil {
		panic(err)
	}
	return newParser
}()

var associativeListParserNewKeyWithDefault = func() *typed.Parser {
	newParser, err := typed.NewParser(`types:
- name: v1
  map:
    fields:
      - name: list
        type:
          namedType: associativeList
- name: associativeList
  list:
    elementType:
      namedType: myElement
    elementRelationship: associative
    keys:
    - name
    - id
- name: myElement
  map:
    fields:
    - name: name
      type:
        scalar: string
    - name: id
      type:
        scalar: numeric
      default: 1
    - name: value
      type:
        scalar: numeric
`)
	if err != nil {
		panic(err)
	}
	return newParser
}()

func TestAssociativeListSchemaChanges(t *testing.T) {
	tests := map[string]TestCase{
		"new required key with default": {
			Ops: []Operation{
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  value: 1
						- name: b
						  value: 1
						- name: c
						  value: 1
					`,
					APIVersion: "v1",
				},
				ChangeParser{Parser: associativeListParserNewKeyWithDefault},
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  value: 2
						- name: b
						  id: 1
						  value: 2
						- name: c
						  value: 1
						- name: c
						  id: 2
						  value: 2
						- name: c
						  id: 3
						  value: 3
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  value: 2
				- name: b
				  id: 1
				  value: 2
				- name: c
				  value: 1
				- name: c
				  id: 2
				  value: 2
				- name: c
				  id: 3
				  value: 3
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"one": fieldpath.NewVersionedSet(_NS(
					_P("list", _KBF("name", "a", "id", float64(1))),
					_P("list", _KBF("name", "a", "id", float64(1)), "name"),
					_P("list", _KBF("name", "a", "id", float64(1)), "value"),
					_P("list", _KBF("name", "b", "id", float64(1))),
					_P("list", _KBF("name", "b", "id", float64(1)), "name"),
					_P("list", _KBF("name", "b", "id", float64(1)), "id"),
					_P("list", _KBF("name", "b", "id", float64(1)), "value"),
					_P("list", _KBF("name", "c", "id", float64(1))),
					_P("list", _KBF("name", "c", "id", float64(1)), "name"),
					_P("list", _KBF("name", "c", "id", float64(1)), "value"),
					_P("list", _KBF("name", "c", "id", float64(2))),
					_P("list", _KBF("name", "c", "id", float64(2)), "name"),
					_P("list", _KBF("name", "c", "id", float64(2)), "id"),
					_P("list", _KBF("name", "c", "id", float64(2)), "value"),
					_P("list", _KBF("name", "c", "id", float64(3))),
					_P("list", _KBF("name", "c", "id", float64(3)), "name"),
					_P("list", _KBF("name", "c", "id", float64(3)), "id"),
					_P("list", _KBF("name", "c", "id", float64(3)), "value"),
				), "v1", true),
			},
		},
		"new optional key": {
			Ops: []Operation{
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  value: 1
					`,
					APIVersion: "v1",
				},
				ChangeParser{Parser: associativeListParserNewOptionalKey},
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  value: 2
						- name: a
						  id: 1
						  value: 1
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  value: 2
				- name: a
				  id: 1
				  value: 1
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"one": fieldpath.NewVersionedSet(_NS(
					_P("list", _KBF("name", "a")),
					_P("list", _KBF("name", "a"), "name"),
					_P("list", _KBF("name", "a"), "value"),
					_P("list", _KBF("name", "a", "id", float64(1))),
					_P("list", _KBF("name", "a", "id", float64(1)), "name"),
					_P("list", _KBF("name", "a", "id", float64(1)), "id"),
					_P("list", _KBF("name", "a", "id", float64(1)), "value"),
				), "v1", true),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test.Test(associativeListParserOld); err != nil {
				t.Fatal(err)
			}
		})
	}
}

var associativeListParserPromoteKeyBefore = func() *typed.Parser {
	p, err := typed.NewParser(`types:
- name: v1
  map:
    fields:
      - name: list
        type:
          namedType: associativeList
- name: associativeList
  list:
    elementType:
      namedType: myElement
    elementRelationship: associative
    keys:
    - name
- name: myElement
  map:
    fields:
    - name: name
      type:
        scalar: string
    - name: id
      type:
        scalar: numeric
    - name: value
      type:
        scalar: numeric
`)
	if err != nil {
		panic(err)
	}
	return p
}()

var associativeListParserPromoteKeyAfter = func() *typed.Parser {
	p, err := typed.NewParser(`types:
- name: v1
  map:
    fields:
      - name: list
        type:
          namedType: associativeList
- name: associativeList
  list:
    elementType:
      namedType: myElement
    elementRelationship: associative
    keys:
    - name
    - id
- name: myElement
  map:
    fields:
    - name: name
      type:
        scalar: string
    - name: id
      type:
        scalar: numeric
    - name: value
      type:
        scalar: numeric
`)
	if err != nil {
		panic(err)
	}
	return p
}()

func TestPromoteFieldToAssociativeListKey(t *testing.T) {
	tests := map[string]TestCase{
		"identical item merges": {
			Ops: []Operation{
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  id: 1
						  value: 1
					`,
					APIVersion: "v1",
				},
				ChangeParser{Parser: associativeListParserPromoteKeyAfter},
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  id: 1
						  value: 2
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  id: 1
				  value: 2
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"one": fieldpath.NewVersionedSet(_NS(
					_P("list", _KBF("name", "a", "id", float64(1))),
					_P("list", _KBF("name", "a", "id", float64(1)), "name"),
					_P("list", _KBF("name", "a", "id", float64(1)), "id"),
					_P("list", _KBF("name", "a", "id", float64(1)), "value"),
				), "v1", true),
			},
		},
		"distinct item added": {
			Ops: []Operation{
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  id: 1
						  value: 1
					`,
					APIVersion: "v1",
				},
				ChangeParser{Parser: associativeListParserPromoteKeyAfter},
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  id: 1
						  value: 1
						- name: a
						  id: 2
						  value: 2
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  id: 1
				  value: 1
				- name: a
				  id: 2
				  value: 2
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"one": fieldpath.NewVersionedSet(_NS(
					_P("list", _KBF("name", "a", "id", float64(1))),
					_P("list", _KBF("name", "a", "id", float64(1)), "name"),
					_P("list", _KBF("name", "a", "id", float64(1)), "id"),
					_P("list", _KBF("name", "a", "id", float64(1)), "value"),
					_P("list", _KBF("name", "a", "id", float64(2))),
					_P("list", _KBF("name", "a", "id", float64(2)), "name"),
					_P("list", _KBF("name", "a", "id", float64(2)), "id"),
					_P("list", _KBF("name", "a", "id", float64(2)), "value"),
				), "v1", true),
			},
		},
		"item missing new key field is distinct": {
			Ops: []Operation{
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  value: 1
					`,
					APIVersion: "v1",
				},
				ChangeParser{Parser: associativeListParserPromoteKeyAfter},
				Apply{
					Manager: "one",
					Object: `
						list:
						- name: a
						  value: 1
						- name: a
						  id: 2
						  value: 2
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  value: 1
				- name: a
				  id: 2
				  value: 2
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"one": fieldpath.NewVersionedSet(_NS(
					_P("list", _KBF("name", "a")),
					_P("list", _KBF("name", "a"), "name"),
					_P("list", _KBF("name", "a"), "value"),
					_P("list", _KBF("name", "a", "id", float64(2))),
					_P("list", _KBF("name", "a", "id", float64(2)), "name"),
					_P("list", _KBF("name", "a", "id", float64(2)), "id"),
					_P("list", _KBF("name", "a", "id", float64(2)), "value"),
				), "v1", true),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test.Test(associativeListParserPromoteKeyBefore); err != nil {
				t.Fatal(err)
			}
		})
	}
}

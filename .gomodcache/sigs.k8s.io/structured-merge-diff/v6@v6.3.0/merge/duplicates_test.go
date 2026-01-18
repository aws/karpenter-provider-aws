/*
Copyright 2019 The Kubernetes Authors.

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

var duplicatesParser = func() Parser {
	parser, err := typed.NewParser(`types:
- name: type
  map:
    fields:
      - name: list
        type:
          namedType: associativeList
      - name: unrelated
        type:
          scalar: numeric
      - name: set
        type:
          namedType: set
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
    - name: value1
      type:
        scalar: numeric
    - name: value2
      type:
        scalar: numeric
- name: set
  list:
    elementType:
      scalar: numeric
    elementRelationship: associative
`)
	if err != nil {
		panic(err)
	}
	return SameVersionParser{T: parser.Type("type")}
}()

func TestDuplicates(t *testing.T) {
	tests := map[string]TestCase{
		"sets/ownership/duplicates": {
			Ops: []Operation{
				Update{
					Manager: "updater-one",
					Object: `
						set: [1, 1, 3, 4]
					`,
					APIVersion: "v1",
				},
			},
			Managed: fieldpath.ManagedFields{
				"updater-one": fieldpath.NewVersionedSet(
					_NS(
						_P("set"),
						_P("set", _V(1)),
						_P("set", _V(3)),
						_P("set", _V(4)),
					),
					"v1",
					false,
				),
			},
		},
		"sets/ownership/add_duplicate": {
			Ops: []Operation{
				Update{
					Manager: "updater-one",
					Object: `
						set: [1, 3, 4]
					`,
					APIVersion: "v1",
				},
				Update{
					Manager: "updater-two",
					Object: `
						set: [1, 1, 3, 4]
					`,
					APIVersion: "v1",
				},
			},
			Managed: fieldpath.ManagedFields{
				"updater-one": fieldpath.NewVersionedSet(
					_NS(
						_P("set"),
						_P("set", _V(3)),
						_P("set", _V(4)),
					),
					"v1",
					false,
				),
				"updater-two": fieldpath.NewVersionedSet(
					_NS(
						_P("set", _V(1)),
					),
					"v1",
					false,
				),
			},
		},
		"sets/ownership/remove_duplicate": {
			Ops: []Operation{
				Update{
					Manager: "updater-one",
					Object: `
						set: [1, 1, 3, 4]
					`,
					APIVersion: "v1",
				},
				Update{
					Manager: "updater-two",
					Object: `
						set: [1, 3, 4]
					`,
					APIVersion: "v1",
				},
			},
			Managed: fieldpath.ManagedFields{
				"updater-one": fieldpath.NewVersionedSet(
					_NS(
						_P("set"),
						_P("set", _V(3)),
						_P("set", _V(4)),
					),
					"v1",
					false,
				),
				"updater-two": fieldpath.NewVersionedSet(
					_NS(
						_P("set", _V(1)),
					),
					"v1",
					false,
				),
			},
		},
		"sets/merging/remove_duplicate": {
			Ops: []Operation{
				Update{
					Manager: "updater",
					Object: `
						set: [1, 1, 3, 4]
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "applier",
					Object: `
						set: [1]
					`,
					APIVersion: "v1",
					Conflicts: merge.Conflicts{
						{Manager: "updater", Path: _P("set", _V(1))},
					},
				},
				ForceApply{
					Manager: "applier",
					Object: `
						set: [1]
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				set: [1, 3, 4]
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"updater": fieldpath.NewVersionedSet(
					_NS(
						_P("set"),
						_P("set", _V(3)),
						_P("set", _V(4)),
					),
					"v1",
					false,
				),
				"applier": fieldpath.NewVersionedSet(
					_NS(
						_P("set", _V(1)),
					),
					"v1",
					true,
				),
			},
		},
		"sets/merging/ignore_duplicate": {
			Ops: []Operation{
				Update{
					Manager: "updater",
					Object: `
						set: [1, 1, 3, 4]
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "applier",
					Object: `
						set: [5]
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				set: [1, 1, 3, 4, 5]
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"updater": fieldpath.NewVersionedSet(
					_NS(
						_P("set"),
						_P("set", _V(1)),
						_P("set", _V(3)),
						_P("set", _V(4)),
					),
					"v1",
					false,
				),
				"applier": fieldpath.NewVersionedSet(
					_NS(
						_P("set", _V(5)),
					),
					"v1",
					true,
				),
			},
		},
		"list/ownership/duplicated_items": {
			Ops: []Operation{
				Update{
					Manager: "updater",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
			},
			// `name: a` is only owned once.
			Managed: fieldpath.ManagedFields{
				"updater": fieldpath.NewVersionedSet(
					_NS(
						_P("list"),
						_P("list", _KBF("name", "a")),
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					false,
				),
			},
		},
		"list/ownership/change_duplicated_items": {
			Ops: []Operation{
				Update{
					Manager: "updater-one",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
				Update{
					Manager: "updater-two",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 3
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
			},
			// `name: a` is only owned once, by actor who changed some of it.
			Managed: fieldpath.ManagedFields{
				"updater-one": fieldpath.NewVersionedSet(
					_NS(
						_P("list"),
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					false,
				),
				"updater-two": fieldpath.NewVersionedSet(
					_NS(
						_P("list", _KBF("name", "a")),
					),
					"v1",
					false,
				),
			},
		},
		"list/ownership/change_fields_duplicated_items": {
			Ops: []Operation{
				Update{
					Manager: "updater-one",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
				Update{
					Manager: "updater-two",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						  value2: 3 # New field
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
			},
			Managed: fieldpath.ManagedFields{
				"updater-one": fieldpath.NewVersionedSet(
					_NS(
						_P("list"),
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					false,
				),
				"updater-two": fieldpath.NewVersionedSet(
					_NS(
						_P("list", _KBF("name", "a")),
					),
					"v1",
					false,
				),
			},
		},
		"list/ownership/add_duplicated_items_different_field": {
			Ops: []Operation{
				Update{
					Manager: "updater-one",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
				Update{
					Manager: "updater-two",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value2: 3 # New field
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
			},
			Managed: fieldpath.ManagedFields{
				"updater-one": fieldpath.NewVersionedSet(
					_NS(
						_P("list"),
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					false,
				),
				"updater-two": fieldpath.NewVersionedSet(
					_NS(
						_P("list", _KBF("name", "a")),
					),
					"v1",
					false,
				),
			},
		},
		"list/ownership/add_unrelated_to_list_with_duplicates": {
			Ops: []Operation{
				Update{
					Manager: "updater-one",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
					`,
					APIVersion: "v1",
				},
				Update{
					Manager: "updater-two",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
			},
			Managed: fieldpath.ManagedFields{
				"updater-one": fieldpath.NewVersionedSet(
					_NS(
						_P("list"),
						_P("list", _KBF("name", "a")),
					),
					"v1",
					false,
				),
				"updater-two": fieldpath.NewVersionedSet(
					_NS(
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					false,
				),
			},
		},
		"list/merge/unrelated_with_duplicated_items": {
			Ops: []Operation{
				Update{
					Manager: "updater",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
				ForceApply{
					Manager: "applier",
					Object: `
						unrelated: 5
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  value1: 1
				- name: a
				  value1: 2
				- name: b
				  value1: 3
				unrelated: 5
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"updater": fieldpath.NewVersionedSet(
					_NS(
						_P("list"),
						_P("list", _KBF("name", "a")),
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					false,
				),
				"applier": fieldpath.NewVersionedSet(
					_NS(
						_P("unrelated"),
					),
					"v1",
					true,
				),
			},
		},
		// TODO: Owning the key is a little messed-up.
		"list/merge/change_duplicated_item": {
			Ops: []Operation{
				Update{
					Manager: "updater",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "applier",
					Object: `
						list:
						- name: a
						  value1: 3
					`,
					APIVersion: "v1",
					Conflicts: merge.Conflicts{
						{Manager: "updater", Path: _P("list", _KBF("name", "a"))},
					},
				},
				ForceApply{
					Manager: "applier",
					Object: `
						list:
						- name: a
						  value1: 3
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  value1: 3
				- name: b
				  value1: 3
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"updater": fieldpath.NewVersionedSet(
					_NS(
						_P("list"),
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					false,
				),
				"applier": fieldpath.NewVersionedSet(
					_NS(
						_P("list", _KBF("name", "a")),
						_P("list", _KBF("name", "a"), "name"),
						_P("list", _KBF("name", "a"), "value1"),
					),
					"v1",
					true,
				),
			},
		},

		"list/merge/unchanged_duplicated_item": {
			Ops: []Operation{
				Update{
					Manager: "updater",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "applier",
					Object: `
						list:
						- name: a
						  value1: 2
					`,
					APIVersion: "v1",
					Conflicts: merge.Conflicts{
						{Manager: "updater", Path: _P("list", _KBF("name", "a"))},
					},
				},
				ForceApply{
					Manager: "applier",
					Object: `
						list:
						- name: a
						  value1: 3
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  value1: 3
				- name: b
				  value1: 3
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"updater": fieldpath.NewVersionedSet(
					_NS(
						_P("list"),
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					false,
				),
				"applier": fieldpath.NewVersionedSet(
					_NS(
						_P("list", _KBF("name", "a")),
						_P("list", _KBF("name", "a"), "name"),
						_P("list", _KBF("name", "a"), "value1"),
					),
					"v1",
					true,
				),
			},
		},
		"list/merge/change_non_duplicated_item": {
			Ops: []Operation{
				Update{
					Manager: "updater",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
				ForceApply{
					Manager: "applier",
					Object: `
						list:
						- name: b
						  value1: 4
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  value1: 1
				- name: a
				  value1: 2
				- name: b
				  value1: 4
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"updater": fieldpath.NewVersionedSet(
					_NS(
						_P("list"),
						_P("list", _KBF("name", "a")),
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
					),
					"v1",
					false,
				),
				"applier": fieldpath.NewVersionedSet(
					_NS(
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					true,
				),
			},
		},
		"list/merge/apply_update_duplicates_apply_without": {
			Ops: []Operation{
				Apply{
					Manager: "applier",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
				Update{
					Manager: "updater",
					Object: `
						list:
						- name: a
						  value1: 1
						- name: a
						  value1: 2
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
				Apply{
					Manager: "applier",
					Object: `
						list:
						- name: b
						  value1: 3
					`,
					APIVersion: "v1",
				},
			},
			Object: `
				list:
				- name: a
				  value1: 1
				- name: a
				  value1: 2
				- name: b
				  value1: 3
			`,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"applier": fieldpath.NewVersionedSet(
					_NS(
						_P("list", _KBF("name", "b")),
						_P("list", _KBF("name", "b"), "name"),
						_P("list", _KBF("name", "b"), "value1"),
					),
					"v1",
					true,
				),
				"updater": fieldpath.NewVersionedSet(
					_NS(
						_P("list", _KBF("name", "a")),
					),
					"v1",
					false,
				),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test.Test(duplicatesParser); err != nil {
				t.Fatal(err)
			}
		})
	}
}

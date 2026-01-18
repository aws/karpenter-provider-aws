package merge_test

import (
	"testing"

	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v6/internal/fixture"
	"sigs.k8s.io/structured-merge-diff/v6/merge"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

func TestFieldLevelOverrides(t *testing.T) {
	var overrideStructTypeParser = func() fixture.Parser {
		parser, err := typed.NewParser(`
        types:
        - name: type
          map:
            fields:
              - name: associativeListReference
                type:
                  namedType: associativeList
                  elementRelationship: atomic
              - name: separableInlineList
                type:
                  list:
                    elementType:
                      scalar: numeric
                    elementRelationship: atomic
                  elementRelationship: associative
              - name: separableMapReference
                type:
                  namedType: atomicMap
                  elementRelationship: separable
              - name: atomicMapReference
                type:
                  namedType: unspecifiedMap
                  elementRelationship: atomic

        - name: associativeList
          list:
            elementType:
              namedType: unspecifiedMap
              elementRelationship: atomic
            elementRelationship: associative
            keys:
            - name
        - name: unspecifiedMap
          map:
            fields:
            - name: name
              type:
                scalar: string
            - name: value
              type:
                scalar: numeric
        - name: atomicMap
          map:
            elementRelationship: atomic
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
		return fixture.SameVersionParser{T: parser.Type("type")}
	}()

	tests := map[string]fixture.TestCase{
		"test_override_atomic_map_with_separable": {
			// Test that a reference with an separable override to an atomic type
			// is treated as separable
			Ops: []fixture.Operation{
				fixture.Apply{
					Manager: "apply_one",
					Object: `
                        separableMapReference:
                          name: a
                    `,
					APIVersion: "v1",
				},
				fixture.Apply{
					Manager: "apply_two",
					Object: `
                        separableMapReference:
                          value: 2
                    `,
					APIVersion: "v1",
				},
			},
			Object: `
                separableMapReference:
                  name: a
                  value: 2
            `,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"apply_one": fieldpath.NewVersionedSet(
					_NS(
						_P("separableMapReference", "name"),
					),
					"v1",
					false,
				),
				"apply_two": fieldpath.NewVersionedSet(
					_NS(
						_P("separableMapReference", "value"),
					),
					"v1",
					false,
				),
			},
		},
		"test_override_unspecified_map_with_atomic": {
			// Test that a map which has its element relaetionship left as defualt
			// (granular) can be overriden to be atomic
			Ops: []fixture.Operation{
				fixture.Apply{
					Manager: "apply_one",
					Object: `
                        atomicMapReference:
                          name: a
                    `,
					APIVersion: "v1",
				},
				fixture.Apply{
					Manager: "apply_two",
					Object: `
                        atomicMapReference:
                          value: 2
                    `,
					APIVersion: "v1",
					Conflicts: merge.Conflicts{
						merge.Conflict{Manager: "apply_one", Path: _P("atomicMapReference")},
					},
				},
				fixture.Apply{
					Manager: "apply_one",
					Object: `
                        atomicMapReference:
                          name: b
                          value: 2
                    `,
					APIVersion: "v1",
				},
			},
			Object: `
                atomicMapReference:
                  name: b
                  value: 2
            `,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"apply_one": fieldpath.NewVersionedSet(
					_NS(
						_P("atomicMapReference"),
					),
					"v1",
					false,
				),
			},
		},
		"test_override_associative_list_with_atomic": {
			// Test that if a list type is listed associative but referred to as atomic
			// that attempting to add to the list fauks
			Ops: []fixture.Operation{
				fixture.Apply{
					Manager: "apply_one",
					Object: `
                        associativeListReference:
                          - name: a
                            value: 1
                    `,
					APIVersion: "v1",
				},
				fixture.Apply{
					Manager: "apply_two",
					Object: `
                        associativeListReference:
                        - name: b
                          value: 2
                    `,
					APIVersion: "v1",
					Conflicts: merge.Conflicts{
						merge.Conflict{Manager: "apply_one", Path: _P("associativeListReference")},
					},
				},
			},
			Object: `
                associativeListReference:
                  - name: a
                    value: 1
            `,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"apply_one": fieldpath.NewVersionedSet(
					_NS(
						_P("associativeListReference"),
					),
					"v1",
					false,
				),
			},
		},
		"test_override_inline_atomic_list_with_associative": {
			// Tests that an inline atomic list can have its type overridden to be
			// associative
			Ops: []fixture.Operation{
				fixture.Apply{
					Manager: "apply_one",
					Object: `
                        separableInlineList:
                        - 1
                    `,
					APIVersion: "v1",
				},
				fixture.Apply{
					Manager: "apply_two",
					Object: `
                        separableInlineList:
                        - 2
                    `,
					APIVersion: "v1",
				},
			},
			Object: `
                separableInlineList:
                - 1
                - 2
            `,
			APIVersion: "v1",
			Managed: fieldpath.ManagedFields{
				"apply_one": fieldpath.NewVersionedSet(
					_NS(
						_P("separableInlineList", _V(1)),
					),
					"v1",
					true,
				),
				"apply_two": fieldpath.NewVersionedSet(
					_NS(
						_P("separableInlineList", _V(2)),
					),
					"v1",
					true,
				),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := test.Test(overrideStructTypeParser); err != nil {
				t.Fatal(err)
			}
		})
	}
}

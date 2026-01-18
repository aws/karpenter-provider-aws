package rules

import (
	"reflect"
	"testing"

	"k8s.io/gengo/v2/types"
)

func TestListTypeMissing(t *testing.T) {
	tcs := []struct {
		// name of test case
		name string
		t    *types.Type

		// expected list of violation fields
		expected []string
	}{
		{
			name:     "none",
			t:        &types.Type{},
			expected: []string{},
		},
		{
			name: "simple missing",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "Containers",
						Type: &types.Type{
							Kind: types.Slice,
						},
					},
				},
			},
			expected: []string{"Containers"},
		},
		{
			name: "simple passing",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "Containers",
						Type: &types.Type{
							Kind: types.Slice,
						},
						CommentLines: []string{"+listType=map"},
					},
				},
			},
			expected: []string{},
		},

		{
			name: "list Items field should not be annotated",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "Items",
						Type: &types.Type{
							Kind: types.Slice,
						},
						CommentLines: []string{"+listType=map"},
					},
					{
						Name:     "ListMeta",
						Embedded: true,
						Type: &types.Type{
							Kind: types.Struct,
						},
					},
				},
			},
			expected: []string{"Items"},
		},

		{
			name: "list Items field without annotation should pass validation",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "Items",
						Type: &types.Type{
							Kind: types.Slice,
						},
					},
					{
						Name:     "ListMeta",
						Embedded: true,
						Type: &types.Type{
							Kind: types.Struct,
						},
					},
				},
			},
			expected: []string{},
		},

		{
			name: "a list that happens to be called Items (i.e. nested, not top-level list) needs annotations",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "Items",
						Type: &types.Type{
							Kind: types.Slice,
						},
					},
				},
			},
			expected: []string{"Items"},
		},

		{
			name: "a byte-slice field without annotation should pass validation",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "ByteSliceField",
						Type: &types.Type{
							Kind: types.Slice,
							Elem: types.Byte,
						},
					},
				},
			},
			expected: []string{},
		},
	}

	rule := &ListTypeMissing{}
	for _, tc := range tcs {
		if violations, _ := rule.Validate(tc.t); !reflect.DeepEqual(violations, tc.expected) {
			t.Errorf("unexpected validation result: test name %q, want: %v, got: %v",
				tc.name, tc.expected, violations)
		}
	}
}

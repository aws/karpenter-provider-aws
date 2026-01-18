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

package rules

import (
	"reflect"
	"testing"

	"k8s.io/gengo/v2/types"
)

func TestNamesMatch(t *testing.T) {
	someStruct := &types.Type{
		Name: types.Name{Name: "SomeStruct"},
		Kind: types.Struct,
	}
	someStructPtr := &types.Type{
		Name: types.Name{Name: "SomeStructPtr"},
		Kind: types.Pointer,
		Elem: someStruct,
	}
	intPtr := &types.Type{
		Name: types.Name{Name: "IntPtr"},
		Kind: types.Pointer,
		Elem: types.Int,
	}
	listMeta := &types.Type{
		Name: types.Name{Package: "k8s.io/apimachinery/pkg/apis/meta/v1", Name: "ListMeta"},
		Kind: types.Struct,
	}
	listMetaPtr := &types.Type{
		Name: types.Name{Package: "k8s.io/apimachinery/pkg/apis/meta/v1", Name: "ListMetaPtr"},
		Kind: types.Pointer,
		Elem: listMeta,
	}
	objectMeta := &types.Type{
		Name: types.Name{Package: "k8s.io/apimachinery/pkg/apis/meta/v1", Name: "ObjectMeta"},
		Kind: types.Struct,
	}
	objectMetaPtr := &types.Type{
		Name: types.Name{Package: "k8s.io/apimachinery/pkg/apis/meta/v1", Name: "ObjectMetaPtr"},
		Kind: types.Pointer,
		Elem: objectMeta,
	}

	tcs := []struct {
		// name of test case
		name string
		t    *types.Type

		// expected list of violation fields
		expected []string
	}{
		// The comments are in format of {goName, jsonName, match},
		// {"PodSpec", "podSpec", true},
		{
			name: "simple",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "PodSpec",
						Tags: `json:"podSpec"`,
					},
				},
			},
			expected: []string{},
		},
		// {"PodSpec", "podSpec", true},
		{
			name: "multiple_json_tags",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "PodSpec",
						Tags: `json:"podSpec,omitempty"`,
					},
				},
			},
			expected: []string{},
		},
		// {"PodSpec", "podSpec", true},
		{
			name: "protobuf_tag",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "PodSpec",
						Tags: `json:"podSpec,omitempty" protobuf:"bytes,1,opt,name=podSpec"`,
					},
				},
			},
			expected: []string{},
		},
		// {"", "podSpec", false},
		{
			name: "empty",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "",
						Tags: `json:"podSpec"`,
					},
				},
			},
			expected: []string{""},
		},
		// {"PodSpec", "PodSpec", false},
		{
			name: "CamelCase_CamelCase",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "PodSpec",
						Tags: `json:"PodSpec"`,
					},
				},
			},
			expected: []string{"PodSpec"},
		},
		// {"podSpec", "podSpec", false},
		{
			name: "camelCase_camelCase",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "podSpec",
						Tags: `json:"podSpec"`,
					},
				},
			},
			expected: []string{"podSpec"},
		},
		// {"PodSpec", "spec", false},
		{
			name: "short_json_name",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "PodSpec",
						Tags: `json:"spec"`,
					},
				},
			},
			expected: []string{"PodSpec"},
		},
		// {"Spec", "podSpec", false},
		{
			name: "long_json_name",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "Spec",
						Tags: `json:"podSpec"`,
					},
				},
			},
			expected: []string{"Spec"},
		},
		// {"JSONSpec", "jsonSpec", true},
		{
			name: "acronym",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "JSONSpec",
						Tags: `json:"jsonSpec"`,
					},
				},
			},
			expected: []string{},
		},
		// {"JSONSpec", "jsonspec", false},
		{
			name: "acronym_invalid",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "JSONSpec",
						Tags: `json:"jsonspec"`,
					},
				},
			},
			expected: []string{"JSONSpec"},
		},
		// {"HTTPJSONSpec", "httpJSONSpec", true},
		{
			name: "multiple_acronym",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "HTTPJSONSpec",
						Tags: `json:"httpJSONSpec"`,
					},
				},
			},
			expected: []string{},
		},
		// // NOTE: this validator cannot tell two sequential all-capital words from one word,
		// // therefore the case below is also considered matched.
		// {"HTTPJSONSpec", "httpjsonSpec", true},
		{
			name: "multiple_acronym_as_one",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "HTTPJSONSpec",
						Tags: `json:"httpjsonSpec"`,
					},
				},
			},
			expected: []string{},
		},
		// NOTE: JSON tags in jsonTagBlacklist should skip evaluation
		{
			name: "blacklist_tag_dash",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "podSpec",
						Tags: `json:"-"`,
					},
				},
			},
			expected: []string{},
		},
		// {"PodSpec", "-", false},
		{
			name: "invalid_json_name_dash",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "PodSpec",
						Tags: `json:"-,"`,
					},
				},
			},
			expected: []string{"PodSpec"},
		},
		// {"podSpec", "metadata", false},
		{
			name: "blacklist_metadata",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "podSpec",
						Tags: `json:"metadata"`,
					},
				},
			},
			expected: []string{"podSpec"},
		},
		{
			name: "non_struct",
			t: &types.Type{
				Kind: types.Map,
			},
			expected: []string{},
		},
		{
			name: "no_json_tag",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "PodSpec",
						Tags: `podSpec`,
					},
				},
			},
			expected: []string{"PodSpec"},
		},
		// NOTE: this is to expand test coverage
		// {"S", "s", true},
		{
			name: "single_character",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "S",
						Tags: `json:"s"`,
					},
				},
			},
			expected: []string{},
		},
		// NOTE: names with disallowed substrings should fail evaluation
		// {"Pod-Spec", "pod-Spec", false},
		{
			name: "disallowed_substring_dash",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "Pod-Spec",
						Tags: `json:"pod-Spec"`,
					},
				},
			},
			expected: []string{"Pod-Spec"},
		},
		// {"Pod_Spec", "pod_Spec", false},
		{
			name: "disallowed_substring_underscore",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "Pod_Spec",
						Tags: `json:"pod_Spec"`,
					},
				},
			},
			expected: []string{"Pod_Spec"},
		},
		{
			name: "empty_JSON_name",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "Int",
						Tags: `json:""`, // Not okay!
						Type: types.Int,
					},
					{
						Name: "Struct",
						Tags: `json:""`, // Okay, inlined.
						Type: someStruct,
					},
					{
						Name: "IntPtr",
						Tags: `json:""`, // Not okay!
						Type: intPtr,
					},
					{
						Name: "StructPtr",
						Tags: `json:""`, // Okay, inlined.
						Type: someStructPtr,
					},
				},
			},
			expected: []string{
				"Int",
				"IntPtr",
			},
		},
		{
			name: "metadata_no_pointers",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "ListMeta",
						Tags: `json:"listMeta"`, // Not okay, should be "metadata"!
						Type: listMeta,
					},
					{
						Name: "ObjectMeta",
						Tags: `json:"objectMeta"`, // Not okay, should be metadata"!
						Type: objectMeta,
					},
					{
						Name: "SomeStruct",
						Tags: `json:"metadata"`, // Not okay, name mismatch!
						Type: someStruct,
					},
				},
			},
			expected: []string{
				"ListMeta",
				"ObjectMeta",
				"SomeStruct",
			},
		},
		{
			name: "metadata_pointers",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "ListMeta",
						Tags: `json:"listMeta"`, // Okay, convention only applies to struct.
						Type: listMetaPtr,
					},
					{
						Name: "ObjectMeta",
						Tags: `json:"objectMeta"`, // Okay, convention only applies to struct.
						Type: objectMetaPtr,
					},
					{
						Name: "SomeStruct",
						Tags: `json:"metadata"`, // Not okay, name mismatch!
						Type: someStructPtr,
					},
				},
			},
			expected: []string{
				"SomeStruct",
			},
		},
	}

	n := &NamesMatch{}
	for _, tc := range tcs {
		if violations, _ := n.Validate(tc.t); !reflect.DeepEqual(violations, tc.expected) {
			t.Errorf("unexpected validation result: test name %v, want: %v, got: %v",
				tc.name, tc.expected, violations)
		}
	}
}

// TestRuleName tests the Name of API rule. This is to expand test coverage
func TestRuleName(t *testing.T) {
	ruleName := "names_match"
	n := &NamesMatch{}
	if n.Name() != ruleName {
		t.Errorf("unexpected API rule name: want: %v, got: %v", ruleName, n.Name())
	}
}

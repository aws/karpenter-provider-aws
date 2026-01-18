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

func TestStreamingListTypeFieldOrder(t *testing.T) {
	tcs := []struct {
		// name of test case
		name string
		t    *types.Type

		// expected list of violation fields
		expectedFields []string
	}{
		{
			name: "simple list",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
					},
					{
						Name: "ListMeta",
					},
					{
						Name: "Items",
					},
				},
			},
		},
		{
			name: "not a list",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "ListMeta",
					},
				},
			},
		},
		{
			name: "extended list",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
					},
					{
						Name: "ListMeta",
					},
					{
						Name: "Items",
					},
					{
						Name: "Additional",
					},
				},
			},
		},
		{
			name: "list with bad field order",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
					},
					{
						Name: "Items",
					},
					{
						Name: "ListMeta",
					},
				},
			},
			expectedFields: []string{"ListMeta", "Items"},
		},
	}

	n := &StreamingListTypeFieldOrder{}
	for _, tc := range tcs {
		if fields, _ := n.Validate(tc.t); !reflect.DeepEqual(fields, tc.expectedFields) {
			t.Errorf("unexpected validation result: test name %v, want: %v, got: %v",
				tc.name, tc.expectedFields, fields)
		}
	}
}

func TestStreamingListTypeJSONTags(t *testing.T) {
	tcs := []struct {
		// name of test case
		name string
		t    *types.Type

		// expected list of violation fields
		expectedFields []string
	}{
		{
			name: "simple list",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
						Tags: `json:",inline"`,
					},
					{
						Name: "ListMeta",
						Tags: `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`,
					},
					{
						Name: "Items",
						Tags: `json:"items" protobuf:"bytes,2,rep,name=items"`,
					},
				},
			},
		},
		{
			name: "not a list",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "ListMeta",
					},
				},
			},
		},
		{
			name: "extended list",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
					},
					{
						Name: "ListMeta",
					},
					{
						Name: "Items",
					},
					{
						Name: "Additional",
					},
				},
			},
		},
		{
			name: "bad typemeta json tag",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
						Tags: `json:"typemeta"`, // subfield typemeta instead of inline
					},
					{
						Name: "ListMeta",
						Tags: `json:"metadata,omitempty"`,
					},
					{
						Name: "Items",
						Tags: `json:"items"`,
					},
				},
			},
			expectedFields: []string{"TypeMeta"},
		},
		{
			name: "bad listmeta json tag",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
						Tags: `json:",inline"`,
					},
					{
						Name: "ListMeta",
						Tags: `json:"listmeta,omitempty"`, // renamed from "metadata" to "listmeta"
					},
					{
						Name: "Items",
						Tags: `json:"items"`,
					},
				},
			},
			expectedFields: []string{"ListMeta"},
		},
		{
			name: "bad listmeta json tag",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
						Tags: `json:",inline"`,
					},
					{
						Name: "ListMeta",
						Tags: `json:"metadata,omitempty"`,
					},
					{
						Name: "Items",
						Tags: `json:"items,omitempty"`, // added omitempty
					},
				},
			},
			expectedFields: []string{"Items"},
		},
	}

	n := &StreamingListTypeJSONTags{}
	for _, tc := range tcs {
		if fields, _ := n.Validate(tc.t); !reflect.DeepEqual(fields, tc.expectedFields) {
			t.Errorf("unexpected validation result: test name %v, want: %v, got: %v",
				tc.name, tc.expectedFields, fields)
		}
	}
}

func TestStreamingListTypeProtoTags(t *testing.T) {
	tcs := []struct {
		// name of test case
		name string
		t    *types.Type

		// expected list of violation fields
		expectedFields []string
	}{
		{
			name: "simple list",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
						Tags: `json:",inline"`,
					},
					{
						Name: "ListMeta",
						Tags: `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`,
					},
					{
						Name: "Items",
						Tags: `json:"items" protobuf:"bytes,2,rep,name=items"`,
					},
				},
			},
		},
		{
			name: "not a list",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "ListMeta",
					},
				},
			},
		},
		{
			name: "extended list",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
					},
					{
						Name: "ListMeta",
					},
					{
						Name: "Items",
					},
					{
						Name: "Additional",
					},
				},
			},
		},
		{
			name: "bad typemeta proto tag",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
						Tags: `json:",inline" protobuf:"bytes,3,opt,name=typemeta"`, // Added protobuf tag
					},
					{
						Name: "ListMeta",
						Tags: `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`,
					},
					{
						Name: "Items",
						Tags: `json:"items" protobuf:"bytes,2,rep,name=items"`,
					},
				},
			},
			expectedFields: []string{"TypeMeta"},
		},
		{
			name: "bad listmeta json tag",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
						Tags: `json:",inline"`,
					},
					{
						Name: "ListMeta",
						Tags: `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=listmetadata"`, // Changed name to listmeta
					},
					{
						Name: "Items",
						Tags: `json:"items" protobuf:"bytes,2,rep,name=items"`,
					},
				},
			},
			expectedFields: []string{"ListMeta"},
		},
		{
			name: "bad listmeta json tag",
			t: &types.Type{
				Kind: types.Struct,
				Members: []types.Member{
					{
						Name: "TypeMeta",
						Tags: `json:",inline"`,
					},
					{
						Name: "ListMeta",
						Tags: `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`,
					},
					{
						Name: "Items",
						Tags: `json:"items" protobuf:"bytes,3,rep,name=items"`, // Change field number to 3
					},
				},
			},
			expectedFields: []string{"Items"},
		},
	}

	n := &StreamingListTypeProtoTags{}
	for _, tc := range tcs {
		if fields, _ := n.Validate(tc.t); !reflect.DeepEqual(fields, tc.expectedFields) {
			t.Errorf("unexpected validation result: test name %v, want: %v, got: %v",
				tc.name, tc.expectedFields, fields)
		}
	}
}

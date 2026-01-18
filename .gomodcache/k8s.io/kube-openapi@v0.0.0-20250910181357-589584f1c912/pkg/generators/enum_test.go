/*
Copyright 2022 The Kubernetes Authors.

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

package generators

import (
	"reflect"
	"sort"
	"testing"

	"k8s.io/gengo/v2/generator"
	"k8s.io/gengo/v2/types"
)

func TestParseEnums(t *testing.T) {
	for _, tc := range []struct {
		name     string
		universe types.Universe
		expected map[string][]string
	}{
		{
			name: "value in different package",
			universe: types.Universe{
				"foo": &types.Package{
					Name: "foo",
					Types: map[string]*types.Type{
						"Foo": {
							Name: types.Name{
								Package: "foo",
								Name:    "Foo",
							},
							Kind:         types.Alias,
							Underlying:   types.String,
							CommentLines: []string{"+enum"},
						},
					},
				},
				"bar": &types.Package{
					Name: "bar",
					Constants: map[string]*types.Type{
						"Bar": {
							Name: types.Name{
								Package: "bar",
								Name:    "Bar",
							},
							Kind: types.Alias,
							Underlying: &types.Type{
								Name: types.Name{
									Package: "foo",
									Name:    "Foo",
								},
							},
							ConstValue: &[]string{"bar"}[0],
						},
					},
				},
			},
			expected: map[string][]string{
				"foo.Foo": {"bar"},
			},
		},
		{
			name: "value in same package",
			universe: types.Universe{
				"foo": &types.Package{
					Name: "foo",
					Types: map[string]*types.Type{
						"Foo": {
							Name: types.Name{
								Package: "foo",
								Name:    "Foo",
							},
							Kind:         types.Alias,
							Underlying:   types.String,
							CommentLines: []string{"+enum"},
						},
					},
					Constants: map[string]*types.Type{
						"Bar": {
							Name: types.Name{
								Package: "foo",
								Name:    "Bar",
							},
							Kind: types.Alias,
							Underlying: &types.Type{
								Name: types.Name{
									Package: "foo",
									Name:    "Foo",
								},
							},
							ConstValue: &[]string{"bar"}[0],
						},
					},
				},
			},
			expected: map[string][]string{
				"foo.Foo": {"bar"},
			},
		},
		{
			name: "values in same and different packages",
			universe: types.Universe{
				"foo": &types.Package{
					Name: "foo",
					Types: map[string]*types.Type{
						"Foo": {
							Name: types.Name{
								Package: "foo",
								Name:    "Foo",
							},
							Kind:         types.Alias,
							Underlying:   types.String,
							CommentLines: []string{"+enum"},
						},
					},
					Constants: map[string]*types.Type{
						"FooSame": {
							Name: types.Name{
								Package: "foo",
								Name:    "FooSame",
							},
							Kind: types.Alias,
							Underlying: &types.Type{
								Name: types.Name{
									Package: "foo",
									Name:    "Foo",
								},
							},
							ConstValue: &[]string{"same"}[0],
						},
					},
				},
				"bar": &types.Package{
					Name: "bar",
					Constants: map[string]*types.Type{
						"FooDifferent": {
							Name: types.Name{
								Package: "bar",
								Name:    "FooDifferent",
							},
							Kind: types.Alias,
							Underlying: &types.Type{
								Name: types.Name{
									Package: "foo",
									Name:    "Foo",
								},
							},
							ConstValue: &[]string{"different"}[0],
						},
					},
				},
			},
			expected: map[string][]string{
				"foo.Foo": {"different", "same"},
			},
		},
		{
			name: "aliasing and re-exporting enum from different package",
			universe: types.Universe{
				"foo": &types.Package{
					Name: "foo",
					Types: map[string]*types.Type{
						"Foo": {
							Name: types.Name{
								Package: "foo",
								Name:    "Foo",
							},
							Kind:         types.Alias,
							Underlying:   types.String,
							CommentLines: []string{"+enum"},
						},
					},
					Constants: map[string]*types.Type{
						"FooCase1": {
							Name: types.Name{
								Package: "foo",
								Name:    "FooCase1",
							},
							Kind: types.DeclarationOf,
							Underlying: &types.Type{
								Name: types.Name{
									Package: "foo",
									Name:    "Foo",
								},
							},
							ConstValue: &[]string{"case1"}[0],
						},
						"FooCase2": {
							Name: types.Name{
								Package: "foo",
								Name:    "FooCase2",
							},
							Kind: types.DeclarationOf,
							Underlying: &types.Type{
								Name: types.Name{
									Package: "foo",
									Name:    "Foo",
								},
							},
							ConstValue: &[]string{"case2"}[0],
						},
					},
				},
				"bar": &types.Package{
					Name: "bar",
					Constants: map[string]*types.Type{
						"FooCase1": {
							Name: types.Name{
								Package: "foo",
								Name:    "FooCase1",
							},
							Kind: types.DeclarationOf,
							Underlying: &types.Type{
								Name: types.Name{
									Package: "foo",
									Name:    "Foo",
								},
							},
							ConstValue: &[]string{"case1"}[0],
						},
						"FooCase2": {
							Name: types.Name{
								Package: "foo",
								Name:    "FooCase2",
							},
							Kind: types.DeclarationOf,
							Underlying: &types.Type{
								Name: types.Name{
									Package: "foo",
									Name:    "Foo",
								},
							},
							ConstValue: &[]string{"case2"}[0],
						},
					},
				},
			},
			expected: map[string][]string{
				"foo.Foo": {"case1", "case2"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enums := parseEnums(&generator.Context{Universe: tc.universe})

			actual := make(map[string][]string)
			for _, enum := range enums {
				values := make([]string, len(enum.Values))
				for i := range values {
					values[i] = enum.Values[i].Value
				}
				sort.Strings(values)
				actual[enum.Name.String()] = values
			}

			if !reflect.DeepEqual(tc.expected, actual) {
				t.Errorf("expected: %#v, got %#v", tc.expected, actual)
			}
		})
	}
}

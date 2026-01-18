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
package generators_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"k8s.io/gengo/v2/types"
	"k8s.io/kube-openapi/pkg/generators"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/utils/ptr"
)

var structKind *types.Type = &types.Type{Kind: types.Struct, Name: types.Name{Name: "struct"}}
var mapType *types.Type = &types.Type{Kind: types.Map, Name: types.Name{Name: "map[string]int"}}
var arrayType *types.Type = &types.Type{Kind: types.Slice, Name: types.Name{Name: "[]int"}}

func TestParseCommentTags(t *testing.T) {

	cases := []struct {
		t        *types.Type
		name     string
		comments []string
		expected *spec.Schema

		// regex pattern matching the error, or empty string/unset if no error
		// is expected
		expectedError string
	}{
		{
			t:    structKind,
			name: "basic example",
			comments: []string{
				"comment",
				"another + comment",
				"+k8s:validation:minimum=10.0",
				"+k8s:validation:maximum=20.0",
				"+k8s:validation:minLength=20",
				"+k8s:validation:maxLength=30",
				`+k8s:validation:pattern="asdf"`,
				"+k8s:validation:multipleOf=1.0",
				"+k8s:validation:minItems=1",
				"+k8s:validation:maxItems=2",
				"+k8s:validation:uniqueItems=true",
				"exclusiveMaximum=true",
				"not+k8s:validation:Minimum=0.0",
			},
			expectedError: `invalid marker comments: maxItems can only be used on array types
minItems can only be used on array types
uniqueItems can only be used on array types
minLength can only be used on string types
maxLength can only be used on string types
pattern can only be used on string types
minimum can only be used on numeric types
maximum can only be used on numeric types
multipleOf can only be used on numeric types`,
		},
		{
			t:    arrayType,
			name: "basic array example",
			comments: []string{
				"comment",
				"another + comment",
				"+k8s:validation:minItems=1",
				"+k8s:validation:maxItems=2",
				"+k8s:validation:uniqueItems=true",
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					MinItems:    ptr.To[int64](1),
					MaxItems:    ptr.To[int64](2),
					UniqueItems: true,
				},
			},
		},
		{
			t:    mapType,
			name: "basic map example",
			comments: []string{
				"comment",
				"another + comment",
				"+k8s:validation:minProperties=1",
				"+k8s:validation:maxProperties=2",
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					MinProperties: ptr.To[int64](1),
					MaxProperties: ptr.To[int64](2),
				},
			},
		},
		{
			t:    types.String,
			name: "basic string example",
			comments: []string{
				"comment",
				"another + comment",
				"+k8s:validation:minLength=20",
				"+k8s:validation:maxLength=30",
				`+k8s:validation:pattern="asdf"`,
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					MinLength: ptr.To[int64](20),
					MaxLength: ptr.To[int64](30),
					Pattern:   "asdf",
				},
			},
		},
		{
			t:    types.Int,
			name: "basic int example",
			comments: []string{
				"comment",
				"another + comment",
				"+k8s:validation:minimum=10.0",
				"+k8s:validation:maximum=20.0",
				"+k8s:validation:multipleOf=1.0",
				"exclusiveMaximum=true",
				"not+k8s:validation:Minimum=0.0",
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Maximum:    ptr.To(20.0),
					Minimum:    ptr.To(10.0),
					MultipleOf: ptr.To(1.0),
				},
			},
		},
		{
			t:        structKind,
			name:     "empty",
			expected: &spec.Schema{},
		},
		{
			t:    types.Float64,
			name: "single",
			comments: []string{
				"+k8s:validation:minimum=10.0",
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Minimum: ptr.To(10.0),
				},
			},
		},
		{
			t:    types.Float64,
			name: "multiple",
			comments: []string{
				"+k8s:validation:minimum=10.0",
				"+k8s:validation:maximum=20.0",
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Maximum: ptr.To(20.0),
					Minimum: ptr.To(10.0),
				},
			},
		},
		{
			t:    types.Float64,
			name: "invalid duplicate key",
			comments: []string{
				"+k8s:validation:minimum=10.0",
				"+k8s:validation:maximum=20.0",
				"+k8s:validation:minimum=30.0",
			},
			expectedError: `failed to parse marker comments: cannot have multiple values for key 'minimum'`,
		},
		{
			t:    structKind,
			name: "unrecognized key is ignored",
			comments: []string{
				"+ignored=30.0",
			},
			expected: &spec.Schema{},
		},
		{
			t:    types.Float64,
			name: "invalid: non-JSON value",
			comments: []string{
				`+k8s:validation:minimum=asdf`,
			},
			expectedError: `failed to parse marker comments: failed to parse value for key minimum as JSON: invalid character 'a' looking for beginning of value`,
		},
		{
			t:    types.Float64,
			name: "invalid: invalid value type",
			comments: []string{
				`+k8s:validation:minimum="asdf"`,
			},
			expectedError: `failed to unmarshal marker comments: json: cannot unmarshal string into Go struct field commentTags.minimum of type float64`,
		},
		{

			t:    structKind,
			name: "invalid: empty key",
			comments: []string{
				"+k8s:validation:",
			},
			expectedError: `failed to parse marker comments: cannot have empty key for marker comment`,
		},
		{
			t: types.Float64,
			// temporary test. ref support may be added in the future
			name: "ignore refs",
			comments: []string{
				"+k8s:validation:pattern=ref(asdf)",
			},
			expected: &spec.Schema{},
		},
		{
			t:    types.Float64,
			name: "cel rule",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:message="immutable field"`,
			},
			expected: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{
						"x-kubernetes-validations": []interface{}{
							map[string]interface{}{
								"rule":    "oldSelf == self",
								"message": "immutable field",
							},
						},
					},
				},
			},
		},
		{
			t:    types.Float64,
			name: "skipped CEL rule",
			comments: []string{
				// This should parse, but return an error in validation since
				// index 1 is missing
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:message="immutable field"`,
				`+k8s:validation:cel[2]:rule="self > 5"`,
				`+k8s:validation:cel[2]:message="must be greater than 5"`,
			},
			expectedError: `failed to parse marker comments: error parsing cel[2]:rule="self > 5": non-consecutive index 2 for key 'cel'`,
		},
		{
			t:    types.Float64,
			name: "multiple CEL params",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:message="immutable field"`,
				`+k8s:validation:cel[1]:rule="self > 5"`,
				`+k8s:validation:cel[1]:optionalOldSelf=true`,
				`+k8s:validation:cel[1]:message="must be greater than 5"`,
			},
			expected: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{
						"x-kubernetes-validations": []interface{}{
							map[string]interface{}{
								"rule":    "oldSelf == self",
								"message": "immutable field",
							},
							map[string]interface{}{
								"rule":            "self > 5",
								"optionalOldSelf": true,
								"message":         "must be greater than 5",
							},
						},
					},
				},
			},
		},
		{
			t:    types.Float64,
			name: "multiple rules with multiple params",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:optionalOldSelf`,
				`+k8s:validation:cel[0]:messageExpression="self + ' must be equal to old value'"`,
				`+k8s:validation:cel[1]:rule="self > 5"`,
				`+k8s:validation:cel[1]:optionalOldSelf=true`,
				`+k8s:validation:cel[1]:message="must be greater than 5"`,
			},
			expected: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{
						"x-kubernetes-validations": []interface{}{
							map[string]interface{}{
								"rule":              "oldSelf == self",
								"optionalOldSelf":   true,
								"messageExpression": "self + ' must be equal to old value'",
							},
							map[string]interface{}{
								"rule":            "self > 5",
								"optionalOldSelf": true,
								"message":         "must be greater than 5",
							},
						},
					},
				},
			},
		},
		{
			t:    types.Float64,
			name: "skipped array index",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:optionalOldSelf`,
				`+k8s:validation:cel[0]:messageExpression="self + ' must be equal to old value'"`,
				`+k8s:validation:cel[2]:rule="self > 5"`,
				`+k8s:validation:cel[2]:optionalOldSelf=true`,
				`+k8s:validation:cel[2]:message="must be greater than 5"`,
			},
			expectedError: `failed to parse marker comments: error parsing cel[2]:rule="self > 5": non-consecutive index 2 for key 'cel'`,
		},
		{
			t:    types.Float64,
			name: "non-consecutive array index",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[1]:rule="self > 5"`,
				`+k8s:validation:cel[1]:message="self > 5"`,
				`+k8s:validation:cel[0]:optionalOldSelf=true`,
				`+k8s:validation:cel[0]:message="must be greater than 5"`,
			},
			expectedError: "failed to parse marker comments: error parsing cel[0]:optionalOldSelf=true: non-consecutive index 0 for key 'cel'",
		},
		{
			t:    types.Float64,
			name: "interjected array index",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:message="cant change"`,
				`+k8s:validation:cel[1]:rule="self > 5"`,
				`+k8s:validation:cel[1]:message="must be greater than 5"`,
				`+k8s:validation:minimum=5`,
				`+k8s:validation:cel[2]:rule="a rule"`,
				`+k8s:validation:cel[2]:message="message 2"`,
			},
			expectedError: "failed to parse marker comments: error parsing cel[2]:rule=\"a rule\": non-consecutive index 2 for key 'cel'",
		},
		{
			t:    types.Float64,
			name: "interjected array index with non-prefixed comment",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:message="cant change"`,
				`+k8s:validation:cel[1]:rule="self > 5"`,
				`+k8s:validation:cel[1]:message="must be greater than 5"`,
				`+minimum=5`,
				`+k8s:validation:cel[2]:rule="a rule"`,
				`+k8s:validation:cel[2]:message="message 2"`,
			},
			expectedError: "failed to parse marker comments: error parsing cel[2]:rule=\"a rule\": non-consecutive index 2 for key 'cel'",
		},
		{
			t:    types.Float64,
			name: "non-consecutive raw string indexing",
			comments: []string{
				`+k8s:validation:cel[0]:rule> raw string rule`,
				`+k8s:validation:cel[1]:rule="self > 5"`,
				`+k8s:validation:cel[1]:message="must be greater than 5"`,
				`+k8s:validation:cel[0]:message>another raw string message`,
			},
			expectedError: "failed to parse marker comments: error parsing cel[0]:message>another raw string message: non-consecutive index 0 for key 'cel'",
		},
		{
			t:    types.String,
			name: "non-consecutive string indexing false positive",
			comments: []string{
				`+k8s:validation:cel[0]:message>[3]string rule [1]`,
				`+k8s:validation:cel[0]:rule="string rule [1]"`,
				`+k8s:validation:pattern="self[3] == 'hi'"`,
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Pattern: `self[3] == 'hi'`,
				},
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{
						"x-kubernetes-validations": []interface{}{
							map[string]interface{}{
								"rule":    "string rule [1]",
								"message": "[3]string rule [1]",
							},
						},
					},
				},
			},
		},
		{
			t:    types.String,
			name: "non-consecutive raw string indexing false positive",
			comments: []string{
				`+k8s:validation:cel[0]:message>[3]raw string message with subscirpt [3]"`,
				`+k8s:validation:cel[0]:rule> raw string rule [1]`,
				`+k8s:validation:pattern>"self[3] == 'hi'"`,
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					Pattern: `"self[3] == 'hi'"`,
				},
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{
						"x-kubernetes-validations": []interface{}{
							map[string]interface{}{
								"rule":    "raw string rule [1]",
								"message": "[3]raw string message with subscirpt [3]\"",
							},
						},
					},
				},
			},
		},
		{
			t:    types.Float64,
			name: "boolean key at invalid index",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:message="cant change"`,
				`+k8s:validation:cel[2]:optionalOldSelf`,
			},
			expectedError: `failed to parse marker comments: error parsing cel[2]:optionalOldSelf: non-consecutive index 2 for key 'cel'`,
		},
		{
			t:    types.Float64,
			name: "boolean key after non-prefixed comment",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:message="cant change"`,
				`+k8s:validation:cel[1]:rule="self > 5"`,
				`+k8s:validation:cel[1]:message="must be greater than 5"`,
				`+minimum=5`,
				`+k8s:validation:cel[1]:optionalOldSelf`,
			},
			expectedError: `failed to parse marker comments: error parsing cel[1]:optionalOldSelf: non-consecutive index 1 for key 'cel'`,
		},
		{
			t:    types.Float64,
			name: "boolean key at index allowed",
			comments: []string{
				`+k8s:validation:cel[0]:rule="oldSelf == self"`,
				`+k8s:validation:cel[0]:message="cant change"`,
				`+k8s:validation:cel[1]:rule="self > 5"`,
				`+k8s:validation:cel[1]:message="must be greater than 5"`,
				`+k8s:validation:cel[1]:optionalOldSelf`,
			},
			expected: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{
						"x-kubernetes-validations": []interface{}{
							map[string]interface{}{
								"rule":    "oldSelf == self",
								"message": "cant change",
							},
							map[string]interface{}{
								"rule":            "self > 5",
								"message":         "must be greater than 5",
								"optionalOldSelf": true,
							},
						},
					},
				},
			},
		},
		{
			t:    types.Float64,
			name: "raw string rule",
			comments: []string{
				`+k8s:validation:cel[0]:rule> raw string rule`,
				`+k8s:validation:cel[0]:message="raw string message"`,
			},
			expected: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{
						"x-kubernetes-validations": []interface{}{
							map[string]interface{}{
								"rule":    "raw string rule",
								"message": "raw string message",
							},
						},
					},
				},
			},
		},
		{
			t:    types.Float64,
			name: "multiline string rule",
			comments: []string{
				`+k8s:validation:cel[0]:rule> self.length() % 2 == 0`,
				`+k8s:validation:cel[0]:rule>   ? self.field == self.name + ' is even'`,
				`+k8s:validation:cel[0]:rule>   : self.field == self.name + ' is odd'`,
				`+k8s:validation:cel[0]:message>raw string message`,
			},
			expected: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{
						"x-kubernetes-validations": []interface{}{
							map[string]interface{}{
								"rule":    "self.length() % 2 == 0\n? self.field == self.name + ' is even'\n: self.field == self.name + ' is odd'",
								"message": "raw string message",
							},
						},
					},
				},
			},
		},
		{
			t:    types.Float64,
			name: "mix raw and non-raw string marker",
			comments: []string{
				`+k8s:validation:cel[0]:message>raw string message`,
				`+k8s:validation:cel[0]:rule="self.length() % 2 == 0"`,
				`+k8s:validation:cel[0]:rule>  ? self.field == self.name + ' is even'`,
				`+k8s:validation:cel[0]:rule>  : self.field == self.name + ' is odd'`,
			},
			expected: &spec.Schema{
				VendorExtensible: spec.VendorExtensible{
					Extensions: map[string]interface{}{
						"x-kubernetes-validations": []interface{}{
							map[string]interface{}{
								"rule":    "self.length() % 2 == 0\n? self.field == self.name + ' is even'\n: self.field == self.name + ' is odd'",
								"message": "raw string message",
							},
						},
					},
				},
			},
		},
		{
			name: "raw string with different key in between",
			t:    types.Float64,
			comments: []string{
				`+k8s:validation:cel[0]:message>raw string message`,
				`+k8s:validation:cel[0]:rule="self.length() % 2 == 0"`,
				`+k8s:validation:cel[0]:message>raw string message 2`,
			},
			expectedError: `failed to parse marker comments: concatenations to key 'cel[0]:message' must be consecutive with its assignment`,
		},
		{
			name: "raw string with different raw string key in between",
			t:    types.Float64,
			comments: []string{
				`+k8s:validation:cel[0]:message>raw string message`,
				`+k8s:validation:cel[0]:rule>self.length() % 2 == 0`,
				`+k8s:validation:cel[0]:message>raw string message 2`,
			},
			expectedError: `failed to parse marker comments: concatenations to key 'cel[0]:message' must be consecutive with its assignment`,
		},
		{
			name: "nested cel",
			comments: []string{
				`+k8s:validation:items:cel[0]:rule="self.length() % 2 == 0"`,
				`+k8s:validation:items:cel[0]:message="must be even"`,
			},
			t: &types.Type{
				Kind: types.Alias,
				Underlying: &types.Type{
					Kind: types.Slice,
					Elem: types.String,
				},
			},
			expected: &spec.Schema{
				SchemaProps: spec.SchemaProps{
					AllOf: []spec.Schema{
						{
							SchemaProps: spec.SchemaProps{
								Items: &spec.SchemaOrArray{
									Schema: &spec.Schema{
										VendorExtensible: spec.VendorExtensible{
											Extensions: map[string]interface{}{
												"x-kubernetes-validations": []interface{}{
													map[string]interface{}{
														"rule":    "self.length() % 2 == 0",
														"message": "must be even",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := generators.ParseCommentTags(tc.t, tc.comments, "+k8s:validation:")
			if tc.expectedError != "" {
				require.Error(t, err)
				require.EqualError(t, err, tc.expectedError)
				return
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expected, actual)
		})
	}
}

// Test comment tag validation function
func TestCommentTags_Validate(t *testing.T) {

	testCases := []struct {
		name         string
		comments     []string
		t            *types.Type
		errorMessage string
	}{
		{
			name: "invalid minimum type",
			comments: []string{
				`+k8s:validation:minimum=10.5`,
			},
			t:            types.String,
			errorMessage: "minimum can only be used on numeric types",
		},
		{
			name: "invalid minLength type",
			comments: []string{
				`+k8s:validation:minLength=10`,
			},
			t:            types.Bool,
			errorMessage: "minLength can only be used on string types",
		},
		{
			name: "invalid minItems type",
			comments: []string{
				`+k8s:validation:minItems=10`,
			},
			t:            types.String,
			errorMessage: "minItems can only be used on array types",
		},
		{
			name: "invalid minProperties type",
			comments: []string{
				`+k8s:validation:minProperties=10`,
			},
			t:            types.String,
			errorMessage: "minProperties can only be used on map types",
		},
		{
			name: "invalid exclusiveMinimum type",
			comments: []string{
				`+k8s:validation:exclusiveMinimum=true`,
			},
			t:            arrayType,
			errorMessage: "exclusiveMinimum can only be used on numeric types",
		},
		{
			name: "invalid maximum type",
			comments: []string{
				`+k8s:validation:maximum=10.5`,
			},
			t:            arrayType,
			errorMessage: "maximum can only be used on numeric types",
		},
		{
			name: "invalid maxLength type",
			comments: []string{
				`+k8s:validation:maxLength=10`,
			},
			t:            mapType,
			errorMessage: "maxLength can only be used on string types",
		},
		{
			name: "invalid maxItems type",
			comments: []string{
				`+k8s:validation:maxItems=10`,
			},
			t:            types.Bool,
			errorMessage: "maxItems can only be used on array types",
		},
		{
			name: "invalid maxProperties type",
			comments: []string{
				`+k8s:validation:maxProperties=10`,
			},
			t:            types.Bool,
			errorMessage: "maxProperties can only be used on map types",
		},
		{
			name: "invalid exclusiveMaximum type",
			comments: []string{
				`+k8s:validation:exclusiveMaximum=true`,
			},
			t:            mapType,
			errorMessage: "exclusiveMaximum can only be used on numeric types",
		},
		{
			name: "invalid pattern type",
			comments: []string{
				`+k8s:validation:pattern=".*"`,
			},
			t:            types.Int,
			errorMessage: "pattern can only be used on string types",
		},
		{
			name: "invalid multipleOf type",
			comments: []string{
				`+k8s:validation:multipleOf=10.5`,
			},
			t:            types.String,
			errorMessage: "multipleOf can only be used on numeric types",
		},
		{
			name: "invalid uniqueItems type",
			comments: []string{
				`+k8s:validation:uniqueItems=true`,
			},
			t:            types.Int,
			errorMessage: "uniqueItems can only be used on array types",
		},
		{
			name: "negative minLength",
			comments: []string{
				`+k8s:validation:minLength=-10`,
			},
			t:            types.String,
			errorMessage: "minLength cannot be negative",
		},
		{
			name: "negative minItems",
			comments: []string{
				`+k8s:validation:minItems=-10`,
			},
			t:            arrayType,
			errorMessage: "minItems cannot be negative",
		},
		{
			name: "negative minProperties",
			comments: []string{
				`+k8s:validation:minProperties=-10`,
			},
			t:            mapType,
			errorMessage: "minProperties cannot be negative",
		},
		{
			name: "negative maxLength",
			comments: []string{
				`+k8s:validation:maxLength=-10`,
			},
			t:            types.String,
			errorMessage: "maxLength cannot be negative",
		},
		{
			name: "negative maxItems",
			comments: []string{
				`+k8s:validation:maxItems=-10`,
			},
			t:            arrayType,
			errorMessage: "maxItems cannot be negative",
		},
		{
			name: "negative maxProperties",
			comments: []string{
				`+k8s:validation:maxProperties=-10`,
			},
			t:            mapType,
			errorMessage: "maxProperties cannot be negative",
		},
		{
			name: "minimum > maximum",
			comments: []string{
				`+k8s:validation:minimum=10.5`,
				`+k8s:validation:maximum=5.5`,
			},
			t:            types.Float64,
			errorMessage: "minimum 10.500000 is greater than maximum 5.500000",
		},
		{
			name: "exclusiveMinimum when minimum == maximum",
			comments: []string{
				`+k8s:validation:minimum=10.5`,
				`+k8s:validation:maximum=10.5`,
				`+k8s:validation:exclusiveMinimum=true`,
			},
			t:            types.Float64,
			errorMessage: "exclusiveMinimum/Maximum cannot be set when minimum == maximum",
		},
		{
			name: "exclusiveMaximum when minimum == maximum",
			comments: []string{
				`+k8s:validation:minimum=10.5`,
				`+k8s:validation:maximum=10.5`,
				`+k8s:validation:exclusiveMaximum=true`,
			},
			t:            types.Float64,
			errorMessage: "exclusiveMinimum/Maximum cannot be set when minimum == maximum",
		},
		{
			name: "minLength > maxLength",
			comments: []string{
				`+k8s:validation:minLength=10`,
				`+k8s:validation:maxLength=5`,
			},
			t:            types.String,
			errorMessage: "minLength 10 is greater than maxLength 5",
		},
		{
			name: "minItems > maxItems",
			comments: []string{
				`+k8s:validation:minItems=10`,
				`+k8s:validation:maxItems=5`,
			},
			t:            arrayType,
			errorMessage: "minItems 10 is greater than maxItems 5",
		},
		{
			name: "minProperties > maxProperties",
			comments: []string{
				`+k8s:validation:minProperties=10`,
				`+k8s:validation:maxProperties=5`,
			},
			t:            mapType,
			errorMessage: "minProperties 10 is greater than maxProperties 5",
		},
		{
			name: "invalid pattern",
			comments: []string{
				`+k8s:validation:pattern="([a-z]+"`,
			},
			t:            types.String,
			errorMessage: "invalid pattern \"([a-z]+\": error parsing regexp: missing closing ): `([a-z]+`",
		},
		{
			name: "multipleOf = 0",
			comments: []string{
				`+k8s:validation:multipleOf=0.0`,
			},
			t:            types.Int,
			errorMessage: "multipleOf cannot be 0",
		},
		{
			name: "valid comment tags with no invalid validations",
			comments: []string{
				`+k8s:validation:pattern=".*"`,
			},
			t:            types.String,
			errorMessage: "",
		},
		{
			name: "additionalProperties on non-map",
			comments: []string{
				`+k8s:validation:additionalProperties:pattern=".*"`,
			},
			t:            types.String,
			errorMessage: "additionalProperties can only be used on map types",
		},
		{
			name: "properties on non-struct",
			comments: []string{
				`+k8s:validation:properties:name:pattern=".*"`,
			},
			t:            types.String,
			errorMessage: "properties can only be used on struct types",
		},
		{
			name: "items on non-array",
			comments: []string{
				`+k8s:validation:items:pattern=".*"`,
			},
			t:            types.String,
			errorMessage: "items can only be used on array types",
		},
		{
			name: "property missing from struct",
			comments: []string{
				`+k8s:validation:properties:name:pattern=".*"`,
			},
			t: &types.Type{
				Kind: types.Struct,
				Name: types.Name{Name: "struct"},
				Members: []types.Member{
					{
						Name: "notname",
						Type: types.String,
						Tags: `json:"notname"`,
					},
				},
			},
			errorMessage: `property used in comment tag "name" not found in struct struct`,
		},
		{
			name: "nested comments also type checked",
			comments: []string{
				`+k8s:validation:properties:name:items:pattern=".*"`,
			},
			t: &types.Type{
				Kind: types.Struct,
				Name: types.Name{Name: "struct"},
				Members: []types.Member{
					{
						Name: "name",
						Type: types.String,
						Tags: `json:"name"`,
					},
				},
			},
			errorMessage: `failed to validate property "name": items can only be used on array types`,
		},
		{
			name: "nested comments also type checked - passing",
			comments: []string{
				`+k8s:validation:properties:name:pattern=".*"`,
			},
			t: &types.Type{
				Kind: types.Struct,
				Name: types.Name{Name: "struct"},
				Members: []types.Member{
					{
						Name: "name",
						Type: types.String,
						Tags: `json:"name"`,
					},
				},
			},
		},
		{
			name: "nested marker type checking through alias",
			comments: []string{
				`+k8s:validation:properties:name:pattern=".*"`,
			},
			t: &types.Type{
				Kind: types.Struct,
				Name: types.Name{Name: "struct"},
				Members: []types.Member{
					{
						Name: "name",
						Tags: `json:"name"`,
						Type: &types.Type{
							Kind: types.Alias,
							Underlying: &types.Type{
								Kind: types.Slice,
								Elem: types.String,
							},
						},
					},
				},
			},
			errorMessage: `failed to validate property "name": pattern can only be used on string types`,
		},
		{
			name: "ignore unknown field with unparsable value",
			comments: []string{
				`+k8s:validation:xyz=a=b`, // a=b is not a valid value
			},
			t: &types.Type{
				Kind: types.Struct,
				Name: types.Name{Name: "struct"},
				Members: []types.Member{
					{
						Name: "name",
						Type: types.String,
						Tags: `json:"name"`,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := generators.ParseCommentTags(tc.t, tc.comments, "+k8s:validation:")
			if tc.errorMessage != "" {
				require.Error(t, err)
				require.Equal(t, "invalid marker comments: "+tc.errorMessage, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

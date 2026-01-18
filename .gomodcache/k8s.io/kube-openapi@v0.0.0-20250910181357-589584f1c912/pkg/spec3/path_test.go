/*
Copyright 2021 The Kubernetes Authors.

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

package spec3_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	jsonv2 "k8s.io/kube-openapi/pkg/internal/third_party/go-json-experiment/json"
	"k8s.io/kube-openapi/pkg/spec3"
	jsontesting "k8s.io/kube-openapi/pkg/util/jsontesting"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestPathRoundTrip(t *testing.T) {
	cases := []jsontesting.RoundTripTestCase{
		{
			Name: "Basic Roundtrip",
			Object: &spec3.Path{
				spec.Refable{Ref: spec.MustCreateRef("Dog")},
				spec3.PathProps{
					Description: "foo",
				},
				spec.VendorExtensible{Extensions: spec.Extensions{
					"x-framework": "go-swagger",
				}},
			},
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.Name, func(t *testing.T) {
			require.NoError(t, tcase.RoundTripTest(&spec3.Path{}))
		})
	}
}

func TestPathJSONSerialization(t *testing.T) {
	cases := []struct {
		name           string
		target         *spec3.Path
		expectedOutput string
	}{
		{
			name: "basic",
			target: &spec3.Path{
				PathProps: spec3.PathProps{
					Get: &spec3.Operation{
						OperationProps: spec3.OperationProps{
							Description: "Returns pets based on ID",
							Summary:     "Find pets by ID",
							OperationId: "getPetsById",
							Responses: &spec3.Responses{
								ResponsesProps: spec3.ResponsesProps{
									StatusCodeResponses: map[int]*spec3.Response{
										200: {
											ResponseProps: spec3.ResponseProps{
												Description: "Pet response",
												Content: map[string]*spec3.MediaType{
													"*/*": {
														MediaTypeProps: spec3.MediaTypeProps{
															Schema: &spec.Schema{
																SchemaProps: spec.SchemaProps{
																	Type: []string{"array"},
																	Items: &spec.SchemaOrArray{
																		Schema: &spec.Schema{
																			SchemaProps: spec.SchemaProps{
																				Ref: spec.MustCreateRef("#/components/schemas/Pet"),
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
							},
						},
					},
					Parameters: []*spec3.Parameter{
						{
							ParameterProps: spec3.ParameterProps{
								Name:        "id",
								In:          "path",
								Description: "ID of the pet to use",
								Required:    true,
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"array"},
										Items: &spec.SchemaOrArray{
											Schema: &spec.Schema{
												SchemaProps: spec.SchemaProps{
													Ref: spec.MustCreateRef("#/components/schemas/Pet"),
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
			expectedOutput: `{"get":{"summary":"Find pets by ID","description":"Returns pets based on ID","operationId":"getPetsById","responses":{"200":{"description":"Pet response","content":{"*/*":{"schema":{"type":"array","items":{"$ref":"#/components/schemas/Pet"}}}}}}},"parameters":[{"name":"id","in":"path","description":"ID of the pet to use","required":true,"schema":{"type":"array","items":{"$ref":"#/components/schemas/Pet"}}}]}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawTarget, err := json.Marshal(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			serializedTarget := string(rawTarget)
			if !cmp.Equal(serializedTarget, tc.expectedOutput) {
				t.Fatalf("%s", serializedTarget)
				t.Fatalf("diff %s", cmp.Diff(serializedTarget, tc.expectedOutput))
			}
		})
	}
}

func TestPathsNullUnmarshal(t *testing.T) {
	nullByte := []byte(`null`)

	expected := spec3.Paths{}
	test := spec3.Paths{
		Paths: map[string]*spec3.Path{"/path": {}},
	}
	jsonv2.Unmarshal(nullByte, &test)
	if !reflect.DeepEqual(test, expected) {
		t.Error("Expected unmarshal of null to reset the Paths struct")
	}
}

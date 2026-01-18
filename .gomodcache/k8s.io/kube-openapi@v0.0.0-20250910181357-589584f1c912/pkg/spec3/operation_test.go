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
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/util/jsontesting"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestOperationRoundTrip(t *testing.T) {
	cases := []jsontesting.RoundTripTestCase{
		{
			Name: "Basic Roundtrip",
			Object: &spec3.Operation{
				spec3.OperationProps{
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
			require.NoError(t, tcase.RoundTripTest(&spec3.Operation{}))
		})
	}
}

func TestOperationJSONSerialization(t *testing.T) {
	cases := []struct {
		name           string
		target         *spec3.Operation
		expectedOutput string
	}{
		{
			name: "basic",
			target: &spec3.Operation{
				OperationProps: spec3.OperationProps{
					Tags:        []string{"pet"},
					Summary:     "Updates a pet in the store with form data",
					OperationId: "updatePetWithForm",
					Parameters: []*spec3.Parameter{
						{
							ParameterProps: spec3.ParameterProps{
								Name:        "petId",
								In:          "path",
								Description: "ID of pet that needs to be updated",
								Required:    true,
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									},
								},
							},
						},
					},
					RequestBody: &spec3.RequestBody{
						RequestBodyProps: spec3.RequestBodyProps{
							Content: map[string]*spec3.MediaType{
								"application/x-www-form-urlencoded": {
									MediaTypeProps: spec3.MediaTypeProps{
										Schema: &spec.Schema{
											SchemaProps: spec.SchemaProps{
												Type: []string{"object"},
												Properties: map[string]spec.Schema{
													"name": {
														SchemaProps: spec.SchemaProps{
															Description: "Updated name of the pet",
															Type:        []string{"string"},
														},
													},
													"status": {
														SchemaProps: spec.SchemaProps{
															Description: "Updated status of the pet",
															Type:        []string{"string"},
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
					Responses: &spec3.Responses{
						ResponsesProps: spec3.ResponsesProps{
							StatusCodeResponses: map[int]*spec3.Response{
								200: {
									ResponseProps: spec3.ResponseProps{
										Description: "Pet updated.",
										Content: map[string]*spec3.MediaType{
											"application/json": {},
											"application/xml":  {},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedOutput: `{"tags":["pet"],"summary":"Updates a pet in the store with form data","operationId":"updatePetWithForm","parameters":[{"name":"petId","in":"path","description":"ID of pet that needs to be updated","required":true,"schema":{"type":"string"}}],"requestBody":{"content":{"application/x-www-form-urlencoded":{"schema":{"type":"object","properties":{"name":{"description":"Updated name of the pet","type":"string"},"status":{"description":"Updated status of the pet","type":"string"}}}}}},"responses":{"200":{"description":"Pet updated.","content":{"application/json":{},"application/xml":{}}}}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawTarget, err := json.Marshal(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			serializedTarget := string(rawTarget)
			if err := jsontesting.JsonCompare([]byte(tc.expectedOutput), []byte(serializedTarget)); err != nil {
				t.Fatalf("diff %s", err)
			}
		})
	}
}

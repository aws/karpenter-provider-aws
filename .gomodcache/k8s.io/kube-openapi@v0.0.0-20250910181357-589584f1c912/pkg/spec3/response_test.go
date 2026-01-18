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

func TestResponsesRoundTrip(t *testing.T) {
	cases := []jsontesting.RoundTripTestCase{
		{
			Name: "Basic Test With Extensions",
			Object: &spec3.Responses{
				VendorExtensible: spec.VendorExtensible{
					Extensions: spec.Extensions{
						"x-framework": "swagger-go",
					},
				},
				ResponsesProps: spec3.ResponsesProps{
					Default: &spec3.Response{
						Refable: spec.Refable{Ref: spec.MustCreateRef("/components/some/ref.foo")},
					},
				},
			},
		},
	}
	for _, tcase := range cases {
		t.Run(tcase.Name, func(t *testing.T) {
			require.NoError(t, tcase.RoundTripTest(&spec3.Responses{}))
		})
	}
}

func TestResponseRoundTrip(t *testing.T) {
	cases := []jsontesting.RoundTripTestCase{
		{
			Name: "Basic Roundtrip",
			Object: &spec3.Response{
				spec.Refable{Ref: spec.MustCreateRef("Dog")},
				spec3.ResponseProps{
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
			require.NoError(t, tcase.RoundTripTest(&spec3.Response{}))
		})
	}
}

func TestResponseJSONSerialization(t *testing.T) {
	cases := []struct {
		name           string
		target         *spec3.Response
		expectedOutput string
	}{
		// scenario 1
		{
			name: "basic",
			target: &spec3.Response{
				ResponseProps: spec3.ResponseProps{
					Content: map[string]*spec3.MediaType{
						"text/plain": {
							MediaTypeProps: spec3.MediaTypeProps{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									},
								},
							},
						},
					},
				},
			},
			expectedOutput: `{"content":{"text/plain":{"schema":{"type":"string"}}}}`,
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
				t.Fatalf("diff %s", cmp.Diff(serializedTarget, tc.expectedOutput))
			}
		})
	}
}

func TestResponsesNullUnmarshal(t *testing.T) {
	nullByte := []byte(`null`)

	expected := spec3.Responses{}
	test := spec3.Responses{
		ResponsesProps: spec3.ResponsesProps{
			Default: &spec3.Response{},
		},
	}
	jsonv2.Unmarshal(nullByte, &test)
	if !reflect.DeepEqual(test, expected) {
		t.Error("Expected unmarshal of null to reset the Responses struct")
	}
}

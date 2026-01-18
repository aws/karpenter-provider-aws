// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spec

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/kube-openapi/pkg/internal"
	jsontesting "k8s.io/kube-openapi/pkg/util/jsontesting"
	"sigs.k8s.io/randfill"
)

var spec = Swagger{
	SwaggerProps: SwaggerProps{
		ID:          "http://localhost:3849/api-docs",
		Swagger:     "2.0",
		Consumes:    []string{"application/json", "application/x-yaml"},
		Produces:    []string{"application/json"},
		Schemes:     []string{"http", "https"},
		Info:        &info,
		Host:        "some.api.out.there",
		BasePath:    "/",
		Paths:       &paths,
		Definitions: map[string]Schema{"Category": {SchemaProps: SchemaProps{Type: []string{"string"}}}},
		Parameters: map[string]Parameter{
			"categoryParam": {ParamProps: ParamProps{Name: "category", In: "query"}, SimpleSchema: SimpleSchema{Type: "string"}},
		},
		Responses: map[string]Response{
			"EmptyAnswer": {
				ResponseProps: ResponseProps{
					Description: "no data to return for this operation",
				},
			},
		},
		SecurityDefinitions: map[string]*SecurityScheme{
			"internalApiKey": &(SecurityScheme{SecuritySchemeProps: SecuritySchemeProps{Type: "apiKey", Name: "api_key", In: "header"}}),
		},
		Security: []map[string][]string{
			{"internalApiKey": {}},
		},
		Tags:         []Tag{{TagProps: TagProps{Description: "", Name: "pets", ExternalDocs: nil}}},
		ExternalDocs: &ExternalDocumentation{Description: "the name", URL: "the url"},
	},
	VendorExtensible: VendorExtensible{Extensions: map[string]interface{}{
		"x-some-extension": "vendor",
		"x-schemes":        []interface{}{"unix", "amqp"},
	}},
}

const specJSON = `{
	"id": "http://localhost:3849/api-docs",
	"consumes": ["application/json", "application/x-yaml"],
	"produces": ["application/json"],
	"schemes": ["http", "https"],
	"swagger": "2.0",
	"info": {
		"contact": {
			"name": "wordnik api team",
			"url": "http://developer.wordnik.com"
		},
		"description": "A sample API that uses a petstore as an example to demonstrate features in the swagger-2.0` +
	` specification",
		"license": {
			"name": "Creative Commons 4.0 International",
			"url": "http://creativecommons.org/licenses/by/4.0/"
		},
		"termsOfService": "http://helloreverb.com/terms/",
		"title": "Swagger Sample API",
		"version": "1.0.9-abcd",
		"x-framework": "go-swagger"
	},
	"host": "some.api.out.there",
	"basePath": "/",
	"paths": {"x-framework":"go-swagger","/":{"$ref":"cats"}},
	"definitions": { "Category": { "type": "string"} },
	"parameters": {
		"categoryParam": {
			"name": "category",
			"in": "query",
			"type": "string"
		}
	},
	"responses": { "EmptyAnswer": { "description": "no data to return for this operation" } },
	"securityDefinitions": {
		"internalApiKey": {
			"type": "apiKey",
			"in": "header",
			"name": "api_key"
		}
	},
	"security": [{"internalApiKey":[]}],
	"tags": [{"name":"pets"}],
	"externalDocs": {"description":"the name","url":"the url"},
	"x-some-extension": "vendor",
	"x-schemes": ["unix","amqp"]
}`

func TestSwaggerSpec_Serialize(t *testing.T) {
	expected := make(map[string]interface{})
	_ = json.Unmarshal([]byte(specJSON), &expected)
	b, err := spec.MarshalJSON()
	if assert.NoError(t, err) {
		var actual map[string]interface{}
		err := json.Unmarshal(b, &actual)
		if assert.NoError(t, err) {
			assert.EqualValues(t, actual, expected)
		}
	}
}

func TestSwaggerSpec_Deserialize(t *testing.T) {
	var actual Swagger
	err := json.Unmarshal([]byte(specJSON), &actual)
	if assert.NoError(t, err) {
		assert.EqualValues(t, actual, spec)
	}
}

func TestSwaggerRoundtrip(t *testing.T) {
	cases := []jsontesting.RoundTripTestCase{
		{
			// Show at least one field from each embededd struct sitll allows
			// roundtrips successfully
			Name: "UnmarshalEmbedded",
			Object: &Swagger{
				VendorExtensible{Extensions{
					"x-framework": "go-swagger",
				}},
				SwaggerProps{
					Swagger: "2.0.0",
				},
			},
		}, {
			Name:   "BasicCase",
			JSON:   specJSON,
			Object: &spec,
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.Name, func(t *testing.T) {
			require.NoError(t, tcase.RoundTripTest(&Swagger{}))
		})
	}
}

func TestSwaggerSpec_Marshalv2Fuzzed(t *testing.T) {
	fuzzer := randfill.
		NewWithSeed(1646791953).
		NilChance(0.075).
		MaxDepth(13).
		NumElements(1, 2)

	fuzzer.Funcs(
		SwaggerFuzzFuncs...,
	)

	for i := 0; i < 100; i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			swagger := Swagger{}
			fuzzer.Fill(&swagger)

			internal.UseOptimizedJSONMarshaling = false
			want, err := json.Marshal(swagger)
			if err != nil {
				t.Errorf("failed to marshal swagger: %v", err)
			}
			internal.UseOptimizedJSONMarshaling = true
			got, err := swagger.MarshalJSON()
			if err != nil {
				t.Errorf("failed to marshal next swagger: %v", err)
			}
			if err := jsontesting.JsonCompare(want, got); err != nil {
				t.Errorf("fuzzed marshal doesn't match: %v", err)
			}
		})
	}
}

func TestSwaggerSpec_Marshalv2FuzzedIsStable(t *testing.T) {
	swagFile, err := os.Open("../../schemaconv/testdata/swagger.json")
	if err != nil {
		t.Fatal(err)
	}
	defer swagFile.Close()

	js, err := io.ReadAll(swagFile)
	if err != nil {
		t.Fatal(err)
	}

	swagger := Swagger{}
	assert.NoError(t, json.Unmarshal(js, &swagger))

	for i := 0; i < 5; i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			want, err := swagger.MarshalJSON()
			if err != nil {
				t.Errorf("failed to marshal swagger: %v", err)
			}
			got, err := swagger.MarshalJSON()
			if err != nil {
				t.Errorf("failed to marshal swagger again: %v", err)
			}
			if err := cmp.Diff(want, got); err != "" {
				t.Fatalf("expected both marshal to be identical/stable: %v", err)
			}
		})
	}
}

func TestUnmarshalAdditionalProperties(t *testing.T) {
	cases := []string{
		`{}`,
		`{"description": "the description of this schema"}`,
		`false`,
		`true`,
	}

	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			var v1, v2 SchemaOrBool
			internal.UseOptimizedJSONUnmarshaling = true
			require.NoError(t, json.Unmarshal([]byte(tc), &v2))
			internal.UseOptimizedJSONUnmarshaling = false
			require.NoError(t, json.Unmarshal([]byte(tc), &v1))
			if !cmp.Equal(v1, v2, SwaggerDiffOptions...) {
				t.Fatal(cmp.Diff(v1, v2, SwaggerDiffOptions...))
			}
		})
	}
}

func TestSwaggerSpec_ExperimentalUnmarshal(t *testing.T) {
	fuzzer := randfill.
		NewWithSeed(1646791953).
		NilChance(0.01).
		MaxDepth(10).
		NumElements(1, 2)

	fuzzer.Funcs(
		SwaggerFuzzFuncs...,
	)

	expected := Swagger{}
	fuzzer.Fill(&expected)

	// Serialize into JSON
	jsonBytes, err := json.Marshal(expected)
	require.NoError(t, err)

	t.Log("Specimen", string(jsonBytes))

	actual := Swagger{}
	internal.UseOptimizedJSONUnmarshaling = true

	err = json.Unmarshal(jsonBytes, &actual)
	require.NoError(t, err)

	if !cmp.Equal(expected, actual, SwaggerDiffOptions...) {
		t.Fatal(cmp.Diff(expected, actual, SwaggerDiffOptions...))
	}

	control := Swagger{}
	internal.UseOptimizedJSONUnmarshaling = false
	err = json.Unmarshal(jsonBytes, &control)
	require.NoError(t, err)

	if !reflect.DeepEqual(control, actual) {
		t.Fatal(cmp.Diff(control, actual, SwaggerDiffOptions...))
	}
}

func BenchmarkSwaggerSpec_ExperimentalUnmarshal(b *testing.B) {
	// Download kube-openapi swagger json
	swagFile, err := os.Open("../../schemaconv/testdata/swagger.json")
	if err != nil {
		b.Fatal(err)
	}
	defer swagFile.Close()

	originalJSON, err := io.ReadAll(swagFile)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	// Parse into kube-openapi types
	b.Run("jsonv1", func(b2 *testing.B) {
		internal.UseOptimizedJSONUnmarshaling = false
		for i := 0; i < b2.N; i++ {
			var result *Swagger
			if err := json.Unmarshal(originalJSON, &result); err != nil {
				b2.Fatal(err)
			}
		}
	})

	b.Run("jsonv2 via jsonv1", func(b2 *testing.B) {
		internal.UseOptimizedJSONUnmarshaling = true
		for i := 0; i < b2.N; i++ {
			var result *Swagger
			if err := json.Unmarshal(originalJSON, &result); err != nil {
				b2.Fatal(err)
			}
		}
	})

	// Our UnmarshalJSON implementation which defers to jsonv2 causes the
	// text to be parsed/validated twice. This costs a significant amount of time.
	b.Run("jsonv2", func(b2 *testing.B) {
		internal.UseOptimizedJSONUnmarshaling = true
		for i := 0; i < b2.N; i++ {
			var result Swagger
			if err := result.UnmarshalJSON(originalJSON); err != nil {
				b2.Fatal(err)
			}
		}
	})
}

func BenchmarkSwaggerSpec_ExperimentalMarshal(b *testing.B) {
	// Load kube-openapi swagger json
	swagFile, err := os.Open("../../schemaconv/testdata/swagger.json")
	if err != nil {
		b.Fatal(err)
	}
	defer swagFile.Close()

	originalJSON, err := io.ReadAll(swagFile)
	if err != nil {
		b.Fatal(err)
	}

	var swagger *Swagger
	if err := json.Unmarshal(originalJSON, &swagger); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	// Serialize kube-openapi types
	b.Run("jsonv1", func(b2 *testing.B) {
		b2.ReportAllocs()
		internal.UseOptimizedJSONMarshaling = false
		for i := 0; i < b2.N; i++ {
			if _, err = json.Marshal(swagger); err != nil {
				b2.Fatal(err)
			}
		}
	})

	b.Run("jsonv2 via jsonv1", func(b2 *testing.B) {
		b2.ReportAllocs()
		internal.UseOptimizedJSONMarshaling = true
		for i := 0; i < b2.N; i++ {
			if _, err := json.Marshal(swagger); err != nil {
				b2.Fatal(err)
			}
		}
	})

	b.Run("jsonv2", func(b2 *testing.B) {
		b2.ReportAllocs()
		internal.UseOptimizedJSONUnmarshaling = true
		for i := 0; i < b2.N; i++ {
			if _, err = swagger.MarshalJSON(); err != nil {
				b2.Fatal(err)
			}
		}
	})
}

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

package spec_test

import (
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/gnostic-models/compiler"
	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
	"google.golang.org/protobuf/proto"
	jsontesting "k8s.io/kube-openapi/pkg/util/jsontesting"
	. "k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/randfill"
)

func gnosticCommonTest(t testing.TB, fuzzer *randfill.Filler) {
	fuzzer.Funcs(
		SwaggerFuzzFuncs...,
	)

	expected := Swagger{}
	fuzzer.Fill(&expected)

	// Convert to gnostic via JSON to compare
	jsonBytes, err := expected.MarshalJSON()
	require.NoError(t, err)

	t.Log("Specimen", string(jsonBytes))

	gnosticSpec, err := openapi_v2.ParseDocument(jsonBytes)
	require.NoError(t, err)

	actual := Swagger{}
	ok, err := actual.FromGnostic(gnosticSpec)
	require.NoError(t, err)
	require.True(t, ok)
	if !cmp.Equal(expected, actual, SwaggerDiffOptions...) {
		t.Fatal(cmp.Diff(expected, actual, SwaggerDiffOptions...))
	}

	newJsonBytes, err := actual.MarshalJSON()
	require.NoError(t, err)
	if err := jsontesting.JsonCompare(jsonBytes, newJsonBytes); err != nil {
		t.Fatal(err)
	}
}

func TestGnosticConversionSmallDeterministic(t *testing.T) {
	gnosticCommonTest(
		t,
		randfill.
			NewWithSeed(15).
			NilChance(0.8).
			MaxDepth(10).
			NumElements(1, 2),
	)
}

func TestGnosticConversionSmallDeterministic2(t *testing.T) {
	// A failed case of TestGnosticConversionSmallRandom
	// which failed during development/testing loop
	gnosticCommonTest(
		t,
		randfill.
			NewWithSeed(1646770841).
			NilChance(0.8).
			MaxDepth(10).
			NumElements(1, 2),
	)
}

func TestGnosticConversionSmallDeterministic3(t *testing.T) {
	// A failed case of TestGnosticConversionSmallRandom
	// which failed during development/testing loop
	gnosticCommonTest(
		t,
		randfill.
			NewWithSeed(1646772024).
			NilChance(0.8).
			MaxDepth(10).
			NumElements(1, 2),
	)
}

func TestGnosticConversionSmallDeterministic4(t *testing.T) {
	// A failed case of TestGnosticConversionSmallRandom
	// which failed during development/testing loop
	gnosticCommonTest(
		t,
		randfill.
			NewWithSeed(1646791953).
			NilChance(0.8).
			MaxDepth(10).
			NumElements(1, 2),
	)
}

func TestGnosticConversionSmallDeterministic5(t *testing.T) {
	// A failed case of TestGnosticConversionSmallRandom
	// which failed during development/testing loop
	gnosticCommonTest(
		t,
		randfill.
			NewWithSeed(1646940131).
			NilChance(0.8).
			MaxDepth(10).
			NumElements(1, 2),
	)
}

func TestGnosticConversionSmallDeterministic6(t *testing.T) {
	// A failed case of TestGnosticConversionSmallRandom
	// which failed during development/testing loop
	gnosticCommonTest(
		t,
		randfill.
			NewWithSeed(1646941926).
			NilChance(0.8).
			MaxDepth(10).
			NumElements(1, 2),
	)
}

func TestGnosticConversionSmallDeterministic7(t *testing.T) {
	// A failed case of TestGnosticConversionSmallRandom
	// which failed during development/testing loop
	// This case did not convert nil/empty array within OperationProps.Security
	// correctly
	gnosticCommonTest(
		t,
		randfill.
			NewWithSeed(1647297721085690000).
			NilChance(0.8).
			MaxDepth(10).
			NumElements(1, 2),
	)
}

func TestGnosticConversionSmallRandom(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Log("Using seed: ", seed)
	fuzzer := randfill.
		NewWithSeed(seed).
		NilChance(0.8).
		MaxDepth(10).
		NumElements(1, 2)

	for i := 0; i <= 50; i++ {
		gnosticCommonTest(
			t,
			fuzzer,
		)
	}
}

func TestGnosticConversionMediumDeterministic(t *testing.T) {
	gnosticCommonTest(
		t,
		randfill.
			NewWithSeed(15).
			NilChance(0.4).
			MaxDepth(12).
			NumElements(3, 5),
	)
}

func TestGnosticConversionLargeDeterministic(t *testing.T) {
	gnosticCommonTest(
		t,
		randfill.
			NewWithSeed(15).
			NilChance(0.1).
			MaxDepth(15).
			NumElements(3, 5),
	)
}

func TestGnosticConversionLargeRandom(t *testing.T) {
	var seed int64 = time.Now().UnixNano()
	t.Log("Using seed: ", seed)
	fuzzer := randfill.
		NewWithSeed(seed).
		NilChance(0).
		MaxDepth(15).
		NumElements(3, 5)

	for i := 0; i < 5; i++ {
		gnosticCommonTest(
			t,
			fuzzer,
		)
	}
}

func BenchmarkGnosticConversion(b *testing.B) {
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

	// Parse into kube-openapi types
	var result *Swagger
	b.Run("json->swagger", func(b2 *testing.B) {
		for i := 0; i < b2.N; i++ {
			if err := json.Unmarshal(originalJSON, &result); err != nil {
				b2.Fatal(err)
			}
		}
	})

	// Convert to JSON
	var encodedJSON []byte
	b.Run("swagger->json", func(b2 *testing.B) {
		for i := 0; i < b2.N; i++ {
			encodedJSON, err = json.Marshal(result)
			if err != nil {
				b2.Fatal(err)
			}
		}
	})

	// Convert to gnostic
	var originalGnostic *openapi_v2.Document
	b.Run("json->gnostic", func(b2 *testing.B) {
		for i := 0; i < b2.N; i++ {
			originalGnostic, err = openapi_v2.ParseDocument(encodedJSON)
			if err != nil {
				b2.Fatal(err)
			}
		}
	})

	// Convert to PB
	var encodedProto []byte
	b.Run("gnostic->pb", func(b2 *testing.B) {
		for i := 0; i < b2.N; i++ {
			encodedProto, err = proto.Marshal(originalGnostic)
			if err != nil {
				b2.Fatal(err)
			}
		}
	})

	// Convert to gnostic
	var backToGnostic openapi_v2.Document
	b.Run("pb->gnostic", func(b2 *testing.B) {
		for i := 0; i < b2.N; i++ {
			if err := proto.Unmarshal(encodedProto, &backToGnostic); err != nil {
				b2.Fatal(err)
			}
		}
	})

	for i := 0; i < b.N; i++ {
		b.Run("gnostic->kube", func(b2 *testing.B) {
			for i := 0; i < b2.N; i++ {
				decodedSwagger := &Swagger{}
				if ok, err := decodedSwagger.FromGnostic(&backToGnostic); err != nil {
					b2.Fatal(err)
				} else if !ok {
					b2.Fatal("conversion lost data")
				}
			}
		})
	}
}

// Ensure all variants of SecurityDefinition are being exercised by tests
func TestSecurityDefinitionVariants(t *testing.T) {
	type TestPattern struct {
		Name    string
		Pattern string
	}

	patterns := []TestPattern{
		{
			Name:    "Basic Authentication",
			Pattern: `{"type": "basic", "description": "cool basic auth"}`,
		},
		{
			Name:    "API Key Query",
			Pattern: `{"type": "apiKey", "description": "cool api key auth", "in": "query", "name": "coolAuth"}`,
		},
		{
			Name:    "API Key Header",
			Pattern: `{"type": "apiKey", "description": "cool api key auth", "in": "header", "name": "coolAuth"}`,
		},
		{
			Name:    "OAuth2 Implicit",
			Pattern: `{"type": "oauth2", "flow": "implicit", "authorizationUrl": "https://google.com", "scopes": {"scope1": "a scope", "scope2": "a scope"}, "description": "cool oauth2 auth"}`,
		},
		{
			Name:    "OAuth2 Password",
			Pattern: `{"type": "oauth2", "flow": "password", "tokenUrl": "https://google.com", "scopes": {"scope1": "a scope", "scope2": "a scope"}, "description": "cool oauth2 auth"}`,
		},
		{
			Name:    "OAuth2 Application",
			Pattern: `{"type": "oauth2", "flow": "application", "tokenUrl": "https://google.com", "scopes": {"scope1": "a scope", "scope2": "a scope"}, "description": "cool oauth2 auth"}`,
		},
		{
			Name:    "OAuth2 Access Code",
			Pattern: `{"type": "oauth2", "flow": "accessCode", "authorizationUrl": "https://google.com", "tokenUrl": "https://google.com", "scopes": {"scope1": "a scope", "scope2": "a scope"}, "description": "cool oauth2 auth"}`,
		},
	}

	for _, p := range patterns {
		t.Run(p.Name, func(t *testing.T) {
			// Parse JSON into yaml
			var nodes yaml.Node
			if err := yaml.Unmarshal([]byte(p.Pattern), &nodes); err != nil {
				t.Error(err)
				return
			} else if len(nodes.Content) != 1 {
				t.Errorf("unexpected yaml parse result")
				return
			}

			root := nodes.Content[0]

			parsed, err := openapi_v2.NewSecurityDefinitionsItem(root, compiler.NewContextWithExtensions("$root", root, nil, nil))
			if err != nil {
				t.Error(err)
				return
			}

			converted := SecurityScheme{}
			if err := converted.FromGnostic(parsed); err != nil {
				t.Error(err)
				return
			}

			// Ensure that the same JSON parsed via kube-openapi gives the same
			// result
			var expected SecurityScheme
			if err := json.Unmarshal([]byte(p.Pattern), &expected); err != nil {
				t.Error(err)
				return
			} else if !reflect.DeepEqual(expected, converted) {
				t.Errorf("expected equal values: %v", cmp.Diff(expected, converted, SwaggerDiffOptions...))
				return
			}
		})
	}
}

// Ensure all variants of Parameter are being exercised by tests
func TestParamVariants(t *testing.T) {
	type TestPattern struct {
		Name    string
		Pattern string
	}

	patterns := []TestPattern{
		{
			Name:    "Body Parameter",
			Pattern: `{"in": "body", "name": "myBodyParam", "schema": {}}`,
		},
		{
			Name:    "NonBody Header Parameter",
			Pattern: `{"in": "header", "name": "myHeaderParam", "description": "a cool parameter", "type": "string", "collectionFormat": "pipes"}`,
		},
		{
			Name:    "NonBody FormData Parameter",
			Pattern: `{"in": "formData", "name": "myFormDataParam", "description": "a cool parameter", "type": "string", "collectionFormat": "pipes"}`,
		},
		{
			Name:    "NonBody Query Parameter",
			Pattern: `{"in": "query", "name": "myQueryParam", "description": "a cool parameter", "type": "string", "collectionFormat": "pipes"}`,
		},
		{
			Name:    "NonBody Path Parameter",
			Pattern: `{"required": true, "in": "path", "name": "myPathParam", "description": "a cool parameter", "type": "string", "collectionFormat": "pipes"}`,
		},
	}

	for _, p := range patterns {
		t.Run(p.Name, func(t *testing.T) {
			// Parse JSON into yaml
			var nodes yaml.Node
			if err := yaml.Unmarshal([]byte(p.Pattern), &nodes); err != nil {
				t.Error(err)
				return
			} else if len(nodes.Content) != 1 {
				t.Errorf("unexpected yaml parse result")
				return
			}

			root := nodes.Content[0]

			ctx := compiler.NewContextWithExtensions("$root", root, nil, nil)
			parsed, err := openapi_v2.NewParameter(root, ctx)
			if err != nil {
				t.Error(err)
				return
			}

			converted := Parameter{}
			if ok, err := converted.FromGnostic(parsed); err != nil {
				t.Error(err)
				return
			} else if !ok {
				t.Errorf("expected no data loss while converting parameter: %v", p.Pattern)
				return
			}

			// Ensure that the same JSON parsed via kube-openapi gives the same
			// result
			var expected Parameter
			if err := json.Unmarshal([]byte(p.Pattern), &expected); err != nil {
				t.Error(err)
				return
			} else if !reflect.DeepEqual(expected, converted) {
				t.Errorf("expected equal values: %v", cmp.Diff(expected, converted, SwaggerDiffOptions...))
				return
			}
		})
	}
}

// Test that a few patterns of obvious data loss are detected
func TestCommonDataLoss(t *testing.T) {
	type TestPattern struct {
		Name          string
		BadInstance   string
		FixedInstance string
	}

	patterns := []TestPattern{
		{
			Name:          "License with Vendor Extension",
			BadInstance:   `{"swagger": "2.0", "info": {"title": "test", "version": "1.0", "license": {"name": "MIT", "x-hello": "ignored"}}, "paths": {}}`,
			FixedInstance: `{"swagger": "2.0", "info": {"title": "test", "version": "1.0", "license": {"name": "MIT"}}, "paths": {}}`,
		},
		{
			Name:          "Contact with Vendor Extension",
			BadInstance:   `{"swagger": "2.0", "info": {"title": "test", "version": "1.0", "contact": {"name": "bill", "x-hello": "ignored"}}, "paths": {}}`,
			FixedInstance: `{"swagger": "2.0", "info": {"title": "test", "version": "1.0", "contact": {"name": "bill"}}, "paths": {}}`,
		},
		{
			Name:          "External Documentation with Vendor Extension",
			BadInstance:   `{"swagger": "2.0", "info": {"title": "test", "version": "1.0", "contact": {"name": "bill", "x-hello": "ignored"}}, "paths": {}}`,
			FixedInstance: `{"swagger": "2.0", "info": {"title": "test", "version": "1.0", "contact": {"name": "bill"}}, "paths": {}}`,
		},
	}

	for _, v := range patterns {
		t.Run(v.Name, func(t *testing.T) {
			bad, err := openapi_v2.ParseDocument([]byte(v.BadInstance))
			if err != nil {
				t.Error(err)
				return
			}

			fixed, err := openapi_v2.ParseDocument([]byte(v.FixedInstance))
			if err != nil {
				t.Error(err)
				return
			}

			badConverted := Swagger{}
			if ok, err := badConverted.FromGnostic(bad); err != nil {
				t.Error(err)
				return
			} else if ok {
				t.Errorf("expected test to have data loss")
				return
			}

			fixedConverted := Swagger{}
			if ok, err := fixedConverted.FromGnostic(fixed); err != nil {
				t.Error(err)
				return
			} else if !ok {
				t.Errorf("expected fixed test to not have data loss")
				return
			}

			// Convert JSON directly into our kube-openapi type and check that
			// it is exactly equal to the converted instance
			fixedDirect := Swagger{}
			if err := json.Unmarshal([]byte(v.FixedInstance), &fixedDirect); err != nil {
				t.Error(err)
				return
			}

			if !reflect.DeepEqual(fixedConverted, badConverted) {
				t.Errorf("expected equal documents: %v", cmp.Diff(fixedConverted, badConverted, SwaggerDiffOptions...))
				return
			}

			// Make sure that they were exactly the same, except for the data loss
			//	by checking JSON encodes the some
			badConvertedJSON, err := badConverted.MarshalJSON()
			if err != nil {
				t.Error(err)
				return
			}

			fixedConvertedJSON, err := fixedConverted.MarshalJSON()
			if err != nil {
				t.Error(err)
				return
			}

			fixedDirectJSON, err := fixedDirect.MarshalJSON()
			if err != nil {
				t.Error(err)
				return
			}

			if !reflect.DeepEqual(badConvertedJSON, fixedConvertedJSON) {
				t.Errorf("encoded json values for bad and fixed tests are not identical: %v", cmp.Diff(string(badConvertedJSON), string(fixedConvertedJSON)))
			}

			if !reflect.DeepEqual(fixedDirectJSON, fixedConvertedJSON) {
				t.Errorf("encoded json values for fixed direct and fixed-from-gnostic tests are not identical: %v", cmp.Diff(string(fixedDirectJSON), string(fixedConvertedJSON)))
			}
		})
	}
}

func TestBadStatusCode(t *testing.T) {
	const testCase = `{"swagger": "2.0", "info": {"title": "test", "version": "1.0"}, "paths": {"/": {"get": {"responses" : { "default": { "$ref": "#/definitions/a" }, "200": { "$ref": "#/definitions/b" }}}}}}`
	const dropped = `{"swagger": "2.0", "info": {"title": "test", "version": "1.0"}, "paths": {"/": {"get": {"responses" : { "200": { "$ref": "#/definitions/b" }}}}}}`
	gnosticInstance, err := openapi_v2.ParseDocument([]byte(testCase))
	if err != nil {
		t.Fatal(err)
	}

	droppedGnosticInstance, err := openapi_v2.ParseDocument([]byte(dropped))
	if err != nil {
		t.Fatal(err)
	}

	// Manually poke an response code name which gnostic's json parser would not allow
	gnosticInstance.Paths.Path[0].Value.Get.Responses.ResponseCode[0].Name = "bad"

	badConverted := Swagger{}
	droppedConverted := Swagger{}

	if ok, err := badConverted.FromGnostic(gnosticInstance); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatalf("expected data loss converting an operation with a response code 'bad'")
	}

	if ok, err := droppedConverted.FromGnostic(droppedGnosticInstance); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatalf("expected no data loss converting a known good operation")
	}

	// Make sure that they were exactly the same, except for the data loss
	//	by checking JSON encodes the some
	badConvertedJSON, err := badConverted.MarshalJSON()
	if err != nil {
		t.Error(err)
		return
	}

	droppedConvertedJSON, err := droppedConverted.MarshalJSON()
	if err != nil {
		t.Error(err)
		return
	}

	if !reflect.DeepEqual(badConvertedJSON, droppedConvertedJSON) {
		t.Errorf("encoded json values for bad and fixed tests are not identical: %v", cmp.Diff(string(badConvertedJSON), string(droppedConvertedJSON)))
	}
}

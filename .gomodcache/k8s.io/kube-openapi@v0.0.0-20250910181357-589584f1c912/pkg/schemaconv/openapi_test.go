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

package schemaconv_test

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/require"

	"k8s.io/kube-openapi/pkg/schemaconv"
	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/structured-merge-diff/v6/schema"
)

var swaggerJSONPath = "testdata/swagger.json"
var testCRDPath = "testdata/crds"

var deducedName string = "__untyped_deduced_"
var untypedName string = "__untyped_atomic_"

const (
	quantityResource     = "io.k8s.apimachinery.pkg.api.resource.Quantity"
	rawExtensionResource = "io.k8s.apimachinery.pkg.runtime.RawExtension"
)

func toPtrMap[T comparable, V any](m map[T]V) map[T]*V {
	if m == nil {
		return nil
	}

	res := map[T]*V{}
	for k, v := range m {
		vCopy := v
		res[k] = &vCopy
	}
	return res
}

func normalizeTypeRef(tr *schema.TypeRef) {
	var untypedScalar schema.Scalar = "untyped"

	// Deduplicate deducedDef
	if tr.Inlined.Equals(&schema.Atom{
		Scalar: &untypedScalar,
		List: &schema.List{
			ElementType: schema.TypeRef{
				NamedType: &untypedName,
			},
			ElementRelationship: schema.Atomic,
		},
		Map: &schema.Map{
			ElementType: schema.TypeRef{
				NamedType: &deducedName,
			},
			ElementRelationship: schema.Separable,
		},
	}) {
		*tr = schema.TypeRef{
			NamedType: &deducedName,
		}
	} else if tr.NamedType != nil && *tr.NamedType == rawExtensionResource {
		// In old conversion all references to rawExtension were
		// replaced with untyped. In new implementation we preserve
		// the references and instead change the raw extension type
		// to untyped.
		// For normalization, just convert rawextension references
		// to "untyped"
		*tr = schema.TypeRef{
			NamedType: &untypedName,
		}
	} else {
		normalizeType(&tr.Inlined)
	}
}

// There are minor differences in new API that are semantically equivalent:
//  1. old openapi would replace refs to RawExtensoin with "untyped" and leave
//     RawExtension with a non-referenced/nonworking definition. New implemenatation
//     makes RawExtensoin "untyped", and leaves references to RawExtension.
//  2. old openapi would include "separable" relationship with
//     arbitrary/deduced maps where the new implementation leaves it unset
//     if it is unset by the user.
func normalizeType(typ *schema.Atom) {
	if typ.List != nil {
		if typ.List.ElementType.Inlined != (schema.Atom{}) {
			typ.List = &*typ.List
			normalizeTypeRef(&typ.List.ElementType)
		}
	}

	if typ.Map != nil {
		typ.Map = &*typ.Map

		fields := make([]schema.StructField, 0)
		copy(typ.Fields, fields)
		typ.Fields = fields

		for i, f := range typ.Fields {
			// Known Difference: Old conversion parses "{}" as empty map[any]any.
			// 					 New conversion parses it as empty map[string]any
			if reflect.DeepEqual(f.Default, map[any]any{}) {
				f.Default = map[string]any{}
			}

			normalizeTypeRef(&f.Type)
			typ.Fields[i] = f
		}

		sort.SliceStable(typ.Fields, func(i, j int) bool {
			return strings.Compare(typ.Fields[i].Name, typ.Fields[j].Name) < 0
		})

		// Current unions implementation is busted and not supported in new
		// format. Do not include in comparison
		typ.Unions = nil

		if typ.Map.ElementType.NamedType != nil {
			if len(typ.Map.ElementRelationship) == 0 && typ.Scalar != nil && typ.List != nil && *typ.Map.ElementType.NamedType == deducedName {
				// In old implementation arbitrary/deduced map would always also
				// include "separable".
				// New implementation has some code paths that dont follow that
				// (separable is default) so always attaach separable to deduced.
				typ.Map.ElementRelationship = schema.Separable
			}
		}

		normalizeTypeRef(&typ.Map.ElementType)
	}
}

// Can't directly proto models conversion to direct conversion due to subtle,
// expected differences between the two conversion methods.
// i.e. toProtoModels preserves sort order of fields. Direct conversion does not
// (due to spec.Schema using map for fields vs gnostic's slice)
func normalizeTypes(types []schema.TypeDef) map[string]schema.TypeDef {
	res := map[string]schema.TypeDef{}
	for _, typ := range types {
		if _, exists := res[typ.Name]; !exists {
			normalizeType(&typ.Atom)
			res[typ.Name] = typ
		}
	}

	// Old conversion would leave broken raw-extension definition, and just replace
	// references to it with an inlined __untyped_atomic_
	// The new conversion leaves references to rawextension in place, and instead
	// fixes the definition of RawExtension.
	//
	// This bit of code reverts the new conversion's fix, and puts in place the old
	// broken raw extension definition.
	res["io.k8s.apimachinery.pkg.runtime.RawExtension"] = schema.TypeDef{
		Name: "io.k8s.apimachinery.pkg.runtime.RawExtension",
		Atom: schema.Atom{
			Map: &schema.Map{
				ElementType: schema.TypeRef{
					NamedType: &deducedName,
				},
			},
		},
	}

	// v3 CRDs do not contain these orphaned, unnecessary definitions but v2 does
	//!TODO: either bring v3 to parity, or just remove these from v2 samples
	ignoreList := []string{
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceColumnDefinition",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceConversion",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceDefinition",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceDefinitionCondition",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceDefinitionNames",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceDefinitionSpec",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceDefinitionStatus",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceDefinitionVersion",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceSubresourceScale",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceSubresourceStatus",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceSubresources",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.CustomResourceValidation",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.ExternalDocumentation",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSON",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaProps",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaPropsOrArray",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaPropsOrBool",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaPropsOrStringArray",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.ServiceReference",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.ValidationRule",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.WebhookClientConfig",
		"io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.WebhookConversion",
		"io.k8s.apimachinery.pkg.apis.meta.v1.DeleteOptions",
		"io.k8s.apimachinery.pkg.apis.meta.v1.Patch",
		"io.k8s.apimachinery.pkg.apis.meta.v1.Preconditions",
		"io.k8s.apimachinery.pkg.apis.meta.v1.Status",
		"io.k8s.apimachinery.pkg.apis.meta.v1.StatusCause",
		"io.k8s.apimachinery.pkg.apis.meta.v1.StatusDetails",
	}

	for _, k := range ignoreList {
		delete(res, k)
	}

	return res
}

func TestCRDOpenAPIConversion(t *testing.T) {
	files, err := os.ReadDir("testdata/crds/openapiv2")
	require.NoError(t, err)
	for _, entry := range files {
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			openAPIV2Contents, err := os.ReadFile("testdata/crds/openapiv2/" + entry.Name())
			require.NoError(t, err)

			openAPIV3Contents, err := os.ReadFile("testdata/crds/openapiv3/" + entry.Name())
			require.NoError(t, err)

			var v3 spec3.OpenAPI

			err = json.Unmarshal(openAPIV3Contents, &v3)
			require.NoError(t, err)

			v2Types, err := specToSchemaViaProtoModels(openAPIV2Contents)
			require.NoError(t, err)
			v3Types, err := schemaconv.ToSchemaFromOpenAPI(v3.Components.Schemas, false)
			require.NoError(t, err)

			require.Equal(t, normalizeTypes(v2Types.Types), normalizeTypes(v3Types.Types))
		})
	}
}

// Using all models defined in swagger.json
// Convert to SMD using two methods:
//  1. Spec -> SMD
//  2. Spec -> JSON -> gnostic -> SMD
//
// Compare YAML forms. We have some allowed differences...
func TestOpenAPIImplementation(t *testing.T) {
	swaggerJSON, err := os.ReadFile(swaggerJSONPath)
	require.NoError(t, err)

	protoModels, err := specToSchemaViaProtoModels(swaggerJSON)
	require.NoError(t, err)

	var swag spec.Swagger
	err = json.Unmarshal(swaggerJSON, &swag)
	require.NoError(t, err)

	newConversionTypes, err := schemaconv.ToSchemaFromOpenAPI(toPtrMap(swag.Definitions), false)
	require.NoError(t, err)

	require.Equal(t, normalizeTypes(protoModels.Types), normalizeTypes(newConversionTypes.Types))
}

func specToSchemaViaProtoModels(input []byte) (*schema.Schema, error) {
	document, err := openapi_v2.ParseDocument(input)
	if err != nil {
		return nil, err
	}

	models, err := proto.NewOpenAPIData(document)
	if err != nil {
		return nil, err
	}

	newSchema, err := schemaconv.ToSchema(models)
	if err != nil {
		return nil, err
	}

	return newSchema, nil
}

func BenchmarkOpenAPIConversion(b *testing.B) {
	swaggerJSON, err := os.ReadFile(swaggerJSONPath)
	require.NoError(b, err)

	doc := spec.Swagger{}
	require.NoError(b, doc.UnmarshalJSON(swaggerJSON))

	// Beginning the benchmark from spec.Schema, since that is the format
	// stored by the kube-apiserver
	b.Run("spec.Schema->schema.Schema", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := schemaconv.ToSchemaFromOpenAPI(toPtrMap(doc.Definitions), false)
			require.NoError(b, err)
		}
	})

	b.Run("spec.Schema->json->gnostic_v2->proto.Models->schema.Schema", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			jsonText, err := doc.MarshalJSON()
			require.NoError(b, err)

			_, err = specToSchemaViaProtoModels(jsonText)
			require.NoError(b, err)
		}
	})
}

func BenchmarkOpenAPICRDConversion(b *testing.B) {
	files, err := os.ReadDir("testdata/crds/openapiv2")
	require.NoError(b, err)
	for _, entry := range files {
		b.Run(entry.Name(), func(b *testing.B) {
			openAPIV2Contents, err := os.ReadFile("testdata/crds/openapiv2/" + entry.Name())
			require.NoError(b, err)

			openAPIV3Contents, err := os.ReadFile("testdata/crds/openapiv3/" + entry.Name())
			require.NoError(b, err)

			var v2 spec.Swagger
			var v3 spec3.OpenAPI

			err = json.Unmarshal(openAPIV2Contents, &v2)
			require.NoError(b, err)

			err = json.Unmarshal(openAPIV3Contents, &v3)
			require.NoError(b, err)

			// Beginning the benchmark from spec.Schema, since that is the format
			// stored by the kube-apiserver
			b.Run("spec.Schema->schema.Schema", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, err := schemaconv.ToSchemaFromOpenAPI(v3.Components.Schemas, false)
					require.NoError(b, err)
				}
			})

			b.Run("spec.Schema->json->gnostic_v2->proto.Models->schema.Schema", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					jsonText, err := v2.MarshalJSON()
					require.NoError(b, err)

					_, err = specToSchemaViaProtoModels(jsonText)
					require.NoError(b, err)
				}
			})
		})
	}
}

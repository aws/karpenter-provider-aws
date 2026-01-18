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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	jsontesting "k8s.io/kube-openapi/pkg/util/jsontesting"
)

var parameter = Parameter{
	VendorExtensible: VendorExtensible{Extensions: map[string]interface{}{
		"x-framework": "swagger-go",
	}},
	Refable: Refable{Ref: MustCreateRef("Dog")},
	CommonValidations: CommonValidations{
		Maximum:          float64Ptr(100),
		ExclusiveMaximum: true,
		ExclusiveMinimum: true,
		Minimum:          float64Ptr(5),
		MaxLength:        int64Ptr(100),
		MinLength:        int64Ptr(5),
		Pattern:          "\\w{1,5}\\w+",
		MaxItems:         int64Ptr(100),
		MinItems:         int64Ptr(5),
		UniqueItems:      true,
		MultipleOf:       float64Ptr(5),
		Enum:             []interface{}{"hello", "world"},
	},
	SimpleSchema: SimpleSchema{
		Type:             "string",
		Format:           "date",
		CollectionFormat: "csv",
		Items: &Items{
			Refable: Refable{Ref: MustCreateRef("Cat")},
		},
		Default: "8",
	},
	ParamProps: ParamProps{
		Name:        "param-name",
		In:          "header",
		Required:    true,
		Schema:      &Schema{SchemaProps: SchemaProps{Type: []string{"string"}}},
		Description: "the description of this parameter",
	},
}

var parameterJSON = `{
	"items": {
		"$ref": "Cat"
	},
	"x-framework": "swagger-go",
  "$ref": "Dog",
  "description": "the description of this parameter",
  "maximum": 100,
  "minimum": 5,
  "exclusiveMaximum": true,
  "exclusiveMinimum": true,
  "maxLength": 100,
  "minLength": 5,
  "pattern": "\\w{1,5}\\w+",
  "maxItems": 100,
  "minItems": 5,
  "uniqueItems": true,
  "multipleOf": 5,
  "enum": ["hello", "world"],
  "type": "string",
  "format": "date",
	"name": "param-name",
	"in": "header",
	"required": true,
	"schema": {
		"type": "string"
	},
	"collectionFormat": "csv",
	"default": "8"
}`

func TestIntegrationParameter(t *testing.T) {
	var actual Parameter
	if assert.NoError(t, json.Unmarshal([]byte(parameterJSON), &actual)) {
		assert.EqualValues(t, actual, parameter)
	}

	assertParsesJSON(t, parameterJSON, parameter)
}

func TestParameterRoundtrip(t *testing.T) {
	cases := []jsontesting.RoundTripTestCase{
		{
			// Show at least one field from each embededd struct sitll allows
			// roundtrips successfully
			Name: "UnmarshalEmbedded",
			JSON: `{
				"$ref": "/components/ref/to/something.foo",
				"maxLength": 100,
				"type": "string",
				"x-framework": "swagger-go",
				"description": "a really cool description"
			  }`,
			Object: &Parameter{
				Refable{MustCreateRef("/components/ref/to/something.foo")},
				CommonValidations{
					MaxLength: int64Ptr(100),
				},
				SimpleSchema{
					Type: "string",
				},
				VendorExtensible{Extensions{
					"x-framework": "swagger-go",
				}},
				ParamProps{
					Description: "a really cool description",
				},
			},
		}, {
			Name:   "BasicCase",
			JSON:   parameterJSON,
			Object: &parameter,
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.Name, func(t *testing.T) {
			require.NoError(t, tcase.RoundTripTest(&Parameter{}))
		})
	}
}

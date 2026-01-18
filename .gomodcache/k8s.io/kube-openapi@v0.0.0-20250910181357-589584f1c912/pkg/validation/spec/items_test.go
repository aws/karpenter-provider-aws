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

var items = Items{
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
		Type:   "string",
		Format: "date",
		Items: &Items{
			Refable: Refable{Ref: MustCreateRef("Cat")},
		},
		CollectionFormat: "csv",
		Default:          "8",
	},
}

const itemsJSON = `{
	"items": {
		"$ref": "Cat"
	},
  "$ref": "Dog",
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
	"collectionFormat": "csv",
	"default": "8"
}`

func TestIntegrationItems(t *testing.T) {
	var actual Items
	if assert.NoError(t, json.Unmarshal([]byte(itemsJSON), &actual)) {
		assert.EqualValues(t, actual, items)
	}

	assertParsesJSON(t, itemsJSON, items)
}

func TestItemsRoundTrip(t *testing.T) {
	cases := []jsontesting.RoundTripTestCase{
		{
			// Show at least one field from each embededd struct sitll allows
			// roundtrips successfully
			Name: "UnmarshalEmbedded",
			JSON: `{
				"$ref": "/components/my.cool.Schema",
				"pattern": "x-^",
				"type": "string",
				"x-framework": "swagger-go"
			  }`,
			Object: &Items{
				Refable{MustCreateRef("/components/my.cool.Schema")},
				CommonValidations{
					Pattern: "x-^",
				},
				SimpleSchema{
					Type: "string",
				},
				VendorExtensible{Extensions{
					"x-framework": "swagger-go",
				}},
			},
		}, {
			Name:   "BasicCase",
			JSON:   itemsJSON,
			Object: &items,
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.Name, func(t *testing.T) {
			require.NoError(t, tcase.RoundTripTest(&Items{}))
		})
	}
}

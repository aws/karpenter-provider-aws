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

var paths = Paths{
	VendorExtensible: VendorExtensible{Extensions: map[string]interface{}{"x-framework": "go-swagger"}},
	Paths: map[string]PathItem{
		"/": {
			Refable: Refable{Ref: MustCreateRef("cats")},
		},
	},
}

const pathsJSON = `{"x-framework":"go-swagger","/":{"$ref":"cats"}}`
const pathsJSONInvalidKey = `{"x-framework":"go-swagger","not-path-nor-extension":"invalid","/":{"$ref":"cats"}}`

func TestIntegrationPaths(t *testing.T) {
	var actual Paths
	if assert.NoError(t, json.Unmarshal([]byte(pathsJSON), &actual)) {
		assert.EqualValues(t, actual, paths)
	}
	if assert.NoError(t, json.Unmarshal([]byte(pathsJSONInvalidKey), &actual)) {
		assert.EqualValues(t, actual, paths)
	}

	assertParsesJSON(t, pathsJSON, paths)
	assertParsesJSON(t, pathsJSONInvalidKey, paths)
}

func TestPathsRoundtrip(t *testing.T) {
	cases := []jsontesting.RoundTripTestCase{
		{
			// Show at least one field from each embededd struct sitll allows
			// roundtrips successfully
			Name: "UnmarshalEmbedded",
			JSON: `{
				"x-framework": "swagger-go",
				"/this-is-a-path": {
					"$ref": "/components/a/path/item"
				}
			  }`,
			Object: &Paths{
				VendorExtensible{Extensions{
					"x-framework": "swagger-go",
				}},
				map[string]PathItem{
					"/this-is-a-path": {Refable: Refable{MustCreateRef("/components/a/path/item")}},
				},
			},
		}, {
			Name:   "BasicCase",
			JSON:   pathsJSON,
			Object: &paths,
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.Name, func(t *testing.T) {
			require.NoError(t, tcase.RoundTripTest(&Paths{}))
		})
	}
}

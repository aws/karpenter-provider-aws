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

package builder

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestCollectSharedParameters(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want map[string]string
	}{
		{
			name: "empty",
			spec: "",
			want: nil,
		},
		{
			name: "no shared",
			spec: `{
  "parameters": {"pre": {"in": "body", "name": "body", "required": true, "schema": {}}},
  "paths": {
    "/api/v1/a/{name}": {"get": {"parameters": [
		  {"description": "x","in":"query","name": "x","type":"boolean","uniqueItems":true},
		  {"description": "y","in":"query","name": "y","type":"boolean","uniqueItems":true}
    ]}},
    "/api/v1/a/{name}/foo": {"get": {"parameters": [
		  {"description": "z","in":"query","name": "z","type":"boolean","uniqueItems":true}
    ]}},
    "/api/v1/b/{name}": {"get": {"parameters": [
		  {"description": "x","in":"query","name": "x2","type":"boolean","uniqueItems":true},
		  {"description": "y","in":"query","name": "y2","type":"boolean","uniqueItems":true}
    ]}},
    "/api/v1/b/{name}/foo": {"get": {"parameters": [
		  {"description": "z","in":"query","name": "z2","type":"boolean","uniqueItems":true}
    ]}}
  }
}`,
			want: map[string]string{
				`{"uniqueItems":true,"type":"boolean","description":"x","name":"x","in":"query"}`:  "x-yaDSHpi7",
				`{"uniqueItems":true,"type":"boolean","description":"y","name":"y","in":"query"}`:  "y-g6h7lEsz",
				`{"uniqueItems":true,"type":"boolean","description":"z","name":"z","in":"query"}`:  "z--SXYWoM_",
				`{"uniqueItems":true,"type":"boolean","description":"x","name":"x2","in":"query"}`: "x2-nds6MpS1",
				`{"uniqueItems":true,"type":"boolean","description":"y","name":"y2","in":"query"}`: "y2-exnalzYE",
				`{"uniqueItems":true,"type":"boolean","description":"z","name":"z2","in":"query"}`: "z2-8oJfzBQF",
			},
		},
		{
			name: "shared per operation",
			spec: `{
  "parameters": {"pre": {"in": "body", "name": "body", "required": true, "schema": {}}},
  "paths": {
    "/api/v1/a/{name}": {"get": {"parameters": [
		  {"description": "x","in":"query","name": "x","type":"boolean","uniqueItems":true},
		  {"description": "y","in":"query","name": "y","type":"boolean","uniqueItems":true}
    ]}},
    "/api/v1/a/{name}/foo": {"get": {"parameters": [
		  {"description": "z","in":"query","name": "z","type":"boolean","uniqueItems":true},
		  {"description": "y","in":"query","name": "y","type":"boolean","uniqueItems":true}
    ]}},
    "/api/v1/b/{name}": {"get": {"parameters": [
		  {"description": "z","in":"query","name": "z","type":"boolean","uniqueItems":true}
    ]}},
    "/api/v1/b/{name}/foo": {"get": {"parameters": [
		  {"description": "x","in":"query","name": "x","type":"boolean","uniqueItems":true}
    ]}}
  }
}`,
			want: map[string]string{
				`{"uniqueItems":true,"type":"boolean","description":"x","name":"x","in":"query"}`: "x-yaDSHpi7",
				`{"uniqueItems":true,"type":"boolean","description":"y","name":"y","in":"query"}`: "y-g6h7lEsz",
				`{"uniqueItems":true,"type":"boolean","description":"z","name":"z","in":"query"}`: "z--SXYWoM_",
			},
		},
		{
			name: "shared per path",
			spec: `{
  "parameters": {"pre": {"in": "body", "name": "body", "required": true, "schema": {}}},
  "paths": {
    "/api/v1/a/{name}": {"get": {},
      "parameters": [
		  {"description": "x","in":"query","name": "x","type":"boolean","uniqueItems":true},
		  {"description": "y","in":"query","name": "y","type":"boolean","uniqueItems":true}
      ]
    },
    "/api/v1/a/{name}/foo": {"get": {"parameters": [
		  {"description": "z","in":"query","name": "z","type":"boolean","uniqueItems":true},
		  {"description": "y","in":"query","name": "y","type":"boolean","uniqueItems":true}
    ]}},
    "/api/v1/b/{name}": {"get": {},
      "parameters": [
		  {"description": "z","in":"query","name": "z","type":"boolean","uniqueItems":true}
      ]
    },
    "/api/v1/b/{name}/foo": {"get": {"parameters": [
		  {"description": "x","in":"query","name": "x","type":"boolean","uniqueItems":true}
    ]}}
  }
}`,
			want: map[string]string{
				`{"uniqueItems":true,"type":"boolean","description":"x","name":"x","in":"query"}`: "x-yaDSHpi7",
				`{"uniqueItems":true,"type":"boolean","description":"y","name":"y","in":"query"}`: "y-g6h7lEsz",
				`{"uniqueItems":true,"type":"boolean","description":"z","name":"z","in":"query"}`: "z--SXYWoM_",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sp *spec.Swagger
			if tt.spec != "" {
				err := json.Unmarshal([]byte(tt.spec), &sp)
				require.NoError(t, err)
			}

			gotNamesByJSON, _, err := collectSharedParameters(sp)
			require.NoError(t, err)
			require.Equalf(t, tt.want, gotNamesByJSON, "unexpected shared parameters")
		})
	}
}

func TestReplaceSharedParameters(t *testing.T) {
	shared := map[string]string{
		`{"uniqueItems":true,"type":"boolean","description":"x","name":"x","in":"query"}`: "x",
		`{"uniqueItems":true,"type":"boolean","description":"y","name":"y","in":"query"}`: "y",
		`{"uniqueItems":true,"type":"boolean","description":"z","name":"z","in":"query"}`: "z",
	}

	tests := []struct {
		name string
		spec string
		want string
	}{
		{
			name: "empty",
			spec: "{}",
			want: `{"paths":null}`,
		},
		{
			name: "existing parameters",
			spec: `{"parameters": {"a":{"type":"boolean"}}}`,
			want: `{"parameters": {"a":{"type":"boolean"}},"paths":null}`,
		},
		{
			name: "replace",
			spec: `{
  "parameters": {"pre": {"in": "body", "name": "body", "required": true, "schema": {}}},
  "paths": {
    "/api/v1/a/{name}": {"get": {"description":"foo"},
      "parameters": [
        {"description": "x","in":"query","name": "x","type":"boolean","uniqueItems":true},
	    {"description": "y","in":"query","name": "y","type":"boolean","uniqueItems":true}
      ]
    },
    "/api/v1/a/{name}/foo": {"get": {"parameters": [
      {"description": "z","in":"query","name": "z","type":"boolean","uniqueItems":true},
	  {"description": "y","in":"query","name": "y","type":"boolean","uniqueItems":true}
    ]}},
    "/api/v1/b/{name}": {"get": {"parameters": [
	  {"description": "z","in":"query","name": "z","type":"boolean","uniqueItems":true}
    ]}},
    "/api/v1/b/{name}/foo": {"get": {"parameters": [
	  {"description": "x","in":"query","name": "x","type":"boolean","uniqueItems":true},
      {"description": "w","in":"query","name": "w","type":"boolean","uniqueItems":true}
    ]}}
  }
}`,
			want: `{
  "parameters": {"pre":{"in":"body","name":"body","required":true,"schema":{}}},
  "paths": {
    "/api/v1/a/{name}": {"get": {"description":"foo"},
      "parameters": [
        {"$ref": "#/parameters/x"},
        {"$ref": "#/parameters/y"}
      ]
    },
    "/api/v1/a/{name}/foo": {"get": {"parameters": [
      {"$ref": "#/parameters/z"},
      {"$ref": "#/parameters/y"}
    ]}},
    "/api/v1/b/{name}": {"get": {"parameters": [
      {"$ref": "#/parameters/z"}
    ]}},
    "/api/v1/b/{name}/foo": {"get": {"parameters": [
      {"$ref":"#/parameters/x"},
      {"description": "w","in":"query","name": "w","type":"boolean","uniqueItems":true}
    ]}}
  }
}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var unmarshalled *spec.Swagger
			err := json.Unmarshal([]byte(tt.spec), &unmarshalled)
			require.NoError(t, err)

			got, err := replaceSharedParameters(shared, unmarshalled)
			require.NoError(t, err)

			require.Equalf(t, normalizeJSON(t, tt.want), normalizeJSON(t, toJSON(t, got)), "unexpected result")
		})
	}
}

func toJSON(t *testing.T, x interface{}) string {
	bs, err := json.Marshal(x)
	require.NoError(t, err)

	return string(bs)
}

func normalizeJSON(t *testing.T, j string) string {
	var obj interface{}
	err := json.Unmarshal([]byte(j), &obj)
	require.NoError(t, err)
	return toJSON(t, obj)
}

func TestOperations(t *testing.T) {
	t.Log("Ensuring that operations() returns all operations in spec.PathItemProps")
	path := spec.PathItem{}
	v := reflect.ValueOf(path.PathItemProps)
	var rOps []any
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.Ptr {
			rOps = append(rOps, v.Field(i).Interface())
		}
	}

	ops := operations(&path)
	require.Equal(t, len(rOps), len(ops), "operations() should return all operations in spec.PathItemProps")
}

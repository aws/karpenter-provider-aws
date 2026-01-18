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

package openapiconv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"k8s.io/kube-openapi/pkg/spec3"
	"k8s.io/kube-openapi/pkg/util/jsontesting"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func TestConvert(t *testing.T) {

	tcs := []struct {
		groupVersion string
	}{{
		"batch.v1",
	}, {
		"api.v1",
	}, {
		"apiextensions.k8s.io.v1",
	}}

	for _, tc := range tcs {

		spec2JSON, err := os.ReadFile(filepath.Join("testdata_generated_from_k8s/v2_" + tc.groupVersion + ".json"))
		if err != nil {
			t.Fatal(err)
		}
		var swaggerSpec spec.Swagger
		err = json.Unmarshal(spec2JSON, &swaggerSpec)
		if err != nil {
			t.Fatal(err)
		}

		openAPIV2JSONBeforeConversion, err := swaggerSpec.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}

		convertedV3Spec := ConvertV2ToV3(&swaggerSpec)

		openAPIV2JSONAfterConversion, err := swaggerSpec.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		if err := jsontesting.JsonCompare(openAPIV2JSONBeforeConversion, openAPIV2JSONAfterConversion); err != nil {
			t.Errorf("Expected OpenAPI V2 to be untouched before and after conversion: %v", err)
		}

		spec3JSON, err := os.ReadFile(filepath.Join("testdata_generated_from_k8s/v3_" + tc.groupVersion + ".json"))
		if err != nil {
			t.Fatal(err)
		}

		var V3Spec spec3.OpenAPI
		json.Unmarshal(spec3JSON, &V3Spec)
		if !reflect.DeepEqual(V3Spec, *convertedV3Spec) {
			t.Error("Expected specs to be equal")
		}
	}
}

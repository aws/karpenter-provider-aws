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

package internal

import (
	"github.com/go-openapi/jsonreference"
	"reflect"
	"testing"
)

func TestJSONRefFromMap(t *testing.T) {
	testcases := []struct {
		name            string
		fromMap         map[string]interface{}
		expectNil       bool
		expectRefString string
	}{
		{
			name:            "nil map",
			expectRefString: "",
		}, {
			name:            "map with no $ref",
			fromMap:         map[string]interface{}{"a": "b"},
			expectRefString: "",
		}, {
			name:            "ref path",
			fromMap:         map[string]interface{}{"$ref": "#/path"},
			expectRefString: "#/path",
		},
	}

	for _, tc := range testcases {
		var tmp jsonreference.Ref
		err := JSONRefFromMap(&tmp, tc.fromMap)
		if err != nil {
			t.Errorf("Expect no error from JSONRefMap %s, got error %v", tc.name, err)
		}

		if tmp.String() != tc.expectRefString {
			t.Errorf("Expect jsonRef to be %s, got %s for %s", tc.expectRefString, tmp.String(), tc.name)
		}

	}
}

func TestSanitizeExtensions(t *testing.T) {
	testcases := []struct {
		in       map[string]interface{}
		expected map[string]interface{}
	}{
		{
			in:       map[string]interface{}{"a": "b", "x-extension": "foo"},
			expected: map[string]interface{}{"x-extension": "foo"},
		},
		{
			in:       map[string]interface{}{"a": "b"},
			expected: nil,
		},
		{
			in:       map[string]interface{}{},
			expected: nil,
		},
	}

	for _, tc := range testcases {
		e := SanitizeExtensions(tc.in)
		if !reflect.DeepEqual(tc.expected, e) {
			t.Errorf("Error: sanitize extensions does not match expected")
		}
	}
}

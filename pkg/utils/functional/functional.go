/*
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

package functional

import (
	"bytes"
	"encoding/json"
	"strings"
)

// HasAnyPrefix returns true if any of the provided prefixes match the given string s
func HasAnyPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// SplitCommaSeparatedString splits a string by commas, removes whitespace, and returns
// a slice of strings
func SplitCommaSeparatedString(value string) []string {
	var result []string
	for _, value := range strings.Split(value, ",") {
		result = append(result, strings.TrimSpace(value))
	}
	return result
}

func DeepCopy[T any](v *T) (*T, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(&buf)
	var cp T
	if err := dec.Decode(&cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

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
	"strings"

	"github.com/aws/aws-sdk-go/aws"
)

// UnionStringMaps merges all key value pairs into a single map, last write wins.
func UnionStringMaps(maps ...map[string]string) map[string]string {
	result := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func StringSliceWithout(vals []string, remove ...string) []string {
	if vals == nil {
		return nil
	}
	var without []string
	for _, val := range vals {
		if ContainsString(remove, val) {
			continue
		}
		without = append(without, val)
	}
	return without
}

func ContainsString(strings []string, candidate string) bool {
	for _, s := range strings {
		if candidate == s {
			return true
		}
	}
	return false
}

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
func SplitCommaSeparatedString(value string) []*string {
	var result []*string

	for _, value := range strings.Split(value, ",") {
		s := aws.String(strings.TrimSpace(value))
		result = append(result, s)
	}

	return result
}

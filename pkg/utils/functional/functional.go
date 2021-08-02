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
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"go.uber.org/multierr"
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

func StringSliceWithout(vals []string, remove string) []string {
	without := []string{}
	for _, val := range vals {
		if val == remove {
			continue
		}
		without = append(without, val)
	}
	return without
}

// IntersectStringSlice takes the intersection of all string slices
func IntersectStringSlice(slices ...[]string) []string {
	// count occurrences
	counts := map[string]int{}
	for _, strings := range slices {
		for _, s := range UniqueStrings(strings) {
			counts[s] = counts[s] + 1
		}
	}
	// select if occurred in all
	var intersection []string
	for key, count := range counts {
		if count == len(slices) {
			intersection = append(intersection, key)
		}
	}
	return intersection
}

func UniqueStrings(strings []string) []string {
	exists := map[string]bool{}
	for _, s := range strings {
		exists[s] = true
	}
	var unique []string
	for s := range exists {
		unique = append(unique, s)
	}
	return unique
}

func ContainsString(strings []string, candidate string) bool {
	for _, s := range strings {
		if candidate == s {
			return true
		}
	}
	return false
}

// ValidateAll returns nil if all errorables return nil, otherwise returns the concatenated failure messages.
func ValidateAll(errorables ...func() error) error {
	var err error
	for _, errorable := range errorables {
		err = multierr.Append(err, errorable())
	}
	return err
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

// InvertStringMap swaps keys to values and values to keys. All values must be distinct.
func InvertStringMap(stringMap map[string]string) map[string]string {
	inverted := map[string]string{}
	for k, v := range stringMap {
		inverted[v] = k
	}
	return inverted
}

// MaxDuration returns the largest duration
func MaxDuration(durations ...time.Duration) time.Duration {
	var max time.Duration
	for _, duration := range durations {
		if duration > max {
			max = duration
		}
	}
	return max
}

func JsonEquals(a, b interface{}) bool {
	aJson, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}
	bJson, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}
	return reflect.DeepEqual(string(aJson), string(bJson))
}

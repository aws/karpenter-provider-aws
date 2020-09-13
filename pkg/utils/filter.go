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

package utils

// GreaterThan returns values greater than the target value
func GreaterThan(values []int32, target int32) (results []int32) {
	return Filter(values, target, func(a int32, b int32) bool {
		return a > b
	})
}

// LessThan returns values less than the target value
func LessThan(values []int32, target int32) (results []int32) {
	return Filter(values, target, func(a int32, b int32) bool {
		return a < b
	})
}

// Filter returns values for which the predicate returns true
func Filter(values []int32, target int32, predicate func(a int32, b int32) bool) (results []int32) {
	for _, value := range values {
		if predicate(value, target) {
			results = append(results, value)
		}
	}
	return results
}

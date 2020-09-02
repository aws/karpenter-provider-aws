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

func greaterThan(values []int32, target int32) (results []int32) {
	return filter(values, target, func(a int32, b int32) bool {
		return a > b
	})
}

func lessThan(values []int32, target int32) (results []int32) {
	return filter(values, target, func(a int32, b int32) bool {
		return a < b
	})
}

func filter(values []int32, target int32, predicate func(a int32, b int32) bool) (results []int32) {
	for _, value := range values {
		if predicate(value, target) {
			results = append(results, value)
		}
	}
	return results
}

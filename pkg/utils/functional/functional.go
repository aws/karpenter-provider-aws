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

package f

import (
	"math"
)

// GreaterThanInt32 returns values greater than the target value
func GreaterThanInt32(values []int32, target int32) (results []int32) {
	return FilterInt32(values, target, func(a int32, b int32) bool {
		return a > b
	})
}

// LessThanInt32 returns values less than the target value
func LessThanInt32(values []int32, target int32) (results []int32) {
	return FilterInt32(values, target, func(a int32, b int32) bool {
		return a < b
	})
}

// Filter returns values for which the predicate returns true
func FilterInt32(values []int32, target int32, predicate func(a int32, b int32) bool) (results []int32) {
	for _, value := range values {
		if predicate(value, target) {
			results = append(results, value)
		}
	}
	return results
}

// MaxInt32 returns the maximum value in the slice.
func MaxInt32(values []int32) int32 {
	return SelectInt32(values, func(got int32, want int32) int32 {
		return int32(math.Max(float64(got), float64((want))))
	})
}

// MinInt32 returns the minimum value in the slice.
func MinInt32(values []int32) int32 {
	return SelectInt32(values, func(got int32, want int32) int32 {
		return int32(math.Min(float64(got), float64((want))))
	})
}

// SelectInt32 returns the victor of the slice selected by the comparison function.
func SelectInt32(values []int32, selector func(int32, int32) int32) int32 {
	selected := values[0]
	for _, value := range values {
		selected = selector(selected, int32(value))
	}
	return selected
}

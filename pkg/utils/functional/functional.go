package f

import "math"

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
	for value := range values {
		selected = selector(selected, int32(value))
	}
	return selected
}

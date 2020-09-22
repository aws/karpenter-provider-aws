package f

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

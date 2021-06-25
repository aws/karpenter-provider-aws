package predicates

// WithinStrings returns a func that returns true if string is within strings.
func WithinStrings(allowed []string) func(string) bool {
	return func(actual string) bool {
		for _, expected := range allowed {
			if expected == actual {
				return true
			}
		}
		return false
	}
}

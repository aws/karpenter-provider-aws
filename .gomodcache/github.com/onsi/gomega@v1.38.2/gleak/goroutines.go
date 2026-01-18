package gleak

import "github.com/onsi/gomega/gleak/goroutine"

// Goroutine represents information about a single goroutine and is a
// convenience type alias.
type Goroutine = goroutine.Goroutine

// Goroutines returns information about all goroutines: their goroutine IDs, the
// names of the topmost functions in the backtraces, and finally the goroutine
// backtraces.
func Goroutines() []Goroutine {
	return goroutine.Goroutines()
}

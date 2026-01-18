package gleak

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/onsi/gomega/format"
)

// G takes an actual "any" untyped value and returns it as a typed Goroutine, if
// possible. It returns an error if actual isn't of either type Goroutine or a
// pointer to it. G is intended to be mainly used by goroutine-related Gomega
// matchers, such as IgnoringTopFunction, et cetera.
func G(actual any, matchername string) (Goroutine, error) {
	if actual != nil {
		switch actual := actual.(type) {
		case Goroutine:
			return actual, nil
		case *Goroutine:
			return *actual, nil
		}
	}
	return Goroutine{},
		fmt.Errorf("%s matcher expects a Goroutine or *Goroutine.  Got:\n%s",
			matchername, format.Object(actual, 1))
}

// goids returns a (sorted) list of Goroutine IDs in textual format.
func goids(gs []Goroutine) string {
	ids := make([]uint64, len(gs))
	for idx, g := range gs {
		ids[idx] = g.ID
	}
	sort.Sort(Uint64Slice(ids))
	var buff strings.Builder
	for idx, id := range ids {
		if idx > 0 {
			buff.WriteString(", ")
		}
		buff.WriteString(strconv.FormatInt(int64(id), 10))
	}
	return buff.String()
}

// Uint64Slice implements the sort.Interface for a []uint64 to sort in
// increasing order.
type Uint64Slice []uint64

func (s Uint64Slice) Len() int           { return len(s) }
func (s Uint64Slice) Less(a, b int) bool { return s[a] < s[b] }
func (s Uint64Slice) Swap(a, b int)      { s[a], s[b] = s[b], s[a] }

package gleak

import (
	"sort"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

// IgnoringGoroutines succeeds if an actual goroutine, identified by its ID, is
// in a slice of expected goroutines. A typical use of the IgnoringGoroutines
// matcher is to take a snapshot of the current goroutines just right before a
// test and then at the end of a test filtering out these "good" and known
// goroutines.
func IgnoringGoroutines(goroutines []Goroutine) types.GomegaMatcher {
	m := &ignoringGoroutinesMatcher{
		ignoreGoids: map[uint64]struct{}{},
	}
	for _, g := range goroutines {
		m.ignoreGoids[g.ID] = struct{}{}
	}
	return m
}

type ignoringGoroutinesMatcher struct {
	ignoreGoids map[uint64]struct{}
}

// Match succeeds if actual is a Goroutine and its ID is in the set of
// goroutine IDs to expect and thus to ignore in leak checks.
func (matcher *ignoringGoroutinesMatcher) Match(actual any) (success bool, err error) {
	g, err := G(actual, "IgnoringGoroutines")
	if err != nil {
		return false, err
	}
	_, ok := matcher.ignoreGoids[g.ID]
	return ok, nil
}

// FailureMessage returns a failure message if the actual goroutine isn't in the
// set of goroutines to be ignored.
func (matcher *ignoringGoroutinesMatcher) FailureMessage(actual any) (message string) {
	return format.Message(actual, "to be contained in the list of expected goroutine IDs", matcher.expectedGoids())
}

// NegatedFailureMessage returns a negated failure message if the actual
// goroutine actually is in the set of goroutines to be ignored.
func (matcher *ignoringGoroutinesMatcher) NegatedFailureMessage(actual any) (message string) {
	return format.Message(actual, "not to be contained in the list of expected goroutine IDs", matcher.expectedGoids())
}

// expectedGoids returns the sorted list of expected goroutine IDs.
func (matcher *ignoringGoroutinesMatcher) expectedGoids() []uint64 {
	ids := make([]uint64, 0, len(matcher.ignoreGoids))
	for id := range matcher.ignoreGoids {
		ids = append(ids, id)
	}
	sort.Sort(Uint64Slice(ids))
	return ids
}

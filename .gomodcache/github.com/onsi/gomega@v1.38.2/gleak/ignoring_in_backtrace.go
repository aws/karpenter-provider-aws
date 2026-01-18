package gleak

import (
	"fmt"
	"strings"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

// IgnoringInBacktrace succeeds if a function name is contained in the backtrace
// of the actual goroutine description.
func IgnoringInBacktrace(fname string) types.GomegaMatcher {
	return &ignoringInBacktraceMatcher{fname: fname}
}

type ignoringInBacktraceMatcher struct {
	fname string
}

// Match succeeds if actual's backtrace contains the specified function name.
func (matcher *ignoringInBacktraceMatcher) Match(actual any) (success bool, err error) {
	g, err := G(actual, "IgnoringInBacktrace")
	if err != nil {
		return false, err
	}
	return strings.Contains(g.Backtrace, matcher.fname), nil
}

// FailureMessage returns a failure message if the actual's backtrace does not
// contain the specified function name.
func (matcher *ignoringInBacktraceMatcher) FailureMessage(actual any) (message string) {
	return format.Message(actual, fmt.Sprintf("to contain %q in the goroutine's backtrace", matcher.fname))
}

// NegatedFailureMessage returns a failure message if the actual's backtrace
// does contain the specified function name.
func (matcher *ignoringInBacktraceMatcher) NegatedFailureMessage(actual any) (message string) {
	return format.Message(actual, fmt.Sprintf("not to contain %q in the goroutine's backtrace", matcher.fname))
}

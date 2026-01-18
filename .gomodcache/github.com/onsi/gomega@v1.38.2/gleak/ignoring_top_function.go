package gleak

import (
	"fmt"
	"strings"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

// IgnoringTopFunction succeeds if the topmost function in the backtrace of an
// actual goroutine has the specified function name, and optionally the actual
// goroutine has the specified goroutine state.
//
// The expected top function name topfn is either in the form of
// "topfunction-name", "topfunction-name...", or "topfunction-name [state]".
//
// An ellipsis "..." after a topfunction-name matches any goroutine's top
// function name if topfunction-name is a prefix and the goroutine's top
// function name is at least one level deeper. For instance, "foo.bar..."
// matches "foo.bar.baz", but doesn't match "foo.bar".
//
// If the optional expected state is specified, then a goroutine's state needs
// to start with this expected state text. For instance, "foo.bar [running]"
// matches a goroutine where the name of the top function is "foo.bar" and the
// goroutine's state starts with "running".
func IgnoringTopFunction(topfname string) types.GomegaMatcher {
	if brIndex := strings.Index(topfname, " ["); brIndex >= 0 {
		expectedState := strings.Trim(topfname[brIndex+1:], "[]")
		expectedTopFunction := strings.Trim(topfname[:brIndex+1], " ")
		return &ignoringTopFunctionMatcher{
			expectedTopFunction: expectedTopFunction,
			expectedState:       expectedState,
		}
	}
	if strings.HasSuffix(topfname, "...") {
		expectedTopFunction := topfname[:len(topfname)-3+1] // ...one trailing dot still expected
		return &ignoringTopFunctionMatcher{
			expectedTopFunction: expectedTopFunction,
			matchPrefix:         true,
		}
	}
	return &ignoringTopFunctionMatcher{
		expectedTopFunction: topfname,
	}
}

type ignoringTopFunctionMatcher struct {
	expectedTopFunction string
	expectedState       string
	matchPrefix         bool
}

// Match succeeds if an actual goroutine's top function in the backtrace matches
// the specified function name or function name prefix, or name and goroutine
// state.
func (matcher *ignoringTopFunctionMatcher) Match(actual any) (success bool, err error) {
	g, err := G(actual, "IgnoringTopFunction")
	if err != nil {
		return false, err
	}
	if matcher.matchPrefix {
		return strings.HasPrefix(g.TopFunction, matcher.expectedTopFunction), nil
	}
	if g.TopFunction != matcher.expectedTopFunction {
		return false, nil
	}
	if matcher.expectedState == "" {
		return true, nil
	}
	return strings.HasPrefix(g.State, matcher.expectedState), nil
}

// FailureMessage returns a failure message if the actual goroutine doesn't have
// the specified function name/prefix (and optional state) at the top of the
// backtrace.
func (matcher *ignoringTopFunctionMatcher) FailureMessage(actual any) (message string) {
	return format.Message(actual, matcher.message())
}

// NegatedFailureMessage returns a failure message if the actual goroutine has
// the specified function name/prefix (and optional state) at the top of the
// backtrace.
func (matcher *ignoringTopFunctionMatcher) NegatedFailureMessage(actual any) (message string) {
	return format.Message(actual, "not "+matcher.message())
}

func (matcher *ignoringTopFunctionMatcher) message() string {
	if matcher.matchPrefix {
		return fmt.Sprintf("to have the prefix %q for its topmost function", matcher.expectedTopFunction)
	}
	if matcher.expectedState != "" {
		return fmt.Sprintf("to have the topmost function %q and the state %q",
			matcher.expectedTopFunction, matcher.expectedState)
	}
	return fmt.Sprintf("to have the topmost function %q", matcher.expectedTopFunction)
}

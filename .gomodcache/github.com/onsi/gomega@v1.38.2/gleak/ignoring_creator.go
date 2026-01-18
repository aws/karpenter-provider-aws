package gleak

import (
	"fmt"
	"strings"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

// IgnoringCreator succeeds if the goroutine was created by a function matching
// the specified name. The expected creator function name is either in the form
// of "creatorfunction-name" or "creatorfunction-name...".
//
// An ellipsis "..." after a creatorfunction-name matches any creator function
// name if creatorfunction-name is a prefix and the goroutine's creator function
// name is at least one level deeper. For instance, "foo.bar..." matches
// "foo.bar.baz", but doesn't match "foo.bar".
func IgnoringCreator(creatorfname string) types.GomegaMatcher {
	if strings.HasSuffix(creatorfname, "...") {
		expectedCreatorFunction := creatorfname[:len(creatorfname)-3+1] // ...one trailing dot still expected
		return &ignoringCreator{
			expectedCreatorFunction: expectedCreatorFunction,
			matchPrefix:             true,
		}
	}
	return &ignoringCreator{
		expectedCreatorFunction: creatorfname,
	}
}

type ignoringCreator struct {
	expectedCreatorFunction string
	matchPrefix             bool
}

// Match succeeds if an actual goroutine's creator function in the backtrace
// matches the specified function name or function name prefix.
func (matcher *ignoringCreator) Match(actual any) (success bool, err error) {
	g, err := G(actual, "IgnoringCreator")
	if err != nil {
		return false, err
	}
	if matcher.matchPrefix {
		return strings.HasPrefix(g.CreatorFunction, matcher.expectedCreatorFunction), nil
	}
	return g.CreatorFunction == matcher.expectedCreatorFunction, nil
}

// FailureMessage returns a failure message if the actual goroutine doesn't have
// the specified function name/prefix (and optional state) at the top of the
// backtrace.
func (matcher *ignoringCreator) FailureMessage(actual any) (message string) {
	return format.Message(actual, matcher.message())
}

// NegatedFailureMessage returns a failure message if the actual goroutine has
// the specified function name/prefix (and optional state) at the top of the
// backtrace.
func (matcher *ignoringCreator) NegatedFailureMessage(actual any) (message string) {
	return format.Message(actual, "not "+matcher.message())
}

func (matcher *ignoringCreator) message() string {
	if matcher.matchPrefix {
		return fmt.Sprintf("to be created by a function with prefix %q", matcher.expectedCreatorFunction)
	}
	return fmt.Sprintf("to be created by %q", matcher.expectedCreatorFunction)
}

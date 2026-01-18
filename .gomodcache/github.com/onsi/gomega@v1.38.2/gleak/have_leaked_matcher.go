package gleak

import (
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gleak/goroutine"
	"github.com/onsi/gomega/types"
)

// ReportFilenameWithPath controls whether to show call locations in leak
// reports by default in abbreviated form with only source code filename with
// package name and line number, or alternatively with source code filename with
// path and line number.
//
// That is, with ReportFilenameWithPath==false:
//
//	foo/bar.go:123
//
// Or with ReportFilenameWithPath==true:
//
//	/home/goworld/coolprojects/mymodule/foo/bar.go:123
var ReportFilenameWithPath = false

// standardFilters specifies the always automatically included no-leak goroutine
// filter matchers.
//
// Note: it's okay to instantiate the Gomega Matchers here, as all goroutine
// filtering-related gleak matchers are stateless with respect to any actual
// value they try to match. This allows us to simply prepend them to any
// user-supplied optional matchers when HaveLeaked returns a new goroutine
// leakage detecting matcher.
//
// Note: cgo's goroutines with status "[syscall, locked to thread]" do not
// appear any longer (since mid-2017), as these cgo goroutines are put into the
// "dead" state when not in use. See: https://github.com/golang/go/issues/16714
// and https://go-review.googlesource.com/c/go/+/45030/.
var standardFilters = []types.GomegaMatcher{
	// Ginkgo testing framework
	IgnoringTopFunction("github.com/onsi/ginkgo/v2/internal.(*Suite).runNode"),
	IgnoringTopFunction("github.com/onsi/ginkgo/v2/internal.(*Suite).runNode..."),
	gomega.And(IgnoringTopFunction("runtime.goexit1"), IgnoringCreator("github.com/onsi/ginkgo/v2/internal.(*Suite).runNode")),
	IgnoringTopFunction("github.com/onsi/ginkgo/v2/internal/interrupt_handler.(*InterruptHandler).registerForInterrupts..."),
	IgnoringTopFunction("github.com/onsi/ginkgo/internal/specrunner.(*SpecRunner).registerForInterrupts"),
	IgnoringCreator("github.com/onsi/ginkgo/v2/internal.(*genericOutputInterceptor).ResumeIntercepting"),
	IgnoringCreator("github.com/onsi/ginkgo/v2/internal.(*genericOutputInterceptor).ResumeIntercepting..."),
	IgnoringCreator("github.com/onsi/ginkgo/v2/internal.RegisterForProgressSignal"),

	// goroutines of Go's own testing package for its own workings...
	IgnoringTopFunction("testing.RunTests [chan receive]"),
	IgnoringTopFunction("testing.(*T).Run [chan receive]"),
	IgnoringTopFunction("testing.(*T).Parallel [chan receive]"),

	// os/signal starts its own runtime goroutine, where loop calls signal_recv
	// in a loop, so we need to expect them both...
	IgnoringTopFunction("os/signal.signal_recv"),
	IgnoringTopFunction("os/signal.loop"),

	// signal.Notify starts a runtime goroutine...
	IgnoringInBacktrace("runtime.ensureSigM"),

	// reading a trace...
	IgnoringInBacktrace("runtime.ReadTrace"),
}

// HaveLeaked succeeds (or rather, "suckceeds" considering it appears in failing
// tests) if after filtering out ("ignoring") the expected goroutines from the
// list of actual goroutines the remaining list of goroutines is non-empty.
// These goroutines not filtered out are considered to have been leaked.
//
// For convenience, HaveLeaked automatically filters out well-known runtime and
// testing goroutines using a built-in standard filter matchers list. In
// addition to the built-in filters, HaveLeaked accepts an optional list of
// non-leaky goroutine filter matchers. These filtering matchers can be
// specified in different formats, as described below.
//
// Since there might be "pending" goroutines at the end of tests that eventually
// will properly wind down so they aren't leaking, HaveLeaked is best paired
// with Eventually instead of Expect. In its shortest form this will use
// Eventually's default timeout and polling interval settings, but these can be
// overridden as usual:
//
//	// Remember to use "Goroutines" and not "Goroutines()" with Eventually()!
//	Eventually(Goroutines).ShouldNot(HaveLeaked())
//	Eventually(Goroutines).WithTimeout(5 * time.Second).ShouldNot(HaveLeaked())
//
// In its simplest form, an expected non-leaky goroutine can be identified by
// passing the (fully qualified) name (in form of a string) of the topmost
// function in the backtrace. For instance:
//
//	Eventually(Goroutines).ShouldNot(HaveLeaked("foo.bar"))
//
// This is the shorthand equivalent to this explicit form:
//
//	Eventually(Goroutines).ShouldNot(HaveLeaked(IgnoringTopFunction("foo.bar")))
//
// HaveLeak also accepts passing a slice of Goroutine objects to be considered
// non-leaky goroutines.
//
//	snapshot := Goroutines()
//	DoSomething()
//	Eventually(Goroutines).ShouldNot(HaveLeaked(snapshot))
//
// Again, this is shorthand for the following explicit form:
//
//	snapshot := Goroutines()
//	DoSomething()
//	Eventually(Goroutines).ShouldNot(HaveLeaked(IgnoringGoroutines(snapshot)))
//
// Finally, HaveLeaked accepts any GomegaMatcher and will repeatedly pass it a
// Goroutine object: if the matcher succeeds, the Goroutine object in question
// is considered to be non-leaked and thus filtered out. While the following
// built-in Goroutine filter matchers should hopefully cover most situations,
// any suitable GomegaMatcher can be used for tricky leaky Goroutine filtering.
//
//	IgnoringTopFunction("foo.bar")
//	IgnoringTopFunction("foo.bar...")
//	IgnoringTopFunction("foo.bar [chan receive]")
//	IgnoringGoroutines(expectedGoroutines)
//	IgnoringInBacktrace("foo.bar.baz")
func HaveLeaked(ignoring ...any) types.GomegaMatcher {
	m := &HaveLeakedMatcher{filters: standardFilters}
	for _, ign := range ignoring {
		switch ign := ign.(type) {
		case string:
			m.filters = append(m.filters, IgnoringTopFunction(ign))
		case []Goroutine:
			m.filters = append(m.filters, IgnoringGoroutines(ign))
		case types.GomegaMatcher:
			m.filters = append(m.filters, ign)
		default:
			panic(fmt.Sprintf("HaveLeaked expected a string, []Goroutine, or GomegaMatcher, but got:\n%s", format.Object(ign, 1)))
		}
	}
	return m
}

// HaveLeakedMatcher implements the HaveLeaked Gomega Matcher that succeeds if
// the actual list of goroutines is non-empty after filtering out the expected
// goroutines.
type HaveLeakedMatcher struct {
	filters []types.GomegaMatcher // expected goroutines that aren't leaks.
	leaked  []Goroutine           // surplus goroutines which we consider to be leaks.
}

var gsT = reflect.TypeOf([]Goroutine{})

// Match succeeds if actual is an array or slice of Goroutine
// information and still contains goroutines after filtering out all expected
// goroutines that were specified when creating the matcher.
func (matcher *HaveLeakedMatcher) Match(actual any) (success bool, err error) {
	val := reflect.ValueOf(actual)
	switch val.Kind() {
	case reflect.Array, reflect.Slice:
		if !val.Type().AssignableTo(gsT) {
			return false, fmt.Errorf(
				"HaveLeaked matcher expects an array or slice of goroutines.  Got:\n%s",
				format.Object(actual, 1))
		}
	default:
		return false, fmt.Errorf(
			"HaveLeaked matcher expects an array or slice of goroutines.  Got:\n%s",
			format.Object(actual, 1))
	}
	goroutines := val.Convert(gsT).Interface().([]Goroutine)
	matcher.leaked, err = matcher.filter(goroutines, matcher.filters)
	if err != nil {
		return false, err
	}
	if len(matcher.leaked) == 0 {
		return false, nil
	}
	return true, nil // we have leak(ed)
}

// FailureMessage returns a failure message if there are leaked goroutines.
func (matcher *HaveLeakedMatcher) FailureMessage(actual any) (message string) {
	return fmt.Sprintf("Expected to leak %d goroutines:\n%s", len(matcher.leaked), matcher.listGoroutines(matcher.leaked, 1))
}

// NegatedFailureMessage returns a negated failure message if there aren't any leaked goroutines.
func (matcher *HaveLeakedMatcher) NegatedFailureMessage(actual any) (message string) {
	return fmt.Sprintf("Expected not to leak %d goroutines:\n%s", len(matcher.leaked), matcher.listGoroutines(matcher.leaked, 1))
}

// listGoroutines returns a somewhat compact textual representation of the
// specified goroutines, by ignoring the often quite lengthy backtrace
// information.
func (matcher *HaveLeakedMatcher) listGoroutines(gs []Goroutine, indentation uint) string {
	var buff strings.Builder
	indent := strings.Repeat(format.Indent, int(indentation))
	backtraceIdent := strings.Repeat(format.Indent, int(indentation+1))
	for gidx, g := range gs {
		if gidx > 0 {
			buff.WriteRune('\n')
		}
		buff.WriteString(indent)
		buff.WriteString("goroutine ")
		buff.WriteString(strconv.FormatUint(g.ID, 10))
		buff.WriteString(" [")
		buff.WriteString(g.State)
		buff.WriteString("]\n")

		backtrace := g.Backtrace
		for backtrace != "" {
			buff.WriteString(backtraceIdent)
			// take the next two lines (function name and file name plus line
			// number) and output them as a single indented line.
			nlIdx := strings.IndexRune(backtrace, '\n')
			if nlIdx < 0 {
				// ...a dodgy single line
				buff.WriteString(backtrace)
				break
			}
			calledFuncName := backtrace[:nlIdx]
			// Take care of not mangling the optional "created by " prefix is
			// present, when formatting the location to use either long or
			// shortened filenames and paths.
			location := backtrace[nlIdx+1:]
			nnlIdx := strings.IndexRune(location, '\n')
			if nnlIdx >= 0 {
				backtrace, location = location[nnlIdx+1:], location[:nnlIdx]
			} else {
				backtrace = "" // ...the next location line is missing
			}
			// Don't accidentally strip off the "created by" prefix when
			// shortening the call site location filename...
			location = strings.TrimSpace(location) // strip of indentation
			lineno := ""
			if linenoIdx := strings.LastIndex(location, ":"); linenoIdx >= 0 {
				location, lineno = location[:linenoIdx], location[linenoIdx+1:]
			}
			location = formatFilename(location) + ":" + lineno
			// Add to compact backtrace
			buff.WriteString(calledFuncName)
			buff.WriteString(" at ")
			// Don't output any program counter hex offsets, so strip them out
			// here, if present; well, they should always be present, but better
			// safe than sorry.
			if offsetIdx := strings.LastIndexFunc(location,
				func(r rune) bool { return r == ' ' }); offsetIdx >= 0 {
				buff.WriteString(location[:offsetIdx])
			} else {
				buff.WriteString(location)
			}
			if backtrace != "" {
				buff.WriteRune('\n')
			}
		}
	}
	return buff.String()
}

// filter returns a list of leaked goroutines by removing all expected
// goroutines from the given list of goroutines, using the specified checkers.
// The calling goroutine is always filtered out automatically. A checker checks
// if a certain goroutine is expected (then it gets filtered out), or not. If
// all checkers do not signal that they expect a certain goroutine then this
// goroutine is considered to be a leak.
func (matcher *HaveLeakedMatcher) filter(
	goroutines []Goroutine, filters []types.GomegaMatcher,
) ([]Goroutine, error) {
	gs := make([]Goroutine, 0, len(goroutines))
	myID := goroutine.Current().ID
nextgoroutine:
	for _, g := range goroutines {
		if g.ID == myID {
			continue
		}
		for _, filter := range filters {
			matches, err := filter.Match(g)
			if err != nil {
				return nil, err
			}
			if matches {
				continue nextgoroutine
			}
		}
		gs = append(gs, g)
	}
	return gs, nil
}

// formatFilename takes the ReportFilenameWithPath setting into account to
// either return the full specified filename with a path or alternatively
// shortening it to contain only the package name and the filename, but not the
// full path.
func formatFilename(filename string) string {
	if ReportFilenameWithPath {
		return filename
	}
	dir := filepath.Dir(filename)
	pkg := filepath.Base(dir)
	switch pkg {
	case ".", "..", "/", "\\":
		pkg = ""
	}
	// Go dumps stacks always with file locations containing forward slashes,
	// even on Windows. Thus, we do NOT use filepath.Join here, but instead
	// path.Join in order to keep with using forward slashes.
	return path.Join(pkg, filepath.ToSlash(filepath.Base(filename)))
}

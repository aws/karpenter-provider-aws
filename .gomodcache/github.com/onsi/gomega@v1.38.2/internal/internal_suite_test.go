package internal_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/internal"
)

func TestInternal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal Suite")
}

// InstrumentedGomega
type InstrumentedGomega struct {
	G                 *internal.Gomega
	FailureMessage    string
	FailureSkip       []int
	RegisteredHelpers []string
}

func NewInstrumentedGomega() *InstrumentedGomega {
	out := &InstrumentedGomega{}

	out.G = internal.NewGomega(internal.FetchDefaultDurationBundle())
	out.G.Fail = func(message string, skip ...int) {
		out.FailureMessage = message
		out.FailureSkip = skip
	}
	out.G.THelper = func() {
		pc, _, _, _ := runtime.Caller(1)
		f := runtime.FuncForPC(pc)
		funcName := strings.TrimPrefix(f.Name(), "github.com/onsi/gomega/internal.")
		out.RegisteredHelpers = append(out.RegisteredHelpers, funcName)
	}

	return out
}

// TestMatcher
var MATCH = "match"
var NO_MATCH = "no match"
var ERR_MATCH = "err match"
var TEST_MATCHER_ERR = errors.New("spec matcher error")

type SpecMatcher struct{}

func (matcher SpecMatcher) Match(actual any) (bool, error) {
	switch actual {
	case MATCH:
		return true, nil
	case NO_MATCH:
		return false, nil
	case ERR_MATCH:
		return false, TEST_MATCHER_ERR
	}
	return false, fmt.Errorf("unknown actual %v", actual)
}

func (matcher SpecMatcher) FailureMessage(actual any) string {
	return fmt.Sprintf("positive: %s", actual)
}

func (matcher SpecMatcher) NegatedFailureMessage(actual any) string {
	return fmt.Sprintf("negative: %s", actual)
}

func SpecMatch() SpecMatcher {
	return SpecMatcher{}
}

// FakeGomegaTestingT
type FakeGomegaTestingT struct {
	CalledHelper bool
	CalledFatalf string
}

func (f *FakeGomegaTestingT) Helper() {
	f.CalledHelper = true
}

func (f *FakeGomegaTestingT) Fatalf(s string, args ...any) {
	f.CalledFatalf = fmt.Sprintf(s, args...)
}

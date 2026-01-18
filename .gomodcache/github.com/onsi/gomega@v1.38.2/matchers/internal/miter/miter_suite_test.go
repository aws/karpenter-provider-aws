package miter_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMatcherIter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Matcher Iter Support Suite")
}

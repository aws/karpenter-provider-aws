package gleak

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// In case this suite is run in parallel with other test suites using "ginkgo
// -p", then there is a Ginkgo-related background go routine that we need to
// ignore in all tests and that can be identified only by its random ID.
var _ = BeforeSuite(func() {
	IgnoreGinkgoParallelClient()
})

func TestGleak(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gleak Suite")
}

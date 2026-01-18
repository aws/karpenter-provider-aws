package gmeasure_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gleak"
)

func TestGmeasure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gmeasure Suite")
}

var _ = BeforeEach(func() {
	g := gleak.Goroutines()
	DeferCleanup(func() {
		Eventually(gleak.Goroutines).ShouldNot(gleak.HaveLeaked(g))
	})
})

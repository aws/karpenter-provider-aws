package chaos

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	environmentaws "github.com/aws/karpenter/test/pkg/environment/aws"
)

var env *environmentaws.Environment

func TestConsolidation(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		env = environmentaws.NewEnvironment(t)
	})
	AfterSuite(func() {
		env.Stop()
	})
	RunSpecs(t, "Consolidation")
}

var _ = BeforeEach(func() { env.BeforeEach() })
var _ = AfterEach(func() { env.Cleanup() })
var _ = AfterEach(func() { env.ForceCleanup() })
var _ = AfterEach(func() { env.AfterEach() })

var _ = Describe("Chaos", func() {
})

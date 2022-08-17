package integration_test

import (
	"testing"

	"github.com/aws/karpenter/test/pkg/environment"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var env *environment.Environment

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	BeforeSuite(func() {
		var err error
		env, err = environment.NewEnvironment(t)
		Expect(err).ToNot(HaveOccurred())
	})
	RunSpecs(t, "Integration")
}

var _ = BeforeEach(func() { env.BeforeEach() })
var _ = AfterEach(func() { env.AfterEach() })

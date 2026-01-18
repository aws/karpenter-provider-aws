package gcustom_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestGcustom(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gcustom Suite")
}

package autoscaler

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAutoscaler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Autoscaler Suite")
}

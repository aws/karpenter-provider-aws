package priorityqueue

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestControllerWorkqueue(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerWorkqueue Suite")
}

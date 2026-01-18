package matchers_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/internal/gutil"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeADirectoryMatcher", func() {
	When("passed a string", func() {
		It("should do the right thing", func() {
			Expect("/dne/test").ShouldNot(BeADirectory())

			tmpFile, err := os.CreateTemp("", "gomega-test-tempfile")
			Expect(err).ShouldNot(HaveOccurred())
			defer os.Remove(tmpFile.Name())
			Expect(tmpFile.Name()).ShouldNot(BeADirectory())

			tmpDir, err := gutil.MkdirTemp("", "gomega-test-tempdir")
			Expect(err).ShouldNot(HaveOccurred())
			defer os.Remove(tmpDir)
			Expect(tmpDir).Should(BeADirectory())
		})
	})

	When("passed something else", func() {
		It("should error", func() {
			success, err := (&BeADirectoryMatcher{}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeADirectoryMatcher{}).Match(true)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})

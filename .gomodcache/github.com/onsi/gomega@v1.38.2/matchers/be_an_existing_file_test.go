package matchers_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/internal/gutil"
	. "github.com/onsi/gomega/matchers"
)

var _ = Describe("BeAnExistingFileMatcher", func() {
	When("passed a string", func() {
		It("should do the right thing", func() {
			Expect("/dne/test").ShouldNot(BeAnExistingFile())

			tmpFile, err := os.CreateTemp("", "gomega-test-tempfile")
			Expect(err).ShouldNot(HaveOccurred())
			defer os.Remove(tmpFile.Name())
			Expect(tmpFile.Name()).Should(BeAnExistingFile())

			tmpDir, err := gutil.MkdirTemp("", "gomega-test-tempdir")
			Expect(err).ShouldNot(HaveOccurred())
			defer os.Remove(tmpDir)
			Expect(tmpDir).Should(BeAnExistingFile())
		})
	})

	When("passed something else", func() {
		It("should error", func() {
			success, err := (&BeAnExistingFileMatcher{}).Match(nil)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())

			success, err = (&BeAnExistingFileMatcher{}).Match(true)
			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})

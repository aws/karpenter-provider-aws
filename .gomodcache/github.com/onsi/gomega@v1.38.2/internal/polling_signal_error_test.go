package internal_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/internal"
)

var _ = Describe("PollingSignalError", func() {
	Describe("StopTrying", func() {
		Describe("building StopTrying errors", func() {
			It("returns a correctly configured StopTrying error", func() {
				st := StopTrying("I've tried 17 times - give up!")
				Ω(st.Error()).Should(Equal("I've tried 17 times - give up!"))
				Ω(errors.Unwrap(st)).Should(BeNil())
				Ω(st.(*internal.PollingSignalErrorImpl).IsStopTrying()).Should(BeTrue())
			})
		})

		Describe("Wrapping other errors", func() {
			It("can wrap other errors", func() {
				st := StopTrying("Welp! Time to give up")
				Ω(st.Error()).Should(Equal("Welp! Time to give up"))
				st = st.Wrap(fmt.Errorf("ERR_GIVE_UP"))
				Ω(errors.Unwrap(st)).Should(Equal(fmt.Errorf("ERR_GIVE_UP")))
				Ω(st.Error()).Should(Equal("Welp! Time to give up: ERR_GIVE_UP"))
			})
		})

		Describe("When attaching objects", func() {
			It("attaches them, with their descriptions", func() {
				st := StopTrying("Welp!").Attach("Max retries attained", 17).Attach("Got this response", "FLOOP").(*internal.PollingSignalErrorImpl)
				Ω(st.Attachments).Should(HaveLen(2))
				Ω(st.Attachments[0]).Should(Equal(internal.PollingSignalErrorAttachment{"Max retries attained", 17}))
				Ω(st.Attachments[1]).Should(Equal(internal.PollingSignalErrorAttachment{"Got this response", "FLOOP"}))
			})
		})

		Describe("when invoking Now()", func() {
			It("should panic with itself", func() {
				st := StopTrying("bam").(*internal.PollingSignalErrorImpl)
				Ω(st.Now).Should(PanicWith(st))
			})
		})

		Describe("AsPollingSignalError", func() {
			It("should return false for nils", func() {
				st, ok := internal.AsPollingSignalError(nil)
				Ω(st).Should(BeNil())
				Ω(ok).Should(BeFalse())
			})

			It("should work when passed a StopTrying error", func() {
				st, ok := internal.AsPollingSignalError(StopTrying("bam"))
				Ω(st).Should(Equal(StopTrying("bam")))
				Ω(ok).Should(BeTrue())
			})

			It("should work when passed a wrapped error", func() {
				st, ok := internal.AsPollingSignalError(fmt.Errorf("STOP TRYING %w", StopTrying("bam")))
				Ω(st).Should(Equal(StopTrying("bam")))
				Ω(ok).Should(BeTrue())
			})
		})
	})
})

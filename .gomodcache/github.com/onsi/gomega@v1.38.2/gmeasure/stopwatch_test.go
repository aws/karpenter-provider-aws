package gmeasure_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
)

var _ = Describe("Stopwatch", func() {
	var e *gmeasure.Experiment
	var stopwatch *gmeasure.Stopwatch

	BeforeEach(func() {
		e = gmeasure.NewExperiment("My Test Experiment")
		stopwatch = e.NewStopwatch()
	})

	It("records durations", func() {
		time.Sleep(100 * time.Millisecond)
		stopwatch.Record("recordings", gmeasure.Annotation("A"))
		time.Sleep(100 * time.Millisecond)
		stopwatch.Record("recordings", gmeasure.Annotation("B")).Reset()
		time.Sleep(100 * time.Millisecond)
		stopwatch.Record("recordings", gmeasure.Annotation("C")).Reset()
		time.Sleep(100 * time.Millisecond)
		stopwatch.Pause()
		time.Sleep(100 * time.Millisecond)
		stopwatch.Resume()
		time.Sleep(100 * time.Millisecond)
		stopwatch.Pause()
		time.Sleep(100 * time.Millisecond)
		stopwatch.Resume()
		time.Sleep(100 * time.Millisecond)
		stopwatch.Record("recordings", gmeasure.Annotation("D"))
		durations := e.Get("recordings").Durations
		annotations := e.Get("recordings").Annotations
		Ω(annotations).Should(Equal([]string{"A", "B", "C", "D"}))
		Ω(durations[0]).Should(BeNumerically("~", 100*time.Millisecond, 50*time.Millisecond))
		Ω(durations[1]).Should(BeNumerically("~", 200*time.Millisecond, 50*time.Millisecond))
		Ω(durations[2]).Should(BeNumerically("~", 100*time.Millisecond, 50*time.Millisecond))
		Ω(durations[3]).Should(BeNumerically("~", 300*time.Millisecond, 50*time.Millisecond))

	})

	It("panics when asked to record but not running", func() {
		stopwatch.Pause()
		Ω(func() {
			stopwatch.Record("A")
		}).Should(PanicWith("stopwatch is not running - call Resume or Reset before calling Record"))
	})

	It("panics when paused but not running", func() {
		stopwatch.Pause()
		Ω(func() {
			stopwatch.Pause()
		}).Should(PanicWith("stopwatch is not running - call Resume or Reset before calling Pause"))
	})

	It("panics when asked to resume but not paused", func() {
		Ω(func() {
			stopwatch.Resume()
		}).Should(PanicWith("stopwatch is running - call Pause before calling Resume"))
	})
})

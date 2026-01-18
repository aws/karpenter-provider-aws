package gmeasure_test

import (
	"fmt"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gmeasure"
)

var _ = Describe("Experiment", func() {
	var e *gmeasure.Experiment
	BeforeEach(func() {
		e = gmeasure.NewExperiment("Test Experiment")
	})

	Describe("Recording Notes", func() {
		It("creates a note Measurement", func() {
			e.RecordNote("I'm a note", gmeasure.Style("{{blue}}"))
			measurement := e.Measurements[0]
			Ω(measurement.Type).Should(Equal(gmeasure.MeasurementTypeNote))
			Ω(measurement.ExperimentName).Should(Equal("Test Experiment"))
			Ω(measurement.Note).Should(Equal("I'm a note"))
			Ω(measurement.Style).Should(Equal("{{blue}}"))
		})
	})

	Describe("Recording Durations", func() {
		commonMeasurementAssertions := func() gmeasure.Measurement {
			measurement := e.Get("runtime")
			Ω(measurement.Type).Should(Equal(gmeasure.MeasurementTypeDuration))
			Ω(measurement.ExperimentName).Should(Equal("Test Experiment"))
			Ω(measurement.Name).Should(Equal("runtime"))
			Ω(measurement.Units).Should(Equal("duration"))
			Ω(measurement.Style).Should(Equal("{{red}}"))
			Ω(measurement.PrecisionBundle.Duration).Should(Equal(time.Millisecond))
			return measurement
		}

		BeforeEach(func() {
			e.RecordDuration("runtime", time.Second, gmeasure.Annotation("first"), gmeasure.Style("{{red}}"), gmeasure.Precision(time.Millisecond), gmeasure.Units("ignored"))
		})

		Describe("RecordDuration", func() {
			It("generates a measurement and records the passed-in duration along with any relevant decorations", func() {
				e.RecordDuration("runtime", time.Minute, gmeasure.Annotation("second"))
				measurement := commonMeasurementAssertions()
				Ω(measurement.Durations).Should(Equal([]time.Duration{time.Second, time.Minute}))
				Ω(measurement.Annotations).Should(Equal([]string{"first", "second"}))
			})
		})

		Describe("MeasureDuration", func() {
			It("measure the duration of the passed-in function", func() {
				e.MeasureDuration("runtime", func() {
					time.Sleep(200 * time.Millisecond)
				}, gmeasure.Annotation("second"))
				measurement := commonMeasurementAssertions()
				Ω(measurement.Durations[0]).Should(Equal(time.Second))
				Ω(measurement.Durations[1]).Should(BeNumerically("~", 200*time.Millisecond, 20*time.Millisecond))
				Ω(measurement.Annotations).Should(Equal([]string{"first", "second"}))
			})
		})

		Describe("SampleDuration", func() {
			It("samples the passed-in function according to SampleConfig and records the measured durations", func() {
				e.SampleDuration("runtime", func(_ int) {
					time.Sleep(100 * time.Millisecond)
				}, gmeasure.SamplingConfig{N: 3}, gmeasure.Annotation("sampled"))
				measurement := commonMeasurementAssertions()
				Ω(measurement.Durations[0]).Should(Equal(time.Second))
				Ω(measurement.Durations[1]).Should(BeNumerically("~", 100*time.Millisecond, 20*time.Millisecond))
				Ω(measurement.Durations[2]).Should(BeNumerically("~", 100*time.Millisecond, 20*time.Millisecond))
				Ω(measurement.Durations[3]).Should(BeNumerically("~", 100*time.Millisecond, 20*time.Millisecond))
				Ω(measurement.Annotations).Should(Equal([]string{"first", "sampled", "sampled", "sampled"}))
			})
		})

		Describe("SampleAnnotatedDuration", func() {
			It("samples the passed-in function according to SampleConfig and records the measured durations and returned annotations", func() {
				e.SampleAnnotatedDuration("runtime", func(idx int) gmeasure.Annotation {
					time.Sleep(100 * time.Millisecond)
					return gmeasure.Annotation(fmt.Sprintf("sampled-%d", idx+1))
				}, gmeasure.SamplingConfig{N: 3}, gmeasure.Annotation("ignored"))
				measurement := commonMeasurementAssertions()
				Ω(measurement.Durations[0]).Should(Equal(time.Second))
				Ω(measurement.Durations[1]).Should(BeNumerically("~", 100*time.Millisecond, 20*time.Millisecond))
				Ω(measurement.Durations[2]).Should(BeNumerically("~", 100*time.Millisecond, 20*time.Millisecond))
				Ω(measurement.Durations[3]).Should(BeNumerically("~", 100*time.Millisecond, 20*time.Millisecond))
				Ω(measurement.Annotations).Should(Equal([]string{"first", "sampled-1", "sampled-2", "sampled-3"}))
			})
		})
	})

	Describe("Stopwatch Support", func() {
		It("can generate a new stopwatch tied to the experiment", func() {
			s := e.NewStopwatch()
			time.Sleep(50 * time.Millisecond)
			s.Record("runtime", gmeasure.Annotation("first")).Reset()
			time.Sleep(100 * time.Millisecond)
			s.Record("runtime", gmeasure.Annotation("second")).Reset()
			time.Sleep(150 * time.Millisecond)
			s.Record("runtime", gmeasure.Annotation("third"))
			measurement := e.Get("runtime")
			Ω(measurement.Durations[0]).Should(BeNumerically("~", 50*time.Millisecond, 20*time.Millisecond))
			Ω(measurement.Durations[1]).Should(BeNumerically("~", 100*time.Millisecond, 20*time.Millisecond))
			Ω(measurement.Durations[2]).Should(BeNumerically("~", 150*time.Millisecond, 20*time.Millisecond))
			Ω(measurement.Annotations).Should(Equal([]string{"first", "second", "third"}))
		})
	})

	Describe("Recording Values", func() {
		commonMeasurementAssertions := func() gmeasure.Measurement {
			measurement := e.Get("sprockets")
			Ω(measurement.Type).Should(Equal(gmeasure.MeasurementTypeValue))
			Ω(measurement.ExperimentName).Should(Equal("Test Experiment"))
			Ω(measurement.Name).Should(Equal("sprockets"))
			Ω(measurement.Units).Should(Equal("widgets"))
			Ω(measurement.Style).Should(Equal("{{yellow}}"))
			Ω(measurement.PrecisionBundle.ValueFormat).Should(Equal("%.0f"))
			return measurement
		}

		BeforeEach(func() {
			e.RecordValue("sprockets", 3.2, gmeasure.Annotation("first"), gmeasure.Style("{{yellow}}"), gmeasure.Precision(0), gmeasure.Units("widgets"))
		})

		Describe("RecordValue", func() {
			It("generates a measurement and records the passed-in value along with any relevant decorations", func() {
				e.RecordValue("sprockets", 17.4, gmeasure.Annotation("second"))
				measurement := commonMeasurementAssertions()
				Ω(measurement.Values).Should(Equal([]float64{3.2, 17.4}))
				Ω(measurement.Annotations).Should(Equal([]string{"first", "second"}))
			})
		})

		Describe("MeasureValue", func() {
			It("records the value returned by the passed-in function", func() {
				e.MeasureValue("sprockets", func() float64 {
					return 17.4
				}, gmeasure.Annotation("second"))
				measurement := commonMeasurementAssertions()
				Ω(measurement.Values).Should(Equal([]float64{3.2, 17.4}))
				Ω(measurement.Annotations).Should(Equal([]string{"first", "second"}))
			})
		})

		Describe("SampleValue", func() {
			It("samples the passed-in function according to SampleConfig and records the resulting values", func() {
				e.SampleValue("sprockets", func(idx int) float64 {
					return 17.4 + float64(idx)
				}, gmeasure.SamplingConfig{N: 3}, gmeasure.Annotation("sampled"))
				measurement := commonMeasurementAssertions()
				Ω(measurement.Values).Should(Equal([]float64{3.2, 17.4, 18.4, 19.4}))
				Ω(measurement.Annotations).Should(Equal([]string{"first", "sampled", "sampled", "sampled"}))
			})
		})

		Describe("SampleAnnotatedValue", func() {
			It("samples the passed-in function according to SampleConfig and records the returned values and annotations", func() {
				e.SampleAnnotatedValue("sprockets", func(idx int) (float64, gmeasure.Annotation) {
					return 17.4 + float64(idx), gmeasure.Annotation(fmt.Sprintf("sampled-%d", idx+1))
				}, gmeasure.SamplingConfig{N: 3}, gmeasure.Annotation("ignored"))
				measurement := commonMeasurementAssertions()
				Ω(measurement.Values).Should(Equal([]float64{3.2, 17.4, 18.4, 19.4}))
				Ω(measurement.Annotations).Should(Equal([]string{"first", "sampled-1", "sampled-2", "sampled-3"}))
			})
		})
	})

	Describe("Sampling", func() {
		var indices []int
		BeforeEach(func() {
			indices = []int{}
		})

		ints := func(n int) []int {
			out := []int{}
			for i := 0; i < n; i++ {
				out = append(out, i)
			}
			return out
		}

		It("calls the function repeatedly passing in an index", func() {
			e.Sample(func(idx int) {
				indices = append(indices, idx)
			}, gmeasure.SamplingConfig{N: 3})

			Ω(indices).Should(Equal(ints(3)))
		})

		It("can cap the maximum number of samples", func() {
			e.Sample(func(idx int) {
				indices = append(indices, idx)
			}, gmeasure.SamplingConfig{N: 10, Duration: time.Minute})

			Ω(indices).Should(Equal(ints(10)))
		})

		It("can cap the maximum sample time", func() {
			e.Sample(func(idx int) {
				indices = append(indices, idx)
				time.Sleep(10 * time.Millisecond)
			}, gmeasure.SamplingConfig{N: 100, Duration: 100 * time.Millisecond, MinSamplingInterval: 5 * time.Millisecond})

			Ω(len(indices)).Should(BeNumerically("~", 10, 3))
			Ω(indices).Should(Equal(ints(len(indices))))
		})

		It("can ensure a minimum interval between samples", func() {
			times := map[int]time.Time{}
			e.Sample(func(idx int) {
				times[idx] = time.Now()
			}, gmeasure.SamplingConfig{N: 10, Duration: 200 * time.Millisecond, MinSamplingInterval: 50 * time.Millisecond, NumParallel: 1})

			Ω(len(times)).Should(BeNumerically("~", 4, 2))
			Ω(times[1]).Should(BeTemporally(">", times[0], 50*time.Millisecond))
			Ω(times[2]).Should(BeTemporally(">", times[1], 50*time.Millisecond))
		})

		It("can run samples in parallel", func() {
			lock := &sync.Mutex{}

			e.Sample(func(idx int) {
				lock.Lock()
				indices = append(indices, idx)
				lock.Unlock()
				time.Sleep(10 * time.Millisecond)
			}, gmeasure.SamplingConfig{N: 100, Duration: 100 * time.Millisecond, NumParallel: 3})

			lock.Lock()
			defer lock.Unlock()
			Ω(len(indices)).Should(BeNumerically("~", 30, 10))
			Ω(indices).Should(ConsistOf(ints(len(indices))))
		})

		It("panics if the SamplingConfig does not specify a ceiling", func() {
			Expect(func() {
				e.Sample(func(_ int) {}, gmeasure.SamplingConfig{MinSamplingInterval: time.Second})
			}).To(PanicWith("you must specify at least one of SamplingConfig.N and SamplingConfig.Duration"))
		})

		It("panics if the SamplingConfig includes both a minimum interval and a directive to run in parallel", func() {
			Expect(func() {
				e.Sample(func(_ int) {}, gmeasure.SamplingConfig{N: 10, MinSamplingInterval: time.Second, NumParallel: 2})
			}).To(PanicWith("you cannot specify both SamplingConfig.MinSamplingInterval and SamplingConfig.NumParallel"))
		})
	})

	Describe("recording multiple entries", func() {
		It("always appends to the correct measurement (by name)", func() {
			e.RecordDuration("alpha", time.Second)
			e.RecordDuration("beta", time.Minute)
			e.RecordValue("gamma", 1)
			e.RecordValue("delta", 2.71)
			e.RecordDuration("alpha", 2*time.Second)
			e.RecordDuration("beta", 2*time.Minute)
			e.RecordValue("gamma", 2)
			e.RecordValue("delta", 3.141)

			Ω(e.Measurements).Should(HaveLen(4))
			Ω(e.Get("alpha").Durations).Should(Equal([]time.Duration{time.Second, 2 * time.Second}))
			Ω(e.Get("beta").Durations).Should(Equal([]time.Duration{time.Minute, 2 * time.Minute}))
			Ω(e.Get("gamma").Values).Should(Equal([]float64{1, 2}))
			Ω(e.Get("delta").Values).Should(Equal([]float64{2.71, 3.141}))
		})

		It("panics if you incorrectly mix types", func() {
			e.RecordDuration("runtime", time.Second)
			Ω(func() {
				e.RecordValue("runtime", 3.141)
			}).Should(PanicWith("attempting to record value with name 'runtime'.  That name is already in-use for recording durations."))

			e.RecordValue("sprockets", 2)
			Ω(func() {
				e.RecordDuration("sprockets", time.Minute)
			}).Should(PanicWith("attempting to record duration with name 'sprockets'.  That name is already in-use for recording values."))
		})
	})

	Describe("Decorators", func() {
		It("uses the default precisions when none is specified", func() {
			e.RecordValue("sprockets", 2)
			e.RecordDuration("runtime", time.Minute)

			Ω(e.Get("sprockets").PrecisionBundle.ValueFormat).Should(Equal("%.3f"))
			Ω(e.Get("runtime").PrecisionBundle.Duration).Should(Equal(100 * time.Microsecond))
		})

		It("panics if an unsupported type is passed into Precision", func() {
			Ω(func() {
				gmeasure.Precision("aardvark")
			}).Should(PanicWith("invalid precision type, must be time.Duration or int"))
		})

		It("panics if an unrecognized argument is passed in", func() {
			Ω(func() {
				e.RecordValue("sprockets", 2, "boom")
			}).Should(PanicWith(`unrecognized argument "boom"`))
		})
	})

	Describe("Getting Measurements", func() {
		Context("when the Measurement does not exist", func() {
			It("returns the zero Measurement", func() {
				Ω(e.Get("not here")).Should(BeZero())
			})
		})
	})

	Describe("Getting Stats", func() {
		It("returns the Measurement's Stats", func() {
			e.RecordValue("alpha", 1)
			e.RecordValue("alpha", 2)
			e.RecordValue("alpha", 3)
			Ω(e.GetStats("alpha")).Should(Equal(e.Get("alpha").Stats()))
		})
	})

	Describe("Generating Reports", func() {
		BeforeEach(func() {
			e.RecordNote("A note")
			e.RecordValue("sprockets", 7, gmeasure.Units("widgets"), gmeasure.Precision(0), gmeasure.Style("{{yellow}}"), gmeasure.Annotation("sprockets-1"))
			e.RecordDuration("runtime", time.Second, gmeasure.Precision(100*time.Millisecond), gmeasure.Style("{{red}}"), gmeasure.Annotation("runtime-1"))
			e.RecordNote("A blue note", gmeasure.Style("{{blue}}"))
			e.RecordValue("gear ratio", 10.3, gmeasure.Precision(2), gmeasure.Style("{{green}}"), gmeasure.Annotation("ratio-1"))

			e.RecordValue("sprockets", 8, gmeasure.Annotation("sprockets-2"))
			e.RecordValue("sprockets", 9, gmeasure.Annotation("sprockets-3"))

			e.RecordDuration("runtime", 2*time.Second, gmeasure.Annotation("runtime-2"))
			e.RecordValue("gear ratio", 13.758, gmeasure.Precision(2), gmeasure.Annotation("ratio-2"))
		})

		It("emits a nicely formatted table", func() {
			expected := strings.Join([]string{
				"Test Experiment",
				"Name                | N | Min         | Median | Mean  | StdDev | Max        ",
				"=============================================================================",
				"A note                                                                       ",
				"-----------------------------------------------------------------------------",
				"sprockets [widgets] | 3 | 7           | 8      | 8     | 1      | 9          ",
				"                    |   | sprockets-1 |        |       |        | sprockets-3",
				"-----------------------------------------------------------------------------",
				"runtime [duration]  | 2 | 1s          | 1.5s   | 1.5s  | 500ms  | 2s         ",
				"                    |   | runtime-1   |        |       |        | runtime-2  ",
				"-----------------------------------------------------------------------------",
				"A blue note                                                                  ",
				"-----------------------------------------------------------------------------",
				"gear ratio          | 2 | 10.30       | 12.03  | 12.03 | 1.73   | 13.76      ",
				"                    |   | ratio-1     |        |       |        | ratio-2    ",
				"",
			}, "\n")
			Ω(e.String()).Should(Equal(expected))
		})

		It("can also emit a styled table", func() {
			expected := strings.Join([]string{
				"{{bold}}Test Experiment",
				"{{/}}{{bold}}Name               {{/}} | {{bold}}N{{/}} | {{bold}}Min        {{/}} | {{bold}}Median{{/}} | {{bold}}Mean {{/}} | {{bold}}StdDev{{/}} | {{bold}}Max        {{/}}",
				"=============================================================================",
				"A note                                                                       ",
				"-----------------------------------------------------------------------------",
				"{{yellow}}sprockets [widgets]{{/}} | {{yellow}}3{{/}} | {{yellow}}7          {{/}} | {{yellow}}8     {{/}} | {{yellow}}8    {{/}} | {{yellow}}1     {{/}} | {{yellow}}9          {{/}}",
				"                    |   | {{yellow}}sprockets-1{{/}} |        |       |        | {{yellow}}sprockets-3{{/}}",
				"-----------------------------------------------------------------------------",
				"{{red}}runtime [duration] {{/}} | {{red}}2{{/}} | {{red}}1s         {{/}} | {{red}}1.5s  {{/}} | {{red}}1.5s {{/}} | {{red}}500ms {{/}} | {{red}}2s         {{/}}",
				"                    |   | {{red}}runtime-1  {{/}} |        |       |        | {{red}}runtime-2  {{/}}",
				"-----------------------------------------------------------------------------",
				"{{blue}}A blue note                                                                  {{/}}",
				"-----------------------------------------------------------------------------",
				"{{green}}gear ratio         {{/}} | {{green}}2{{/}} | {{green}}10.30      {{/}} | {{green}}12.03 {{/}} | {{green}}12.03{{/}} | {{green}}1.73  {{/}} | {{green}}13.76      {{/}}",
				"                    |   | {{green}}ratio-1    {{/}} |        |       |        | {{green}}ratio-2    {{/}}",
				"",
			}, "\n")
			Ω(e.ColorableString()).Should(Equal(expected))
		})
	})
})

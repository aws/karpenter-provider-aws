package gmeasure_test

import (
	"math"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
)

var _ = Describe("Measurement", func() {
	var e *gmeasure.Experiment
	var measurement gmeasure.Measurement

	BeforeEach(func() {
		e = gmeasure.NewExperiment("Test Experiment")
	})

	Describe("Note Measurement", func() {
		BeforeEach(func() {
			e.RecordNote("I'm a red note", gmeasure.Style("{{red}}"))
			measurement = e.Measurements[0]
		})

		Describe("Generating Stats", func() {
			It("returns an empty stats", func() {
				Ω(measurement.Stats()).Should(BeZero())
			})
		})

		Describe("Emitting an unstyled report", func() {
			It("does not include styling", func() {
				Ω(measurement.String()).Should(Equal("Test Experiment - Note\nI'm a red note\n"))
			})
		})

		Describe("Emitting a styled report", func() {
			It("does include styling", func() {
				Ω(measurement.ColorableString()).Should(Equal("{{red}}Test Experiment - Note\nI'm a red note\n{{/}}"))
			})
		})
	})

	Describe("Value Measurement", func() {
		var min, median, mean, stdDev, max float64
		BeforeEach(func() {
			e.RecordValue("flange widths", 7.128, gmeasure.Annotation("A"), gmeasure.Precision(2), gmeasure.Units("inches"), gmeasure.Style("{{blue}}"))
			e.RecordValue("flange widths", 3.141, gmeasure.Annotation("B"))
			e.RecordValue("flange widths", 9.28223, gmeasure.Annotation("C"))
			e.RecordValue("flange widths", 14.249, gmeasure.Annotation("D"))
			e.RecordValue("flange widths", 8.975, gmeasure.Annotation("E"))
			measurement = e.Measurements[0]
			min = 3.141
			max = 14.249
			median = 8.975
			mean = (7.128 + 3.141 + 9.28223 + 14.249 + 8.975) / 5.0
			stdDev = (7.128-mean)*(7.128-mean) + (3.141-mean)*(3.141-mean) + (9.28223-mean)*(9.28223-mean) + (14.249-mean)*(14.249-mean) + (8.975-mean)*(8.975-mean)
			stdDev = math.Sqrt(stdDev / 5.0)
		})

		Describe("Generating Stats", func() {
			It("generates a correctly configured Stats with correct values", func() {
				stats := measurement.Stats()
				Ω(stats.ExperimentName).Should(Equal("Test Experiment"))
				Ω(stats.MeasurementName).Should(Equal("flange widths"))
				Ω(stats.Style).Should(Equal("{{blue}}"))
				Ω(stats.Units).Should(Equal("inches"))
				Ω(stats.PrecisionBundle.ValueFormat).Should(Equal("%.2f"))

				Ω(stats.ValueBundle[gmeasure.StatMin]).Should(Equal(min))
				Ω(stats.AnnotationBundle[gmeasure.StatMin]).Should(Equal("B"))
				Ω(stats.ValueBundle[gmeasure.StatMax]).Should(Equal(max))
				Ω(stats.AnnotationBundle[gmeasure.StatMax]).Should(Equal("D"))
				Ω(stats.ValueBundle[gmeasure.StatMedian]).Should(Equal(median))
				Ω(stats.ValueBundle[gmeasure.StatMean]).Should(Equal(mean))
				Ω(stats.ValueBundle[gmeasure.StatStdDev]).Should(BeNumerically("~", stdDev))
			})
		})

		Describe("Emitting an unstyled report", func() {
			It("does not include styling", func() {
				expected := strings.Join([]string{
					"Test Experiment - flange widths [inches]",
					"3.14 < [8.97] | <8.56> ±3.59 < 14.25",
					"Value | Annotation",
					"==================",
					" 7.13 | A         ",
					"------------------",
					" 3.14 | B         ",
					"------------------",
					" 9.28 | C         ",
					"------------------",
					"14.25 | D         ",
					"------------------",
					" 8.97 | E         ",
					"",
				}, "\n")
				Ω(measurement.String()).Should(Equal(expected))
			})
		})

		Describe("Emitting a styled report", func() {
			It("does include styling", func() {
				expected := strings.Join([]string{
					"{{blue}}Test Experiment - flange widths [inches]{{/}}",
					"3.14 < [8.97] | <8.56> ±3.59 < 14.25",
					"{{blue}}Value{{/}} | {{blue}}Annotation{{/}}",
					"==================",
					" 7.13 | {{gray}}A         {{/}}",
					"------------------",
					" 3.14 | {{gray}}B         {{/}}",
					"------------------",
					" 9.28 | {{gray}}C         {{/}}",
					"------------------",
					"14.25 | {{gray}}D         {{/}}",
					"------------------",
					" 8.97 | {{gray}}E         {{/}}",
					"",
				}, "\n")
				Ω(measurement.ColorableString()).Should(Equal(expected))
			})
		})

		Describe("Computing medians", func() {
			Context("with an odd number of values", func() {
				It("returns the middle element", func() {
					e.RecordValue("odd", 5)
					e.RecordValue("odd", 1)
					e.RecordValue("odd", 2)
					e.RecordValue("odd", 4)
					e.RecordValue("odd", 3)

					Ω(e.GetStats("odd").ValueBundle[gmeasure.StatMedian]).Should(Equal(3.0))
				})
			})

			Context("when an even number of values", func() {
				It("returns the mean of the two middle elements", func() {
					e.RecordValue("even", 1)
					e.RecordValue("even", 2)
					e.RecordValue("even", 4)
					e.RecordValue("even", 3)

					Ω(e.GetStats("even").ValueBundle[gmeasure.StatMedian]).Should(Equal(2.5))
				})
			})
		})
	})

	Describe("Duration Measurement", func() {
		var min, median, mean, stdDev, max time.Duration
		BeforeEach(func() {
			e.RecordDuration("runtime", 7128*time.Millisecond, gmeasure.Annotation("A"), gmeasure.Precision(time.Millisecond*100), gmeasure.Style("{{blue}}"))
			e.RecordDuration("runtime", 3141*time.Millisecond, gmeasure.Annotation("B"))
			e.RecordDuration("runtime", 9282*time.Millisecond, gmeasure.Annotation("C"))
			e.RecordDuration("runtime", 14249*time.Millisecond, gmeasure.Annotation("D"))
			e.RecordDuration("runtime", 8975*time.Millisecond, gmeasure.Annotation("E"))
			measurement = e.Measurements[0]
			min = 3141 * time.Millisecond
			max = 14249 * time.Millisecond
			median = 8975 * time.Millisecond
			mean = ((7128 + 3141 + 9282 + 14249 + 8975) * time.Millisecond) / 5
			stdDev = time.Duration(math.Sqrt((float64(7128*time.Millisecond-mean)*float64(7128*time.Millisecond-mean) + float64(3141*time.Millisecond-mean)*float64(3141*time.Millisecond-mean) + float64(9282*time.Millisecond-mean)*float64(9282*time.Millisecond-mean) + float64(14249*time.Millisecond-mean)*float64(14249*time.Millisecond-mean) + float64(8975*time.Millisecond-mean)*float64(8975*time.Millisecond-mean)) / 5.0))
		})

		Describe("Generating Stats", func() {
			It("generates a correctly configured Stats with correct values", func() {
				stats := measurement.Stats()
				Ω(stats.ExperimentName).Should(Equal("Test Experiment"))
				Ω(stats.MeasurementName).Should(Equal("runtime"))
				Ω(stats.Style).Should(Equal("{{blue}}"))
				Ω(stats.Units).Should(Equal("duration"))
				Ω(stats.PrecisionBundle.Duration).Should(Equal(time.Millisecond * 100))

				Ω(stats.DurationBundle[gmeasure.StatMin]).Should(Equal(min))
				Ω(stats.AnnotationBundle[gmeasure.StatMin]).Should(Equal("B"))
				Ω(stats.DurationBundle[gmeasure.StatMax]).Should(Equal(max))
				Ω(stats.AnnotationBundle[gmeasure.StatMax]).Should(Equal("D"))
				Ω(stats.DurationBundle[gmeasure.StatMedian]).Should(Equal(median))
				Ω(stats.DurationBundle[gmeasure.StatMean]).Should(Equal(mean))
				Ω(stats.DurationBundle[gmeasure.StatStdDev]).Should(Equal(stdDev))
			})
		})

		Describe("Emitting an unstyled report", func() {
			It("does not include styling", func() {
				expected := strings.Join([]string{
					"Test Experiment - runtime [duration]",
					"3.1s < [9s] | <8.6s> ±3.6s < 14.2s",
					"Duration | Annotation",
					"=====================",
					"    7.1s | A         ",
					"---------------------",
					"    3.1s | B         ",
					"---------------------",
					"    9.3s | C         ",
					"---------------------",
					"   14.2s | D         ",
					"---------------------",
					"      9s | E         ",
					"",
				}, "\n")
				Ω(measurement.String()).Should(Equal(expected))
			})
		})

		Describe("Emitting a styled report", func() {
			It("does include styling", func() {
				expected := strings.Join([]string{
					"{{blue}}Test Experiment - runtime [duration]{{/}}",
					"3.1s < [9s] | <8.6s> ±3.6s < 14.2s",
					"{{blue}}Duration{{/}} | {{blue}}Annotation{{/}}",
					"=====================",
					"{{blue}}    7.1s{{/}} | {{gray}}A         {{/}}",
					"---------------------",
					"{{blue}}    3.1s{{/}} | {{gray}}B         {{/}}",
					"---------------------",
					"{{blue}}    9.3s{{/}} | {{gray}}C         {{/}}",
					"---------------------",
					"{{blue}}   14.2s{{/}} | {{gray}}D         {{/}}",
					"---------------------",
					"{{blue}}      9s{{/}} | {{gray}}E         {{/}}",
					"",
				}, "\n")
				Ω(measurement.ColorableString()).Should(Equal(expected))
			})
		})

		Describe("Computing medians", func() {
			Context("with an odd number of values", func() {
				It("returns the middle element", func() {
					e.RecordDuration("odd", 5*time.Second)
					e.RecordDuration("odd", 1*time.Second)
					e.RecordDuration("odd", 2*time.Second)
					e.RecordDuration("odd", 4*time.Second)
					e.RecordDuration("odd", 3*time.Second)

					Ω(e.GetStats("odd").DurationBundle[gmeasure.StatMedian]).Should(Equal(3 * time.Second))
				})
			})

			Context("when an even number of values", func() {
				It("returns the mean of the two middle elements", func() {
					e.RecordDuration("even", 1*time.Second)
					e.RecordDuration("even", 2*time.Second)
					e.RecordDuration("even", 4*time.Second)
					e.RecordDuration("even", 3*time.Second)

					Ω(e.GetStats("even").DurationBundle[gmeasure.StatMedian]).Should(Equal(2500 * time.Millisecond))
				})
			})
		})
	})
})

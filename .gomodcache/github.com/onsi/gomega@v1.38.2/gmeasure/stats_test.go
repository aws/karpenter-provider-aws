package gmeasure_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
)

var _ = Describe("Stats", func() {
	var stats gmeasure.Stats

	Describe("Stats representing values", func() {
		BeforeEach(func() {
			stats = gmeasure.Stats{
				Type:            gmeasure.StatsTypeValue,
				ExperimentName:  "My Test Experiment",
				MeasurementName: "Sprockets",
				Units:           "widgets",
				N:               100,
				PrecisionBundle: gmeasure.Precision(2),
				ValueBundle: map[gmeasure.Stat]float64{
					gmeasure.StatMin:    17.48992,
					gmeasure.StatMax:    293.4820,
					gmeasure.StatMean:   187.3023,
					gmeasure.StatMedian: 87.2235,
					gmeasure.StatStdDev: 73.6394,
				},
			}
		})

		Describe("String()", func() {
			It("returns a one-line summary", func() {
				Ω(stats.String()).Should(Equal("17.49 < [87.22] | <187.30> ±73.64 < 293.48"))
			})
		})

		Describe("ValueFor()", func() {
			It("returns the value for the requested stat", func() {
				Ω(stats.ValueFor(gmeasure.StatMin)).Should(Equal(17.48992))
				Ω(stats.ValueFor(gmeasure.StatMean)).Should(Equal(187.3023))
			})
		})

		Describe("FloatFor", func() {
			It("returns the requested stat as a float", func() {
				Ω(stats.FloatFor(gmeasure.StatMin)).Should(Equal(17.48992))
				Ω(stats.FloatFor(gmeasure.StatMean)).Should(Equal(187.3023))
			})
		})

		Describe("StringFor", func() {
			It("returns the requested stat rendered with the configured precision", func() {
				Ω(stats.StringFor(gmeasure.StatMin)).Should(Equal("17.49"))
				Ω(stats.StringFor(gmeasure.StatMean)).Should(Equal("187.30"))
			})
		})
	})

	Describe("Stats representing durations", func() {
		BeforeEach(func() {
			stats = gmeasure.Stats{
				Type:            gmeasure.StatsTypeDuration,
				ExperimentName:  "My Test Experiment",
				MeasurementName: "Runtime",
				N:               100,
				PrecisionBundle: gmeasure.Precision(time.Millisecond * 100),
				DurationBundle: map[gmeasure.Stat]time.Duration{
					gmeasure.StatMin:    17375 * time.Millisecond,
					gmeasure.StatMax:    890321 * time.Millisecond,
					gmeasure.StatMean:   328712 * time.Millisecond,
					gmeasure.StatMedian: 552390 * time.Millisecond,
					gmeasure.StatStdDev: 186259 * time.Millisecond,
				},
			}
		})
		Describe("String()", func() {
			It("returns a one-line summary", func() {
				Ω(stats.String()).Should(Equal("17.4s < [9m12.4s] | <5m28.7s> ±3m6.3s < 14m50.3s"))
			})
		})
		Describe("DurationFor()", func() {
			It("returns the duration for the requested stat", func() {
				Ω(stats.DurationFor(gmeasure.StatMin)).Should(Equal(17375 * time.Millisecond))
				Ω(stats.DurationFor(gmeasure.StatMean)).Should(Equal(328712 * time.Millisecond))
			})
		})

		Describe("FloatFor", func() {
			It("returns the float64 representation for the requested duration stat", func() {
				Ω(stats.FloatFor(gmeasure.StatMin)).Should(Equal(float64(17375 * time.Millisecond)))
				Ω(stats.FloatFor(gmeasure.StatMean)).Should(Equal(float64(328712 * time.Millisecond)))
			})
		})

		Describe("StringFor", func() {
			It("returns the requested stat rendered with the configured precision", func() {
				Ω(stats.StringFor(gmeasure.StatMin)).Should(Equal("17.4s"))
				Ω(stats.StringFor(gmeasure.StatMean)).Should(Equal("5m28.7s"))
			})
		})
	})
})

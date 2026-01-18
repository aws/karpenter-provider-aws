package gmeasure_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
)

var _ = Describe("Rank", func() {
	var A, B, C, D gmeasure.Stats

	Describe("Ranking Values", func() {
		makeStats := func(name string, min float64, max float64, mean float64, median float64) gmeasure.Stats {
			return gmeasure.Stats{
				Type:            gmeasure.StatsTypeValue,
				ExperimentName:  "Exp-" + name,
				MeasurementName: name,
				N:               100,
				PrecisionBundle: gmeasure.Precision(2),
				ValueBundle: map[gmeasure.Stat]float64{
					gmeasure.StatMin:    min,
					gmeasure.StatMax:    max,
					gmeasure.StatMean:   mean,
					gmeasure.StatMedian: median,
					gmeasure.StatStdDev: 2.0,
				},
			}
		}

		BeforeEach(func() {
			A = makeStats("A", 1, 2, 3, 4)
			B = makeStats("B", 2, 3, 4, 1)
			C = makeStats("C", 3, 4, 1, 2)
			D = makeStats("D", 4, 1, 2, 3)
		})

		DescribeTable("ranking by criteria",
			func(criteria gmeasure.RankingCriteria, expectedOrder func() []gmeasure.Stats) {
				ranking := gmeasure.RankStats(criteria, A, B, C, D)
				expected := expectedOrder()
				Ω(ranking.Winner()).Should(Equal(expected[0]))
				Ω(ranking.Stats).Should(Equal(expected))
			},
			Entry("entry", gmeasure.LowerMeanIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{C, D, A, B} }),
			Entry("entry", gmeasure.HigherMeanIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{B, A, D, C} }),
			Entry("entry", gmeasure.LowerMedianIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{B, C, D, A} }),
			Entry("entry", gmeasure.HigherMedianIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{A, D, C, B} }),
			Entry("entry", gmeasure.LowerMinIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{A, B, C, D} }),
			Entry("entry", gmeasure.HigherMinIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{D, C, B, A} }),
			Entry("entry", gmeasure.LowerMaxIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{D, A, B, C} }),
			Entry("entry", gmeasure.HigherMaxIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{C, B, A, D} }),
		)

		Describe("Generating Reports", func() {
			It("can generate an unstyled report", func() {
				ranking := gmeasure.RankStats(gmeasure.LowerMeanIsBetter, A, B, C, D)
				Ω(ranking.String()).Should(Equal(strings.Join([]string{
					"Ranking Criteria: Lower Mean is Better",
					"Experiment | Name     | N   | Min  | Median | Mean | StdDev | Max ",
					"==================================================================",
					"Exp-C      | C        | 100 | 3.00 | 2.00   | 1.00 | 2.00   | 4.00",
					"*Winner*   | *Winner* |     |      |        |      |        |     ",
					"------------------------------------------------------------------",
					"Exp-D      | D        | 100 | 4.00 | 3.00   | 2.00 | 2.00   | 1.00",
					"------------------------------------------------------------------",
					"Exp-A      | A        | 100 | 1.00 | 4.00   | 3.00 | 2.00   | 2.00",
					"------------------------------------------------------------------",
					"Exp-B      | B        | 100 | 2.00 | 1.00   | 4.00 | 2.00   | 3.00",
					"",
				}, "\n")))
			})

			It("can generate a styled report", func() {
				ranking := gmeasure.RankStats(gmeasure.LowerMeanIsBetter, A, B, C, D)
				Ω(ranking.ColorableString()).Should(Equal(strings.Join([]string{
					"{{bold}}Ranking Criteria: Lower Mean is Better",
					"{{/}}{{bold}}Experiment{{/}} | {{bold}}Name    {{/}} | {{bold}}N  {{/}} | {{bold}}Min {{/}} | {{bold}}Median{{/}} | {{bold}}Mean{{/}} | {{bold}}StdDev{{/}} | {{bold}}Max {{/}}",
					"==================================================================",
					"{{bold}}Exp-C     {{/}} | {{bold}}C       {{/}} | {{bold}}100{{/}} | {{bold}}3.00{{/}} | {{bold}}2.00  {{/}} | {{bold}}1.00{{/}} | {{bold}}2.00  {{/}} | {{bold}}4.00{{/}}",
					"{{bold}}*Winner*  {{/}} | {{bold}}*Winner*{{/}} |     |      |        |      |        |     ",
					"------------------------------------------------------------------",
					"Exp-D      | D        | 100 | 4.00 | 3.00   | 2.00 | 2.00   | 1.00",
					"------------------------------------------------------------------",
					"Exp-A      | A        | 100 | 1.00 | 4.00   | 3.00 | 2.00   | 2.00",
					"------------------------------------------------------------------",
					"Exp-B      | B        | 100 | 2.00 | 1.00   | 4.00 | 2.00   | 3.00",
					"",
				}, "\n")))
			})
		})
	})

	Describe("Ranking Durations", func() {
		makeStats := func(name string, min time.Duration, max time.Duration, mean time.Duration, median time.Duration) gmeasure.Stats {
			return gmeasure.Stats{
				Type:            gmeasure.StatsTypeDuration,
				ExperimentName:  "Exp-" + name,
				MeasurementName: name,
				N:               100,
				PrecisionBundle: gmeasure.Precision(time.Millisecond * 100),
				DurationBundle: map[gmeasure.Stat]time.Duration{
					gmeasure.StatMin:    min,
					gmeasure.StatMax:    max,
					gmeasure.StatMean:   mean,
					gmeasure.StatMedian: median,
					gmeasure.StatStdDev: 2.0,
				},
			}
		}

		BeforeEach(func() {
			A = makeStats("A", 1*time.Second, 2*time.Second, 3*time.Second, 4*time.Second)
			B = makeStats("B", 2*time.Second, 3*time.Second, 4*time.Second, 1*time.Second)
			C = makeStats("C", 3*time.Second, 4*time.Second, 1*time.Second, 2*time.Second)
			D = makeStats("D", 4*time.Second, 1*time.Second, 2*time.Second, 3*time.Second)
		})

		DescribeTable("ranking by criteria",
			func(criteria gmeasure.RankingCriteria, expectedOrder func() []gmeasure.Stats) {
				ranking := gmeasure.RankStats(criteria, A, B, C, D)
				expected := expectedOrder()
				Ω(ranking.Winner()).Should(Equal(expected[0]))
				Ω(ranking.Stats).Should(Equal(expected))
			},
			Entry("entry", gmeasure.LowerMeanIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{C, D, A, B} }),
			Entry("entry", gmeasure.HigherMeanIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{B, A, D, C} }),
			Entry("entry", gmeasure.LowerMedianIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{B, C, D, A} }),
			Entry("entry", gmeasure.HigherMedianIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{A, D, C, B} }),
			Entry("entry", gmeasure.LowerMinIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{A, B, C, D} }),
			Entry("entry", gmeasure.HigherMinIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{D, C, B, A} }),
			Entry("entry", gmeasure.LowerMaxIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{D, A, B, C} }),
			Entry("entry", gmeasure.HigherMaxIsBetter, func() []gmeasure.Stats { return []gmeasure.Stats{C, B, A, D} }),
		)

		Describe("Generating Reports", func() {
			It("can generate an unstyled report", func() {
				ranking := gmeasure.RankStats(gmeasure.LowerMeanIsBetter, A, B, C, D)
				Ω(ranking.String()).Should(Equal(strings.Join([]string{
					"Ranking Criteria: Lower Mean is Better",
					"Experiment | Name     | N   | Min | Median | Mean | StdDev | Max",
					"================================================================",
					"Exp-C      | C        | 100 | 3s  | 2s     | 1s   | 0s     | 4s ",
					"*Winner*   | *Winner* |     |     |        |      |        |    ",
					"----------------------------------------------------------------",
					"Exp-D      | D        | 100 | 4s  | 3s     | 2s   | 0s     | 1s ",
					"----------------------------------------------------------------",
					"Exp-A      | A        | 100 | 1s  | 4s     | 3s   | 0s     | 2s ",
					"----------------------------------------------------------------",
					"Exp-B      | B        | 100 | 2s  | 1s     | 4s   | 0s     | 3s ",
					"",
				}, "\n")))
			})

			It("can generate a styled report", func() {
				ranking := gmeasure.RankStats(gmeasure.LowerMeanIsBetter, A, B, C, D)
				Ω(ranking.ColorableString()).Should(Equal(strings.Join([]string{
					"{{bold}}Ranking Criteria: Lower Mean is Better",
					"{{/}}{{bold}}Experiment{{/}} | {{bold}}Name    {{/}} | {{bold}}N  {{/}} | {{bold}}Min{{/}} | {{bold}}Median{{/}} | {{bold}}Mean{{/}} | {{bold}}StdDev{{/}} | {{bold}}Max{{/}}",
					"================================================================",
					"{{bold}}Exp-C     {{/}} | {{bold}}C       {{/}} | {{bold}}100{{/}} | {{bold}}3s {{/}} | {{bold}}2s    {{/}} | {{bold}}1s  {{/}} | {{bold}}0s    {{/}} | {{bold}}4s {{/}}",
					"{{bold}}*Winner*  {{/}} | {{bold}}*Winner*{{/}} |     |     |        |      |        |    ",
					"----------------------------------------------------------------",
					"Exp-D      | D        | 100 | 4s  | 3s     | 2s   | 0s     | 1s ",
					"----------------------------------------------------------------",
					"Exp-A      | A        | 100 | 1s  | 4s     | 3s   | 0s     | 2s ",
					"----------------------------------------------------------------",
					"Exp-B      | B        | 100 | 2s  | 1s     | 4s   | 0s     | 3s ",
					"",
				}, "\n")))
			})
		})
	})

})

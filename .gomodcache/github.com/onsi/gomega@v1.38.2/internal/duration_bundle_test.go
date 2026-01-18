package internal_test

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/internal"
)

var _ = Describe("DurationBundle and Duration Support", func() {
	Describe("fetching default durations from the environment", func() {
		var envVars []string
		var originalValues map[string]string

		BeforeEach(func() {
			envVars = []string{internal.EventuallyTimeoutEnvVarName, internal.EventuallyPollingIntervalEnvVarName, internal.ConsistentlyDurationEnvVarName, internal.ConsistentlyPollingIntervalEnvVarName}
			originalValues = map[string]string{}

			for _, envVar := range envVars {
				originalValues[envVar] = os.Getenv(envVar)
			}
		})

		AfterEach(func() {
			for _, envVar := range envVars {
				Ω(os.Setenv(envVar, originalValues[envVar])).Should(Succeed())
			}
			os.Unsetenv(internal.EnforceDefaultTimeoutsWhenUsingContextsEnvVarName)
		})

		Context("with no environment set", func() {
			BeforeEach(func() {
				for _, envVar := range envVars {
					os.Unsetenv(envVar)
				}
			})

			It("returns the default bundle", func() {
				bundle := internal.FetchDefaultDurationBundle()
				Ω(bundle.EventuallyTimeout).Should(Equal(time.Second))
				Ω(bundle.EventuallyPollingInterval).Should(Equal(10 * time.Millisecond))
				Ω(bundle.ConsistentlyDuration).Should(Equal(100 * time.Millisecond))
				Ω(bundle.ConsistentlyPollingInterval).Should(Equal(10 * time.Millisecond))
				Ω(bundle.EnforceDefaultTimeoutsWhenUsingContexts).Should(BeFalse())
			})
		})

		Context("with a valid environment set", func() {
			BeforeEach(func() {
				os.Setenv(internal.EventuallyTimeoutEnvVarName, "1m")
				os.Setenv(internal.EventuallyPollingIntervalEnvVarName, "2s")
				os.Setenv(internal.ConsistentlyDurationEnvVarName, "1h")
				os.Setenv(internal.ConsistentlyPollingIntervalEnvVarName, "3ms")
				os.Setenv(internal.EnforceDefaultTimeoutsWhenUsingContextsEnvVarName, "")
			})

			It("returns an appropriate bundle", func() {
				bundle := internal.FetchDefaultDurationBundle()
				Ω(bundle.EventuallyTimeout).Should(Equal(time.Minute))
				Ω(bundle.EventuallyPollingInterval).Should(Equal(2 * time.Second))
				Ω(bundle.ConsistentlyDuration).Should(Equal(time.Hour))
				Ω(bundle.ConsistentlyPollingInterval).Should(Equal(3 * time.Millisecond))
				Ω(bundle.EnforceDefaultTimeoutsWhenUsingContexts).Should(BeTrue())
			})
		})

		Context("with an invalid environment set", func() {
			BeforeEach(func() {
				os.Setenv(internal.EventuallyTimeoutEnvVarName, "chicken nuggets")
			})

			It("panics", func() {
				Ω(func() {
					internal.FetchDefaultDurationBundle()
				}).Should(PanicWith(`Expected a duration when using GOMEGA_DEFAULT_EVENTUALLY_TIMEOUT!  Parse error time: invalid duration "chicken nuggets"`))
			})
		})
	})

	Describe("specifying default durations on a Gomega instance", func() {
		It("is supported", func() {
			ig := NewInstrumentedGomega()
			ig.G.SetDefaultConsistentlyDuration(50 * time.Millisecond)
			ig.G.SetDefaultConsistentlyPollingInterval(5 * time.Millisecond)
			ig.G.SetDefaultEventuallyTimeout(200 * time.Millisecond)
			ig.G.SetDefaultEventuallyPollingInterval(20 * time.Millisecond)

			counter := 0
			t := time.Now()
			ig.G.Consistently(func() bool {
				counter += 1
				return true
			}).Should(BeTrue())
			dt := time.Since(t)
			Ω(dt).Should(BeNumerically("~", 50*time.Millisecond, 25*time.Millisecond))
			Ω(counter).Should(BeNumerically("~", 10, 5))

			t = time.Now()
			counter = 0
			ig.G.Eventually(func() bool {
				counter += 1
				if counter >= 6 {
					return true
				}
				return false
			}).Should(BeTrue())
			dt = time.Since(t)
			Ω(dt).Should(BeNumerically("~", 120*time.Millisecond, 20*time.Millisecond))
		})
	})

	Describe("specifying durations", func() {
		It("supports passing in a duration", func() {
			t := time.Now()
			Consistently(true, 50*time.Millisecond).Should(BeTrue())
			Ω(time.Since(t)).Should(BeNumerically("~", 50*time.Millisecond, 30*time.Millisecond))
		})

		It("supports passing in a raw integer # of seconds", func() {
			t := time.Now()
			Consistently(true, 1).Should(BeTrue())
			Ω(time.Since(t)).Should(BeNumerically("~", time.Second, 100*time.Millisecond))
		})

		It("supports passing in an unsigned integer # of seconds", func() {
			t := time.Now()
			Consistently(true, uint(1)).Should(BeTrue())
			Ω(time.Since(t)).Should(BeNumerically("~", time.Second, 100*time.Millisecond))
		})

		It("supports passing in a float number of seconds", func() {
			t := time.Now()
			Consistently(true, 0.05).Should(BeTrue())
			Ω(time.Since(t)).Should(BeNumerically("~", 50*time.Millisecond, 30*time.Millisecond))
		})

		It("supports passing in a duration string", func() {
			t := time.Now()
			Consistently(true, "50ms").Should(BeTrue())
			Ω(time.Since(t)).Should(BeNumerically("~", 50*time.Millisecond, 30*time.Millisecond))
		})

		It("fails when the duration string can't be parsed", func() {
			ig := NewInstrumentedGomega()
			ig.G.Consistently(true, "fries").Should(BeTrue())
			Ω(ig.FailureMessage).Should(Equal(`"fries" is not a valid parsable duration string: time: invalid duration "fries"`))
		})

		It("fails when the duration is the wrong type", func() {
			ig := NewInstrumentedGomega()
			ig.G.Consistently(true, true).Should(BeTrue())
			Ω(ig.FailureMessage).Should(Equal(`true is not a valid interval. Must be a time.Duration, a parsable duration string, or a number.`))
		})
	})
})

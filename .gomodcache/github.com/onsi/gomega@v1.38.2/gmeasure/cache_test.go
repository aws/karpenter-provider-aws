package gmeasure_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"
)

var _ = Describe("Cache", func() {
	var path string
	var cache gmeasure.ExperimentCache
	var e1, e2 *gmeasure.Experiment

	BeforeEach(func() {
		var err error
		path = fmt.Sprintf("./cache-%d", GinkgoParallelProcess())
		cache, err = gmeasure.NewExperimentCache(path)
		Ω(err).ShouldNot(HaveOccurred())
		e1 = gmeasure.NewExperiment("Experiment-1")
		e1.RecordValue("foo", 32)
		e2 = gmeasure.NewExperiment("Experiment-2")
		e2.RecordValue("bar", 64)
	})

	AfterEach(func() {
		Ω(os.RemoveAll(path)).Should(Succeed())
	})

	Describe("when creating a cache that points to a file", func() {
		It("errors", func() {
			f, err := os.Create("cache-temp-file")
			Ω(err).ShouldNot(HaveOccurred())
			f.Close()
			cache, err := gmeasure.NewExperimentCache("cache-temp-file")
			Ω(err).Should(MatchError("cache-temp-file is not a directory"))
			Ω(cache).Should(BeZero())
			Ω(os.RemoveAll("cache-temp-file")).Should(Succeed())
		})
	})

	Describe("the happy path", func() {
		It("can save, load, list, delete, and clear the cache", func() {
			Ω(cache.Save("e1", 1, e1)).Should(Succeed())
			Ω(cache.Save("e2", 7, e2)).Should(Succeed())

			Ω(cache.Load("e1", 1)).Should(Equal(e1))
			Ω(cache.Load("e2", 7)).Should(Equal(e2))

			Ω(cache.List()).Should(ConsistOf(
				gmeasure.CachedExperimentHeader{"e1", 1},
				gmeasure.CachedExperimentHeader{"e2", 7},
			))

			Ω(cache.Delete("e2")).Should(Succeed())
			Ω(cache.Load("e1", 1)).Should(Equal(e1))
			Ω(cache.Load("e2", 7)).Should(BeNil())
			Ω(cache.List()).Should(ConsistOf(
				gmeasure.CachedExperimentHeader{"e1", 1},
			))

			Ω(cache.Clear()).Should(Succeed())
			Ω(cache.List()).Should(BeEmpty())
			Ω(cache.Load("e1", 1)).Should(BeNil())
			Ω(cache.Load("e2", 7)).Should(BeNil())
		})
	})

	Context("with an empty cache", func() {
		It("should list nothing", func() {
			Ω(cache.List()).Should(BeEmpty())
		})

		It("should not error when clearing", func() {
			Ω(cache.Clear()).Should(Succeed())
		})

		It("returns nil when loading a non-existing experiment", func() {
			Ω(cache.Load("floop", 17)).Should(BeNil())
		})
	})

	Describe("version management", func() {
		BeforeEach(func() {
			Ω(cache.Save("e1", 7, e1)).Should(Succeed())
		})

		Context("when the cached version is older than the requested version", func() {
			It("returns nil", func() {
				Ω(cache.Load("e1", 8)).Should(BeNil())
			})
		})

		Context("when the cached version equals the requested version", func() {
			It("returns the cached version", func() {
				Ω(cache.Load("e1", 7)).Should(Equal(e1))
			})
		})

		Context("when the cached version is newer than the requested version", func() {
			It("returns the cached version", func() {
				Ω(cache.Load("e1", 6)).Should(Equal(e1))
			})
		})
	})

})

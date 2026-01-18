package gleak

import "os"

// IgnoreGinkgoParallelClient must be called in a BeforeSuite whenever a test
// suite is run in parallel with other test suites using "ginkgo -p". Calling
// IgnoreGinkgoParallelClient checks for a Ginkgo-related background go routine
// and then updates gleak's internal ignore list to specifically ignore this
// background go routine by its ("random") ID.
func IgnoreGinkgoParallelClient() {
	ignoreCreator := "net/rpc.NewClientWithCodec"
	if os.Getenv("GINKGO_PARALLEL_PROTOCOL") == "HTTP" {
		ignoreCreator = "net/http.(*Transport).dialConn"
	}
	ignores := []Goroutine{}
	for _, g := range Goroutines() {
		if g.CreatorFunction == ignoreCreator {
			ignores = append(ignores, g)
		}
	}
	if len(ignores) == 0 {
		return
	}
	standardFilters = append(standardFilters, IgnoringGoroutines(ignores))
}

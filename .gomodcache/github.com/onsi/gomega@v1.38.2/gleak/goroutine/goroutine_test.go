package goroutine

import (
	"bufio"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing/iotest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("goroutine", func() {

	const stack = `runtime/debug.Stack()
	/usr/local/go-faketime/src/runtime/debug/stack.go:24 +0x65
runtime/debug.PrintStack()
	/usr/local/go-faketime/src/runtime/debug/stack.go:16 +0x19
main.main()
	/tmp/sandbox3386995578/prog.go:10 +0x17
`
	const header = `goroutine 666 [running]:
`
	const nextStack = header + `main.hades()
	/tmp/sandbox3386995578/prog.go:10 +0x17
`

	It("prints", func() {
		Expect(Goroutine{
			ID:          1234,
			State:       "gone",
			TopFunction: "gopher.hole",
		}.String()).To(Equal(
			"Goroutine ID: 1234, state: gone, top function: gopher.hole"))

		Expect(Goroutine{
			ID:              1234,
			State:           "gone",
			TopFunction:     "gopher.hole",
			CreatorFunction: "google",
			BornAt:          "/plan/10:2009",
		}.String()).To(Equal(
			"Goroutine ID: 1234, state: gone, top function: gopher.hole, created by: google, at: /plan/10:2009"))

		Expect(Goroutine{
			ID:              1234,
			State:           "gone",
			TopFunction:     "gopher.hole",
			CreatorFunction: "google",
			BornAt:          "/plan/10:2009",
		}.GomegaString()).To(Equal(
			"{ID: 1234, State: \"gone\", TopFunction: \"gopher.hole\", CreatorFunction: \"google\", BornAt: \"/plan/10:2009\"}"))
	})

	Context("goroutine header", func() {

		It("parses goroutine header", func() {
			g := new(header)
			Expect(g.ID).To(Equal(uint64(666)))
			Expect(g.State).To(Equal("running"))
		})

		It("panics on malformed goroutine header", func() {
			Expect(func() { _ = new("a") }).To(PanicWith(MatchRegexp(`invalid stack header: .*`)))
			Expect(func() { _ = new("a b") }).To(PanicWith(MatchRegexp(`invalid stack header: .*`)))
		})

		It("panics on malformed goroutine ID", func() {
			Expect(func() { _ = new("a b c:\n") }).To(PanicWith(MatchRegexp(`invalid stack header ID: "b", header: ".*"`)))
		})

	})

	Context("goroutine backtrace", func() {

		It("parses goroutine's backtrace", func() {
			r := bufio.NewReader(strings.NewReader(stack))
			topF, backtrace := parseGoroutineBacktrace(r)
			Expect(topF).To(Equal("runtime/debug.Stack"))
			Expect(backtrace).To(Equal(stack))

			r.Reset(strings.NewReader(stack[:len(stack)-1]))
			topF, backtrace = parseGoroutineBacktrace(r)
			Expect(topF).To(Equal("runtime/debug.Stack"))
			Expect(backtrace).To(Equal(stack[:len(stack)-1]))
		})

		It("parses goroutine's backtrace until next goroutine header", func() {
			r := bufio.NewReader(strings.NewReader(stack + nextStack))
			topF, backtrace := parseGoroutineBacktrace(r)
			Expect(topF).To(Equal("runtime/debug.Stack"))
			Expect(backtrace).To(Equal(stack))
		})

		It("panics on invalid function call stack entry", func() {
			r := bufio.NewReader(strings.NewReader(`main.main
	/somewhere/prog.go:123 +0x666
	`))
			Expect(func() { parseGoroutineBacktrace(r) }).To(PanicWith(MatchRegexp(`invalid function call stack entry: "main.main"`)))
		})

		It("panics on failing reader", func() {
			Expect(func() {
				parseGoroutineBacktrace(bufio.NewReader(
					iotest.ErrReader(errors.New("foo failure"))))
			}).To(PanicWith("parsing backtrace failed: foo failure"))

			Expect(func() {
				parseGoroutineBacktrace(
					bufio.NewReaderSize(
						iotest.TimeoutReader(strings.NewReader(strings.Repeat("x", 32))),
						16))
			}).To(PanicWith("parsing backtrace failed: timeout"))

			Expect(func() {
				parseGoroutineBacktrace(bufio.NewReader(
					iotest.ErrReader(io.ErrClosedPipe)))
			}).To(PanicWith(MatchRegexp(`parsing backtrace failed: .*`)))
		})

		It("parses goroutine information and stack", func() {
			gs := parseStack([]byte(header + stack))
			Expect(gs).To(HaveLen(1))
			Expect(gs[0]).To(And(
				HaveField("ID", uint64(666)),
				HaveField("State", "running"),
				HaveField("TopFunction", "runtime/debug.Stack"),
				HaveField("Backtrace", stack)))
		})

		It("finds its Creator", func() {
			creator, location := findCreator(`
goroutine 42 [chan receive]:
main.foo.func1()
		/home/foo/test.go:6 +0x28
created by main.foo
		/home/foo/test.go:5 +0x64
`)
			Expect(creator).To(Equal("main.foo"))
			Expect(location).To(Equal("/home/foo/test.go:5"))
		})

		It("handles missing or invalid creator information", func() {
			creator, location := findCreator("")
			Expect(creator).To(BeEmpty())
			Expect(location).To(BeEmpty())

			creator, location = findCreator(`
goroutine 42 [chan receive]:
main.foo.func1()
		/home/foo/test.go:6 +0x28
created by`)
			Expect(creator).To(BeEmpty())
			Expect(location).To(BeEmpty())

			creator, location = findCreator(`
goroutine 42 [chan receive]:
main.foo.func1()
		/home/foo/test.go:6 +0x28
created by main.foo`)
			Expect(creator).To(BeEmpty())
			Expect(location).To(BeEmpty())

			creator, location = findCreator(`
goroutine 42 [chan receive]:
main.foo.func1()
		/home/foo/test.go:6 +0x28
created by main.foo
		/home/foo/test.go:5
`)
			Expect(creator).To(BeEmpty())
			Expect(location).To(BeEmpty())
		})

	})

	Context("live", func() {

		It("discovers current goroutine information", func() {
			type T struct{}
			pkg := reflect.TypeOf(T{}).PkgPath()
			gs := goroutines(false)
			Expect(gs).To(HaveLen(1))
			Expect(gs[0]).To(And(
				HaveField("ID", Not(BeZero())),
				HaveField("State", "running"),
				HaveField("TopFunction", pkg+".stacks"),
				HaveField("Backtrace", MatchRegexp(pkg+`.stacks.*
`))))
		})

		It("discovers a goroutine's creator", func() {
			ch := make(chan Goroutine)
			go func() {
				ch <- Current()
			}()
			g := <-ch
			Expect(g.CreatorFunction).NotTo(BeEmpty(), "no creator: %s", g.Backtrace)
			Expect(g.BornAt).NotTo(BeEmpty())
		})

		It("discovers all goroutine information", func() {
			By("creating a chan receive canary goroutine")
			done := make(chan struct{})
			go testWait(done)
			once := sync.Once{}
			cloze := func() { once.Do(func() { close(done) }) }
			defer cloze()

			By("getting all goroutines including canary")
			type T struct{}
			pkg := reflect.TypeOf(T{}).PkgPath()
			Eventually(Goroutines).
				WithTimeout(1 * time.Second).WithPolling(250 * time.Millisecond).
				Should(ContainElements(
					And(
						HaveField("TopFunction", pkg+".stacks"),
						HaveField("State", "running")),
					And(
						HaveField("TopFunction", pkg+".testWait"),
						HaveField("State", "chan receive")),
				))

			By("getting all goroutines after being done with the canary")
			cloze()
			Eventually(Goroutines).
				WithTimeout(1 * time.Second).WithPolling(250 * time.Millisecond).
				ShouldNot(ContainElement(HaveField("TopFunction", pkg+".testWait")))
		})

	})

})

func testWait(done <-chan struct{}) {
	<-done
}

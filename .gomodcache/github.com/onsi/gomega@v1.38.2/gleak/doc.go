/*
package gleak complements the Gingko/Gomega testing and matchers framework with
matchers for Goroutine leakage detection.

# Basics of gleak

To start with,

	Goroutines()

returns information about all (non-dead) goroutines at a particular moment. This
is useful to capture a known correct snapshot and then later taking a new
snapshot and comparing these two snapshots for leaked goroutines.

Next, the matcher

	HaveLeaked(...)

filters out well-known and expected "non-leaky" goroutines from an actual list
of goroutines (passed from Eventually or Expect), hopefully ending up with an
empty list of leaked goroutines. If there are still goroutines left after
filtering, then HaveLeaked() will succeed ... which usually is actually
considered to be failure. So, this can be rather declared to be "suckcess"
because no one wants leaked goroutines.

A typical pattern to detect goroutines leaked in individual tests is as follows:

	var ignoreGood []Goroutine

	BeforeEach(func() {
	    ignoreGood = Goroutines()
	})

	AfterEach(func() {
	    // Note: it's "Goroutines", but not "Goroutines()", when using with Eventually!
	    Eventually(Goroutines).ShouldNot(HaveLeaked(ignoreGood))
	})

Using Eventually instead of Expect ensures that there is some time given for
temporary goroutines to finally wind down. Gomega's default values apply: the 1s
timeout and 10ms polling interval.

Please note that the form

	HaveLeaked(ignoreGood)

is the same as the slightly longer, but also more expressive variant:

	HaveLeaked(IgnoringGoroutines(ignoreGood))

# Leak-Related Matchers

Depending on your tests and the dependencies used, you might need to identify
additional goroutines as not being leaks. The gleak packages comes with the
following predefined goroutine "filter" matchers that can be specified as
arguments to HaveLeaked(...):

	IgnoringTopFunction("foo.bar")                // exactly "foo.bar"
	IgnoringTopFunction("foo.bar...")             // top function name with prefix "foo.bar." (note the trailing dot!)
	IgnoringTopFunction("foo.bar [chan receive]") // exactly "foo.bar" with state starting with "chan receive"
	IgnoringGoroutines(expectedGoroutines)        // ignore specified goroutines with these IDs
	IgnoringInBacktrace("foo.bar.baz")            // "foo.bar.baz" within the backtrace
	IgnoringCreator("foo.bar")                    // exact creator function name "foo.bar"
	IgnoringCreator("foo.bar...")                 // creator function name with prefix "foo.bar."

In addition, you can use any other GomegaMatcher, as long as it can work on a
(single) Goroutine. For instance, Gomega's HaveField and WithTransform
matchers are good foundations for writing project-specific gleak matchers.

# Leaked Goroutine Dump

By default, when gleak's HaveLeaked matcher finds one or more leaked
goroutines, it dumps the goroutine backtraces in a condensed format that uses
only a single line per call instead of two lines. Moreover, the backtraces
abbreviate the source file location in the form of package/source.go:lineno:

	goroutine 42 [flabbergasted]
	    main.foo.func1() at foo/test.go:6
	    created by main.foo at foo/test.go:5

By setting gleak.ReportFilenameWithPath=true the leaky goroutine backtraces
will show full path names for each source file:

	goroutine 42 [flabbergasted]
	    main.foo.func1() at /home/go/foo/test.go:6
	    created by main.foo at home/go/foo/test.go:5

# Acknowledgement

gleak has been heavily inspired by the Goroutine leak detector
github.com/uber-go/goleak. That's definitely a fine piece of work!

But then why another goroutine leak package? After a deep analysis of Uber's
goleak we decided against crunching goleak somehow half-assed into the Gomega
TDD matcher ecosystem. In particular, reusing and wrapping of the existing Uber
implementation would have become very awkward: goleak.Find combines all the
different elements of getting actual goroutines information, filtering them,
arriving at a leak conclusion, and even retrying multiple times all in just one
single exported function. Unfortunately, goleak makes gathering information
about all goroutines an internal matter, so we cannot reuse such functionality
elsewhere.

Users of the Gomega ecosystem are already experienced in arriving at conclusions
and retrying temporarily failing expectations: Gomega does it in form of
Eventually().ShouldNot(), and (without the trying aspect) with Expect().NotTo().
So what is missing is only a goroutine leak detector in form of the HaveLeaked
matcher, as well as the ability to specify goroutine filters in order to sort
out the non-leaking (and therefore expected) goroutines, using a few filter
criteria. That is, a few new goroutine-related matchers. In this architecture,
even existing Gomega matchers can optionally be (re)used as the need arises.

# References

https://github.com/onsi/gomega and https://github.com/onsi/ginkgo.
*/
package gleak

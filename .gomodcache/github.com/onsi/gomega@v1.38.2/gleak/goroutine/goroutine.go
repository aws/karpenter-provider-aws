package goroutine

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Goroutine represents information about a single goroutine, such as its unique
// ID, state, backtrace, creator, and more.
//
// Go's runtime assigns unique IDs to goroutines, also called "goid" in Go
// runtime parlance. These IDs start with 1 for the main goroutine and only
// increase (unless you manage to create 2**64-2 goroutines during the lifetime
// of your tests so that the goids wrap around). Due to runtime-internal
// optimizations, not all IDs might be used, so that there might be gaps. But
// IDs are never reused, so they're fine as unique goroutine identities.
//
// The size of a goidsis always 64bits, even on 32bit architectures (if you
// like, you might want to double-check for yourself in runtime/runtime2.go and
// runtime/proc.go).
//
// A Goroutine's State field starts with one of the following strings:
//   - "idle"
//   - "runnable"
//   - "running"
//   - "syscall"
//   - ("waiting" ... see below)
//   - ("dead" ... these goroutines should never appear in dumps)
//   - "copystack"
//   - "preempted"
//   - ("???" ... something IS severely broken.)
//
// In case a goroutine is in waiting state, the State field instead starts with
// one of the following strings, never showing a lonely "waiting" string, but
// rather one of the reasons for waiting:
//   - "chan receive"
//   - "chan send"
//   - "select"
//   - "sleep"
//   - "finalizer wait"
//   - ...quite some more waiting states.
//
// The State description may next contain "(scan)", separated by a single blank
// from the preceding goroutine state text.
//
// If a goroutine is blocked from more than at least a minute, then the state
// description next contains the string "X minutes", where X is the number of
// minutes blocked. This text is separated by a "," and a blank from the
// preceding information.
//
// Finally, OS thread-locked goroutines finally contain "locked to thread" in
// their State description, again separated by a "," and a blank from the
// preceding information.
//
// Please note that the State field never contains the opening and closing
// square brackets as used in plain stack dumps.
type Goroutine struct {
	ID              uint64 // unique goroutine ID ("goid" in Go's runtime parlance)
	State           string // goroutine state, such as "running"
	TopFunction     string // topmost function on goroutine's stack
	CreatorFunction string // name of function creating this goroutine, if any
	BornAt          string // location where the goroutine was started from, if any; format "file-path:line-number"
	Backtrace       string // goroutine's backtrace (of the stack)
}

// String returns a short textual description of this goroutine, but without the
// potentially lengthy and ugly backtrace details.
func (g Goroutine) String() string {
	s := fmt.Sprintf("Goroutine ID: %d, state: %s, top function: %s",
		g.ID, g.State, g.TopFunction)
	if g.CreatorFunction == "" {
		return s
	}
	s += fmt.Sprintf(", created by: %s, at: %s",
		g.CreatorFunction, g.BornAt)
	return s
}

// GomegaString returns the Gomega struct representation of a Goroutine, but
// without a potentially rather lengthy backtrace. This Gomega object value
// dumps getting happily truncated as to become more or less useless.
func (g Goroutine) GomegaString() string {
	return fmt.Sprintf(
		"{ID: %d, State: %q, TopFunction: %q, CreatorFunction: %q, BornAt: %q}",
		g.ID, g.State, g.TopFunction, g.CreatorFunction, g.BornAt)
}

// Goroutines returns information about all goroutines.
func Goroutines() []Goroutine {
	return goroutines(true)
}

// Current returns information about the current goroutine in which it is
// called. Please note that the topmost function name will always be
// runtime.Stack.
func Current() Goroutine {
	return goroutines(false)[0]
}

// goroutines is an internal wrapper around dumping either only the stack of the
// current goroutine of the caller or dumping the stacks of all goroutines, and
// then parsing the dump into separate Goroutine descriptions.
func goroutines(all bool) []Goroutine {
	return parseStack(stacks(all))
}

// parseStack parses the stack dump of one or multiple goroutines, as returned
// by runtime.Stack() and then returns a list of Goroutine descriptions based on
// the dump.
func parseStack(stacks []byte) []Goroutine {
	gs := []Goroutine{}
	r := bufio.NewReader(bytes.NewReader(stacks))
	for {
		// We expect a line describing a new "goroutine", everything else is a
		// failure. And yes, if we get an EOF already with this line, bail out.
		line, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		g := new(line)
		// Read the rest ... that is, the backtrace for this goroutine.
		g.TopFunction, g.Backtrace = parseGoroutineBacktrace(r)
		if strings.HasSuffix(g.Backtrace, "\n\n") {
			g.Backtrace = g.Backtrace[:len(g.Backtrace)-1]
		}
		g.CreatorFunction, g.BornAt = findCreator(g.Backtrace)
		gs = append(gs, g)
	}
	return gs
}

// new takes a goroutine line from a stack dump and returns a Goroutine object
// based on the information contained in the dump.
func new(s string) Goroutine {
	s = strings.TrimSuffix(s, ":\n")
	fields := strings.SplitN(s, " ", 3)
	if len(fields) != 3 {
		panic(fmt.Sprintf("invalid stack header: %q", s))
	}
	id, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid stack header ID: %q, header: %q", fields[1], s))
	}
	state := strings.TrimSuffix(strings.TrimPrefix(fields[2], "["), "]")
	return Goroutine{ID: id, State: state}
}

// Beginning of line indicating the creator of a Goroutine, if any. This
// indication is missing for the main goroutine as it appeared in a big bang or
// something similar.
const backtraceGoroutineCreator = "created by "

// findCreator solves the great mystery of Gokind, answering the question of who
// created this goroutine? Given a backtrace, that is.
func findCreator(backtrace string) (creator, location string) {
	pos := strings.LastIndex(backtrace, backtraceGoroutineCreator)
	if pos < 0 {
		return
	}
	// Split the "created by ..." line from the following line giving us the
	// (indented) file name:line number and the hex offset of the call location
	// within the function.
	details := strings.SplitN(backtrace[pos+len(backtraceGoroutineCreator):], "\n", 3)
	if len(details) < 2 {
		return
	}
	// Split off the call location hex offset which is of no use to us, and only
	// keep the file path and line number information. This will be useful for
	// diagnosis, when dumping leaked goroutines.
	offsetpos := strings.LastIndex(details[1], " +0x")
	if offsetpos < 0 {
		return
	}
	location = strings.TrimSpace(details[1][:offsetpos])
	creator = details[0]
	if offsetpos := strings.LastIndex(creator, " in goroutine "); offsetpos >= 0 {
		creator = creator[:offsetpos]
	}
	return
}

// Beginning of header line introducing a (new) goroutine in a backtrace.
const backtraceGoroutineHeader = "goroutine "

// Length of the header line prefix introducing a (new) goroutine in a
// backtrace.
const backtraceGoroutineHeaderLen = len(backtraceGoroutineHeader)

// parseGoroutineBacktrace reads from reader r the backtrace information until
// the end or until the next goroutine header is seen. This next goroutine
// header is NOT consumed so that callers can still read the next header from
// the reader.
func parseGoroutineBacktrace(r *bufio.Reader) (topFn string, backtrace string) {
	bt := bytes.Buffer{}
	// Read backtrace information belonging to this goroutine until we meet
	// another goroutine header.
	for {
		header, err := r.Peek(backtraceGoroutineHeaderLen)
		if string(header) == backtraceGoroutineHeader {
			// next goroutine header is up for read, so we're done with parsing
			// the backtrace of this goroutine.
			break
		}
		if err != nil && err != io.EOF {
			// There is some serious problem with the stack dump, so we
			// decidedly panic now.
			panic("parsing backtrace failed: " + err.Error())
		}
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			// There is some serious problem with the stack dump, so we
			// decidedly panic now.
			panic("parsing backtrace failed: " + err.Error())
		}
		// The first line after a goroutine header lists the "topmost" function.
		if topFn == "" {
			line := /*sic!*/ strings.TrimSpace(line)
			idx := strings.LastIndex(line, "(")
			if idx <= 0 {
				panic(fmt.Sprintf("invalid function call stack entry: %q", line))
			}
			topFn = line[:idx]
		}
		// Always append the line read to the goroutine's backtrace.
		bt.WriteString(line)
		if err == io.EOF {
			// we're reached the end of the stack dump, so that's it.
			break
		}
	}
	return topFn, bt.String()
}

// Go support for leveled logs, analogous to https://code.google.com/p/google-glog/
//
// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package klog

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	stdLog "log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/klog/v2/internal/buffer"
	testingclock "k8s.io/klog/v2/internal/clock/testing"
	"k8s.io/klog/v2/internal/severity"
	"k8s.io/klog/v2/internal/test"
	"k8s.io/klog/v2/internal/test/require"
)

// TODO: This test package should be refactored so that tests cannot
// interfere with each-other.

// Test that shortHostname works as advertised.
func TestShortHostname(t *testing.T) {
	for hostname, expect := range map[string]string{
		"":                "",
		"host":            "host",
		"host.google.com": "host",
	} {
		if got := shortHostname(hostname); expect != got {
			t.Errorf("shortHostname(%q): expected %q, got %q", hostname, expect, got)
		}
	}
}

// flushBuffer wraps a bytes.Buffer to satisfy flushSyncWriter.
type flushBuffer struct {
	bytes.Buffer
}

func (f *flushBuffer) Flush() error {
	return nil
}

func (f *flushBuffer) Sync() error {
	return nil
}

// swap sets the log writers and returns the old array.
func (l *loggingT) swap(writers [severity.NumSeverity]io.Writer) (old [severity.NumSeverity]io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	old = l.file
	logging.file = writers
	return
}

// newBuffers sets the log writers to all new byte buffers and returns the old array.
func (l *loggingT) newBuffers() [severity.NumSeverity]io.Writer {
	return l.swap([severity.NumSeverity]io.Writer{new(flushBuffer), new(flushBuffer), new(flushBuffer), new(flushBuffer)})
}

// contents returns the specified log value as a string.
func contents(s severity.Severity) string {
	return logging.file[s].(*flushBuffer).String()
}

// contains reports whether the string is contained in the log.
func contains(s severity.Severity, str string) bool {
	return strings.Contains(contents(s), str)
}

// setFlags configures the logging flags how the test expects them.
func setFlags() {
	logging.toStderr = false
	logging.addDirHeader = false
}

// Test that Info works as advertised.
func TestInfo(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	Info("test")
	if !contains(severity.InfoLog, "I") {
		t.Errorf("Info has wrong character: %q", contents(severity.InfoLog))
	}
	if !contains(severity.InfoLog, "test") {
		t.Error("Info failed")
	}
}

func TestInfoDepth(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())

	f := func() { InfoDepth(1, "depth-test1") }

	// The next three lines must stay together
	_, _, wantLine, _ := runtime.Caller(0)
	InfoDepth(0, "depth-test0")
	f()

	msgs := strings.Split(strings.TrimSuffix(contents(severity.InfoLog), "\n"), "\n")
	if len(msgs) != 2 {
		t.Fatalf("Got %d lines, expected 2", len(msgs))
	}

	for i, m := range msgs {
		if !strings.HasPrefix(m, "I") {
			t.Errorf("InfoDepth[%d] has wrong character: %q", i, m)
		}
		w := fmt.Sprintf("depth-test%d", i)
		if !strings.Contains(m, w) {
			t.Errorf("InfoDepth[%d] missing %q: %q", i, w, m)
		}

		// pull out the line number (between : and ])
		msg := m[strings.LastIndex(m, ":")+1:]
		x := strings.Index(msg, "]")
		if x < 0 {
			t.Errorf("InfoDepth[%d]: missing ']': %q", i, m)
			continue
		}
		line, err := strconv.Atoi(msg[:x])
		if err != nil {
			t.Errorf("InfoDepth[%d]: bad line number: %q", i, m)
			continue
		}
		wantLine++
		if wantLine != line {
			t.Errorf("InfoDepth[%d]: got line %d, want %d", i, line, wantLine)
		}
	}
}

func init() {
	CopyStandardLogTo("INFO")
}

// Test that CopyStandardLogTo panics on bad input.
func TestCopyStandardLogToPanic(t *testing.T) {
	defer func() {
		if s, ok := recover().(string); !ok || !strings.Contains(s, "LOG") {
			t.Errorf(`CopyStandardLogTo("LOG") should have panicked: %v`, s)
		}
	}()
	CopyStandardLogTo("LOG")
}

// Test that using the standard log package logs to INFO.
func TestStandardLog(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	stdLog.Print("test")
	if !contains(severity.InfoLog, "I") {
		t.Errorf("Info has wrong character: %q", contents(severity.InfoLog))
	}
	if !contains(severity.InfoLog, "test") {
		t.Error("Info failed")
	}
}

// Test that the header has the correct format.
func TestHeader(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	defer func(previous func() time.Time) { timeNow = previous }(timeNow)
	timeNow = func() time.Time {
		return time.Date(2006, 1, 2, 15, 4, 5, .067890e9, time.Local)
	}
	buffer.Pid = 1234
	Info("test")
	var line int
	format := "I0102 15:04:05.067890    1234 klog_test.go:%d] test\n"
	n, err := fmt.Sscanf(contents(severity.InfoLog), format, &line)
	if n != 1 || err != nil {
		t.Errorf("log format error: %d elements, error %s:\n%s", n, err, contents(severity.InfoLog))
	}
	// Scanf treats multiple spaces as equivalent to a single space,
	// so check for correct space-padding also.
	want := fmt.Sprintf(format, line)
	if contents(severity.InfoLog) != want {
		t.Errorf("log format error: got:\n\t%q\nwant:\t%q", contents(severity.InfoLog), want)
	}
}

func TestHeaderWithDir(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	logging.addDirHeader = true
	defer logging.swap(logging.newBuffers())
	defer func(previous func() time.Time) { timeNow = previous }(timeNow)
	timeNow = func() time.Time {
		return time.Date(2006, 1, 2, 15, 4, 5, .067890e9, time.Local)
	}
	pid = 1234
	Info("test")
	re := regexp.MustCompile(`I0102 15:04:05.067890    1234 (klog|v2)/klog_test.go:(\d+)] test\n`)
	if !re.MatchString(contents(severity.InfoLog)) {
		t.Errorf("log format error: line does not match regex:\n\t%q\n", contents(severity.InfoLog))
	}
}

// Test that an Error log goes to Warning and Info.
// Even in the Info log, the source character will be E, so the data should
// all be identical.
func TestError(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	Error("test")
	if !contains(severity.ErrorLog, "E") {
		t.Errorf("Error has wrong character: %q", contents(severity.ErrorLog))
	}
	if !contains(severity.ErrorLog, "test") {
		t.Error("Error failed")
	}
	str := contents(severity.ErrorLog)
	if !contains(severity.WarningLog, str) {
		t.Error("Warning failed")
	}
	if !contains(severity.InfoLog, str) {
		t.Error("Info failed")
	}
}

// Test that an Error log does not goes to Warning and Info.
// Even in the Info log, the source character will be E, so the data should
// all be identical.
func TestErrorWithOneOutput(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	logging.oneOutput = true
	defer logging.swap(logging.newBuffers())
	Error("test")
	if !contains(severity.ErrorLog, "E") {
		t.Errorf("Error has wrong character: %q", contents(severity.ErrorLog))
	}
	if !contains(severity.ErrorLog, "test") {
		t.Error("Error failed")
	}
	str := contents(severity.ErrorLog)
	if contains(severity.WarningLog, str) {
		t.Error("Warning failed")
	}
	if contains(severity.InfoLog, str) {
		t.Error("Info failed")
	}
}

// Test that a Warning log goes to Info.
// Even in the Info log, the source character will be W, so the data should
// all be identical.
func TestWarning(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	Warning("test")
	if !contains(severity.WarningLog, "W") {
		t.Errorf("Warning has wrong character: %q", contents(severity.WarningLog))
	}
	if !contains(severity.WarningLog, "test") {
		t.Error("Warning failed")
	}
	str := contents(severity.WarningLog)
	if !contains(severity.InfoLog, str) {
		t.Error("Info failed")
	}
}

// Test that a Warning log does not goes to Info.
// Even in the Info log, the source character will be W, so the data should
// all be identical.
func TestWarningWithOneOutput(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	logging.oneOutput = true
	defer logging.swap(logging.newBuffers())
	Warning("test")
	if !contains(severity.WarningLog, "W") {
		t.Errorf("Warning has wrong character: %q", contents(severity.WarningLog))
	}
	if !contains(severity.WarningLog, "test") {
		t.Error("Warning failed")
	}
	str := contents(severity.WarningLog)
	if contains(severity.InfoLog, str) {
		t.Error("Info failed")
	}
}

// Test that a V log goes to Info.
func TestV(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	require.NoError(t, logging.verbosity.Set("2"))
	V(2).Info("test")
	if !contains(severity.InfoLog, "I") {
		t.Errorf("Info has wrong character: %q", contents(severity.InfoLog))
	}
	if !contains(severity.InfoLog, "test") {
		t.Error("Info failed")
	}
}

// Test that a vmodule enables a log in this file.
func TestVmoduleOn(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	require.NoError(t, logging.vmodule.Set("klog_test=2"))
	if !V(1).Enabled() {
		t.Error("V not enabled for 1")
	}
	if !V(2).Enabled() {
		t.Error("V not enabled for 2")
	}
	if V(3).Enabled() {
		t.Error("V enabled for 3")
	}
	V(2).Info("test")
	if !contains(severity.InfoLog, "I") {
		t.Errorf("Info has wrong character: %q", contents(severity.InfoLog))
	}
	if !contains(severity.InfoLog, "test") {
		t.Error("Info failed")
	}
}

// Test that a vmodule of another file does not enable a log in this file.
func TestVmoduleOff(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	require.NoError(t, logging.vmodule.Set("notthisfile=2"))
	for i := 1; i <= 3; i++ {
		if V(Level(i)).Enabled() {
			t.Errorf("V enabled for %d", i)
		}
	}
	V(2).Info("test")
	if contents(severity.InfoLog) != "" {
		t.Error("V logged incorrectly")
	}
}

func TestSetOutputDataRace(*testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	var wg sync.WaitGroup
	var daemons []*flushDaemon
	for i := 1; i <= 50; i++ {
		daemon := newFlushDaemon(logging.lockAndFlushAll, nil)
		daemon.run(time.Second)
		daemons = append(daemons, daemon)
	}
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetOutput(ioutil.Discard)
		}()
	}
	for i := 1; i <= 50; i++ {
		daemon := newFlushDaemon(logging.lockAndFlushAll, nil)
		daemon.run(time.Second)
		daemons = append(daemons, daemon)
	}
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetOutputBySeverity("INFO", ioutil.Discard)
		}()
	}
	for i := 1; i <= 50; i++ {
		daemon := newFlushDaemon(logging.lockAndFlushAll, nil)
		daemon.run(time.Second)
		daemons = append(daemons, daemon)
	}
	wg.Wait()
	for _, d := range daemons {
		d.stop()
	}
}

func TestLogToOutput(t *testing.T) {
	defer CaptureState().Restore()
	logging.toStderr = true
	defer logging.swap(logging.newBuffers())
	buf := new(bytes.Buffer)
	SetOutput(buf)
	LogToStderr(false)

	Info("Does logging to an output work?")

	str := buf.String()
	if !strings.Contains(str, "Does logging to an output work?") {
		t.Fatalf("Expected %q to contain \"Does logging to an output work?\"", str)
	}
}

// vGlobs are patterns that match/don't match this file at V=2.
var vGlobs = map[string]bool{
	// Easy to test the numeric match here.
	"klog_test=1": false, // If -vmodule sets V to 1, V(2) will fail.
	"klog_test=2": true,
	"klog_test=3": true, // If -vmodule sets V to 1, V(3) will succeed.
	// These all use 2 and check the patterns. All are true.
	"*=2":           true,
	"?l*=2":         true,
	"????_*=2":      true,
	"??[mno]?_*t=2": true,
	// These all use 2 and check the patterns. All are false.
	"*x=2":         false,
	"m*=2":         false,
	"??_*=2":       false,
	"?[abc]?_*t=2": false,
}

// Test that vmodule globbing works as advertised.
func testVmoduleGlob(pat string, match bool, t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	require.NoError(t, logging.vmodule.Set(pat))
	if V(2).Enabled() != match {
		t.Errorf("incorrect match for %q: got %#v expected %#v", pat, V(2), match)
	}
}

// Test that a vmodule globbing works as advertised.
func TestVmoduleGlob(t *testing.T) {
	for glob, match := range vGlobs {
		testVmoduleGlob(glob, match, t)
	}
}

func TestRollover(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	var err error
	defer func(previous func(error)) { logExitFunc = previous }(logExitFunc)
	logExitFunc = func(e error) {
		err = e
	}
	MaxSize = 512
	Info("x") // Be sure we have a file.
	info, ok := logging.file[severity.InfoLog].(*syncBuffer)
	if !ok {
		t.Fatal("info wasn't created")
	}
	if err != nil {
		t.Fatalf("info has initial error: %v", err)
	}
	fname0 := info.file.Name()
	Info(strings.Repeat("x", int(MaxSize))) // force a rollover
	if err != nil {
		t.Fatalf("info has error after big write: %v", err)
	}

	// Make sure the next log file gets a file name with a different
	// time stamp.
	//
	// TODO: determine whether we need to support subsecond log
	// rotation.  C++ does not appear to handle this case (nor does it
	// handle Daylight Savings Time properly).
	time.Sleep(1 * time.Second)

	Info("x") // create a new file
	if err != nil {
		t.Fatalf("error after rotation: %v", err)
	}
	fname1 := info.file.Name()
	if fname0 == fname1 {
		t.Errorf("info.f.Name did not change: %v", fname0)
	}
	if info.nbytes >= info.maxbytes {
		t.Errorf("file size was not reset: %d", info.nbytes)
	}
}

func TestOpenAppendOnStart(t *testing.T) {
	const (
		x string = "xxxxxxxxxx"
		y string = "yyyyyyyyyy"
	)

	defer CaptureState().Restore()
	setFlags()
	var err error
	defer func(previous func(error)) { logExitFunc = previous }(logExitFunc)
	logExitFunc = func(e error) {
		err = e
	}

	f, err := ioutil.TempFile("", "test_klog_OpenAppendOnStart")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(f.Name())
	logging.logFile = f.Name()

	// Erase files created by prior tests,
	for i := range logging.file {
		logging.file[i] = nil
	}

	// Logging creates the file
	Info(x)
	sb, ok := logging.file[severity.InfoLog].(*syncBuffer)
	if !ok {
		t.Fatal("info wasn't created")
	}

	// ensure we wrote what we expected
	needToSync := logging.flushAll()
	if needToSync.num != 1 || needToSync.files[0] != sb.file {
		t.Errorf("Should have received exactly the file from severity.InfoLog for syncing, got instead: %+v", needToSync)
	}
	logging.syncAll(needToSync)
	b, err := ioutil.ReadFile(logging.logFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(b), x) {
		t.Fatalf("got %s, missing expected Info log: %s", string(b), x)
	}

	// Set the file to nil so it gets "created" (opened) again on the next write.
	for i := range logging.file {
		logging.file[i] = nil
	}

	// Logging again should open the file again with O_APPEND instead of O_TRUNC
	Info(y)
	// ensure we wrote what we expected
	logging.lockAndFlushAll()
	b, err = ioutil.ReadFile(logging.logFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(b), y) {
		t.Fatalf("got %s, missing expected Info log: %s", string(b), y)
	}
	// The initial log message should be preserved across create calls.
	logging.lockAndFlushAll()
	b, err = ioutil.ReadFile(logging.logFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(b), x) {
		t.Fatalf("got %s, missing expected Info log: %s", string(b), x)
	}
}

func TestLogBacktraceAt(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	// The peculiar style of this code simplifies line counting and maintenance of the
	// tracing block below.
	var infoLine string
	setTraceLocation := func(file string, line int, ok bool, delta int) {
		if !ok {
			t.Fatal("could not get file:line")
		}
		_, file = filepath.Split(file)
		infoLine = fmt.Sprintf("%s:%d", file, line+delta)
		err := logging.traceLocation.Set(infoLine)
		if err != nil {
			t.Fatal("error setting log_backtrace_at: ", err)
		}
	}
	{
		// Start of tracing block. These lines know about each other's relative position.
		_, file, line, ok := runtime.Caller(0)
		setTraceLocation(file, line, ok, +2) // Two lines between Caller and Info calls.
		Info("we want a stack trace here")
	}
	numAppearances := strings.Count(contents(severity.InfoLog), infoLine)
	if numAppearances < 2 {
		// Need 2 appearances, one in the log header and one in the trace:
		//   log_test.go:281: I0511 16:36:06.952398 02238 log_test.go:280] we want a stack trace here
		//   ...
		//   k8s.io/klog/klog_test.go:280 (0x41ba91)
		//   ...
		// We could be more precise but that would require knowing the details
		// of the traceback format, which may not be dependable.
		t.Fatal("got no trace back; log is ", contents(severity.InfoLog))
	}
}

func BenchmarkHeader(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf, _, _ := logging.header(severity.InfoLog, 0)
		buffer.PutBuffer(buf)
	}
}

func BenchmarkHeaderWithDir(b *testing.B) {
	logging.addDirHeader = true
	for i := 0; i < b.N; i++ {
		buf, _, _ := logging.header(severity.InfoLog, 0)
		buffer.PutBuffer(buf)
	}
}

// Ensure that benchmarks have side effects to avoid compiler optimization
var result interface{}
var enabled bool

func BenchmarkV(b *testing.B) {
	var v Verbose
	for i := 0; i < b.N; i++ {
		v = V(10)
	}
	enabled = v.Enabled()
}

func BenchmarkKRef(b *testing.B) {
	var r ObjectRef
	for i := 0; i < b.N; i++ {
		r = KRef("namespace", "name")
	}
	result = r
}

func BenchmarkKObj(b *testing.B) {
	a := test.KMetadataMock{Name: "a", NS: "a"}
	var r ObjectRef
	for i := 0; i < b.N; i++ {
		r = KObj(&a)
	}
	result = r
}

// BenchmarkKObjs measures the (pretty typical) case
// where KObjs is used in a V(5).InfoS call that never
// emits a log entry because verbosity is lower than 5.
// For performance when the result of KObjs gets formatted,
// see examples/benchmarks.
//
// This uses two different patterns:
// - directly calling klog.V(5).Info
// - guarding the call with Enabled
func BenchmarkKObjs(b *testing.B) {
	for length := 0; length <= 100; length += 10 {
		b.Run(fmt.Sprintf("%d", length), func(b *testing.B) {
			arg := make([]interface{}, length)
			for i := 0; i < length; i++ {
				arg[i] = test.KMetadataMock{Name: "a", NS: "a"}
			}

			b.Run("simple", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					V(5).InfoS("benchmark", "objs", KObjs(arg))
				}
			})

			b.Run("conditional", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if klogV := V(5); klogV.Enabled() {
						klogV.InfoS("benchmark", "objs", KObjs(arg))
					}
				}
			})
		})
	}
}

// BenchmarkKObjSlice corresponds to BenchmarkKObjs except that it uses KObjSlice
func BenchmarkKObjSlice(b *testing.B) {
	for length := 0; length <= 100; length += 10 {
		b.Run(fmt.Sprintf("%d", length), func(b *testing.B) {
			arg := make([]interface{}, length)
			for i := 0; i < length; i++ {
				arg[i] = test.KMetadataMock{Name: "a", NS: "a"}
			}

			b.Run("simple", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					V(5).InfoS("benchmark", "objs", KObjSlice(arg))
				}
			})

			b.Run("conditional", func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if klogV := V(5); klogV.Enabled() {
						klogV.InfoS("benchmark", "objs", KObjSlice(arg))
					}
				}
			})
		})
	}
}

// BenchmarkScalars corresponds to BenchmarkKObjs except that it avoids function
// calls for the parameters.
func BenchmarkScalars(b *testing.B) {
	b.Run("simple", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			V(5).InfoS("benchmark", "str", "hello world", "int", 42)
		}
	})

	b.Run("conditional", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if klogV := V(5); klogV.Enabled() {
				klogV.InfoS("benchmark", "str", "hello world", "int", 42)
			}
		}
	})
}

// BenchmarkScalarsWithLogger is the same as BenchmarkScalars except that it uses
// a go-logr instance.
func BenchmarkScalarsWithLogger(b *testing.B) {
	logger := Background()
	b.Run("simple", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.V(5).Info("benchmark", "str", "hello world", "int", 42)
		}
	})

	b.Run("conditional", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if loggerV := logger.V(5); loggerV.Enabled() {
				loggerV.Info("benchmark", "str", "hello world", "int", 42)
			}
		}
	})
}

// BenchmarKObjSliceWithLogger is the same as BenchmarkKObjSlice except that it
// uses a go-logr instance and a slice with a single entry. BenchmarkKObjSlice
// shows that the overhead for KObjSlice is constant and doesn't depend on the
// slice length when logging is off.
func BenchmarkKObjSliceWithLogger(b *testing.B) {
	logger := Background()
	arg := []interface{}{test.KMetadataMock{Name: "a", NS: "a"}}
	b.Run("simple", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.V(5).Info("benchmark", "objs", KObjSlice(arg))
		}
	})

	b.Run("conditional", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if loggerV := logger.V(5); loggerV.Enabled() {
				loggerV.Info("benchmark", "objs", KObjSlice(arg))
			}
		}
	})
}

func BenchmarkLogs(b *testing.B) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())

	testFile, err := ioutil.TempFile("", "test.log")
	if err != nil {
		b.Fatal("unable to create temporary file")
	}
	defer os.Remove(testFile.Name())

	require.NoError(b, logging.verbosity.Set("0"))
	logging.toStderr = false
	logging.alsoToStderr = false
	logging.stderrThreshold = severityValue{
		Severity: severity.FatalLog,
	}
	logging.logFile = testFile.Name()
	logging.swap([severity.NumSeverity]io.Writer{nil, nil, nil, nil})

	for i := 0; i < b.N; i++ {
		Error("error")
		Warning("warning")
		Info("info")
	}
	needToSync := logging.flushAll()
	sb, ok := logging.file[severity.InfoLog].(*syncBuffer)
	if !ok {
		b.Fatal("info wasn't created")
	}
	if needToSync.num != 1 || needToSync.files[0] != sb.file {
		b.Fatalf("Should have received exactly the file from severity.InfoLog for syncing, got instead: %+v", needToSync)
	}
	logging.syncAll(needToSync)
}

func BenchmarkFlush(b *testing.B) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())

	testFile, err := ioutil.TempFile("", "test.log")
	if err != nil {
		b.Fatal("unable to create temporary file")
	}
	defer os.Remove(testFile.Name())

	require.NoError(b, logging.verbosity.Set("0"))
	logging.toStderr = false
	logging.alsoToStderr = false
	logging.stderrThreshold = severityValue{
		Severity: severity.FatalLog,
	}
	logging.logFile = testFile.Name()
	logging.swap([severity.NumSeverity]io.Writer{nil, nil, nil, nil})

	// Create output file.
	Info("info")
	needToSync := logging.flushAll()

	if needToSync.num != 1 {
		b.Fatalf("expected exactly one file to sync, got: %+v", needToSync)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		needToSync := logging.flushAll()
		logging.syncAll(needToSync)
	}
}

// Test the logic on checking log size limitation.
func TestFileSizeCheck(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	testData := map[string]struct {
		testLogFile          string
		testLogFileMaxSizeMB uint64
		testCurrentSize      uint64
		expectedResult       bool
	}{
		"logFile not specified, exceeds max size": {
			testLogFile:          "",
			testLogFileMaxSizeMB: 1,
			testCurrentSize:      1024 * 1024 * 2000, //exceeds the maxSize
			expectedResult:       true,
		},

		"logFile not specified, not exceeds max size": {
			testLogFile:          "",
			testLogFileMaxSizeMB: 1,
			testCurrentSize:      1024 * 1024 * 1000, //smaller than the maxSize
			expectedResult:       false,
		},
		"logFile specified, exceeds max size": {
			testLogFile:          "/tmp/test.log",
			testLogFileMaxSizeMB: 500,                // 500MB
			testCurrentSize:      1024 * 1024 * 1000, //exceeds the logFileMaxSizeMB
			expectedResult:       true,
		},
		"logFile specified, not exceeds max size": {
			testLogFile:          "/tmp/test.log",
			testLogFileMaxSizeMB: 500,               // 500MB
			testCurrentSize:      1024 * 1024 * 300, //smaller than the logFileMaxSizeMB
			expectedResult:       false,
		},
	}

	for name, test := range testData {
		logging.logFile = test.testLogFile
		logging.logFileMaxSizeMB = test.testLogFileMaxSizeMB
		actualResult := test.testCurrentSize >= CalculateMaxSize()
		if test.expectedResult != actualResult {
			t.Fatalf("Error on test case '%v': Was expecting result equals %v, got %v",
				name, test.expectedResult, actualResult)
		}
	}
}

func TestInitFlags(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()

	fs1 := flag.NewFlagSet("test1", flag.PanicOnError)
	InitFlags(fs1)
	require.NoError(t, fs1.Set("log_dir", "/test1"))
	require.NoError(t, fs1.Set("log_file_max_size", "1"))
	fs2 := flag.NewFlagSet("test2", flag.PanicOnError)
	InitFlags(fs2)
	if logging.logDir != "/test1" {
		t.Fatalf("Expected log_dir to be %q, got %q", "/test1", logging.logDir)
	}
	require.NoError(t, fs2.Set("log_file_max_size", "2048"))
	if logging.logFileMaxSizeMB != 2048 {
		t.Fatal("Expected log_file_max_size to be 2048")
	}
}

func TestCommandLine(t *testing.T) {
	var fs flag.FlagSet
	InitFlags(&fs)

	expectedFlags := `  -add_dir_header
    	If true, adds the file directory to the header of the log messages
  -alsologtostderr
    	log to standard error as well as files (no effect when -logtostderr=true)
  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory (no effect when -logtostderr=true)
  -log_file string
    	If non-empty, use this log file (no effect when -logtostderr=true)
  -log_file_max_size uint
    	Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
  -logtostderr
    	log to standard error instead of files (default true)
  -one_output
    	If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
  -skip_headers
    	If true, avoid header prefixes in the log messages
  -skip_log_headers
    	If true, avoid headers when opening log files (no effect when -logtostderr=true)
  -stderrthreshold value
    	logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -v value
    	number for the log level verbosity
  -vmodule value
    	comma-separated list of pattern=N settings for file-filtered logging
`

	var output bytes.Buffer
	fs.SetOutput(&output)
	fs.PrintDefaults()
	actualFlags := output.String()

	if expectedFlags != actualFlags {
		t.Fatalf("Command line changed.\nExpected:\n%q\nActual:\n%q\n", expectedFlags, actualFlags)
	}
}

func TestInfoObjectRef(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())

	tests := []struct {
		name string
		ref  ObjectRef
		want string
	}{
		{
			name: "with ns",
			ref: ObjectRef{
				Name:      "test-name",
				Namespace: "test-ns",
			},
			want: "test-ns/test-name",
		},
		{
			name: "without ns",
			ref: ObjectRef{
				Name:      "test-name",
				Namespace: "",
			},
			want: "test-name",
		},
		{
			name: "empty",
			ref:  ObjectRef{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Info(tt.ref)
			if !contains(severity.InfoLog, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, contents(severity.InfoLog))
			}
		})
	}
}

func TestKObj(t *testing.T) {
	tests := []struct {
		name string
		obj  KMetadata
		want ObjectRef
	}{
		{
			name: "nil passed as pointer KMetadata implementation",
			obj:  (*test.PtrKMetadataMock)(nil),
			want: ObjectRef{},
		},
		{
			name: "empty struct passed as non-pointer KMetadata implementation",
			obj:  test.KMetadataMock{},
			want: ObjectRef{},
		},
		{
			name: "nil pointer passed to non-pointer KMetadata implementation",
			obj:  (*test.KMetadataMock)(nil),
			want: ObjectRef{},
		},
		{
			name: "nil",
			obj:  nil,
			want: ObjectRef{},
		},
		{
			name: "with ns",
			obj:  &test.KMetadataMock{Name: "test-name", NS: "test-ns"},
			want: ObjectRef{
				Name:      "test-name",
				Namespace: "test-ns",
			},
		},
		{
			name: "without ns",
			obj:  &test.KMetadataMock{Name: "test-name", NS: ""},
			want: ObjectRef{
				Name: "test-name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if KObj(tt.obj) != tt.want {
				t.Errorf("expected %v, got %v", tt.want, KObj(tt.obj))
			}
		})
	}
}

func TestKRef(t *testing.T) {
	tests := []struct {
		testname  string
		name      string
		namespace string
		want      ObjectRef
	}{
		{
			testname:  "with ns",
			name:      "test-name",
			namespace: "test-ns",
			want: ObjectRef{
				Name:      "test-name",
				Namespace: "test-ns",
			},
		},
		{
			testname: "without ns",
			name:     "test-name",
			want: ObjectRef{
				Name: "test-name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.testname, func(t *testing.T) {
			if KRef(tt.namespace, tt.name) != tt.want {
				t.Errorf("expected %v, got %v", tt.want, KRef(tt.namespace, tt.name))
			}
		})
	}
}

// Test that InfoS and InfoSDepth work as advertised.
func TestInfoS(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	timeNow = func() time.Time {
		return time.Date(2006, 1, 2, 15, 4, 5, .067890e9, time.Local)
	}
	pid = 1234
	var testDataInfo = []struct {
		msg        string
		format     string
		keysValues []interface{}
	}{
		{
			msg:        "test",
			format:     "I0102 15:04:05.067890    1234 klog_test.go:%d] \"test\" pod=\"kubedns\"\n",
			keysValues: []interface{}{"pod", "kubedns"},
		},
		{
			msg:        "test",
			format:     "I0102 15:04:05.067890    1234 klog_test.go:%d] \"test\" replicaNum=20\n",
			keysValues: []interface{}{"replicaNum", 20},
		},
		{
			msg:        "test",
			format:     "I0102 15:04:05.067890    1234 klog_test.go:%d] \"test\" err=\"test error\"\n",
			keysValues: []interface{}{"err", errors.New("test error")},
		},
		{
			msg:        "test",
			format:     "I0102 15:04:05.067890    1234 klog_test.go:%d] \"test\" err=\"test error\"\n",
			keysValues: []interface{}{"err", errors.New("test error")},
		},
	}

	functions := []func(msg string, keyAndValues ...interface{}){
		InfoS,
		myInfoS,
	}
	for _, f := range functions {
		for _, data := range testDataInfo {
			logging.file[severity.InfoLog] = &flushBuffer{}
			f(data.msg, data.keysValues...)
			var line int
			n, err := fmt.Sscanf(contents(severity.InfoLog), data.format, &line)
			if n != 1 || err != nil {
				t.Errorf("log format error: %d elements, error %s:\n%s", n, err, contents(severity.InfoLog))
			}
			want := fmt.Sprintf(data.format, line)
			if contents(severity.InfoLog) != want {
				t.Errorf("InfoS has wrong format: \n got:\t%s\nwant:\t%s", contents(severity.InfoLog), want)
			}
		}
	}
}

// Test that Verbose.InfoS works as advertised.
func TestVInfoS(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	timeNow = func() time.Time {
		return time.Date(2006, 1, 2, 15, 4, 5, .067890e9, time.Local)
	}
	pid = 1234
	myData := struct {
		Data string
	}{
		Data: `This is some long text
with a line break.`,
	}
	var testDataInfo = []struct {
		msg        string
		format     string
		keysValues []interface{}
	}{
		{
			msg:        "test",
			format:     "I0102 15:04:05.067890    1234 klog_test.go:%d] \"test\" pod=\"kubedns\"\n",
			keysValues: []interface{}{"pod", "kubedns"},
		},
		{
			msg:        "test",
			format:     "I0102 15:04:05.067890    1234 klog_test.go:%d] \"test\" replicaNum=20\n",
			keysValues: []interface{}{"replicaNum", 20},
		},
		{
			msg:        "test",
			format:     "I0102 15:04:05.067890    1234 klog_test.go:%d] \"test\" err=\"test error\"\n",
			keysValues: []interface{}{"err", errors.New("test error")},
		},
		{
			msg: `first message line
second message line`,
			format: `I0102 15:04:05.067890    1234 klog_test.go:%d] "first message line\nsecond message line" multiLine=<
	first value line
	second value line
 >
`,
			keysValues: []interface{}{"multiLine", `first value line
second value line`},
		},
		{
			msg: `message`,
			format: `I0102 15:04:05.067890    1234 klog_test.go:%d] "message" myData={"Data":"This is some long text\nwith a line break."}
`,
			keysValues: []interface{}{"myData", myData},
		},
	}

	require.NoError(t, logging.verbosity.Set("2"))

	for l := Level(0); l < Level(4); l++ {
		for _, data := range testDataInfo {
			logging.file[severity.InfoLog] = &flushBuffer{}

			V(l).InfoS(data.msg, data.keysValues...)

			var want string
			var line int
			if l <= 2 {
				n, err := fmt.Sscanf(contents(severity.InfoLog), data.format, &line)
				if n != 1 || err != nil {
					t.Errorf("log format error: %d elements, error %s:\n%s", n, err, contents(severity.InfoLog))
				}

				want = fmt.Sprintf(data.format, line)
			} else {
				want = ""
			}
			if contents(severity.InfoLog) != want {
				t.Errorf("V(%d).InfoS has unexpected output:\ngot:\n%s\nwant:\n%s\n", l, contents(severity.InfoLog), want)
			}
		}
	}
}

// Test that ErrorS and ErrorSDepth work as advertised.
func TestErrorS(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	timeNow = func() time.Time {
		return time.Date(2006, 1, 2, 15, 4, 5, .067890e9, time.Local)
	}
	logging.logFile = ""
	pid = 1234

	functions := []func(err error, msg string, keyAndValues ...interface{}){
		ErrorS,
		myErrorS,
	}
	for _, f := range functions {
		var errorList = []struct {
			err    error
			format string
		}{
			{
				err:    fmt.Errorf("update status failed"),
				format: "E0102 15:04:05.067890    1234 klog_test.go:%d] \"Failed to update pod status\" err=\"update status failed\" pod=\"kubedns\"\n",
			},
			{
				err:    nil,
				format: "E0102 15:04:05.067890    1234 klog_test.go:%d] \"Failed to update pod status\" pod=\"kubedns\"\n",
			},
		}
		for _, e := range errorList {
			logging.file[severity.ErrorLog] = &flushBuffer{}
			f(e.err, "Failed to update pod status", "pod", "kubedns")
			var line int
			n, err := fmt.Sscanf(contents(severity.ErrorLog), e.format, &line)
			if n != 1 || err != nil {
				t.Errorf("log format error: %d elements, error %s:\n%s", n, err, contents(severity.ErrorLog))
			}
			want := fmt.Sprintf(e.format, line)
			if contents(severity.ErrorLog) != want {
				t.Errorf("ErrorS has wrong format:\ngot:\n%s\nwant:\n%s\n", contents(severity.ErrorLog), want)
			}
		}
	}
}

func createTestValueOfLoggingT() *loggingT {
	l := new(loggingT)
	l.toStderr = true
	l.alsoToStderr = false
	l.stderrThreshold = severityValue{
		Severity: severity.ErrorLog,
	}
	l.verbosity = Level(0)
	l.skipHeaders = false
	l.skipLogHeaders = false
	l.addDirHeader = false
	return l
}

func createTestValueOfModulePat(p string, li bool, le Level) modulePat {
	m := modulePat{}
	m.pattern = p
	m.literal = li
	m.level = le
	return m
}

func compareModuleSpec(a, b moduleSpec) bool {
	if len(a.filter) != len(b.filter) {
		return false
	}

	for i := 0; i < len(a.filter); i++ {
		if a.filter[i] != b.filter[i] {
			return false
		}
	}

	return true
}

func TestSetVState(t *testing.T) {
	//Target loggingT value
	want := createTestValueOfLoggingT()
	want.verbosity = Level(3)
	want.vmodule.filter = []modulePat{
		createTestValueOfModulePat("recordio", true, Level(2)),
		createTestValueOfModulePat("file", true, Level(1)),
		createTestValueOfModulePat("gfs*", false, Level(3)),
		createTestValueOfModulePat("gopher*", false, Level(3)),
	}
	want.filterLength = 4

	//loggingT value to which test is run
	target := createTestValueOfLoggingT()

	tf := []modulePat{
		createTestValueOfModulePat("recordio", true, Level(2)),
		createTestValueOfModulePat("file", true, Level(1)),
		createTestValueOfModulePat("gfs*", false, Level(3)),
		createTestValueOfModulePat("gopher*", false, Level(3)),
	}

	target.setVState(Level(3), tf, true)

	if want.verbosity != target.verbosity || !compareModuleSpec(want.vmodule, target.vmodule) || want.filterLength != target.filterLength {
		t.Errorf("setVState method doesn't configure loggingT values' verbosity, vmodule or filterLength:\nwant:\n\tverbosity:\t%v\n\tvmodule:\t%v\n\tfilterLength:\t%v\ngot:\n\tverbosity:\t%v\n\tvmodule:\t%v\n\tfilterLength:\t%v", want.verbosity, want.vmodule, want.filterLength, target.verbosity, target.vmodule, target.filterLength)
	}
}

type sampleLogFilter struct{}

func (f *sampleLogFilter) Filter(args []interface{}) []interface{} {
	for i, arg := range args {
		v, ok := arg.(string)
		if ok && strings.Contains(v, "filter me") {
			args[i] = "[FILTERED]"
		}
	}
	return args
}

func (f *sampleLogFilter) FilterF(format string, args []interface{}) (string, []interface{}) {
	return strings.Replace(format, "filter me", "[FILTERED]", 1), f.Filter(args)
}

func (f *sampleLogFilter) FilterS(msg string, keysAndValues []interface{}) (string, []interface{}) {
	return strings.Replace(msg, "filter me", "[FILTERED]", 1), f.Filter(keysAndValues)
}

func TestLogFilter(t *testing.T) {
	defer CaptureState().Restore()
	setFlags()
	defer logging.swap(logging.newBuffers())
	SetLogFilter(&sampleLogFilter{})
	funcs := []struct {
		name     string
		logFunc  func(args ...interface{})
		severity severity.Severity
	}{{
		name:     "Info",
		logFunc:  Info,
		severity: severity.InfoLog,
	}, {
		name: "InfoDepth",
		logFunc: func(args ...interface{}) {
			InfoDepth(1, args...)
		},
		severity: severity.InfoLog,
	}, {
		name:     "Infoln",
		logFunc:  Infoln,
		severity: severity.InfoLog,
	}, {
		name: "Infof",
		logFunc: func(args ...interface{}) {

			Infof(args[0].(string), args[1:]...)
		},
		severity: severity.InfoLog,
	}, {
		name: "InfoS",
		logFunc: func(args ...interface{}) {
			InfoS(args[0].(string), args[1:]...)
		},
		severity: severity.InfoLog,
	}, {
		name:     "Warning",
		logFunc:  Warning,
		severity: severity.WarningLog,
	}, {
		name: "WarningDepth",
		logFunc: func(args ...interface{}) {
			WarningDepth(1, args...)
		},
		severity: severity.WarningLog,
	}, {
		name:     "Warningln",
		logFunc:  Warningln,
		severity: severity.WarningLog,
	}, {
		name: "Warningf",
		logFunc: func(args ...interface{}) {
			Warningf(args[0].(string), args[1:]...)
		},
		severity: severity.WarningLog,
	}, {
		name:     "Error",
		logFunc:  Error,
		severity: severity.ErrorLog,
	}, {
		name: "ErrorDepth",
		logFunc: func(args ...interface{}) {
			ErrorDepth(1, args...)
		},
		severity: severity.ErrorLog,
	}, {
		name:     "Errorln",
		logFunc:  Errorln,
		severity: severity.ErrorLog,
	}, {
		name: "Errorf",
		logFunc: func(args ...interface{}) {
			Errorf(args[0].(string), args[1:]...)
		},
		severity: severity.ErrorLog,
	}, {
		name: "ErrorS",
		logFunc: func(args ...interface{}) {
			ErrorS(errors.New("testerror"), args[0].(string), args[1:]...)
		},
		severity: severity.ErrorLog,
	}, {
		name: "V().Info",
		logFunc: func(args ...interface{}) {
			V(0).Info(args...)
		},
		severity: severity.InfoLog,
	}, {
		name: "V().Infoln",
		logFunc: func(args ...interface{}) {
			V(0).Infoln(args...)
		},
		severity: severity.InfoLog,
	}, {
		name: "V().Infof",
		logFunc: func(args ...interface{}) {
			V(0).Infof(args[0].(string), args[1:]...)
		},
		severity: severity.InfoLog,
	}, {
		name: "V().InfoS",
		logFunc: func(args ...interface{}) {
			V(0).InfoS(args[0].(string), args[1:]...)
		},
		severity: severity.InfoLog,
	}, {
		name: "V().Error",
		logFunc: func(args ...interface{}) {
			V(0).Error(errors.New("test error"), args[0].(string), args[1:]...)
		},
		severity: severity.ErrorLog,
	}, {
		name: "V().ErrorS",
		logFunc: func(args ...interface{}) {
			V(0).ErrorS(errors.New("test error"), args[0].(string), args[1:]...)
		},
		severity: severity.ErrorLog,
	}}

	testcases := []struct {
		name           string
		args           []interface{}
		expectFiltered bool
	}{{
		args:           []interface{}{"%s:%s", "foo", "bar"},
		expectFiltered: false,
	}, {
		args:           []interface{}{"%s:%s", "foo", "filter me"},
		expectFiltered: true,
	}, {
		args:           []interface{}{"filter me %s:%s", "foo", "bar"},
		expectFiltered: true,
	}}

	for _, f := range funcs {
		for _, tc := range testcases {
			logging.newBuffers()
			f.logFunc(tc.args...)
			got := contains(f.severity, "[FILTERED]")
			if got != tc.expectFiltered {
				t.Errorf("%s filter application failed, got %v, want %v", f.name, got, tc.expectFiltered)
			}
		}
	}
}

func TestInfoWithLogr(t *testing.T) {
	logger := new(testLogr)

	testDataInfo := []struct {
		msg      string
		expected testLogrEntry
	}{{
		msg: "foo",
		expected: testLogrEntry{
			severity: severity.InfoLog,
			msg:      "foo",
		},
	}, {
		msg: "",
		expected: testLogrEntry{
			severity: severity.InfoLog,
			msg:      "",
		},
	}}

	for _, data := range testDataInfo {
		t.Run(data.msg, func(t *testing.T) {
			l := logr.New(logger)
			defer CaptureState().Restore()
			SetLogger(l)
			defer logger.reset()

			Info(data.msg)

			if !reflect.DeepEqual(logger.entries, []testLogrEntry{data.expected}) {
				t.Errorf("expected: %+v; but got: %+v", []testLogrEntry{data.expected}, logger.entries)
			}
		})
	}
}

func TestInfoSWithLogr(t *testing.T) {
	logger := new(testLogr)

	testDataInfo := []struct {
		msg        string
		keysValues []interface{}
		expected   testLogrEntry
	}{{
		msg:        "foo",
		keysValues: []interface{}{},
		expected: testLogrEntry{
			severity:      severity.InfoLog,
			msg:           "foo",
			keysAndValues: []interface{}{},
		},
	}, {
		msg:        "bar",
		keysValues: []interface{}{"a", 1},
		expected: testLogrEntry{
			severity:      severity.InfoLog,
			msg:           "bar",
			keysAndValues: []interface{}{"a", 1},
		},
	}}

	for _, data := range testDataInfo {
		t.Run(data.msg, func(t *testing.T) {
			defer CaptureState().Restore()
			l := logr.New(logger)
			SetLogger(l)
			defer logger.reset()

			InfoS(data.msg, data.keysValues...)

			if !reflect.DeepEqual(logger.entries, []testLogrEntry{data.expected}) {
				t.Errorf("expected: %+v; but got: %+v", []testLogrEntry{data.expected}, logger.entries)
			}
		})
	}
}

func TestErrorSWithLogr(t *testing.T) {
	logger := new(testLogr)

	testError := errors.New("testError")

	testDataInfo := []struct {
		err        error
		msg        string
		keysValues []interface{}
		expected   testLogrEntry
	}{{
		err:        testError,
		msg:        "foo1",
		keysValues: []interface{}{},
		expected: testLogrEntry{
			severity:      severity.ErrorLog,
			msg:           "foo1",
			keysAndValues: []interface{}{},
			err:           testError,
		},
	}, {
		err:        testError,
		msg:        "bar1",
		keysValues: []interface{}{"a", 1},
		expected: testLogrEntry{
			severity:      severity.ErrorLog,
			msg:           "bar1",
			keysAndValues: []interface{}{"a", 1},
			err:           testError,
		},
	}, {
		err:        nil,
		msg:        "foo2",
		keysValues: []interface{}{},
		expected: testLogrEntry{
			severity:      severity.ErrorLog,
			msg:           "foo2",
			keysAndValues: []interface{}{},
			err:           nil,
		},
	}, {
		err:        nil,
		msg:        "bar2",
		keysValues: []interface{}{"a", 1},
		expected: testLogrEntry{
			severity:      severity.ErrorLog,
			msg:           "bar2",
			keysAndValues: []interface{}{"a", 1},
			err:           nil,
		},
	}}

	for _, data := range testDataInfo {
		t.Run(data.msg, func(t *testing.T) {
			defer CaptureState().Restore()
			l := logr.New(logger)
			SetLogger(l)
			defer logger.reset()

			ErrorS(data.err, data.msg, data.keysValues...)

			if !reflect.DeepEqual(logger.entries, []testLogrEntry{data.expected}) {
				t.Errorf("expected: %+v; but got: %+v", []testLogrEntry{data.expected}, logger.entries)
			}
		})
	}
}

func TestCallDepthLogr(t *testing.T) {
	logger := &callDepthTestLogr{}
	logger.resetCallDepth()

	testCases := []struct {
		name  string
		logFn func()
	}{
		{
			name:  "Info log",
			logFn: func() { Info("info log") },
		},
		{
			name:  "InfoDepth log",
			logFn: func() { InfoDepth(0, "infodepth log") },
		},
		{
			name:  "InfoSDepth log",
			logFn: func() { InfoSDepth(0, "infoSDepth log") },
		},
		{
			name:  "Warning log",
			logFn: func() { Warning("warning log") },
		},
		{
			name:  "WarningDepth log",
			logFn: func() { WarningDepth(0, "warningdepth log") },
		},
		{
			name:  "Error log",
			logFn: func() { Error("error log") },
		},
		{
			name:  "ErrorDepth log",
			logFn: func() { ErrorDepth(0, "errordepth log") },
		},
		{
			name:  "ErrorSDepth log",
			logFn: func() { ErrorSDepth(0, errors.New("some error"), "errorSDepth log") },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := logr.New(logger)
			defer ClearLogger()
			SetLogger(l)
			defer logger.reset()
			defer logger.resetCallDepth()

			// Keep these lines together.
			_, wantFile, wantLine, _ := runtime.Caller(0)
			tc.logFn()
			wantLine++

			if len(logger.entries) != 1 {
				t.Errorf("expected a single log entry to be generated, got %d", len(logger.entries))
			}
			checkLogrEntryCorrectCaller(t, wantFile, wantLine, logger.entries[0])
		})
	}
}

func TestCallDepthLogrInfoS(t *testing.T) {
	logger := &callDepthTestLogr{}
	logger.resetCallDepth()
	l := logr.New(logger)
	defer CaptureState().Restore()
	SetLogger(l)

	// Add wrapper to ensure callDepthTestLogr +2 offset is correct.
	logFunc := func() {
		InfoS("infoS log")
	}

	// Keep these lines together.
	_, wantFile, wantLine, _ := runtime.Caller(0)
	logFunc()
	wantLine++

	if len(logger.entries) != 1 {
		t.Errorf("expected a single log entry to be generated, got %d", len(logger.entries))
	}
	checkLogrEntryCorrectCaller(t, wantFile, wantLine, logger.entries[0])
}

func TestCallDepthLogrErrorS(t *testing.T) {
	logger := &callDepthTestLogr{}
	logger.resetCallDepth()
	l := logr.New(logger)
	defer CaptureState().Restore()
	SetLogger(l)

	// Add wrapper to ensure callDepthTestLogr +2 offset is correct.
	logFunc := func() {
		ErrorS(errors.New("some error"), "errorS log")
	}

	// Keep these lines together.
	_, wantFile, wantLine, _ := runtime.Caller(0)
	logFunc()
	wantLine++

	if len(logger.entries) != 1 {
		t.Errorf("expected a single log entry to be generated, got %d", len(logger.entries))
	}
	checkLogrEntryCorrectCaller(t, wantFile, wantLine, logger.entries[0])
}

func TestCallDepthLogrGoLog(t *testing.T) {
	defer CaptureState().Restore()
	logger := &callDepthTestLogr{}
	logger.resetCallDepth()
	l := logr.New(logger)
	SetLogger(l)
	CopyStandardLogTo("INFO")

	// Add wrapper to ensure callDepthTestLogr +2 offset is correct.
	logFunc := func() {
		stdLog.Print("some log")
	}

	// Keep these lines together.
	_, wantFile, wantLine, _ := runtime.Caller(0)
	logFunc()
	wantLine++

	if len(logger.entries) != 1 {
		t.Errorf("expected a single log entry to be generated, got %d", len(logger.entries))
	}
	checkLogrEntryCorrectCaller(t, wantFile, wantLine, logger.entries[0])
	fmt.Println(logger.entries[0])
}

// Test callDepthTestLogr logs the expected offsets.
func TestCallDepthTestLogr(t *testing.T) {
	logger := &callDepthTestLogr{}
	logger.resetCallDepth()

	logFunc := func() {
		logger.Info(0, "some info log")
	}
	// Keep these lines together.
	_, wantFile, wantLine, _ := runtime.Caller(0)
	logFunc()
	wantLine++

	if len(logger.entries) != 1 {
		t.Errorf("expected a single log entry to be generated, got %d", len(logger.entries))
	}
	checkLogrEntryCorrectCaller(t, wantFile, wantLine, logger.entries[0])

	logger.reset()

	logFunc = func() {
		logger.Error(errors.New("error"), "some error log")
	}
	// Keep these lines together.
	_, wantFile, wantLine, _ = runtime.Caller(0)
	logFunc()
	wantLine++

	if len(logger.entries) != 1 {
		t.Errorf("expected a single log entry to be generated, got %d", len(logger.entries))
	}
	checkLogrEntryCorrectCaller(t, wantFile, wantLine, logger.entries[0])
}

type testLogr struct {
	entries []testLogrEntry
	mutex   sync.Mutex
}

type testLogrEntry struct {
	severity      severity.Severity
	msg           string
	keysAndValues []interface{}
	err           error
}

func (l *testLogr) reset() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.entries = []testLogrEntry{}
}

func (l *testLogr) Info(_ int, msg string, keysAndValues ...interface{}) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.entries = append(l.entries, testLogrEntry{
		severity:      severity.InfoLog,
		msg:           msg,
		keysAndValues: keysAndValues,
	})
}

func (l *testLogr) Error(err error, msg string, keysAndValues ...interface{}) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.entries = append(l.entries, testLogrEntry{
		severity:      severity.ErrorLog,
		msg:           msg,
		keysAndValues: keysAndValues,
		err:           err,
	})
}

func (l *testLogr) Init(logr.RuntimeInfo)                  {}
func (l *testLogr) Enabled(int) bool                       { return true }
func (l *testLogr) V(int) logr.Logger                      { panic("not implemented") }
func (l *testLogr) WithName(string) logr.LogSink           { panic("not implemented") }
func (l *testLogr) WithValues(...interface{}) logr.LogSink { panic("not implemented") }
func (l *testLogr) WithCallDepth(int) logr.LogSink         { return l }

var _ logr.LogSink = &testLogr{}
var _ logr.CallDepthLogSink = &testLogr{}

type callDepthTestLogr struct {
	testLogr
	callDepth int
}

func (l *callDepthTestLogr) resetCallDepth() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.callDepth = 0
}

func (l *callDepthTestLogr) WithCallDepth(depth int) logr.LogSink {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// Note: Usually WithCallDepth would be implemented by cloning l
	// and setting the call depth on the clone. We modify l instead in
	// this test helper for simplicity.
	l.callDepth = depth + 1
	return l
}

func (l *callDepthTestLogr) Info(_ int, msg string, keysAndValues ...interface{}) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// Add 2 to depth for the wrapper function caller and for invocation in
	// test case.
	_, file, line, _ := runtime.Caller(l.callDepth + 2)
	l.entries = append(l.entries, testLogrEntry{
		severity:      severity.InfoLog,
		msg:           msg,
		keysAndValues: append([]interface{}{file, line}, keysAndValues...),
	})
}

func (l *callDepthTestLogr) Error(err error, msg string, keysAndValues ...interface{}) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// Add 2 to depth for the wrapper function caller and for invocation in
	// test case.
	_, file, line, _ := runtime.Caller(l.callDepth + 2)
	l.entries = append(l.entries, testLogrEntry{
		severity:      severity.ErrorLog,
		msg:           msg,
		keysAndValues: append([]interface{}{file, line}, keysAndValues...),
		err:           err,
	})
}

var _ logr.LogSink = &callDepthTestLogr{}
var _ logr.CallDepthLogSink = &callDepthTestLogr{}

func checkLogrEntryCorrectCaller(t *testing.T, wantFile string, wantLine int, entry testLogrEntry) {
	t.Helper()

	want := fmt.Sprintf("%s:%d", wantFile, wantLine)
	// Log fields contain file and line number as first elements.
	got := fmt.Sprintf("%s:%d", entry.keysAndValues[0], entry.keysAndValues[1])

	if want != got {
		t.Errorf("expected file and line %q but got %q", want, got)
	}
}

// existedFlag contains all existed flag, without KlogPrefix
var existedFlag = map[string]struct{}{
	"log_dir":           {},
	"add_dir_header":    {},
	"alsologtostderr":   {},
	"log_backtrace_at":  {},
	"log_file":          {},
	"log_file_max_size": {},
	"logtostderr":       {},
	"one_output":        {},
	"skip_headers":      {},
	"skip_log_headers":  {},
	"stderrthreshold":   {},
	"v":                 {},
	"vmodule":           {},
}

// KlogPrefix define new flag prefix
const KlogPrefix string = "klog"

// TestKlogFlagPrefix check every klog flag's prefix, exclude flag in existedFlag
func TestKlogFlagPrefix(t *testing.T) {
	fs := &flag.FlagSet{}
	InitFlags(fs)
	fs.VisitAll(func(f *flag.Flag) {
		if _, found := existedFlag[f.Name]; !found {
			if !strings.HasPrefix(f.Name, KlogPrefix) {
				t.Errorf("flag %s not have klog prefix: %s", f.Name, KlogPrefix)
			}
		}
	})
}

func TestKObjs(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want []ObjectRef
	}{
		{
			name: "test for KObjs function with KMetadata slice",
			obj: []test.KMetadataMock{
				{
					Name: "kube-dns",
					NS:   "kube-system",
				},
				{
					Name: "mi-conf",
				},
				{},
			},
			want: []ObjectRef{
				{
					Name:      "kube-dns",
					Namespace: "kube-system",
				},
				{
					Name: "mi-conf",
				},
				{},
			},
		},
		{
			name: "test for KObjs function with KMetadata pointer slice",
			obj: []*test.KMetadataMock{
				{
					Name: "kube-dns",
					NS:   "kube-system",
				},
				{
					Name: "mi-conf",
				},
				nil,
			},
			want: []ObjectRef{
				{
					Name:      "kube-dns",
					Namespace: "kube-system",
				},
				{
					Name: "mi-conf",
				},
				{},
			},
		},
		{
			name: "test for KObjs function with slice does not implement KMetadata",
			obj:  []int{1, 2, 3, 4, 6},
			want: nil,
		},
		{
			name: "test for KObjs function with interface",
			obj:  "test case",
			want: nil,
		},
		{
			name: "test for KObjs function with nil",
			obj:  nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(KObjs(tt.obj), tt.want) {
				t.Errorf("\nwant:\t %v\n got:\t %v", tt.want, KObjs(tt.obj))
			}
		})
	}
}

// Benchmark test for lock with/without defer
type structWithLock struct {
	m sync.Mutex
	n int64
}

func BenchmarkWithoutDeferUnLock(b *testing.B) {
	s := structWithLock{}
	for i := 0; i < b.N; i++ {
		s.addWithoutDefer()
	}
}

func BenchmarkWithDeferUnLock(b *testing.B) {
	s := structWithLock{}
	for i := 0; i < b.N; i++ {
		s.addWithDefer()
	}
}

func (s *structWithLock) addWithoutDefer() {
	s.m.Lock()
	s.n++
	s.m.Unlock()
}

func (s *structWithLock) addWithDefer() {
	s.m.Lock()
	defer s.m.Unlock()
	s.n++
}

func TestFlushDaemon(t *testing.T) {
	for sev := severity.InfoLog; sev < severity.FatalLog; sev++ {
		flushed := make(chan struct{}, 1)
		spyFunc := func() {
			flushed <- struct{}{}
		}
		testClock := testingclock.NewFakeClock(time.Now())
		testLog := loggingT{
			settings: settings{
				flushInterval: time.Second,
			},
			flushD: newFlushDaemon(spyFunc, testClock),
		}

		// Calling testLog will call createFile, which should start the daemon.
		testLog.print(sev, nil, nil, "x")

		if !testLog.flushD.isRunning() {
			t.Error("expected flushD to be running")
		}

		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		testClock.Step(time.Second)
		select {
		case <-flushed:
		case <-timer.C:
			t.Fatal("flushDaemon didn't call flush function on tick")
		}

		timer = time.NewTimer(10 * time.Second)
		defer timer.Stop()
		testClock.Step(time.Second)
		select {
		case <-flushed:
		case <-timer.C:
			t.Fatal("flushDaemon didn't call flush function on second tick")
		}

		timer = time.NewTimer(10 * time.Second)
		defer timer.Stop()
		testLog.flushD.stop()
		select {
		case <-flushed:
		case <-timer.C:
			t.Fatal("flushDaemon didn't call flush function one last time on stop")
		}
	}
}

func TestStopFlushDaemon(t *testing.T) {
	logging.flushD.stop()
	logging.flushD = newFlushDaemon(func() {}, nil)
	logging.flushD.run(time.Second)
	if !logging.flushD.isRunning() {
		t.Error("expected flushD to be running")
	}
	StopFlushDaemon()
	if logging.flushD.isRunning() {
		t.Error("expected flushD to be stopped")
	}
}

func TestCaptureState(t *testing.T) {
	var fs flag.FlagSet
	InitFlags(&fs)

	// Capture state.
	oldState := map[string]string{}
	fs.VisitAll(func(f *flag.Flag) {
		oldState[f.Name] = f.Value.String()
	})
	originalLogger := Background()
	file := logging.file

	// And through dedicated API.
	// Ensure we always restore.
	state := CaptureState()
	defer state.Restore()

	// Change state.
	for name, value := range map[string]string{
		// All of these are non-standard values.
		"v":                 "10",
		"vmodule":           "abc=2",
		"log_dir":           "/tmp",
		"log_file_max_size": "10",
		"logtostderr":       "false",
		"alsologtostderr":   "true",
		"add_dir_header":    "true",
		"skip_headers":      "true",
		"one_output":        "true",
		"skip_log_headers":  "true",
		"stderrthreshold":   "1",
		"log_backtrace_at":  "foobar.go:100",
	} {
		f := fs.Lookup(name)
		if f == nil {
			t.Fatalf("could not look up %q", name)
		}
		currentValue := f.Value.String()
		if currentValue == value {
			t.Fatalf("%q is already set to non-default %q?!", name, value)
		}
		if err := f.Value.Set(value); err != nil {
			t.Fatalf("setting %q to %q: %v", name, value, err)
		}
	}
	StartFlushDaemon(time.Minute)
	if !logging.flushD.isRunning() {
		t.Error("Flush daemon should have been started.")
	}
	logger := logr.Discard()
	SetLoggerWithOptions(logger, ContextualLogger(true))
	actualLogger := Background()
	if logger != actualLogger {
		t.Errorf("Background logger should be %v, got %v", logger, actualLogger)
	}
	buffer := bytes.Buffer{}
	SetOutput(&buffer)
	if file == logging.file {
		t.Error("Output files should have been modified.")
	}

	// Let klog restore the state.
	state.Restore()

	// Verify that the original state is back.
	fs.VisitAll(func(f *flag.Flag) {
		oldValue := oldState[f.Name]
		currentValue := f.Value.String()
		if oldValue != currentValue {
			t.Errorf("%q should have been restored to %q, is %q instead", f.Name, oldValue, currentValue)
		}
	})
	if logging.flushD.isRunning() {
		t.Error("Flush daemon should have been stopped.")
	}
	actualLogger = Background()
	if originalLogger != actualLogger {
		t.Errorf("Background logger should be %v, got %v", originalLogger, actualLogger)
	}
	if file != logging.file {
		t.Errorf("Output files should have been restored to %v, got %v", file, logging.file)
	}
}

func TestSettingsDeepCopy(t *testing.T) {
	logger := logr.Discard()

	settings := settings{
		logger: &logWriter{Logger: logger},
		vmodule: moduleSpec{
			filter: []modulePat{
				{pattern: "a"},
				{pattern: "b"},
				{pattern: "c"},
			},
		},
	}
	clone := settings.deepCopy()
	if !reflect.DeepEqual(settings, clone) {
		t.Fatalf("Copy not identical to original settings. Original:\n    %+v\nCopy:    %+v", settings, clone)
	}
	settings.vmodule.filter[1].pattern = "x"
	if clone.vmodule.filter[1].pattern == settings.vmodule.filter[1].pattern {
		t.Fatal("Copy should not have shared vmodule.filter.")
	}
}

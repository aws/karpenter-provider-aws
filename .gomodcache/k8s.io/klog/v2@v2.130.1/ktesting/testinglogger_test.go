/*
Copyright 2019 The Kubernetes Authors.
Copyright 2020 Intel Corporation.

SPDX-License-Identifier: Apache-2.0
*/

package ktesting_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"regexp"
	"runtime"
	"sync"
	"testing"
	"time"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/internal/test/require"
	"k8s.io/klog/v2/ktesting"
)

var headerRe = regexp.MustCompile(`([IE])[[:digit:]]{4} [[:digit:]]{2}:[[:digit:]]{2}:[[:digit:]]{2}\.[[:digit:]]{6}\] `)

func TestInfo(t *testing.T) {
	tests := map[string]struct {
		text           string
		withValues     []interface{}
		keysAndValues  []interface{}
		names          []string
		err            error
		expectedOutput string
	}{
		"should log with values passed to keysAndValues": {
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue"},
			expectedOutput: `Ixxx test akey="avalue"
`,
		},
		"should support single name": {
			names:         []string{"hello"},
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue"},
			expectedOutput: `Ixxx hello: test akey="avalue"
`,
		},
		"should support multiple names": {
			names:         []string{"hello", "world"},
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue"},
			expectedOutput: `Ixxx hello/world: test akey="avalue"
`,
		},
		"should not print duplicate keys with the same value": {
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue", "akey", "avalue"},
			expectedOutput: `Ixxx test akey="avalue" akey="avalue"
`,
		},
		"should only print the last duplicate key when the values are passed to Info": {
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue", "akey", "avalue2"},
			expectedOutput: `Ixxx test akey="avalue" akey="avalue2"
`,
		},
		"should only print the duplicate key that is passed to Info if one was passed to the logger": {
			withValues:    []interface{}{"akey", "avalue"},
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue"},
			expectedOutput: `Ixxx test akey="avalue"
`,
		},
		"should only print the key passed to Info when one is already set on the logger": {
			withValues:    []interface{}{"akey", "avalue"},
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue2"},
			expectedOutput: `Ixxx test akey="avalue2"
`,
		},
		"should correctly handle odd-numbers of KVs": {
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue", "akey2"},
			expectedOutput: `Ixxx test akey="avalue" akey2="(MISSING)"
`,
		},
		"should correctly html characters": {
			text:          "test",
			keysAndValues: []interface{}{"akey", "<&>"},
			expectedOutput: `Ixxx test akey="<&>"
`,
		},
		"should correctly handle odd-numbers of KVs in both log values and Info args": {
			withValues:    []interface{}{"basekey1", "basevar1", "basekey2"},
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue", "akey2"},
			expectedOutput: `Ixxx test basekey1="basevar1" basekey2="(MISSING)" akey="avalue" akey2="(MISSING)"
`,
		},
		"should correctly print regular error types": {
			text:          "test",
			keysAndValues: []interface{}{"err", errors.New("whoops")},
			expectedOutput: `Ixxx test err="whoops"
`,
		},
		"should correctly print regular error types when using logr.Error": {
			text: "test",
			err:  errors.New("whoops"),
			expectedOutput: `Exxx test err="whoops"
`,
		},
	}
	for n, test := range tests {
		t.Run(n, func(t *testing.T) {
			var buffer logToBuf
			klogr := ktesting.NewLogger(&buffer, ktesting.NewConfig())
			for _, name := range test.names {
				klogr = klogr.WithName(name)
			}
			klogr = klogr.WithValues(test.withValues...)

			if test.err != nil {
				klogr.Error(test.err, test.text, test.keysAndValues...)
			} else {
				klogr.Info(test.text, test.keysAndValues...)
			}

			actual := buffer.String()
			actual = headerRe.ReplaceAllString(actual, `${1}xxx `)
			if actual != test.expectedOutput {
				t.Errorf("Expected:\n%sActual:\n%s\n", test.expectedOutput, actual)
			}
		})
	}
}

func TestCallDepth(t *testing.T) {
	logger := ktesting.NewLogger(t, ktesting.NewConfig())
	logger.Info("hello world")
}

type logToBuf struct {
	ktesting.NopTL
	bytes.Buffer
}

func (l *logToBuf) Helper() {
}

func (l *logToBuf) Log(args ...interface{}) {
	for i, arg := range args {
		if i > 0 {
			l.Write([]byte(" "))
		}
		l.Write([]byte(fmt.Sprintf("%s", arg)))
	}
	l.Write([]byte("\n"))
}

func TestStop(t *testing.T) {
	// This test is set up so that a subtest spawns a goroutine and that
	// goroutine logs through ktesting *after* the subtest has
	// completed. This is not supported by testing.T.Log and normally
	// leads to:
	//   panic: Log in goroutine after TestGoroutines/Sub has completed: INFO hello world
	//
	// It works with ktesting if (and only if) logging gets redirected to klog
	// before returning from the test.

	// Capture output for testing.
	state := klog.CaptureState()
	defer state.Restore()
	var output bytes.Buffer
	var fs flag.FlagSet
	klog.InitFlags(&fs)
	require.NoError(t, fs.Set("alsologtostderr", "false"))
	require.NoError(t, fs.Set("logtostderr", "false"))
	require.NoError(t, fs.Set("stderrthreshold", "FATAL"))
	require.NoError(t, fs.Set("one_output", "true"))
	klog.SetOutput(&output)

	var logger klog.Logger
	var line int
	var wg1, wg2 sync.WaitGroup
	wg1.Add(1)
	wg2.Add(1)
	t.Run("Sub", func(t *testing.T) {
		logger, _ = ktesting.NewTestContext(t)
		go func() {
			defer wg2.Done()

			// Wait for test to have returned.
			wg1.Wait()

			// This output must go to klog because the test has
			// completed.
			_, _, line, _ = runtime.Caller(0)
			logger.Info("simple info message")
			logger.Error(nil, "error message")
			time.Sleep(15 * time.Second)
			logger.WithName("me").WithValues("completed", true).Info("complex info message", "anotherValue", 1)
		}()
	})
	// Allow goroutine above to proceed.
	wg1.Done()

	// Ensure that goroutine has completed.
	wg2.Wait()

	actual := output.String()

	// Strip time and pid prefix.
	actual = regexp.MustCompile(`(?m)^.* testinglogger_test.go:`).ReplaceAllString(actual, `testinglogger_test.go:`)

	// Strip duration.
	actual = regexp.MustCompile(`timeSinceCompletion="\d+s"`).ReplaceAllString(actual, `timeSinceCompletion="<...>s"`)

	// All lines from the callstack get stripped. We can be sure that it was non-empty because otherwise we wouldn't
	// have the < > markers.
	//
	// Full output:
	// 	testinglogger_test.go:194] "WARNING: test kept at least one goroutine running after test completion" logger="TestStop/Sub leaked goroutine.me" completed=true timeSinceCompletion="15s" callstack=<
	//        	goroutine 23 [running]:
	//        	k8s.io/klog/v2/internal/dbg.Stacks(0x0)
	//        		/nvme/gopath/src/k8s.io/klog/internal/dbg/dbg.go:34 +0x8a
	//        	k8s.io/klog/v2/ktesting.tlogger.fallbackLogger({0xc0000f2780, {0x0, 0x0}, {0x0, 0x0, 0x0}})
	//        		/nvme/gopath/src/k8s.io/klog/ktesting/testinglogger.go:292 +0x232
	//        	k8s.io/klog/v2/ktesting.tlogger.Info({0xc0000f2780, {0x0, 0x0}, {0x0, 0x0, 0x0}}, 0x0, {0x5444a5, 0x13}, {0x0, ...})
	//        		/nvme/gopath/src/k8s.io/klog/ktesting/testinglogger.go:316 +0x28a
	//        	github.com/go-logr/logr.Logger.Info({{0x572438?, 0xc0000c0ff0?}, 0x0?}, {0x5444a5, 0x13}, {0x0, 0x0, 0x0})
	//        		/nvme/gopath/pkg/mod/github.com/go-logr/logr@v1.2.0/logr.go:249 +0xd0
	//        	k8s.io/klog/v2/ktesting_test.TestStop.func1.1()
	//        		/nvme/gopath/src/k8s.io/klog/ktesting/testinglogger_test.go:194 +0xe5
	//        	created by k8s.io/klog/v2/ktesting_test.TestStop.func1
	//        		/nvme/gopath/src/k8s.io/klog/ktesting/testinglogger_test.go:185 +0x105
	//         >
	actual = regexp.MustCompile(`(?m)^\t.*?\n`).ReplaceAllString(actual, ``)

	expected := fmt.Sprintf(`testinglogger_test.go:%d] "simple info message" logger="TestStop/Sub leaked goroutine"
testinglogger_test.go:%d] "error message" logger="TestStop/Sub leaked goroutine"
testinglogger_test.go:%d] "WARNING: test kept at least one goroutine running after test completion" logger="TestStop/Sub leaked goroutine.me" completed=true timeSinceCompletion="<...>s" callstack=<
 >
testinglogger_test.go:%d] "complex info message" logger="TestStop/Sub leaked goroutine.me" completed=true anotherValue=1
`,
		line+1, line+2, line+4, line+4)
	if actual != expected {
		t.Errorf("Output does not match. Expected:\n%s\nActual:\n%s\n", expected, actual)
	}

	testingLogger, ok := logger.GetSink().(ktesting.Underlier)
	if !ok {
		t.Fatal("should have had a ktesting logger")
	}
	captured := testingLogger.GetBuffer().String()
	if captured != "" {
		t.Errorf("testing logger should not have captured any output, got instead:\n%s", captured)
	}
}

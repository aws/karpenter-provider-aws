/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package textlogger_test

import (
	"errors"
	"os"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
	internal "k8s.io/klog/v2/internal/buffer"
	"k8s.io/klog/v2/textlogger"
)

var _ logr.Marshaler = coordinatesMarshaler{}

type coordinatesMarshaler struct {
	x, y int
}

func (c coordinatesMarshaler) MarshalLog() interface{} {
	return map[string]int{"X": c.x, "Y": c.y}
}

type variables struct {
	A, B int
}

func ExampleNewLogger() {
	ts, _ := time.Parse(time.RFC3339, "2000-12-24T12:30:40Z")
	internal.Pid = 123 // To get consistent output for each run.
	config := textlogger.NewConfig(
		textlogger.FixedTime(ts), // To get consistent output for each run.
		textlogger.Verbosity(4),  // Matches Kubernetes "debug" level.
		textlogger.Output(os.Stdout),
	)
	logger := textlogger.NewLogger(config)

	logger.V(4).Info("A debug message")
	logger.V(5).Info("A debug message with even lower level, not printed.")
	logger.Info("An info message")
	logger.Error(errors.New("fake error"), "An error")
	logger.WithValues("int", 42).Info("With values",
		"duration", time.Second,
		"float", 3.12,
		"coordinates", coordinatesMarshaler{x: 100, y: 200},
		"variables", variables{A: 1, B: 2},
	)
	// The logr API supports skipping functions during stack unwinding, in contrast to slog.
	someHelper(logger, "hello world")

	// Output:
	// I1224 12:30:40.000000     123 textlogger_test.go:54] "A debug message"
	// I1224 12:30:40.000000     123 textlogger_test.go:56] "An info message"
	// E1224 12:30:40.000000     123 textlogger_test.go:57] "An error" err="fake error"
	// I1224 12:30:40.000000     123 textlogger_test.go:58] "With values" int=42 duration="1s" float=3.12 coordinates={"X":100,"Y":200} variables={"A":1,"B":2}
	// I1224 12:30:40.000000     123 textlogger_test.go:65] "hello world"
}

func someHelper(logger klog.Logger, msg string) {
	logger.WithCallDepth(1).Info(msg)
}

func ExampleBacktrace() {
	ts, _ := time.Parse(time.RFC3339, "2000-12-24T12:30:40Z")
	internal.Pid = 123 // To get consistent output for each run.
	backtraceCounter := 0
	config := textlogger.NewConfig(
		textlogger.FixedTime(ts), // To get consistent output for each run.
		textlogger.Backtrace(func(_ /* skip */ int) (filename string, line int) {
			backtraceCounter++
			if backtraceCounter == 1 {
				// Simulate "missing information".
				return "", 0
			}
			return "fake.go", 42

			// A real implementation could use Ginkgo:
			//
			// import ginkgotypes "github.com/onsi/ginkgo/v2/types"
			//
			// location := ginkgotypes.NewCodeLocation(skip + 1)
			// return location.FileName, location.LineNumber
		}),
		textlogger.Output(os.Stdout),
	)
	logger := textlogger.NewLogger(config)

	logger.Info("First message")
	logger.Info("Second message")

	// Output:
	// I1224 12:30:40.000000     123 ???:1] "First message"
	// I1224 12:30:40.000000     123 fake.go:42] "Second message"
}

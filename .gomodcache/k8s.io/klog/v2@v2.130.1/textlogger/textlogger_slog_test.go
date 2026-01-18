//go:build go1.21
// +build go1.21

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
	"log/slog"
	"os"
	"time"

	"github.com/go-logr/logr/slogr"
	internal "k8s.io/klog/v2/internal/buffer"
	"k8s.io/klog/v2/textlogger"
)

var _ slog.LogValuer = coordinates{}

type coordinates struct {
	x, y int
}

func (c coordinates) LogValue() slog.Value {
	return slog.GroupValue(slog.Attr{Key: "X", Value: slog.IntValue(c.x)}, slog.Attr{Key: "Y", Value: slog.IntValue(c.y)})
}

func ExampleNewLogger_Slog() {
	ts, _ := time.Parse(time.RFC3339, "2000-12-24T12:30:40Z")
	internal.Pid = 123 // To get consistent output for each run.
	config := textlogger.NewConfig(
		textlogger.FixedTime(ts), // To get consistent output for each run.
		textlogger.Verbosity(4),  // Matches slog.LevelDebug.
		textlogger.Output(os.Stdout),
	)
	logrLogger := textlogger.NewLogger(config)
	slogHandler := slogr.NewSlogHandler(logrLogger)
	slogLogger := slog.New(slogHandler)

	slogLogger.Debug("A debug message")
	slogLogger.Log(nil, slog.LevelDebug-1, "A debug message with even lower level, not printed.")
	slogLogger.Info("An info message")
	slogLogger.Warn("A warning")
	slogLogger.Error("An error", "err", errors.New("fake error"))

	// The slog API supports grouping, in contrast to the logr API.
	slogLogger.WithGroup("top").With("int", 42, slog.Group("variables", "a", 1, "b", 2)).Info("Grouping",
		"sub", slog.GroupValue(
			slog.Attr{Key: "str", Value: slog.StringValue("abc")},
			slog.Attr{Key: "bool", Value: slog.BoolValue(true)},
			slog.Attr{Key: "bottom", Value: slog.GroupValue(slog.Attr{Key: "coordinates", Value: slog.AnyValue(coordinates{x: -1, y: -2})})},
		),
		"duration", slog.DurationValue(time.Second),
		slog.Float64("pi", 3.12),
		"e", 2.71,
		"moreCoordinates", coordinates{x: 100, y: 200},
	)

	// slog special values are also supported when passed through the logr API.
	// This works with the textlogger, but might not work with other implementations
	// and thus isn't portable. Passing attributes (= key and value in a single
	// parameter) is not supported.
	logrLogger.Info("slog values",
		"variables", slog.GroupValue(slog.Int("a", 1), slog.Int("b", 2)),
		"duration", slog.DurationValue(time.Second),
		"coordinates", coordinates{x: 100, y: 200},
	)

	// Output:
	// I1224 12:30:40.000000     123 textlogger_slog_test.go:55] "A debug message"
	// I1224 12:30:40.000000     123 textlogger_slog_test.go:57] "An info message"
	// W1224 12:30:40.000000     123 textlogger_slog_test.go:58] "A warning"
	// E1224 12:30:40.000000     123 textlogger_slog_test.go:59] "An error" err="fake error"
	// I1224 12:30:40.000000     123 textlogger_slog_test.go:62] "Grouping" top.int=42 top.variables={"a":1,"b":2} top.sub={"str":"abc","bool":true,"bottom":{"coordinates":{"X":-1,"Y":-2}}} top.duration="1s" top.pi=3.12 top.e=2.71 top.moreCoordinates={"X":100,"Y":200}
	// I1224 12:30:40.000000     123 textlogger_slog_test.go:78] "slog values" variables={"a":1,"b":2} duration="1s" coordinates={"X":100,"Y":200}
}

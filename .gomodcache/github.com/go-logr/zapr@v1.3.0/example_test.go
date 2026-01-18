/*
Copyright 2021 The logr Authors.

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

package zapr_test

import (
	"errors"
	"time"

	"github.com/go-logr/zapr"
	"github.com/go-logr/zapr/internal/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var errSome = errors.New("some error")

func encodeTime(_ time.Time, enc zapcore.PrimitiveArrayEncoder) {
	// Suppress actual time to keep output constant.
	enc.AppendString("TIMESTAMP")
}

func buildZapLogger() *zap.Logger {
	// zap gets configured to not panic on invalid log calls
	// and to produce simple, deterministic output on stdout.
	zc := zap.NewProductionConfig()
	zc.OutputPaths = []string{"stdout"}
	zc.ErrorOutputPaths = zc.OutputPaths
	zc.DisableStacktrace = true
	zc.DisableCaller = true
	zc.EncoderConfig.EncodeTime = encodeTime
	z, _ := zc.Build()
	return z
}

func ExampleNewLogger() {
	log := zapr.NewLogger(buildZapLogger())
	log.Info("info message with default options")
	log.Error(errSome, "error message with default options")
	log.Info("support for zap fields as key/value replacement is disabled", zap.Int("answer", 42))
	log.Info("invalid key", 42, "answer")
	log.Info("missing value", "answer")
	obj := types.ObjectRef{Name: "john", Namespace: "doe"}
	log.Info("marshaler", "stringer", obj.String(), "raw", obj)
	// Output:
	// {"level":"info","ts":"TIMESTAMP","msg":"info message with default options"}
	// {"level":"error","ts":"TIMESTAMP","msg":"error message with default options","error":"some error"}
	// {"level":"dpanic","ts":"TIMESTAMP","msg":"strongly-typed Zap Field passed to logr","zap field":{"Key":"answer","Type":11,"Integer":42,"String":"","Interface":null}}
	// {"level":"info","ts":"TIMESTAMP","msg":"support for zap fields as key/value replacement is disabled"}
	// {"level":"dpanic","ts":"TIMESTAMP","msg":"non-string key argument passed to logging, ignoring all later arguments","invalid key":42}
	// {"level":"info","ts":"TIMESTAMP","msg":"invalid key"}
	// {"level":"dpanic","ts":"TIMESTAMP","msg":"odd number of arguments passed as key-value pairs for logging","ignored key":"answer"}
	// {"level":"info","ts":"TIMESTAMP","msg":"missing value"}
	// {"level":"info","ts":"TIMESTAMP","msg":"marshaler","stringer":"doe/john","raw":{"name":"john","namespace":"doe"}}
}

func ExampleLogInfoLevel() {
	log := zapr.NewLoggerWithOptions(buildZapLogger(), zapr.LogInfoLevel("v"))
	log.Info("info message with numeric verbosity level")
	log.Error(errSome, "error messages have no numeric verbosity level")
	// Output:
	// {"level":"info","ts":"TIMESTAMP","msg":"info message with numeric verbosity level","v":0}
	// {"level":"error","ts":"TIMESTAMP","msg":"error messages have no numeric verbosity level","error":"some error"}
}

func ExampleErrorKey() {
	log := zapr.NewLoggerWithOptions(buildZapLogger(), zapr.ErrorKey("err"))
	log.Error(errSome, "error message with non-default error key")
	// Output:
	// {"level":"error","ts":"TIMESTAMP","msg":"error message with non-default error key","err":"some error"}
}

func ExampleAllowZapFields() {
	log := zapr.NewLoggerWithOptions(buildZapLogger(), zapr.AllowZapFields(true))
	log.Info("log zap field", zap.Int("answer", 42))
	// Output:
	// {"level":"info","ts":"TIMESTAMP","msg":"log zap field","answer":42}
}

func ExampleDPanicOnBugs() {
	log := zapr.NewLoggerWithOptions(buildZapLogger(), zapr.DPanicOnBugs(false))
	log.Info("warnings suppressed", zap.Int("answer", 42))
	// Output:
	// {"level":"info","ts":"TIMESTAMP","msg":"warnings suppressed"}
}

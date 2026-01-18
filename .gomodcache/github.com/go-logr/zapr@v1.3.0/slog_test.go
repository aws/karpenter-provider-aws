//go:build go1.21
// +build go1.21

/*
Copyright 2023 The logr Authors.

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
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"testing/slogtest"

	"github.com/go-logr/logr/slogr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSlogHandler(t *testing.T) {
	var buffer bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey: slog.MessageKey,
		TimeKey:    slog.TimeKey,
		LevelKey:   slog.LevelKey,
		EncodeLevel: func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
			encoder.AppendInt(int(level))
		},
	})
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buffer), zapcore.Level(0))
	zl := zap.New(core)
	logger := zapr.NewLogger(zl)
	handler := slogr.NewSlogHandler(logger)

	err := slogtest.TestHandler(handler, func() []map[string]any {
		_ = zl.Sync()
		return parseOutput(t, buffer.Bytes())
	})
	t.Logf("Log output:\n%s\nAs JSON:\n%v\n", buffer.String(), parseOutput(t, buffer.Bytes()))
	// Correlating failures with individual test cases is hard with the current API.
	// See https://github.com/golang/go/issues/61758
	if err != nil {
		if err, ok := err.(interface {
			Unwrap() []error
		}); ok {
			for _, err := range err.Unwrap() {
				if !containsOne(err.Error(),
					"a Handler should ignore a zero Record.Time",             // zapr always writes a time field.
					"a Handler should not output groups for an empty Record", // Relies on WithGroup and that always opens a group. Text may change, see https://go.dev/cl/516155
				) {
					t.Errorf("Unexpected error: %v", err)
				}
			}
			return
		}
		// Shouldn't be reached, errors from errors.Join can be split up.
		t.Errorf("Unexpected errors:\n%v", err)
	}
}

func containsOne(hay string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(hay, needle) {
			return true
		}
	}
	return false
}

// TestSlogCases covers some gaps in the coverage we get from
// slogtest.TestHandler (empty and invalud PC, see
// https://github.com/golang/go/issues/62280) and verbosity handling in
// combination with V().
func TestSlogCases(t *testing.T) {
	for name, tc := range map[string]struct {
		record   slog.Record
		v        int
		expected string
	}{
		"empty": {
			expected: `{"msg":"", "level":"info", "v":0}`,
		},
		"invalid-pc": {
			record:   slog.Record{PC: 1},
			expected: `{"msg":"", "level":"info", "v":0}`,
		},
		"debug": {
			record:   slog.Record{Level: slog.LevelDebug},
			expected: `{"msg":"", "level":"Level(-4)", "v":4}`,
		},
		"warn": {
			record:   slog.Record{Level: slog.LevelWarn},
			expected: `{"msg":"", "level":"warn", "v":0}`,
		},
		"error": {
			record:   slog.Record{Level: slog.LevelError},
			expected: `{"msg":"", "level":"error"}`,
		},
		"debug-v1": {
			v:        1,
			record:   slog.Record{Level: slog.LevelDebug},
			expected: `{"msg":"", "level":"Level(-5)", "v":5}`,
		},
		"warn-v1": {
			v:        1,
			record:   slog.Record{Level: slog.LevelWarn},
			expected: `{"msg":"", "level":"info", "v":0}`,
		},
		"error-v1": {
			v:        1,
			record:   slog.Record{Level: slog.LevelError},
			expected: `{"msg":"", "level":"error"}`,
		},
		"debug-v4": {
			v:        4,
			record:   slog.Record{Level: slog.LevelDebug},
			expected: `{"msg":"", "level":"Level(-8)", "v":8}`,
		},
		"warn-v4": {
			v:        4,
			record:   slog.Record{Level: slog.LevelWarn},
			expected: `{"msg":"", "level":"info", "v":0}`,
		},
		"error-v4": {
			v:        4,
			record:   slog.Record{Level: slog.LevelError},
			expected: `{"msg":"", "level":"error"}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var buffer bytes.Buffer
			encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
				MessageKey: slog.MessageKey,
				LevelKey:   slog.LevelKey,
				EncodeLevel: func(level zapcore.Level, encoder zapcore.PrimitiveArrayEncoder) {
					encoder.AppendString(level.String())
				},
			})
			core := zapcore.NewCore(encoder, zapcore.AddSync(&buffer), zapcore.Level(-10))
			zl := zap.New(core)
			logger := zapr.NewLoggerWithOptions(zl, zapr.LogInfoLevel("v"))
			handler := slogr.NewSlogHandler(logger.V(tc.v))
			require.NoError(t, handler.Handle(context.Background(), tc.record))
			_ = zl.Sync()
			require.JSONEq(t, tc.expected, buffer.String())
		})
	}
}

func parseOutput(t *testing.T, output []byte) []map[string]any {
	var ms []map[string]any
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatal(err)
		}
		ms = append(ms, m)
	}
	return ms
}

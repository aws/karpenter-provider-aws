// Copyright 2024 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promslog

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v2"
)

var (
	slogStyleLogRegexp  = regexp.MustCompile(`(?P<TimeKey>time)=.*level=(?P<LevelValue>WARN|INFO|ERROR|DEBUG).*(?P<SourceKey>source)=.*`)
	goKitStyleLogRegexp = regexp.MustCompile(`(?P<TimeKey>ts)=.*level=(?P<LevelValue>warn|info|error|debug).*(?P<SourceKey>caller)=.*`)
)

// Make sure creating and using a logger with an empty configuration doesn't
// result in a panic.
func TestDefaultConfig(t *testing.T) {
	require.NotPanics(t, func() {
		logger := New(&Config{})
		logger.Info("empty config `Info()` test", "hello", "world")
		logger.Log(context.Background(), slog.LevelInfo, "empty config `Log()` test", "hello", "world")
		logger.LogAttrs(context.Background(), slog.LevelInfo, "empty config `LogAttrs()` test", slog.String("hello", "world"))
	})
}

func TestUnmarshallLevel(t *testing.T) {
	l := NewLevel()
	err := yaml.Unmarshal([]byte(`debug`), l)
	if err != nil {
		t.Error(err)
	}
	if got := l.String(); got != "debug" {
		t.Errorf("expected %s, got %s", "debug", got)
	}
}

func TestUnmarshallEmptyLevel(t *testing.T) {
	l := NewLevel()
	err := yaml.Unmarshal([]byte(``), l)
	if err != nil {
		t.Error(err)
	}
	if got := l.String(); got != "info" {
		t.Errorf("expected info (default) level, got %s", got)
	}
}

func TestUnmarshallBadLevel(t *testing.T) {
	l := NewLevel()
	err := yaml.Unmarshal([]byte(`debugg`), l)
	if err == nil {
		t.Error("expected error")
	}
	expErr := `unrecognized log level debugg`
	if err.Error() != expErr {
		t.Errorf("expected error %s, got %s", expErr, err.Error())
	}
	if got := l.String(); got != "info" {
		t.Errorf("expected info (default) level, got %s", got)
	}
}

func getLogEntryLevelCounts(s string, re *regexp.Regexp) map[string]int {
	counters := make(map[string]int)
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			levelIndex := re.SubexpIndex("LevelValue")

			counters[strings.ToLower(matches[levelIndex])]++
		}
	}

	return counters
}

func TestDurationValues(t *testing.T) {
	dur, err := time.ParseDuration("1m30s")
	require.NoError(t, err)

	tests := map[string]struct {
		logFormat string
		want      string
	}{
		"logfmt_duration_testing": {want: "duration_raw=1m30s duration_string=1m30s", logFormat: "logfmt"},
		"json_duration_testing":   {want: "\"duration_raw\":\"1m30s\",\"duration_string\":\"1m30s\"", logFormat: "json"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			config := &Config{
				Writer: &buf,
				Format: NewFormat(),
			}
			require.NoError(t, config.Format.Set(tc.logFormat))
			logger := New(config)
			logger.Info("duration testing", "duration_raw", dur, "duration_string", dur.String())
			require.Contains(t, buf.String(), tc.want)
		})
	}
}

func TestDynamicLevels(t *testing.T) {
	var buf bytes.Buffer
	wantedLevelCounts := map[string]int{"info": 1, "debug": 1}

	tests := map[string]struct {
		logStyle         LogStyle
		logStyleRegexp   *regexp.Regexp
		wantedLevelCount map[string]int
	}{
		"slog_log_style":   {logStyle: SlogStyle, logStyleRegexp: slogStyleLogRegexp, wantedLevelCount: wantedLevelCounts},
		"go-kit_log_style": {logStyle: GoKitStyle, logStyleRegexp: goKitStyleLogRegexp, wantedLevelCount: wantedLevelCounts},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			buf.Reset() // Ensure buf is reset prior to tests
			config := &Config{Writer: &buf, Style: tc.logStyle}
			logger := New(config)

			// Test that log level can be adjusted on-the-fly to debug and that a
			// log entry can be written to the file.
			err := config.Level.Set("debug")
			require.NoError(t, err)
			logger.Info("info", "hello", "world")
			logger.Debug("debug", "hello", "world")

			counts := getLogEntryLevelCounts(buf.String(), tc.logStyleRegexp)
			require.Equalf(t, tc.wantedLevelCount["info"], counts["info"], "info log successfully detected")
			require.Equalf(t, tc.wantedLevelCount["debug"], counts["debug"], "debug log successfully detected")
			// Print logs for humans to see, if needed.
			fmt.Println(buf.String())
			buf.Reset()

			// Test that log level can be adjusted on-the-fly to info and that a
			// subsequent call to write a debug level log is _not_ written to the
			// file.
			err = config.Level.Set("info")
			require.NoError(t, err)
			logger.Info("info", "hello", "world")
			logger.Debug("debug", "hello", "world")

			counts = getLogEntryLevelCounts(buf.String(), tc.logStyleRegexp)
			require.Equalf(t, tc.wantedLevelCount["info"], counts["info"], "info log successfully detected")
			require.NotEqualf(t, tc.wantedLevelCount["debug"], counts["debug"], "extra debug log detected")
			// Print logs for humans to see, if needed.
			fmt.Println(buf.String())
			buf.Reset()
		})
	}
}

func TestTruncateSourceFileName_DefaultStyle(t *testing.T) {
	var buf bytes.Buffer

	config := &Config{
		Writer: &buf,
	}

	logger := New(config)
	logger.Info("test message")

	output := buf.String()

	if !strings.Contains(output, "source=slog_test.go:") {
		t.Errorf("Expected source file name to be truncated to basename, got: %s", output)
	}

	if strings.Contains(output, "/") {
		t.Errorf("Expected no directory separators in source file name, got: %s", output)
	}
}

func TestTruncateSourceFileName_GoKitStyle(t *testing.T) {
	var buf bytes.Buffer

	config := &Config{
		Writer: &buf,
		Style:  GoKitStyle,
	}

	logger := New(config)
	logger.Info("test message")

	output := buf.String()

	// In GoKitStyle, the source key is "caller".
	if !strings.Contains(output, "caller=slog_test.go:") {
		t.Errorf("Expected caller to contain basename of source file, got: %s", output)
	}

	if strings.Contains(output, "/") {
		t.Errorf("Expected no directory separators in caller, got: %s", output)
	}
}

func TestReservedKeys(t *testing.T) {
	var buf bytes.Buffer
	reservedKeyTestVal := "surprise! I'm a string"

	tests := map[string]struct {
		logStyle  LogStyle
		levelKey  string
		sourceKey string
		timeKey   string
	}{
		"slog_log_style":   {logStyle: SlogStyle, levelKey: "level", sourceKey: "source", timeKey: "time"},
		"go-kit_log_style": {logStyle: GoKitStyle, levelKey: "level", sourceKey: "caller", timeKey: "ts"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			buf.Reset() // Ensure buf is reset prior to tests
			config := &Config{Writer: &buf, Style: tc.logStyle}
			logger := New(config)

			logger.LogAttrs(context.Background(),
				slog.LevelInfo,
				"reserved keys test for "+name,
				slog.String(tc.levelKey, reservedKeyTestVal),
				slog.String(tc.sourceKey, reservedKeyTestVal),
				slog.String(tc.timeKey, reservedKeyTestVal),
			)

			output := buf.String()
			require.Containsf(t, output, fmt.Sprintf("%s%s=\"%s\"", reservedKeyPrefix, tc.levelKey, reservedKeyTestVal), "Expected duplicate level key to be renamed")
			require.Containsf(t, output, fmt.Sprintf("%s%s=\"%s\"", reservedKeyPrefix, tc.sourceKey, reservedKeyTestVal), "Expected duplicate source key to be renamed")
			require.Containsf(t, output, fmt.Sprintf("%s%s=\"%s\"", reservedKeyPrefix, tc.timeKey, reservedKeyTestVal), "Expected duplicate time key to be renamed")

			// Print logs for humans to see, if needed.
			fmt.Println(buf.String())
		})
	}
}

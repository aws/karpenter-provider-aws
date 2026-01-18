/*
Copyright 2020 The Kubernetes Authors.
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
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
)

const fixedTime = 123.456789

func fixedTimeEncoder(_ time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendFloat64(fixedTime)
}

// discard is a replacement for io.Discard, needed for Go 1.14.
type discard struct{}

func (d discard) Write(_ []byte) (int, error) { return 0, nil }

type marshaler struct {
	msg string
}

func (m *marshaler) String() string {
	return m.msg
}

func (m *marshaler) MarshalLog() interface{} {
	return "msg=" + m.msg
}

var _ fmt.Stringer = &marshaler{}
var _ logr.Marshaler = &marshaler{}

type stringer struct {
	msg string
}

func (s *stringer) String() string {
	return s.msg
}

var _ fmt.Stringer = &stringer{}

type stringerPanic struct {
}

func (s *stringerPanic) String() string {
	panic("fake panic")
}

var _ fmt.Stringer = &stringerPanic{}

func newZapLogger(lvl zapcore.Level, w zapcore.WriteSyncer) *zap.Logger {
	if w == nil {
		w = zapcore.AddSync(discard{})
	}
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:     "msg",
		CallerKey:      "caller",
		TimeKey:        "ts",
		EncodeTime:     fixedTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})
	core := zapcore.NewCore(encoder, zapcore.AddSync(w), lvl)
	l := zap.New(core, zap.WithCaller(true))
	return l
}

// TestInfo tests the JSON info format.
func TestInfo(t *testing.T) {
	type testCase struct {
		msg            string
		format         string // If empty, only formatting with slog as API is supported.
		formatSlog     string // If empty, formatting with slog as API yields the same result as logr.
		names          []string
		withKeysValues []interface{}
		keysValues     []interface{}
		wrapper        func(logr.Logger, string, ...interface{})
		needSlog       bool
	}
	var testDataInfo = []testCase{
		{
			msg: "simple",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"simple","v":0}
`,
		},
		{
			msg: "WithCallDepth",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"WithCallDepth","v":0}
`,
			wrapper: myInfo,
		},
		{
			msg: "incremental WithCallDepth",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"incremental WithCallDepth","v":0}
`,
			wrapper: myInfoInc,
		},
		{
			msg: "one name",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"one name","v":0}
`,
			names: []string{"me"},
		},
		{
			msg: "many names",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"many names","v":0}
`,
			names: []string{"hello", "world"},
		},
		{
			msg: "key-value pairs",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"key-value pairs","v":0,"ns":"default","podnum":2}
`,
			keysValues: []interface{}{"ns", "default", "podnum", 2},
		},
		{
			msg: "WithValues",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"WithValues","ns":"default","podnum":2,"v":0}
`,
			withKeysValues: []interface{}{"ns", "default", "podnum", 2},
		},
		{
			msg: "empty WithValues",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"empty WithValues","v":0,"ns":"default","podnum":2}
`,
			withKeysValues: []interface{}{},
			keysValues:     []interface{}{"ns", "default", "podnum", 2},
		},
		{
			msg: "mixed",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"mixed","ns":"default","v":0,"podnum":2}
`,
			withKeysValues: []interface{}{"ns", "default"},
			keysValues:     []interface{}{"podnum", 2},
		},
		{
			msg: "invalid WithValues",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"non-string key argument passed to logging, ignoring all later arguments","invalid key":200}
{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"invalid WithValues","ns":"default","podnum":2,"v":0}
`,
			formatSlog: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"invalid WithValues","ns":"default","podnum":2,"!BADKEY":200,"replica":"Running","!BADKEY":10,"v":0}
`,
			withKeysValues: []interface{}{"ns", "default", "podnum", 2, 200, "replica", "Running", 10},
		},
		{
			msg: "strongly typed Zap field",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"strongly-typed Zap Field passed to logr","zap field":{"Key":"zap-field-attempt","Type":11,"Integer":3,"String":"","Interface":null}}
{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"strongly typed Zap field","v":0,"ns":"default","podnum":2,"zap-field-attempt":3,"Running":10}
`,
			formatSlog: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"strongly typed Zap field","v":0,"ns":"default","podnum":2,"!BADKEY":{"Key":"zap-field-attempt","Type":11,"Integer":3,"String":"","Interface":null},"Running":10}
`,
			keysValues: []interface{}{"ns", "default", "podnum", 2, zap.Int("zap-field-attempt", 3), "Running", 10},
		},
		{
			msg: "non-string key argument",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"non-string key argument passed to logging, ignoring all later arguments","invalid key":200}
{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"non-string key argument","v":0,"ns":"default","podnum":2}
`,
			formatSlog: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"non-string key argument","v":0,"ns":"default","podnum":2,"!BADKEY":200,"replica":"Running","!BADKEY":10}
`,
			keysValues: []interface{}{"ns", "default", "podnum", 2, 200, "replica", "Running", 10},
		},
		{
			msg: "missing value",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"odd number of arguments passed as key-value pairs for logging","ignored key":"no-value"}
{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"missing value","v":0,"ns":"default","podnum":2}
`,
			formatSlog: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"missing value","v":0,"ns":"default","podnum":2,"!BADKEY":"no-value"}
`,
			keysValues: []interface{}{"ns", "default", "podnum", 2, "no-value"},
		},
		{
			msg: "duration value argument",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"duration value argument","v":0,"duration":"5s"}
`,
			keysValues: []interface{}{"duration", time.Duration(5 * time.Second)},
		},
		{
			msg: "valid marshaler",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"valid marshaler","v":0,"obj":"msg=hello"}
`,
			keysValues: []interface{}{"obj", &marshaler{msg: "hello"}},
		},
		{
			msg: "nil marshaler",
			// Handled by our code: it just formats the error.
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"nil marshaler","v":0,"objError":"PANIC=runtime error: invalid memory address or nil pointer dereference"}
`,
			formatSlog: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"nil marshaler","v":0,"obj":"msg=<nil>"}
`,
			keysValues: []interface{}{"obj", (*marshaler)(nil)},
		},
		{
			msg: "nil stringer",
			// Handled by zap: it detects a nil pointer.
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"nil stringer","v":0,"obj":"<nil>"}
`,
			keysValues: []interface{}{"obj", (*stringer)(nil)},
		},
		{
			msg: "panic stringer",
			// Handled by zap: it prints the panic, but using a different key and format than funcr.
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"panic stringer","v":0,"objError":"PANIC=fake panic"}
`,
			keysValues: []interface{}{"obj", &stringerPanic{}},
		},
		{
			msg: "slog values",
			format: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"slog values","v":0,"valuer":"some string","str":"another string","int64":-1,"uint64":2,"float64":3.124,"bool":true,"duration":"1s","timestamp":123.456789,"struct":{"SomeValue":42}}
`,
			keysValues: []interface{}{"valuer", slogValuer("some string"), "str", slogValue("another string"),
				"int64", slogValue(int64(-1)), "uint64", slogValue(uint64(2)),
				"float64", slogValue(float64(3.124)), "bool", slogValue(true),
				"duration", slogValue(time.Second), "timestamp", slogValue(time.Time{} /* replaced by custom formatter */),
				"struct", slogValue(struct{ SomeValue int }{SomeValue: 42}),
			},
			needSlog: true,
		},
		{
			msg: "group with empty key",
			formatSlog: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"group with empty key","v":0,"int":1,"string":"hello"}
`,
			keysValues: []interface{}{slogGroup("", slogInt("int", 1), slogString("string", "hello"))},
			needSlog:   true,
		},
		{
			msg: "empty group",
			formatSlog: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"empty group","v":0}
`,
			keysValues: []interface{}{slogGroup("obj")},
			needSlog:   true,
		},
		{
			msg: "group with key",
			formatSlog: `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"group with key","v":0,"obj":{"int":1,"string":"hello"}}
`,
			keysValues: []interface{}{slogGroup("obj", slogInt("int", 1), slogString("string", "hello"))},
			needSlog:   true,
		},
	}

	test := func(t *testing.T, logNumeric *string, enablePanics *bool, allowZapFields *bool, useSlog bool, data testCase) {
		if (data.needSlog || useSlog) && !hasSlog() {
			t.Skip("slog is not supported")
		}
		if allowZapFields != nil && *allowZapFields && useSlog {
			t.Skip("zap fields not supported by slog")
		}
		if (enablePanics == nil || *enablePanics) && useSlog {
			t.Skip("printing additional log messages not supported by slog")
		}
		if !useSlog && data.format == "" {
			t.Skip("test case only supported for slog")
		}
		var buffer bytes.Buffer
		writer := bufio.NewWriter(&buffer)
		var sampleInfoLogger logr.Logger
		zl := newZapLogger(zapcore.Level(-100), zapcore.AddSync(writer))
		if logNumeric == nil && enablePanics == nil && allowZapFields == nil {
			// No options.
			sampleInfoLogger = zapr.NewLogger(zl)
		} else {
			opts := []zapr.Option{}
			if logNumeric != nil {
				opts = append(opts, zapr.LogInfoLevel(*logNumeric))
			}
			if enablePanics != nil {
				opts = append(opts, zapr.DPanicOnBugs(*enablePanics))
			}
			if allowZapFields != nil {
				opts = append(opts, zapr.AllowZapFields(*allowZapFields))
			}
			sampleInfoLogger = zapr.NewLoggerWithOptions(zl, opts...)
		}
		if useSlog {
			if len(data.names) > 0 {
				t.Skip("WithName not supported for slog")
				// logger = logger.WithName(name)
			}
			if data.wrapper != nil {
				t.Skip("slog does not support WithCallDepth")
			}
			logWithSlog(sampleInfoLogger, data.msg, data.withKeysValues, data.keysValues)
		} else {
			if data.withKeysValues != nil {
				sampleInfoLogger = sampleInfoLogger.WithValues(data.withKeysValues...)
			}
			for _, name := range data.names {
				sampleInfoLogger = sampleInfoLogger.WithName(name)
			}
			if data.wrapper != nil {
				data.wrapper(sampleInfoLogger, data.msg, data.keysValues...)
			} else {
				sampleInfoLogger.Info(data.msg, data.keysValues...)
			}
		}
		if err := writer.Flush(); err != nil {
			t.Fatalf("unexpected error from Flush: %v", err)
		}
		logStr := buffer.String()

		logStrLines := strings.Split(logStr, "\n")
		var dataFormatLines []string
		noPanics := enablePanics != nil && !*enablePanics
		withZapFields := allowZapFields != nil && *allowZapFields
		format := data.format
		if data.formatSlog != "" && useSlog {
			format = data.formatSlog
		}
		for _, line := range strings.Split(format, "\n") {
			// Potentially filter out all or some panic
			// message. We can recognize them based on the
			// expected special keys.
			if strings.Contains(line, "invalid key") ||
				strings.Contains(line, "ignored key") {
				if noPanics || useSlog {
					continue
				}
			} else if strings.Contains(line, "zap field") {
				if noPanics || withZapFields || useSlog {
					continue
				}
			}
			haveZapField := strings.Index(line, `"zap-field`)
			if haveZapField != -1 && !withZapFields && !strings.Contains(line, "zap field") && !useSlog {
				// When Zap fields are not allowed, output gets truncated at the first Zap field.
				line = line[0:haveZapField-1] + "}"
			}
			dataFormatLines = append(dataFormatLines, line)
		}
		if !assert.Equal(t, len(logStrLines), len(dataFormatLines)) {
			t.Errorf("Info has wrong format: no. of lines in log is incorrect \n expected: %s\n got: %s", dataFormatLines, logStrLines)
			return
		}

		for i := range logStrLines {
			if len(logStrLines[i]) == 0 && len(dataFormatLines[i]) == 0 {
				continue
			}
			var ts float64
			var lineNo int
			format := dataFormatLines[i]
			actual := logStrLines[i]
			// TODO: as soon as all supported Go versions have log/slog,
			// the code from slogzapr_test.go can be moved into zapr_test.go
			// and this Replace can get removed.
			actual = strings.ReplaceAll(actual, "zapr_slog_test.go", "zapr_test.go")
			if logNumeric == nil || *logNumeric == "" {
				format = regexp.MustCompile(`,"v":-?\d`).ReplaceAllString(format, "")
			}
			n, err := fmt.Sscanf(actual, format, &ts, &lineNo)
			if n != 2 || err != nil {
				t.Errorf("log format error: %d elements, error %s:\n%s", n, err, actual)
			}
			expected := fmt.Sprintf(format, fixedTime, lineNo)
			require.JSONEq(t, expected, actual)
		}
	}

	noV := ""
	v := "v"
	for name, logNumeric := range map[string]*string{"default": nil, "disabled": &noV, "v": &v} {
		t.Run(fmt.Sprintf("numeric level %v", name), func(t *testing.T) {
			yes := true
			no := false
			nilYesNo := map[string]*bool{"default": nil, "yes": &yes, "no": &no}
			for name, panicMessages := range nilYesNo {
				t.Run(fmt.Sprintf("panic messages %s", name), func(t *testing.T) {
					for name, allowZapFields := range nilYesNo {
						t.Run(fmt.Sprintf("allow zap fields %s", name), func(t *testing.T) {
							for _, data := range testDataInfo {
								t.Run(data.msg, func(t *testing.T) {
									for name, useSlog := range map[string]bool{"with-logr": false, "with-slog": true} {
										t.Run(name, func(t *testing.T) {
											test(t, logNumeric, panicMessages, allowZapFields, useSlog, data)
										})
									}
								})
							}
						})
					}
				})
			}
		})
	}
}

// TestEnabled tests whether log messages are enabled.
func TestEnabled(t *testing.T) {
	for i := 0; i < 11; i++ {
		t.Run(fmt.Sprintf("logger level %d", i), func(t *testing.T) {
			var sampleInfoLogger = zapr.NewLogger(newZapLogger(zapcore.Level(0-i), nil))
			// Very high levels are theoretically possible and need special
			// handling because zap uses int8.
			for j := 0; j <= 128; j++ {
				shouldBeEnabled := i >= j
				t.Run(fmt.Sprintf("message level %d", j), func(t *testing.T) {
					isEnabled := sampleInfoLogger.V(j).Enabled()
					if !isEnabled && shouldBeEnabled {
						t.Errorf("V(%d).Info should be enabled", j)
					} else if isEnabled && !shouldBeEnabled {
						t.Errorf("V(%d).Info should not be enabled", j)
					}

					log := sampleInfoLogger
					for k := 0; k < j; k++ {
						log = log.V(1)
					}
					isEnabled = log.Enabled()
					if !isEnabled && shouldBeEnabled {
						t.Errorf("repeated V(1).Info should be enabled")
					} else if isEnabled && !shouldBeEnabled {
						t.Errorf("repeated V(1).Info should not be enabled")
					}
				})
			}
		})
	}
}

// TestV tests support for numeric log level logging.
func TestLogNumeric(t *testing.T) {
	for logNumeric, formatted := range map[string]string{"": "", "v": `"v":%d,`, "verbose": `"verbose":%d,`} {
		t.Run(fmt.Sprintf("numeric verbosity field %q", logNumeric), func(t *testing.T) {
			for i := 0; i < 2; i++ {
				t.Run(fmt.Sprintf("message verbosity %d", i), func(t *testing.T) {
					var buffer bytes.Buffer
					writer := bufio.NewWriter(&buffer)
					var sampleInfoLogger logr.Logger
					zl := newZapLogger(zapcore.Level(-100), zapcore.AddSync(writer))
					if logNumeric != "" {
						sampleInfoLogger = zapr.NewLoggerWithOptions(zl, zapr.LogInfoLevel(logNumeric))
					} else {
						sampleInfoLogger = zapr.NewLogger(zl)
					}
					sampleInfoLogger.V(i).Info("test", "ns", "default", "podnum", 2, "time", time.Microsecond)
					if err := writer.Flush(); err != nil {
						t.Fatalf("unexpected error from Flush: %v", err)
					}
					logStr := buffer.String()
					var v, lineNo int
					expectedFormat := `{"ts":123.456789,"caller":"zapr/zapr_test.go:%d","msg":"test",` + formatted + `"ns":"default","podnum":2,"time":"1µs"}
`
					expected := ""
					if logNumeric != "" {
						n, err := fmt.Sscanf(logStr, expectedFormat, &lineNo, &v)
						if n != 2 || err != nil {
							t.Errorf("log format error: %d elements, error %s:\n%s", n, err, logStr)
						}
						if v != i {
							t.Errorf("V(%d).Info...) returned v=%d. expected v=%d", i, v, i)
						}
						expected = fmt.Sprintf(expectedFormat, lineNo, v)
					} else {
						n, err := fmt.Sscanf(logStr, expectedFormat, &lineNo)
						if n != 1 || err != nil {
							t.Errorf("log format error: %d elements, error %s:\n%s", n, err, logStr)
						}
						expected = fmt.Sprintf(expectedFormat, lineNo)
					}
					require.JSONEq(t, logStr, expected)
				})
			}
		})
	}
}

// TestError tests Logger.Error.
func TestError(t *testing.T) {
	for _, logError := range []string{"err", "error"} {
		t.Run(fmt.Sprintf("error field name %s", logError), func(t *testing.T) {
			var buffer bytes.Buffer
			writer := bufio.NewWriter(&buffer)
			opts := []zapr.Option{zapr.LogInfoLevel("v")}
			if logError != "error" {
				opts = append(opts, zapr.ErrorKey(logError))
			}
			// Errors always get logged, regardless of log levels.
			var sampleInfoLogger = zapr.NewLoggerWithOptions(newZapLogger(zapcore.Level(-5), zapcore.AddSync(writer)), opts...)
			sampleInfoLogger.V(10).Error(fmt.Errorf("invalid namespace:%s", "default"), "wrong namespace", "ns", "default", "podnum", 2, "time", time.Microsecond)
			if err := writer.Flush(); err != nil {
				t.Fatalf("unexpected error from Flush: %v", err)
			}
			logStr := buffer.String()
			var ts float64
			var lineNo int
			expectedFormat := `{"ts":%f,"caller":"zapr/zapr_test.go:%d","msg":"wrong namespace","ns":"default","podnum":2,"time":"1µs","` + logError + `":"invalid namespace:default"}`
			n, err := fmt.Sscanf(logStr, expectedFormat, &ts, &lineNo)
			if n != 2 || err != nil {
				t.Errorf("log format error: %d elements, error %s:\n%s", n, err, logStr)
			}
			expected := fmt.Sprintf(expectedFormat, ts, lineNo)
			require.JSONEq(t, expected, logStr)
		})
	}
}

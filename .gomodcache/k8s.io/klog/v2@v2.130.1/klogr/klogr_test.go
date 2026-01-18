package klogr

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/internal/test/require"
	"k8s.io/klog/v2/test"

	"github.com/go-logr/logr"
)

const (
	formatDefault = "Default"
	formatNew     = "New"
)

func testOutput(t *testing.T, format string) {
	createLogger := func() logr.Logger {
		switch format {
		case formatNew:
			return New()
		case formatDefault:
			return NewWithOptions()
		default:
			return NewWithOptions(WithFormat(Format(format)))
		}
	}
	tests := map[string]struct {
		klogr              logr.Logger
		text               string
		keysAndValues      []interface{}
		err                error
		expectedOutput     string
		expectedKlogOutput string
	}{
		"should log with values passed to keysAndValues": {
			klogr:         createLogger().V(0),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue"},
			expectedOutput: `"msg"="test" "akey"="avalue"
`,
			expectedKlogOutput: `"test" akey="avalue"
`,
		},
		"should log with name and values passed to keysAndValues": {
			klogr:         createLogger().V(0).WithName("me"),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue"},
			// Sorted by keys.
			expectedOutput: `"msg"="test" "akey"="avalue" "logger"="me"
`,
			// Not sorted by keys.
			expectedKlogOutput: `"test" logger="me" akey="avalue"
`,
		},
		"should log with multiple names and values passed to keysAndValues": {
			klogr:         createLogger().V(0).WithName("hello").WithName("world"),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue"},
			// Sorted by keys.
			expectedOutput: `"msg"="test" "akey"="avalue" "logger"="hello.world"
`,
			// Not sorted by keys.
			expectedKlogOutput: `"test" logger="hello.world" akey="avalue"
`,
		},
		"may print duplicate keys with the same value": {
			klogr:         createLogger().V(0),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue", "akey", "avalue"},
			expectedOutput: `"msg"="test" "akey"="avalue"
`,
			expectedKlogOutput: `"test" akey="avalue" akey="avalue"
`,
		},
		"may print duplicate keys when the values are passed to Info": {
			klogr:         createLogger().V(0),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue", "akey", "avalue2"},
			expectedOutput: `"msg"="test" "akey"="avalue2"
`,
			expectedKlogOutput: `"test" akey="avalue" akey="avalue2"
`,
		},
		"should only print the duplicate key that is passed to Info if one was passed to the logger": {
			klogr:         createLogger().WithValues("akey", "avalue"),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue"},
			expectedOutput: `"msg"="test" "akey"="avalue"
`,
			expectedKlogOutput: `"test" akey="avalue"
`,
		},
		"should sort within logger and parameter key/value pairs in the default format and dump the logger pairs first": {
			klogr:         createLogger().WithValues("akey9", "avalue9", "akey8", "avalue8", "akey1", "avalue1"),
			text:          "test",
			keysAndValues: []interface{}{"akey5", "avalue5", "akey4", "avalue4"},
			expectedOutput: `"msg"="test" "akey1"="avalue1" "akey4"="avalue4" "akey5"="avalue5" "akey8"="avalue8" "akey9"="avalue9"
`,
			expectedKlogOutput: `"test" akey9="avalue9" akey8="avalue8" akey1="avalue1" akey5="avalue5" akey4="avalue4"
`,
		},
		"should only print the key passed to Info when one is already set on the logger": {
			klogr:         createLogger().WithValues("akey", "avalue"),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue2"},
			expectedOutput: `"msg"="test" "akey"="avalue2"
`,
			expectedKlogOutput: `"test" akey="avalue2"
`,
		},
		"should correctly handle odd-numbers of KVs": {
			klogr:         createLogger(),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue", "akey2"},
			expectedOutput: `"msg"="test" "akey"="avalue" "akey2"="(MISSING)"
`,
			expectedKlogOutput: `"test" akey="avalue" akey2="(MISSING)"
`,
		},
		"should correctly handle odd-numbers of KVs in WithValue": {
			klogr:         createLogger().WithValues("keyWithoutValue"),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue", "akey2"},
			// klogr format sorts all key/value pairs.
			expectedOutput: `"msg"="test" "akey"="avalue" "akey2"="(MISSING)" "keyWithoutValue"="(MISSING)"
`,
			expectedKlogOutput: `"test" keyWithoutValue="(MISSING)" akey="avalue" akey2="(MISSING)"
`,
		},
		"should correctly html characters": {
			klogr:         createLogger(),
			text:          "test",
			keysAndValues: []interface{}{"akey", "<&>"},
			expectedOutput: `"msg"="test" "akey"="<&>"
`,
			expectedKlogOutput: `"test" akey="<&>"
`,
		},
		"should correctly handle odd-numbers of KVs in both log values and Info args": {
			klogr:         createLogger().WithValues("basekey1", "basevar1", "basekey2"),
			text:          "test",
			keysAndValues: []interface{}{"akey", "avalue", "akey2"},
			// klogr format sorts all key/value pairs.
			expectedOutput: `"msg"="test" "akey"="avalue" "akey2"="(MISSING)" "basekey1"="basevar1" "basekey2"="(MISSING)"
`,
			expectedKlogOutput: `"test" basekey1="basevar1" basekey2="(MISSING)" akey="avalue" akey2="(MISSING)"
`,
		},
		"should correctly print regular error types": {
			klogr:         createLogger().V(0),
			text:          "test",
			keysAndValues: []interface{}{"err", errors.New("whoops")},
			expectedOutput: `"msg"="test" "err"="whoops"
`,
			expectedKlogOutput: `"test" err="whoops"
`,
		},
		"should use MarshalJSON in the default format if an error type implements it": {
			klogr:         createLogger().V(0),
			text:          "test",
			keysAndValues: []interface{}{"err", &customErrorJSON{"whoops"}},
			expectedOutput: `"msg"="test" "err"="WHOOPS"
`,
			expectedKlogOutput: `"test" err="whoops"
`,
		},
		"should correctly print regular error types when using logr.Error": {
			klogr: createLogger().V(0),
			text:  "test",
			err:   errors.New("whoops"),
			expectedOutput: `"msg"="test" "error"="whoops" 
`,
			expectedKlogOutput: `"test" err="whoops"
`,
		},
	}
	for n, test := range tests {
		t.Run(n, func(t *testing.T) {

			// hijack the klog output
			tmpWriteBuffer := bytes.NewBuffer(nil)
			klog.SetOutput(tmpWriteBuffer)

			if test.err != nil {
				test.klogr.Error(test.err, test.text, test.keysAndValues...)
			} else {
				test.klogr.Info(test.text, test.keysAndValues...)
			}

			// call Flush to ensure the text isn't still buffered
			klog.Flush()

			actual := tmpWriteBuffer.String()
			expectedOutput := test.expectedOutput
			if format == string(FormatKlog) || format == formatDefault {
				expectedOutput = test.expectedKlogOutput
			}
			if actual != expectedOutput {
				t.Errorf("Expected:\n%s\nActual:\n%s\n", expectedOutput, actual)
			}
		})
	}
}

func TestOutput(t *testing.T) {
	fs := test.InitKlog(t)
	require.NoError(t, fs.Set("skip_headers", "true"))

	formats := []string{
		formatNew,
		formatDefault,
		string(FormatSerialize),
		string(FormatKlog),
	}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			testOutput(t, format)
		})
	}
}

type customErrorJSON struct {
	s string
}

func (e *customErrorJSON) Error() string {
	return e.s
}

func (e *customErrorJSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.ToUpper(e.s))
}

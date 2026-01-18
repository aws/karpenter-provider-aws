package httpbinding

import (
	"fmt"
	"math/big"
	"net/http"
	"reflect"
	"testing"
)

func TestHeaderValue(t *testing.T) {
	const keyName = "test-key"
	const expectedKeyName = "test-key"

	cases := map[string]struct {
		header   http.Header
		args     []interface{}
		append   bool
		expected http.Header
	}{
		"set blob": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{[]byte("baz")},
			expected: map[string][]string{
				expectedKeyName: {"YmF6"},
			},
		},
		"set boolean": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{true},
			expected: map[string][]string{
				expectedKeyName: {"true"},
			},
		},
		"set string": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{"string value"},
			expected: map[string][]string{
				expectedKeyName: {"string value"},
			},
		},
		"set byte": {
			header: http.Header{expectedKeyName: []string{"127"}},
			args:   []interface{}{int8(127)},
			expected: map[string][]string{
				expectedKeyName: {"127"},
			},
		},
		"set short": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{int16(32767)},
			expected: map[string][]string{
				expectedKeyName: {"32767"},
			},
		},
		"set integer": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{int32(2147483647)},
			expected: map[string][]string{
				expectedKeyName: {"2147483647"},
			},
		},
		"set long": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{int64(9223372036854775807)},
			expected: map[string][]string{
				expectedKeyName: {"9223372036854775807"},
			},
		},
		"set float": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{float32(3.14159)},
			expected: map[string][]string{
				expectedKeyName: {"3.14159"},
			},
		},
		"set double": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{float64(3.14159)},
			expected: map[string][]string{
				expectedKeyName: {"3.14159"},
			},
		},
		"set bigInteger": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{new(big.Int).SetInt64(42)},
			expected: map[string][]string{
				expectedKeyName: {"42"},
			},
		},
		"set bigDecimal": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{new(big.Float).SetFloat64(1024.10241024)},
			expected: map[string][]string{
				expectedKeyName: {"1.02410241024e+03"},
			},
		},
		"add blob": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{[]byte("baz")},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "YmF6"},
			},
		},
		"add bool": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{true},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "true"},
			},
		},
		"add string": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{"string value"},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "string value"},
			},
		},
		"add byte": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{int8(127)},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "127"},
			},
		},
		"add short": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{int16(32767)},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "32767"},
			},
		},
		"add integer": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{int32(2147483647)},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "2147483647"},
			},
		},
		"add long": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{int64(9223372036854775807)},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "9223372036854775807"},
			},
		},
		"add float": {
			header: http.Header{expectedKeyName: []string{"1.61803"}},
			args:   []interface{}{float32(3.14159)},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"1.61803", "3.14159"},
			},
		},
		"add double": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{float64(3.14159)},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "3.14159"},
			},
		},
		"add bigInteger": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{new(big.Int).SetInt64(42)},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "42"},
			},
		},
		"add bigDecimal": {
			header: http.Header{expectedKeyName: []string{"foobar"}},
			args:   []interface{}{new(big.Float).SetFloat64(1024.10241024)},
			append: true,
			expected: map[string][]string{
				expectedKeyName: {"foobar", "1.02410241024e+03"},
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			if tt.header == nil {
				tt.header = http.Header{}
			}

			hv := newHeaderValue(tt.header, keyName, tt.append)

			if err := setHeader(hv, tt.args); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if e, a := tt.expected, hv.header; !reflect.DeepEqual(e, a) {
				t.Errorf("expected %v, got %v", e, a)
			}
		})
	}
}

func TestHeaders(t *testing.T) {
	const prefix = "X-Amzn-Meta-"
	cases := map[string]struct {
		headers  http.Header
		values   map[string]string
		append   bool
		expected http.Header
	}{
		"set": {
			headers: http.Header{
				"X-Amzn-Meta-Foo": {"bazValue"},
			},
			values: map[string]string{
				"Foo":   "fooValue",
				" Bar ": "barValue",
			},
			expected: http.Header{
				"X-Amzn-Meta-Foo": {"fooValue"},
				"X-Amzn-Meta-Bar": {"barValue"},
			},
		},
		"add": {
			headers: http.Header{
				"X-Amzn-Meta-Foo": {"bazValue"},
			},
			values: map[string]string{
				"Foo":   "fooValue",
				" Bar ": "barValue",
			},
			append: true,
			expected: http.Header{
				"X-Amzn-Meta-Foo": {"bazValue", "fooValue"},
				"X-Amzn-Meta-Bar": {"barValue"},
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			headers := Headers{header: tt.headers, prefix: prefix}

			var f func(key string) HeaderValue
			if tt.append {
				f = headers.AddHeader
			} else {
				f = headers.SetHeader
			}

			for key, value := range tt.values {
				f(key).String(value)
			}

			if e, a := tt.expected, tt.headers; !reflect.DeepEqual(e, a) {
				t.Errorf("expected %v, but got %v", e, a)
			}
		})
	}
}

func setHeader(hv HeaderValue, args []interface{}) error {
	value := args[0]

	switch value.(type) {
	case []byte:
		return reflectCall(reflect.ValueOf(hv.Blob), args)
	case bool:
		return reflectCall(reflect.ValueOf(hv.Boolean), args)
	case string:
		return reflectCall(reflect.ValueOf(hv.String), args)
	case int8:
		return reflectCall(reflect.ValueOf(hv.Byte), args)
	case int16:
		return reflectCall(reflect.ValueOf(hv.Short), args)
	case int32:
		return reflectCall(reflect.ValueOf(hv.Integer), args)
	case int64:
		return reflectCall(reflect.ValueOf(hv.Long), args)
	case float32:
		return reflectCall(reflect.ValueOf(hv.Float), args)
	case float64:
		return reflectCall(reflect.ValueOf(hv.Double), args)
	case *big.Int:
		return reflectCall(reflect.ValueOf(hv.BigInteger), args)
	case *big.Float:
		return reflectCall(reflect.ValueOf(hv.BigDecimal), args)
	default:
		return fmt.Errorf("unhandled header value type")
	}
}

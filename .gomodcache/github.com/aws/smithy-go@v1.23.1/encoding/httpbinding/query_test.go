package httpbinding

import (
	"fmt"
	"math/big"
	"net/url"
	"reflect"
	"testing"
)

func TestQueryValue(t *testing.T) {
	const queryKey = "someKey"

	cases := map[string]struct {
		values   url.Values
		args     []interface{}
		append   bool
		expected url.Values
	}{
		"set blob": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{[]byte("baz")},
			expected: map[string][]string{
				queryKey: {"YmF6"},
			},
		},
		"set bool": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{true},
			expected: map[string][]string{
				queryKey: {"true"},
			},
		},
		"set string": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{"string value"},
			expected: map[string][]string{
				queryKey: {"string value"},
			},
		},
		"set byte": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{int8(127)},
			expected: map[string][]string{
				queryKey: {"127"},
			},
		},
		"set short": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{int16(32767)},
			expected: map[string][]string{
				queryKey: {"32767"},
			},
		},
		"set integer": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{int32(2147483647)},
			expected: map[string][]string{
				queryKey: {"2147483647"},
			},
		},
		"set long": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{int64(9223372036854775807)},
			expected: map[string][]string{
				queryKey: {"9223372036854775807"},
			},
		},
		"set float": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{float32(3.14159)},
			expected: map[string][]string{
				queryKey: {"3.14159"},
			},
		},
		"set double": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{float64(3.14159)},
			expected: map[string][]string{
				queryKey: {"3.14159"},
			},
		},
		"set bigInteger": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{new(big.Int).SetInt64(1)},
			expected: map[string][]string{
				queryKey: {"1"},
			},
		},
		"set bigDecimal": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{new(big.Float).SetFloat64(1024.10241024)},
			expected: map[string][]string{
				queryKey: {"1.02410241024e+03"},
			},
		},
		"add blob": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{[]byte("baz")},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "YmF6"},
			},
		},
		"add bool": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{true},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "true"},
			},
		},
		"add string": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{"string value"},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "string value"},
			},
		},
		"add byte": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{int8(127)},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "127"},
			},
		},
		"add short": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{int16(32767)},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "32767"},
			},
		},
		"add integer": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{int32(2147483647)},
			expected: map[string][]string{
				queryKey: {"2147483647"},
			},
		},
		"add long": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{int64(9223372036854775807)},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "9223372036854775807"},
			},
		},
		"add float": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{float32(3.14159)},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "3.14159"},
			},
		},
		"add double": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{float64(3.14159)},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "3.14159"},
			},
		},
		"add bigInteger": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{new(big.Int).SetInt64(1)},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "1"},
			},
		},
		"add bigDecimal": {
			values: url.Values{queryKey: []string{"foobar"}},
			args:   []interface{}{new(big.Float).SetFloat64(1024.10241024)},
			append: true,
			expected: map[string][]string{
				queryKey: {"foobar", "1.02410241024e+03"},
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			if tt.values == nil {
				tt.values = url.Values{}
			}

			qv := NewQueryValue(tt.values, queryKey, tt.append)

			if err := setQueryValue(qv, tt.args); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if e, a := tt.expected, qv.query; !reflect.DeepEqual(e, a) {
				t.Errorf("expected %v, got %v", e, a)
			}
		})
	}
}

func setQueryValue(qv QueryValue, args []interface{}) error {
	value := args[0]

	switch value.(type) {
	case []byte:
		return reflectCall(reflect.ValueOf(qv.Blob), args)
	case bool:
		return reflectCall(reflect.ValueOf(qv.Boolean), args)
	case string:
		return reflectCall(reflect.ValueOf(qv.String), args)
	case int8:
		return reflectCall(reflect.ValueOf(qv.Byte), args)
	case int16:
		return reflectCall(reflect.ValueOf(qv.Short), args)
	case int32:
		return reflectCall(reflect.ValueOf(qv.Integer), args)
	case int64:
		return reflectCall(reflect.ValueOf(qv.Long), args)
	case float32:
		return reflectCall(reflect.ValueOf(qv.Float), args)
	case float64:
		return reflectCall(reflect.ValueOf(qv.Double), args)
	case *big.Int:
		return reflectCall(reflect.ValueOf(qv.BigInteger), args)
	case *big.Float:
		return reflectCall(reflect.ValueOf(qv.BigDecimal), args)
	default:
		return fmt.Errorf("unhandled query value type")
	}
}

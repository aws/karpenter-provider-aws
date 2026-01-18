package httpbinding

import (
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"testing"
)

func TestURIValue(t *testing.T) {
	const uriKey = "someKey"
	const path = "/some/{someKey}/{path+}"

	type expected struct {
		path string
		raw  string
	}

	cases := map[string]struct {
		path     string
		args     []interface{}
		expected expected
	}{
		"bool": {
			path: path,
			args: []interface{}{true},
			expected: expected{
				path: "/some/true/{path+}",
				raw:  "/some/true/{path+}",
			},
		},
		"string": {
			path: path,
			args: []interface{}{"someValue"},
			expected: expected{
				path: "/some/someValue/{path+}",
				raw:  "/some/someValue/{path+}",
			},
		},
		"byte": {
			path: path,
			args: []interface{}{int8(127)},
			expected: expected{
				path: "/some/127/{path+}",
				raw:  "/some/127/{path+}",
			},
		},
		"short": {
			path: path,
			args: []interface{}{int16(32767)},
			expected: expected{
				path: "/some/32767/{path+}",
				raw:  "/some/32767/{path+}",
			},
		},
		"integer": {
			path: path,
			args: []interface{}{int32(2147483647)},
			expected: expected{
				path: "/some/2147483647/{path+}",
				raw:  "/some/2147483647/{path+}",
			},
		},
		"long": {
			path: path,
			args: []interface{}{int64(9223372036854775807)},
			expected: expected{
				path: "/some/9223372036854775807/{path+}",
				raw:  "/some/9223372036854775807/{path+}",
			},
		},
		"float32": {
			path: path,
			args: []interface{}{float32(3.14159)},
			expected: expected{
				path: "/some/3.14159/{path+}",
				raw:  "/some/3.14159/{path+}",
			},
		},
		"float64": {
			path: path,
			args: []interface{}{float64(3.14159)},
			expected: expected{
				path: "/some/3.14159/{path+}",
				raw:  "/some/3.14159/{path+}",
			},
		},
		"bigInteger": {
			path: path,
			args: []interface{}{new(big.Int).SetInt64(1)},
			expected: expected{
				path: "/some/1/{path+}",
				raw:  "/some/1/{path+}",
			},
		},
		"bigDecimal": {
			path: path,
			args: []interface{}{new(big.Float).SetFloat64(1024.10241024)},
			expected: expected{
				path: "/some/1.02410241024e+03/{path+}",
				raw:  "/some/1.02410241024e%2B03/{path+}",
			},
		},
	}

	buffer := make([]byte, 1024)

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			pBytes, rBytes := []byte(tt.path), []byte(tt.path)

			uv := newURIValue(&pBytes, &rBytes, &buffer, uriKey)

			if err := setURI(uv, tt.args); err != nil {
				t.Fatalf("expected no error, %v", err)
			}

			if e, a := tt.expected.path, string(pBytes); e != a {
				t.Errorf("expected %v, got %v", e, a)
			}

			if e, a := tt.expected.raw, string(rBytes); e != a {
				t.Errorf("expected %v, got %v", e, a)
			}
		})
	}
}

func setURI(uv URIValue, args []interface{}) error {
	value := args[0]

	switch value.(type) {
	case bool:
		return reflectCall(reflect.ValueOf(uv.Boolean), args)
	case string:
		return reflectCall(reflect.ValueOf(uv.String), args)
	case int8:
		return reflectCall(reflect.ValueOf(uv.Byte), args)
	case int16:
		return reflectCall(reflect.ValueOf(uv.Short), args)
	case int32:
		return reflectCall(reflect.ValueOf(uv.Integer), args)
	case int64:
		return reflectCall(reflect.ValueOf(uv.Long), args)
	case float32:
		return reflectCall(reflect.ValueOf(uv.Float), args)
	case float64:
		return reflectCall(reflect.ValueOf(uv.Double), args)
	case *big.Int:
		return reflectCall(reflect.ValueOf(uv.BigInteger), args)
	case *big.Float:
		return reflectCall(reflect.ValueOf(uv.BigDecimal), args)
	default:
		return fmt.Errorf("unhandled value type")
	}
}

func TestParseURI(t *testing.T) {
	cases := []struct {
		Value string
		Path  string
		Query string
	}{
		{
			Value: "/my/uri/foo/bar/baz",
			Path:  "/my/uri/foo/bar/baz",
			Query: "",
		},
		{
			Value: "/path?requiredKey",
			Path:  "/path",
			Query: "requiredKey",
		},
		{
			Value: "/path?",
			Path:  "/path",
			Query: "",
		},
		{
			Value: "?",
			Path:  "",
			Query: "",
		},
	}

	for i, tt := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			path, query := SplitURI(tt.Value)
			if e, a := tt.Path, path; e != a {
				t.Errorf("expected %v, got %v", e, a)
			}
			if e, a := tt.Query, query; e != a {
				t.Errorf("expected %v, got %v", e, a)
			}
		})
	}
}

package http

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitHeaderListValues(t *testing.T) {
	cases := map[string]struct {
		Values    []string
		Expect    []string
		ExpectErr string
	}{
		"no split": {
			Values: []string{
				"abc", "123", "hello",
			},
			Expect: []string{
				"abc", "123", "hello",
			},
		},
		"with split": {
			Values: []string{
				"a, b, c, 1, 2, 3",
			},
			Expect: []string{
				"a", "b", "c", "1", "2", "3",
			},
		},
		"mixed with split": {
			Values: []string{
				"abc", "1, 23", "hello,world",
			},
			Expect: []string{
				"abc", "1", "23", "hello", "world",
			},
		},
		"empty values": {
			Values: []string{
				"",
				", 1, 23, hello,world",
			},
			Expect: []string{
				"", "", "1", "23", "hello", "world",
			},
		},
		"quoted values": {
			Values: []string{
				`abc, 123, "abc,123", "456,efg"`,
			},
			Expect: []string{
				"abc",
				"123",
				"abc,123",
				"456,efg",
			},
		},
		"quoted escaped values": {
			Values: []string{
				`abc,123, "abc,123"   ,   "   \"abc , 123\"  " , "\\456,efg\\b"  ,`,
			},
			Expect: []string{
				"abc",
				"123",
				"abc,123",
				"   \"abc , 123\"  ",
				"\\456,efg\\b",
				"",
			},
		},
		"wrapping space": {
			Values: []string{
				`   abc,123, "abc,123"   ,   "   \"abc , 123\"  " , "\\456,efg\\b"  ,`,
			},
			Expect: []string{
				"abc",
				"123",
				"abc,123",
				"   \"abc , 123\"  ",
				"\\456,efg\\b",
				"",
			},
		},
		"trailing empty value": {
			Values: []string{
				`, , `,
			},
			Expect: []string{
				"", "", "",
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			actual, err := SplitHeaderListValues(c.Values)
			if err != nil {
				t.Fatalf("expect no error, %v", err)
			}

			if !reflect.DeepEqual(c.Expect, actual) {
				t.Errorf("%v != %v", c.Expect, actual)
			}
		})
	}
}

func TestSplitHTTPDateTimestampHeaderListValues(t *testing.T) {
	cases := map[string]struct {
		Values    []string
		Expect    []string
		ExpectErr string
	}{
		"no split": {
			Values: []string{
				"Mon, 16 Dec 2019 23:48:18 GMT",
			},
			Expect: []string{
				"Mon, 16 Dec 2019 23:48:18 GMT",
			},
		},
		"with split": {
			Values: []string{
				"Mon, 16 Dec 2019 23:48:18 GMT, Tue, 17 Dec 2019 23:48:18 GMT",
			},
			Expect: []string{
				"Mon, 16 Dec 2019 23:48:18 GMT",
				"Tue, 17 Dec 2019 23:48:18 GMT",
			},
		},
		"mixed with split": {
			Values: []string{
				"Sun, 15 Dec 2019 23:48:18 GMT",
				"Mon, 16 Dec 2019 23:48:18 GMT, Tue, 17 Dec 2019 23:48:18 GMT",
				"Wed, 18 Dec 2019 23:48:18 GMT",
			},
			Expect: []string{
				"Sun, 15 Dec 2019 23:48:18 GMT",
				"Mon, 16 Dec 2019 23:48:18 GMT",
				"Tue, 17 Dec 2019 23:48:18 GMT",
				"Wed, 18 Dec 2019 23:48:18 GMT",
			},
		},
		"empty values": {
			Values: []string{
				"",
				"Mon, 16 Dec 2019 23:48:18 GMT, Tue, 17 Dec 2019 23:48:18 GMT",
				"Wed, 18 Dec 2019 23:48:18 GMT",
			},
			Expect: []string{
				"",
				"Mon, 16 Dec 2019 23:48:18 GMT",
				"Tue, 17 Dec 2019 23:48:18 GMT",
				"Wed, 18 Dec 2019 23:48:18 GMT",
			},
		},
		"bad format": {
			Values: []string{
				"Mon, 16 Dec 2019 23:48:18 GMT, , Tue, 17 Dec 2019 23:48:18 GMT",
			},
			ExpectErr: "invalid timestamp",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			actual, err := SplitHTTPDateTimestampHeaderListValues(c.Values)
			if len(c.ExpectErr) != 0 {
				if err == nil {
					t.Fatalf("expect error, got none")
				}
				if e, a := c.ExpectErr, err.Error(); !strings.Contains(a, e) {
					t.Fatalf("expect error to contain %v, got %v", e, a)
				}
				return
			}
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}

			if !reflect.DeepEqual(c.Expect, actual) {
				t.Errorf("%v != %v", c.Expect, actual)
			}
		})
	}
}

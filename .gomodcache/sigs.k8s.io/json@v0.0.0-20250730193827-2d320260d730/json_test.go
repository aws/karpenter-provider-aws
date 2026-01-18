/*
Copyright 2021 The Kubernetes Authors.

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

package json

import (
	"bytes"
	gojson "encoding/json"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
)

func TestSyntaxErrorOffset(t *testing.T) {
	malformedJSON :=
		[]byte(`{
	"test1":true,
	"test2":true
	"test3":true
}`)

	err1 := UnmarshalCaseSensitivePreserveInts(malformedJSON, &map[string]interface{}{})
	if err1 == nil {
		t.Fatal("expected err, got none")
	}
	ok1, offset1 := SyntaxErrorOffset(err1)
	if !ok1 {
		t.Fatal("expected ok, got false")
	}

	err2 := gojson.Unmarshal(malformedJSON, &map[string]interface{}{})
	if err2 == nil {
		t.Fatal("expected err, got none")
	}
	ok2, offset2 := SyntaxErrorOffset(err2)
	if !ok2 {
		t.Fatal("expected ok, got false")
	}
	if offset1 != offset2 {
		t.Fatalf("offset mismatch from stdlib and custom: %d != %d", offset1, offset2)
	}
}

func TestUnmarshal(t *testing.T) {
	type Obj struct {
		A int               `json:"a"`
		B int               `json:"b"`
		C map[string]string `json:"c"`
		D int
		E int
	}

	testcases := []struct {
		name             string
		in               string
		to               func() interface{}
		expect           interface{}
		expectErr        string
		expectStrictErrs []string
	}{
		{
			name:   "simple",
			in:     `{"a":1}`,
			to:     func() interface{} { return map[string]interface{}{} },
			expect: map[string]interface{}{"a": int64(1)},
		},
		{
			name:             "case-sensitive",
			in:               `{"a":1,"A":2,"B":3}`,
			to:               func() interface{} { return &Obj{} },
			expect:           &Obj{A: 1},                                         // case-mismatches don't decode
			expectStrictErrs: []string{`unknown field "A"`, `unknown field "B"`}, // multiple strict errors are returned
		},
		{
			name:             "duplicate untyped",
			in:               `{"a":1,"a":2,"b":1,"b":2}`,
			to:               func() interface{} { return map[string]interface{}{} },
			expect:           map[string]interface{}{"a": int64(2), "b": int64(2)},   // last duplicates win
			expectStrictErrs: []string{`duplicate field "a"`, `duplicate field "b"`}, // multiple strict errors are returned
		},
		{
			name:             "duplicate typed",
			in:               `{"a":1,"a":2,"b":1,"b":2}`,
			to:               func() interface{} { return &Obj{} },
			expect:           &Obj{A: 2, B: 2},                                       // last duplicates win
			expectStrictErrs: []string{`duplicate field "a"`, `duplicate field "b"`}, // multiple strict errors are returned
		},
		{
			name:             "duplicate map field",
			in:               `{"c":{"a":"1","a":"2","b":"1","b":"2"}}`,
			to:               func() interface{} { return &Obj{} },
			expect:           &Obj{C: map[string]string{"a": "2", "b": "2"}},             // last duplicates win
			expectStrictErrs: []string{`duplicate field "c.a"`, `duplicate field "c.b"`}, // multiple strict errors are returned
		},
		{
			name:             "unknown fields",
			in:               `{"a":1,"unknown":true,"unknown2":false,"b":2}`,
			to:               func() interface{} { return &Obj{} },
			expect:           &Obj{A: 1, B: 2},                                                // data is populated
			expectStrictErrs: []string{`unknown field "unknown"`, `unknown field "unknown2"`}, // multiple strict errors are returned
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			unmarshalTo := tc.to()
			err := UnmarshalCaseSensitivePreserveInts([]byte(tc.in), &unmarshalTo)

			strictUnmarshalTo := tc.to()
			strictErrors, strictErr := UnmarshalStrict([]byte(tc.in), &strictUnmarshalTo)

			decodeTo := tc.to()
			decodeErr := NewDecoderCaseSensitivePreserveInts(bytes.NewBuffer([]byte(tc.in))).Decode(&decodeTo)

			// ensure expected errors are returned
			if (len(tc.expectErr) > 0) != (err != nil) {
				t.Fatalf("expected err=%v, got %v", len(tc.expectErr) > 0, err)
			}
			if len(tc.expectErr) > 0 && !strings.Contains(err.Error(), tc.expectErr) {
				t.Fatalf("expected error containing '%s', got %v", tc.expectErr, err)
			}

			// ensure expected strict errors are returned
			if len(tc.expectStrictErrs) != len(strictErrors) {
				t.Fatalf("expected %d strict errors, got %v", len(tc.expectStrictErrs), strictErrors)
			}
			for i := range tc.expectStrictErrs {
				strictFieldErr, ok := strictErrors[i].(FieldError)
				if !ok {
					t.Fatalf("strict error does not implement FieldError: %v", strictErrors[i])
				}
				if !strings.Contains(strictFieldErr.Error(), tc.expectStrictErrs[i]) {
					t.Fatalf("expected strict errors:\n  %s\ngot:\n  %v", strings.Join(tc.expectStrictErrs, "\n  "), strictErrors)
				}
			}

			// ensure expected decode errors are returned
			if (len(tc.expectErr) > 0) != (decodeErr != nil) {
				t.Fatalf("expected err=%v, got %v", len(tc.expectErr) > 0, decodeErr)
			}
			if len(tc.expectErr) > 0 && !strings.Contains(decodeErr.Error(), tc.expectErr) {
				t.Fatalf("expected error containing '%s', got %v", tc.expectErr, decodeErr)
			}

			// ensure we got the expected object back
			if !reflect.DeepEqual(tc.expect, unmarshalTo) {
				t.Fatalf("expected\n%#v\ngot\n%#v", tc.expect, unmarshalTo)
			}
			if !reflect.DeepEqual(tc.expect, decodeTo) {
				t.Fatalf("expected\n%#v\ngot\n%#v", tc.expect, decodeTo)
			}

			// ensure Unmarshal and UnmarshalStrict return identical errors and objects
			if !reflect.DeepEqual(err, strictErr) {
				t.Fatalf("unmarshal/strictunmarshal returned different errors:\n%v\n%v", err, strictErr)
			}
			if !reflect.DeepEqual(unmarshalTo, strictUnmarshalTo) {
				t.Fatalf("unmarshal/strictunmarshal returned different objects:\n%#v\n%#v", unmarshalTo, strictUnmarshalTo)
			}

			// ensure Unmarshal and Decode return identical errors and objects
			if !reflect.DeepEqual(err, decodeErr) {
				t.Fatalf("unmarshal/decode returned different errors:\n%v\n%v", err, decodeErr)
			}
			if !reflect.DeepEqual(unmarshalTo, decodeTo) {
				t.Fatalf("unmarshal/decode returned different objects:\n%#v\n%#v", unmarshalTo, decodeTo)
			}
		})
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	testcases := []struct {
		name      string
		unmarshal func(b *testing.B, data []byte, v interface{})
	}{
		{
			name: "stdlib",
			unmarshal: func(b *testing.B, data []byte, v interface{}) {
				if err := gojson.Unmarshal(data, v); err != nil {
					b.Fatal(err)
				}
			},
		},
		{
			name: "unmarshal",
			unmarshal: func(b *testing.B, data []byte, v interface{}) {
				if err := UnmarshalCaseSensitivePreserveInts(data, v); err != nil {
					b.Fatal(err)
				}
			},
		},
		{
			name: "strict",
			unmarshal: func(b *testing.B, data []byte, v interface{}) {
				if strict, err := UnmarshalStrict(data, v); err != nil {
					b.Fatal(err)
				} else if len(strict) > 0 {
					b.Fatal(strict)
				}
			},
		},
		{
			name: "strict-custom",
			unmarshal: func(b *testing.B, data []byte, v interface{}) {
				if strict, err := UnmarshalStrict(data, v, DisallowDuplicateFields, DisallowUnknownFields); err != nil {
					b.Fatal(err)
				} else if len(strict) > 0 {
					b.Fatal(strict)
				}
			},
		},
	}

	data, err := ioutil.ReadFile("testdata/bench.json")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()

	for _, tc := range testcases {
		b.Run("typed_"+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				tc.unmarshal(b, data, &A{})
			}
		})
	}
	for _, tc := range testcases {
		b.Run("untyped_"+tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				tc.unmarshal(b, data, &map[string]interface{}{})
			}
		})
	}
}

type A struct {
	Int    int    `json:"int"`
	Bool   bool   `json:"bool"`
	String string `json:"string"`

	StringMap   map[string]string `json:"map"`
	ObjectArray []A               `json:"array"`

	Small Small `json:"small"`
	Big   Big   `json:"big"`

	Custom Custom `json:"custom"`
}

type Small struct {
	F01 string `json:"f01"`
	F02 string `json:"f02"`
	F03 string `json:"f03"`
	F04 string `json:"f04"`
	F05 string `json:"f05"`
	F06 string `json:"f06"`
	F07 string `json:"f07"`
	F08 string `json:"f08"`
	F09 string `json:"f09"`
	F10 string `json:"f10"`
	F11 string `json:"f11"`
	F12 string `json:"f12"`
	F13 string `json:"f13"`
	F14 string `json:"f14"`
	F15 string `json:"f15"`
	F16 string `json:"f16"`
	F17 string `json:"f17"`
	F18 string `json:"f18"`
	F19 string `json:"f19"`
	F20 string `json:"f20"`
	F21 string `json:"f21"`
	F22 string `json:"f22"`
	F23 string `json:"f23"`
	F24 string `json:"f24"`
	F25 string `json:"f25"`
	F26 string `json:"f26"`
	F27 string `json:"f27"`
	F28 string `json:"f28"`
	F29 string `json:"f29"`
	F30 string `json:"f30"`
	F31 string `json:"f31"`
	F32 string `json:"f32"`
	F33 string `json:"f33"`
	F34 string `json:"f34"`
	F35 string `json:"f35"`
	F36 string `json:"f36"`
	F37 string `json:"f37"`
	F38 string `json:"f38"`
	F39 string `json:"f39"`
	F40 string `json:"f40"`
	F41 string `json:"f41"`
	F42 string `json:"f42"`
	F43 string `json:"f43"`
	F44 string `json:"f44"`
	F45 string `json:"f45"`
	F46 string `json:"f46"`
	F47 string `json:"f47"`
	F48 string `json:"f48"`
	F49 string `json:"f49"`
	F50 string `json:"f50"`
	F51 string `json:"f51"`
	F52 string `json:"f52"`
	F53 string `json:"f53"`
	F54 string `json:"f54"`
	F55 string `json:"f55"`
	F56 string `json:"f56"`
	F57 string `json:"f57"`
	F58 string `json:"f58"`
	F59 string `json:"f59"`
	F60 string `json:"f60"`
	F61 string `json:"f61"`
	F62 string `json:"f62"`
	F63 string `json:"f63"`
	F64 string `json:"f64"`
}

type Big struct {
	F01 string `json:"f01"`
	F02 string `json:"f02"`
	F03 string `json:"f03"`
	F04 string `json:"f04"`
	F05 string `json:"f05"`
	F06 string `json:"f06"`
	F07 string `json:"f07"`
	F08 string `json:"f08"`
	F09 string `json:"f09"`
	F10 string `json:"f10"`
	F11 string `json:"f11"`
	F12 string `json:"f12"`
	F13 string `json:"f13"`
	F14 string `json:"f14"`
	F15 string `json:"f15"`
	F16 string `json:"f16"`
	F17 string `json:"f17"`
	F18 string `json:"f18"`
	F19 string `json:"f19"`
	F20 string `json:"f20"`
	F21 string `json:"f21"`
	F22 string `json:"f22"`
	F23 string `json:"f23"`
	F24 string `json:"f24"`
	F25 string `json:"f25"`
	F26 string `json:"f26"`
	F27 string `json:"f27"`
	F28 string `json:"f28"`
	F29 string `json:"f29"`
	F30 string `json:"f30"`
	F31 string `json:"f31"`
	F32 string `json:"f32"`
	F33 string `json:"f33"`
	F34 string `json:"f34"`
	F35 string `json:"f35"`
	F36 string `json:"f36"`
	F37 string `json:"f37"`
	F38 string `json:"f38"`
	F39 string `json:"f39"`
	F40 string `json:"f40"`
	F41 string `json:"f41"`
	F42 string `json:"f42"`
	F43 string `json:"f43"`
	F44 string `json:"f44"`
	F45 string `json:"f45"`
	F46 string `json:"f46"`
	F47 string `json:"f47"`
	F48 string `json:"f48"`
	F49 string `json:"f49"`
	F50 string `json:"f50"`
	F51 string `json:"f51"`
	F52 string `json:"f52"`
	F53 string `json:"f53"`
	F54 string `json:"f54"`
	F55 string `json:"f55"`
	F56 string `json:"f56"`
	F57 string `json:"f57"`
	F58 string `json:"f58"`
	F59 string `json:"f59"`
	F60 string `json:"f60"`
	F61 string `json:"f61"`
	F62 string `json:"f62"`
	F63 string `json:"f63"`
	F64 string `json:"f64"`
	F65 string `json:"f65"`
}

type Custom struct{}

func (c *Custom) UnmarshalJSON(data []byte) error {
	return nil
}

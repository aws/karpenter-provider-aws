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
	gojson "encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestUnmarshalWithOptions(t *testing.T) {
	type Typed struct {
		A int `json:"a"`
	}

	testcases := []struct {
		name      string
		in        string
		to        interface{}
		options   []UnmarshalOpt
		expect    interface{}
		expectErr bool
	}{
		{
			name:   "default untyped",
			in:     `{"a":1}`,
			to:     map[string]interface{}{},
			expect: map[string]interface{}{"a": float64(1)},
		},
		{
			name:   "default typed",
			in:     `{"a":1, "unknown":"foo"}`,
			to:     &Typed{},
			expect: &Typed{A: 1},
		},
		{
			name:    "usenumbers untyped",
			in:      `{"a":1}`,
			to:      map[string]interface{}{},
			options: []UnmarshalOpt{UseNumber},
			expect:  map[string]interface{}{"a": gojson.Number("1")},
		},
		{
			name:   "usenumbers typed",
			in:     `{"a":1}`,
			to:     &Typed{},
			expect: &Typed{A: 1},
		},
		{
			name:    "disallowunknown untyped",
			in:      `{"a":1,"unknown":"foo"}`,
			to:      map[string]interface{}{},
			options: []UnmarshalOpt{DisallowUnknownFields},
			expect:  map[string]interface{}{"a": float64(1), "unknown": "foo"},
		},
		{
			name:      "disallowunknown typed",
			in:        `{"a":1,"unknown":"foo"}`,
			to:        &Typed{},
			options:   []UnmarshalOpt{DisallowUnknownFields},
			expectErr: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := Unmarshal([]byte(tc.in), &tc.to, tc.options...)
			if tc.expectErr != (err != nil) {
				t.Fatalf("expected err=%v, got %v", tc.expectErr, err)
			}
			if tc.expectErr {
				return
			}
			if !reflect.DeepEqual(tc.expect, tc.to) {
				t.Fatalf("expected\n%#v\ngot\n%#v", tc.expect, tc.to)
			}
		})
	}
}

func TestStrictErrors(t *testing.T) {
	type SubType struct {
	}
	type Typed struct {
		A int                `json:"a"`
		B map[string]SubType `json:"b"`
		C []SubType          `json:"c"`
	}

	testcases := []struct {
		name            string
		in              string
		expectStrictErr bool
		expectErr       string
	}{
		{
			name:            "malformed 1",
			in:              `{`,
			expectStrictErr: false,
			expectErr:       `unexpected end of JSON input`,
		},
		{
			name:            "malformed 2",
			in:              `{}}`,
			expectStrictErr: false,
			expectErr:       `invalid character '}' after top-level value`,
		},
		{
			name:            "malformed 3",
			in:              `{,}`,
			expectStrictErr: false,
			expectErr:       `invalid character ',' looking for beginning of object key string`,
		},
		{
			name:            "type error",
			in:              `{"a":true}`,
			expectStrictErr: false,
			expectErr:       `json: cannot unmarshal bool into Go struct field Typed.a of type int`,
		},
		{
			name:            "unknown",
			in:              `{"a":1,"unknown":true,"unknown":false}`,
			expectStrictErr: true,
			expectErr:       `json: unknown field "unknown"`,
		},
		{
			name:            "unknowns",
			in:              `{"a":1,"unknown":true,"unknown2":true,"unknown":true,"unknown2":true}`,
			expectStrictErr: true,
			expectErr:       `json: unknown field "unknown", unknown field "unknown2"`,
		},
		{
			name:            "nested unknowns",
			in:              `{"a":1,"unknown":true,"unknown2":true,"unknown":true,"unknown2":true,"b":{"a":{"unknown":true}},"c":[{"unknown":true},{"unknown":true}]}`,
			expectStrictErr: true,
			expectErr:       `json: unknown field "unknown", unknown field "unknown2", unknown field "b.a.unknown", unknown field "c[0].unknown", unknown field "c[1].unknown"`,
		},
		{
			name:            "unknowns and type error",
			in:              `{"unknown":true,"a":true}`,
			expectStrictErr: false,
			expectErr:       `json: cannot unmarshal bool into Go struct field Typed.a of type int`,
		},
		{
			name:            "unknowns and malformed error",
			in:              `{"unknown":true}}`,
			expectStrictErr: false,
			expectErr:       `invalid character '}' after top-level value`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := Unmarshal([]byte(tc.in), &Typed{}, DisallowUnknownFields)
			if err == nil {
				t.Fatal("expected error, got none")
			}
			_, isStrictErr := err.(*UnmarshalStrictError)
			if tc.expectStrictErr != isStrictErr {
				t.Fatalf("expected strictErr=%v, got %v: %v", tc.expectStrictErr, isStrictErr, err)
			}
			if err.Error() != tc.expectErr {
				t.Fatalf("expected error\n%s\n%s", tc.expectErr, err)
			}
			t.Log(err)
		})
	}
}

func TestCaseSensitive(t *testing.T) {
	type Embedded1 struct {
		C int `json:"c"`
		D int
	}
	type Embedded2 struct {
		E int `json:"e"`
		F int
	}

	type Obj struct {
		A         int `json:"a"`
		B         int
		Embedded1 `json:",inline"`
		Embedded2
	}

	testcases := []struct {
		name   string
		in     string
		to     interface{}
		expect interface{}
	}{
		{
			name:   "tagged",
			in:     `{"A":"1","A":2,"a":3,"A":4,"A":"5"}`,
			to:     &Obj{},
			expect: &Obj{A: 3},
		},
		{
			name:   "untagged",
			in:     `{"b":"1","b":2,"B":3,"b":4,"b":"5"}`,
			to:     &Obj{},
			expect: &Obj{B: 3},
		},
		{
			name:   "inline embedded tagged subfield",
			in:     `{"C":"1","C":2,"c":3,"C":4,"C":"5"}`,
			to:     &Obj{},
			expect: &Obj{Embedded1: Embedded1{C: 3}},
		},
		{
			name:   "inline embedded untagged subfield",
			in:     `{"d":"1","d":2,"D":3,"d":4,"d":"5"}`,
			to:     &Obj{},
			expect: &Obj{Embedded1: Embedded1{D: 3}},
		},
		{
			name:   "inline embedded field name",
			in:     `{"Embedded1":{"c":3}}`,
			to:     &Obj{},
			expect: &Obj{}, // inlined embedded is not addressable by field name
		},
		{
			name:   "inline embedded empty name",
			in:     `{"":{"c":3}}`,
			to:     &Obj{},
			expect: &Obj{}, // inlined embedded is not addressable by empty json field name
		},
		{
			name:   "untagged embedded tagged subfield",
			in:     `{"E":"1","E":2,"e":3,"E":4,"E":"5"}`,
			to:     &Obj{},
			expect: &Obj{Embedded2: Embedded2{E: 3}},
		},
		{
			name:   "untagged embedded untagged subfield",
			in:     `{"f":"1","f":2,"F":3,"f":4,"f":"5"}`,
			to:     &Obj{},
			expect: &Obj{Embedded2: Embedded2{F: 3}},
		},
		{
			name:   "untagged embedded field name",
			in:     `{"Embedded2":{"e":3}}`,
			to:     &Obj{},
			expect: &Obj{}, // untagged embedded is not addressable by field name
		},
		{
			name:   "untagged embedded empty name",
			in:     `{"":{"e":3}}`,
			to:     &Obj{},
			expect: &Obj{}, // untagged embedded is not addressable by empty json field name
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Unmarshal([]byte(tc.in), &tc.to, CaseSensitive); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tc.expect, tc.to) {
				t.Fatalf("expected\n%#v\ngot\n%#v", tc.expect, tc.to)
			}
		})
	}
}

func TestPreserveInts(t *testing.T) {
	testCases := []struct {
		In   string
		Data interface{}
		Out  string
		Err  bool
	}{
		// Integers
		{
			In:   `0`,
			Data: int64(0),
			Out:  `0`,
		},
		{
			In:   `-0`,
			Data: int64(-0),
			Out:  `0`,
		},
		{
			In:   `1`,
			Data: int64(1),
			Out:  `1`,
		},
		{
			In:   `2147483647`,
			Data: int64(math.MaxInt32),
			Out:  `2147483647`,
		},
		{
			In:   `-2147483648`,
			Data: int64(math.MinInt32),
			Out:  `-2147483648`,
		},
		{
			In:   `9223372036854775807`,
			Data: int64(math.MaxInt64),
			Out:  `9223372036854775807`,
		},
		{
			In:   `-9223372036854775808`,
			Data: int64(math.MinInt64),
			Out:  `-9223372036854775808`,
		},

		// Int overflow
		{
			In:   `9223372036854775808`, // MaxInt64 + 1
			Data: float64(9223372036854775808),
			Out:  `9223372036854776000`,
		},
		{
			In:   `-9223372036854775809`, // MinInt64 - 1
			Data: float64(math.MinInt64),
			Out:  `-9223372036854776000`,
		},

		// Floats
		{
			In:   `0.0`,
			Data: float64(0),
			Out:  `0`,
		},
		{
			In:   `-0.0`,
			Data: float64(-0.0),
			Out:  `-0`,
		},
		{
			In:   `0.5`,
			Data: float64(0.5),
			Out:  `0.5`,
		},
		{
			In:   `1e3`,
			Data: float64(1e3),
			Out:  `1000`,
		},
		{
			In:   `1.5`,
			Data: float64(1.5),
			Out:  `1.5`,
		},
		{
			In:   `-0.3`,
			Data: float64(-.3),
			Out:  `-0.3`,
		},
		{
			// Largest representable float32
			In:   `3.40282346638528859811704183484516925440e+38`,
			Data: float64(math.MaxFloat32),
			Out:  strconv.FormatFloat(math.MaxFloat32, 'g', -1, 64),
		},
		{
			// Smallest float32 without losing precision
			In:   `1.175494351e-38`,
			Data: float64(1.175494351e-38),
			Out:  `1.175494351e-38`,
		},
		{
			// float32 closest to zero
			In:   `1.401298464324817070923729583289916131280e-45`,
			Data: float64(math.SmallestNonzeroFloat32),
			Out:  strconv.FormatFloat(math.SmallestNonzeroFloat32, 'g', -1, 64),
		},
		{
			// Largest representable float64
			In:   `1.797693134862315708145274237317043567981e+308`,
			Data: float64(math.MaxFloat64),
			Out:  strconv.FormatFloat(math.MaxFloat64, 'g', -1, 64),
		},
		{
			// Closest to zero without losing precision
			In:   `2.2250738585072014e-308`,
			Data: float64(2.2250738585072014e-308),
			Out:  `2.2250738585072014e-308`,
		},

		{
			// float64 closest to zero
			In:   `4.940656458412465441765687928682213723651e-324`,
			Data: float64(math.SmallestNonzeroFloat64),
			Out:  strconv.FormatFloat(math.SmallestNonzeroFloat64, 'g', -1, 64),
		},

		{
			// math.MaxFloat64 + 2 overflow
			In:  `1.7976931348623159e+308`,
			Err: true,
		},

		// Arrays
		{
			In: `[` + strings.Join([]string{
				`null`,
				`true`,
				`false`,
				`0`,
				`9223372036854775807`,
				`0.0`,
				`0.5`,
				`1.0`,
				`1.797693134862315708145274237317043567981e+308`,
				`"0"`,
				`"A"`,
				`"Iñtërnâtiônàlizætiøn"`,
				`[null,true,1,1.0,1.5]`,
				`{"boolkey":true,"floatkey":1.0,"intkey":1,"nullkey":null}`,
			}, ",") + `]`,
			Data: []interface{}{
				nil,
				true,
				false,
				int64(0),
				int64(math.MaxInt64),
				float64(0.0),
				float64(0.5),
				float64(1.0),
				float64(math.MaxFloat64),
				string("0"),
				string("A"),
				string("Iñtërnâtiônàlizætiøn"),
				[]interface{}{nil, true, int64(1), float64(1.0), float64(1.5)},
				map[string]interface{}{"nullkey": nil, "boolkey": true, "intkey": int64(1), "floatkey": float64(1.0)},
			},
			Out: `[` + strings.Join([]string{
				`null`,
				`true`,
				`false`,
				`0`,
				`9223372036854775807`,
				`0`,
				`0.5`,
				`1`,
				strconv.FormatFloat(math.MaxFloat64, 'g', -1, 64),
				`"0"`,
				`"A"`,
				`"Iñtërnâtiônàlizætiøn"`,
				`[null,true,1,1,1.5]`,
				`{"boolkey":true,"floatkey":1,"intkey":1,"nullkey":null}`, // gets alphabetized by Marshal
			}, ",") + `]`,
		},

		// Maps
		{
			In:   `{"boolkey":true,"floatkey":1.0,"intkey":1,"nullkey":null}`,
			Data: map[string]interface{}{"nullkey": nil, "boolkey": true, "intkey": int64(1), "floatkey": float64(1.0)},
			Out:  `{"boolkey":true,"floatkey":1,"intkey":1,"nullkey":null}`, // gets alphabetized by Marshal
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d_map", i), func(t *testing.T) {
			// decode the input as a map item
			inputJSON := fmt.Sprintf(`{"data":%s}`, tc.In)
			expectedJSON := fmt.Sprintf(`{"data":%s}`, tc.Out)
			m := map[string]interface{}{}
			err := Unmarshal([]byte(inputJSON), &m, PreserveInts)
			if tc.Err && err != nil {
				// Expected error
				return
			}
			if err != nil {
				t.Fatalf("%s: error decoding: %v", tc.In, err)
			}
			if tc.Err {
				t.Fatalf("%s: expected error, got none", tc.In)
			}
			data, ok := m["data"]
			if !ok {
				t.Fatalf("%s: decoded object missing data key: %#v", tc.In, m)
			}
			if !reflect.DeepEqual(tc.Data, data) {
				t.Fatalf("%s: expected\n\t%#v (%v), got\n\t%#v (%v)", tc.In, tc.Data, reflect.TypeOf(tc.Data), data, reflect.TypeOf(data))
			}

			outputJSON, err := Marshal(m)
			if err != nil {
				t.Fatalf("%s: error encoding: %v", tc.In, err)
			}

			if expectedJSON != string(outputJSON) {
				t.Fatalf("%s: expected\n\t%s, got\n\t%s", tc.In, expectedJSON, string(outputJSON))
			}
		})

		t.Run(fmt.Sprintf("%d_slice", i), func(t *testing.T) {
			// decode the input as an array item
			inputJSON := fmt.Sprintf(`[0,%s]`, tc.In)
			expectedJSON := fmt.Sprintf(`[0,%s]`, tc.Out)
			m := []interface{}{}
			err := Unmarshal([]byte(inputJSON), &m, PreserveInts)
			if tc.Err && err != nil {
				// Expected error
				return
			}
			if err != nil {
				t.Fatalf("%s: error decoding: %v", tc.In, err)
			}
			if tc.Err {
				t.Fatalf("%s: expected error, got none", tc.In)
			}
			if len(m) != 2 {
				t.Fatalf("%s: decoded object wasn't the right length: %#v", tc.In, m)
			}
			data := m[1]
			if !reflect.DeepEqual(tc.Data, data) {
				t.Fatalf("%s: expected\n\t%#v (%v), got\n\t%#v (%v)", tc.In, tc.Data, reflect.TypeOf(tc.Data), data, reflect.TypeOf(data))
			}

			outputJSON, err := Marshal(m)
			if err != nil {
				t.Fatalf("%s: error encoding: %v", tc.In, err)
			}

			if expectedJSON != string(outputJSON) {
				t.Fatalf("%s: expected\n\t%s, got\n\t%s", tc.In, expectedJSON, string(outputJSON))
			}
		})

		t.Run(fmt.Sprintf("%d_raw", i), func(t *testing.T) {
			// decode the input as a standalone object
			inputJSON := fmt.Sprintf(`%s`, tc.In)
			expectedJSON := fmt.Sprintf(`%s`, tc.Out)
			var m interface{}
			err := Unmarshal([]byte(inputJSON), &m, PreserveInts)
			if tc.Err && err != nil {
				// Expected error
				return
			}
			if err != nil {
				t.Fatalf("%s: error decoding: %v", tc.In, err)
			}
			if tc.Err {
				t.Fatalf("%s: expected error, got none", tc.In)
			}
			data := m
			if !reflect.DeepEqual(tc.Data, data) {
				t.Fatalf("%s: expected\n\t%#v (%v), got\n\t%#v (%v)", tc.In, tc.Data, reflect.TypeOf(tc.Data), data, reflect.TypeOf(data))
			}

			outputJSON, err := Marshal(m)
			if err != nil {
				t.Fatalf("%s: error encoding: %v", tc.In, err)
			}

			if expectedJSON != string(outputJSON) {
				t.Fatalf("%s: expected\n\t%s, got\n\t%s", tc.In, expectedJSON, string(outputJSON))
			}
		})
	}

	// UseNumber takes precedence over PreserveInts
	v := map[string]interface{}{}
	if err := Unmarshal([]byte(`{"a":1}`), &v, PreserveInts, UseNumber); err != nil {
		t.Fatal(err)
	}
	if e, a := map[string]interface{}{"a": gojson.Number("1")}, v; !reflect.DeepEqual(e, a) {
		t.Fatalf("expected\n\t%#v, got\n\t%#v", e, a)
	}
}

func TestDisallowDuplicateFields(t *testing.T) {
	type SubType struct {
		F int `json:"f"`
		G int `json:"g"`
	}
	type MixedObj struct {
		A int `json:"a"`
		B int `json:"b"`
		C int
		D map[string]string `json:"d"`
		E []SubType         `json:"e"`
	}
	type SmallObj struct {
		F01 int
		F02 int
		F03 int
		F04 int
		F05 int
		F06 int
		F07 int
		F08 int
		F09 int
		F10 int
		F11 int
		F12 int
		F13 int
		F14 int
		F15 int
		F16 int
		F17 int
		F18 int
		F19 int
		F20 int
		F21 int
		F22 int
		F23 int
		F24 int
		F25 int
		F26 int
		F27 int
		F28 int
		F29 int
		F30 int
		F31 int
		F32 int
		F33 int
		F34 int
		F35 int
		F36 int
		F37 int
		F38 int
		F39 int
		F40 int
		F41 int
		F42 int
		F43 int
		F44 int
		F45 int
		F46 int
		F47 int
		F48 int
		F49 int
		F50 int
		F51 int
		F52 int
		F53 int
		F54 int
		F55 int
		F56 int
		F57 int
		F58 int
		F59 int
		F60 int
		F61 int
		F62 int
		F63 int
		F64 int
	}

	type BigObj struct {
		F01 int
		F02 int
		F03 int
		F04 int
		F05 int
		F06 int
		F07 int
		F08 int
		F09 int
		F10 int
		F11 int
		F12 int
		F13 int
		F14 int
		F15 int
		F16 int
		F17 int
		F18 int
		F19 int
		F20 int
		F21 int
		F22 int
		F23 int
		F24 int
		F25 int
		F26 int
		F27 int
		F28 int
		F29 int
		F30 int
		F31 int
		F32 int
		F33 int
		F34 int
		F35 int
		F36 int
		F37 int
		F38 int
		F39 int
		F40 int
		F41 int
		F42 int
		F43 int
		F44 int
		F45 int
		F46 int
		F47 int
		F48 int
		F49 int
		F50 int
		F51 int
		F52 int
		F53 int
		F54 int
		F55 int
		F56 int
		F57 int
		F58 int
		F59 int
		F60 int
		F61 int
		F62 int
		F63 int
		F64 int
		F65 int
	}

	testcases := []struct {
		name      string
		in        string
		to        interface{}
		expectErr string
	}{
		{
			name:      "duplicate typed",
			in:        `{"a":1,"a":2,"a":3,"b":4,"b":5,"b":6,"C":7,"C":8,"C":9,"e":[{"f":1,"f":1}]}`,
			to:        &MixedObj{},
			expectErr: `json: duplicate field "a", duplicate field "b", duplicate field "C", duplicate field "e[0].f"`,
		},
		{
			name:      "duplicate typed map field",
			in:        `{"d":{"a":"","b":"","c":"","a":"","b":"","c":""}}`,
			to:        &MixedObj{},
			expectErr: `json: duplicate field "d.a", duplicate field "d.b", duplicate field "d.c"`,
		},
		{
			name:      "duplicate untyped map",
			in:        `{"a":"","b":"","a":"","b":"","c":{"c":"","c":""},"d":{"e":{"f":1,"f":1}},"e":[{"g":1,"g":1}]}`,
			to:        &map[string]interface{}{},
			expectErr: `json: duplicate field "a", duplicate field "b", duplicate field "c.c", duplicate field "d.e.f", duplicate field "e[0].g"`,
		},
		{
			name:      "small obj",
			in:        `{"f01":1,"f01":2,"f32":1,"f32":2,"f64":1,"f64":2}`,
			to:        &SmallObj{},
			expectErr: `json: duplicate field "F01", duplicate field "F32", duplicate field "F64"`,
		},
		{
			name:      "big obj",
			in:        `{"f01":1,"f01":2,"f32":1,"f32":2,"f64":1,"f64":2,"f65":1,"f65":2}`,
			to:        &BigObj{},
			expectErr: `json: duplicate field "F01", duplicate field "F32", duplicate field "F64", duplicate field "F65"`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := Unmarshal([]byte(tc.in), &tc.to, DisallowDuplicateFields)
			if (len(tc.expectErr) > 0) != (err != nil) {
				t.Fatalf("expected err=%v, got %v", len(tc.expectErr) > 0, err)
			}
			if err == nil {
				return
			}
			if err.Error() != tc.expectErr {
				t.Fatalf("expected err\n%s\ngot\n%s", tc.expectErr, err.Error())
			}
		})
	}
}

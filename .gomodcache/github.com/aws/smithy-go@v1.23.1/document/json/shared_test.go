package json_test

import (
	"bytes"
	json2 "encoding/json"
	"math"
	"math/big"
	"strconv"

	"github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/document/json"
	"github.com/aws/smithy-go/ptr"
)

type StructA struct {
	FieldName    string
	FieldPtrName *string

	FieldRename  string `document:"field_rename"`
	FieldIgnored string `document:"-"`

	FieldOmitEmpty    string  `document:",omitempty"`
	FieldPtrOmitEmpty *string `document:",omitempty"`

	FieldRenameOmitEmpty string `document:"field_rename_omit_empty,omitempty"`

	FieldNestedStruct          *StructA `document:"field_nested_struct"`
	FieldNestedStructOmitEmpty *StructA `document:"field_nested_struct_omit_empty,omitempty"`

	FieldMap map[string]string `document:",omitempty"`

	fieldUnexported string

	StructB
}

type StructB struct {
	FieldName string `document:"field_name"`
}

type testCase struct {
	decoderOptions    json.DecoderOptions
	encoderOptions    json.EncoderOptions
	disableJSONNumber bool
	json              []byte
	actual, want      interface{}
	wantErr           bool
}

var sharedStringTests = map[string]testCase{
	"string": {
		json: []byte(`"foo"`),
		actual: func() interface{} {
			var v string
			return &v
		}(),
		want: "foo",
	},
	"interface{}": {
		json: []byte(`"foo"`),
		actual: func() interface{} {
			var v interface{}
			return &v
		}(),
		want: "foo",
	},
}

var sharedObjectTests = map[string]testCase{
	"null for pointer type": {
		json: []byte(`null`),
		actual: func() interface{} {
			var v *StructA
			return &v
		}(),
	},
	"zero value": {
		json: []byte(`{
  "FieldName": "",
  "FieldPtrName": null,
  "field_name": "",
  "field_nested_struct": null,
  "field_rename": ""
}`),
		actual: func() interface{} {
			var v StructA
			return &v
		}(),
		want: StructA{},
	},
	"filled json structure": {
		json: []byte(`{
  "FieldMap": {
    "a": "b",
    "bar": "baz",
    "baz": "qux",
    "c": "d",
    "foo": "bar",
    "z": "a"
  },
  "FieldName": "a",
  "FieldPtrName": "b",
  "field_rename": "c",
  "FieldOmitEmpty": "d",
  "FieldPtrOmitEmpty": "e",
  "field_rename_omit_empty": "f",
  "field_nested_struct": {
    "FieldName": "a",
    "FieldPtrName": null,
    "field_name": "",
    "field_nested_struct": null,
    "field_rename": ""
  },
  "field_nested_struct_omit_empty": {
    "FieldName": "a",
    "FieldPtrName": null,
    "field_name": "",
    "field_nested_struct": null,
    "field_rename": ""
  },
  "field_name": "A"
}`),
		actual: func() interface{} {
			var v *StructA
			return &v
		}(),
		want: &StructA{
			FieldMap: map[string]string{
				"foo": "bar",
				"bar": "baz",
				"baz": "qux",
				"a":   "b",
				"c":   "d",
				"z":   "a",
			},
			FieldName:            "a",
			FieldPtrName:         ptr.String("b"),
			FieldRename:          "c",
			FieldOmitEmpty:       "d",
			FieldPtrOmitEmpty:    ptr.String("e"),
			FieldRenameOmitEmpty: "f",
			FieldNestedStruct: &StructA{
				FieldName: "a",
			},
			FieldNestedStructOmitEmpty: &StructA{
				FieldName: "a",
			},
			StructB: StructB{
				FieldName: "A",
			},
		},
	},
}

var sharedArrayTestCases = map[string]testCase{
	"slice": {
		json: []byte(`["foo", "bar", "baz"]`),
		actual: func() interface{} {
			var v []string
			return &v
		}(),
		want: []string{"foo", "bar", "baz"},
	},
	"array": {
		json: []byte(`["foo", "bar", "baz"]`),
		actual: func() interface{} {
			var v [3]string
			return &v
		}(),
		want: [3]string{"foo", "bar", "baz"},
	},

	"interface{}": {
		json: []byte(`["foo", "bar", "baz"]`),
		actual: func() interface{} {
			var v interface{}
			return &v
		}(),
		want: []interface{}{"foo", "bar", "baz"},
	},
}

var sharedNumberTestCases = map[string]testCase{
	"json.Number to interface{}": {
		json: []byte(`3.14159`),
		actual: func() interface{} {
			var v interface{}
			return &v
		}(),
		want: ptrNumber("3.14159"),
	},

	"json float64 to interface{}": {
		json: []byte(`3.14159`),
		actual: func() interface{} {
			var v interface{}
			return &v
		}(),
		want:              ptr.Float64(3.14159),
		disableJSONNumber: true,
	},

	"json.Number to document.Number": {
		json: []byte(`3.14159`),
		actual: func() interface{} {
			var v document.Number
			return &v
		}(),
		want: document.Number("3.14159"),
	},

	"json.Number to *document.Number": {
		json: []byte(`3.14159`),
		actual: func() interface{} {
			var v *document.Number
			return &v
		}(),
		want: document.Number("3.14159"),
	},

	/*
		int, int16, int32, int64
	*/
	"json.Number to int": {
		json: []byte(`2147483647`),
		actual: func() interface{} {
			var x int
			return &x
		}(),
		want: ptr.Int(2147483647),
	},
	"json float64 to int": {
		json: []byte(`2147483647`),
		actual: func() interface{} {
			var x int
			return &x
		}(),
		want:              ptr.Int(2147483647),
		disableJSONNumber: true,
	},
	"json.Number to int8": {
		json: []byte(`127`),
		actual: func() interface{} {
			var x int8
			return &x
		}(),
		want: ptr.Int8(127),
	},
	"json float64 to int8": {
		json: []byte(`127`),
		actual: func() interface{} {
			var x int8
			return &x
		}(),
		want:              ptr.Int8(127),
		disableJSONNumber: true,
	},
	"json.Number to int16": {
		json: []byte(`32767`),
		actual: func() interface{} {
			var x int16
			return &x
		}(),
		want: ptr.Int16(32767),
	},
	"json float64 to int16": {
		json: []byte(`32767`),
		actual: func() interface{} {
			var x int16
			return &x
		}(),
		want:              ptr.Int16(32767),
		disableJSONNumber: true,
	},
	"json.Number to int32": {
		json: []byte(`2147483647`),
		actual: func() interface{} {
			var x int32
			return &x
		}(),
		want: ptr.Int32(2147483647),
	},
	"json float64 to int32": {
		json: []byte(`2147483647`),
		actual: func() interface{} {
			var x int32
			return &x
		}(),
		want:              ptr.Int32(2147483647),
		disableJSONNumber: true,
	},
	"json.Number to int64": {
		json: []byte("9223372036854775807"),
		actual: func() interface{} {
			var x int64
			return &x
		}(),
		want: ptr.Int64(9223372036854775807),
	},
	"json float64 to int64": {
		json: []byte("2147483648"),
		actual: func() interface{} {
			var x int64
			return &x
		}(),
		want:              ptr.Int64(2147483648),
		disableJSONNumber: true,
	},

	/*
		uint, uint16, uint32, uint64
	*/
	"json.Number to uint": {
		json: []byte(`4294967295`),
		actual: func() interface{} {
			var x uint
			return &x
		}(),
		want: ptr.Uint(4294967295),
	},
	"json float64 to uint": {
		json: []byte(`4294967295`),
		actual: func() interface{} {
			var x uint
			return &x
		}(),
		want:              ptr.Uint(4294967295),
		disableJSONNumber: true,
	},
	"json.Number to uint8": {
		json: []byte(`255`),
		actual: func() interface{} {
			var x uint8
			return &x
		}(),
		want: ptr.Uint8(255),
	},
	"json float64 to uint8": {
		json: []byte(`255`),
		actual: func() interface{} {
			var x uint8
			return &x
		}(),
		want:              ptr.Uint8(255),
		disableJSONNumber: true,
	},
	"json.Number to uint16": {
		json: []byte(`65535`),
		actual: func() interface{} {
			var x uint16
			return &x
		}(),
		want: ptr.Uint16(65535),
	},
	"json float64 to uint16": {
		json: []byte(`65535`),
		actual: func() interface{} {
			var x uint16
			return &x
		}(),
		want:              ptr.Uint16(65535),
		disableJSONNumber: true,
	},
	"json.Number to uint32": {
		json: []byte(`4294967295`),
		actual: func() interface{} {
			var x uint32
			return &x
		}(),
		want: ptr.Uint32(4294967295),
	},
	"json float64 to uint32": {
		json: []byte(`4294967295`),
		actual: func() interface{} {
			var x uint32
			return &x
		}(),
		want:              ptr.Uint32(4294967295),
		disableJSONNumber: true,
	},
	"json.Number to uint64": {
		json: []byte("18446744073709551615"),
		actual: func() interface{} {
			var x uint64
			return &x
		}(),
		want: ptr.Uint64(18446744073709551615),
	},
	"json float64 to uint64": {
		json: []byte("4294967295"),
		actual: func() interface{} {
			var x uint64
			return &x
		}(),
		want:              ptr.Uint64(4294967295),
		disableJSONNumber: true,
	},

	/*
		float32, float64
	*/
	"json.Number to float32": {
		json: []byte(strconv.FormatFloat(math.MaxFloat32, 'e', -1, 32)),
		actual: func() interface{} {
			var x float32
			return &x
		}(),
		want: ptr.Float32(math.MaxFloat32),
	},
	"json float64 to float32": {
		json: []byte(strconv.FormatFloat(3.14159, 'e', -1, 32)),
		actual: func() interface{} {
			var x float32
			return &x
		}(),
		want:              ptr.Float32(3.14159),
		disableJSONNumber: true,
	},
	"json.Number to float64": {
		json: []byte(strconv.FormatFloat(math.MaxFloat64, 'e', -1, 64)),
		actual: func() interface{} {
			var x float64
			return &x
		}(),
		want: ptr.Float64(math.MaxFloat64),
	},
	"json float64 to float64": {
		json: []byte(strconv.FormatFloat(3.14159, 'e', -1, 64)),
		actual: func() interface{} {
			var x float64
			return &x
		}(),
		want:              ptr.Float64(3.14159),
		disableJSONNumber: true,
	},

	/*
		Arbitrary Number Sizes
	*/
	"json.Number to big.Float": {
		json: []byte(strconv.FormatFloat(math.MaxFloat64, 'e', -1, 64)),
		actual: func() interface{} {
			var x big.Float
			return &x
		}(),
		want: func() *big.Float {
			// this is slightly different than big.NewFloat(math.MaxFloat64)
			x, _ := (&big.Float{}).SetString(strconv.FormatFloat(math.MaxFloat64, 'e', -1, 64))
			return x
		}(),
	},
	"float64 to big.Float": {
		json: []byte(strconv.FormatFloat(math.MaxFloat64, 'e', -1, 64)),
		actual: func() interface{} {
			var x big.Float
			return &x
		}(),
		want: func() *big.Float {
			return big.NewFloat(math.MaxFloat64)
		}(),
		disableJSONNumber: true,
	},
	"json.Number to big.Int": {
		json: []byte(strconv.FormatInt(math.MaxInt64, 10)),
		actual: func() interface{} {
			var x big.Int
			return &x
		}(),
		want: func() *big.Int {
			return big.NewInt(math.MaxInt64)
		}(),
	},
	"float64 to big.Int": {
		json: []byte(strconv.FormatInt(math.MaxInt32, 10)),
		actual: func() interface{} {
			var x big.Int
			return &x
		}(),
		want: func() *big.Int {
			return big.NewInt(math.MaxInt32)
		}(),
		disableJSONNumber: true,
	},
}

func ptrNumber(number document.Number) *document.Number {
	return &number
}

func MustJSONUnmarshal(v []byte, useJSONNumber bool) interface{} {
	var jv interface{}
	decoder := json2.NewDecoder(bytes.NewReader(v))
	if useJSONNumber {
		decoder.UseNumber()
	}
	if err := decoder.Decode(&jv); err != nil {
		panic(err)
	}
	return jv
}

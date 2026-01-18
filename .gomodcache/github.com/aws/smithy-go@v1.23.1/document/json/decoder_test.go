package json_test

import (
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/document/internal/serde"
	"github.com/aws/smithy-go/document/json"
)

var decodeArrayTestCases = map[string]testCase{
	"array not enough capacity": {
		json: []byte(`["foo", "bar", "baz"]`),
		actual: func() interface{} {
			var v [2]string
			return &v
		}(),
		want: [2]string{"foo", "bar"},
	},
}

func TestDecoder_DecodeJSONInterface(t *testing.T) {
	t.Run("Shared", func(t *testing.T) {
		t.Run("Object", func(t *testing.T) {
			for name, tt := range sharedObjectTests {
				t.Run(name, func(t *testing.T) {
					testDecodeJSONInterface(t, tt)
				})
			}
		})
		t.Run("Array", func(t *testing.T) {
			for name, tt := range sharedArrayTestCases {
				t.Run(name, func(t *testing.T) {
					testDecodeJSONInterface(t, tt)
				})
			}
		})
		t.Run("Number", func(t *testing.T) {
			for name, tt := range sharedNumberTestCases {
				t.Run(name, func(t *testing.T) {
					testDecodeJSONInterface(t, tt)
				})
			}
		})
		t.Run("String", func(t *testing.T) {
			for name, tt := range sharedStringTests {
				t.Run(name, func(t *testing.T) {
					testDecodeJSONInterface(t, tt)
				})
			}
		})
	})
	for name, tt := range decodeArrayTestCases {
		t.Run("Array", func(t *testing.T) {
			t.Run(name, func(t *testing.T) {
				testDecodeJSONInterface(t, tt)
			})
		})
	}
}

func TestNoSerde(t *testing.T) {
	type noSerde = document.NoSerde
	type InvalidType struct {
		FieldName string
		noSerde
	}
	var v InvalidType

	err := json.NewDecoder().DecodeJSONInterface(MustJSONUnmarshal([]byte(`{
  "FieldName": "value"
}`), true), &v)
	if err == nil {
		t.Fatalf("expect error, got nil")
	}

	type EmbedInvalidType struct {
		FieldName string
		TypeField InvalidType
	}

	var ev EmbedInvalidType

	err = json.NewDecoder().DecodeJSONInterface(MustJSONUnmarshal([]byte(`{
  "FieldName": "value",
  "TypeField": {
    "FieldName": "value"
  }
}`), true), &ev)
	if err == nil {
		t.Fatalf("expect error, got nil")
	}
}

func TestNewDecoderUnsupportedTypes(t *testing.T) {
	cases := []struct {
		input             []byte
		value             interface{}
		disableJSONNumber bool
	}{
		{
			input: []byte(`1000000000`),
			value: func() interface{} {
				var t time.Time
				return &t
			}(),
		},
		{
			input: []byte(`1000000000`),
			value: func() interface{} {
				var t time.Time
				return &t
			}(),
			disableJSONNumber: true,
		},
		{
			input: []byte(`{}`),
			value: func() interface{} {
				var t time.Time
				return &t
			}(),
		},
		{
			input: []byte(`{}`),
			value: func() interface{} {
				type def interface {
					String()
				}
				var i def
				return &i
			}(),
		},
	}

	decoder := json.NewDecoder()
	for _, tt := range cases {
		err := decoder.DecodeJSONInterface(MustJSONUnmarshal(tt.input, !tt.disableJSONNumber), tt.value)
		if err == nil {
			t.Errorf("expect error, got nil")
		}
	}
}

func testDecodeJSONInterface(t *testing.T, tt testCase) {
	t.Helper()

	d := json.NewDecoder(func(options *json.DecoderOptions) {
		*options = tt.decoderOptions
	})
	if err := d.DecodeJSONInterface(MustJSONUnmarshal(tt.json, !tt.disableJSONNumber), tt.actual); (err != nil) != tt.wantErr {
		t.Errorf("DecodeJSONInterface() error = %v, wantErr %v", err, tt.wantErr)
	}

	expect := serde.PtrToValue(tt.want)
	actual := serde.PtrToValue(tt.actual)
	if !reflect.DeepEqual(expect, actual) {
		t.Errorf("%v != %v", expect, actual)
	}
}

func cmpBigFloat() func(x big.Float, y big.Float) bool {
	return func(x, y big.Float) bool {
		return x.String() == y.String()
	}
}

func cmpBigInt() func(x big.Int, y big.Int) bool {
	return func(x, y big.Int) bool {
		return x.String() == y.String()
	}
}

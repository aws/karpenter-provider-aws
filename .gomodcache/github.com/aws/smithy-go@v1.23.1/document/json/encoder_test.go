package json_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/smithy-go/document"
	"github.com/aws/smithy-go/document/json"
)

func TestEncoder_Encode(t *testing.T) {
	t.Run("Object", func(t *testing.T) {
		for name, tt := range sharedObjectTests {
			t.Run(name, func(t *testing.T) {
				testEncode(t, tt)
			})
		}
	})
	t.Run("Array", func(t *testing.T) {
		for name, tt := range sharedArrayTestCases {
			t.Run(name, func(t *testing.T) {
				testEncode(t, tt)
			})
		}
	})
	t.Run("Number", func(t *testing.T) {
		for name, tt := range sharedNumberTestCases {
			t.Run(name, func(t *testing.T) {
				testEncode(t, tt)
			})
		}
	})
	t.Run("String", func(t *testing.T) {
		for name, tt := range sharedStringTests {
			t.Run(name, func(t *testing.T) {
				testEncode(t, tt)
			})
		}
	})
}

func TestNewEncoderUnsupportedTypes(t *testing.T) {
	type customTime time.Time
	type noSerde = document.NoSerde
	type NestedThing struct {
		SomeThing string
		noSerde
	}
	type Thing struct {
		OtherThing  string
		NestedThing NestedThing
	}

	cases := []interface{}{
		time.Now().UTC(),
		customTime(time.Now().UTC()),
		Thing{OtherThing: "foo", NestedThing: NestedThing{SomeThing: "bar"}},
	}

	encoder := json.NewEncoder()
	for _, tt := range cases {
		_, err := encoder.Encode(tt)
		if err == nil {
			t.Errorf("expect error, got nil")
		}
	}
}

func TestDeterministicMapOrder(t *testing.T) {
	value := struct {
		Map map[string]string
	}{
		Map: map[string]string{
			"foo": "bar",
			"a":   "b",
			"bar": "baz",
			"b":   "c",
			"baz": "qux",
			"c":   "d",
		},
	}
	expect := `{"Map":{"a":"b","b":"c","bar":"baz","baz":"qux","c":"d","foo":"bar"}}`

	encoder := json.NewEncoder()
	actual, err := encoder.Encode(value)
	if err != nil {
		t.Fatal(err)
	}

	if expect != string(actual) {
		t.Errorf("encode determinstic order:\n%q !=\n%q", expect, actual)
	}
}

func testEncode(t *testing.T, tt testCase) {
	t.Helper()

	e := json.NewEncoder(func(options *json.EncoderOptions) {
		*options = tt.encoderOptions
	})

	encodeBytes, err := e.Encode(tt.actual)
	if (err != nil) != tt.wantErr {
		t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
	}

	expect := MustJSONUnmarshal(tt.json, !tt.disableJSONNumber)
	got := MustJSONUnmarshal(encodeBytes, !tt.disableJSONNumber)

	if !reflect.DeepEqual(expect, got) {
		t.Errorf("%v != %v", expect, got)
	}
}

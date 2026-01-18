package tests

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/mailru/easyjson"
	"github.com/mailru/easyjson/jwriter"
)

type testType interface {
	json.Marshaler
	json.Unmarshaler
}

var testCases = []struct {
	Decoded testType
	Encoded string
}{
	{&primitiveTypesValue, primitiveTypesString},
	{&namedPrimitiveTypesValue, namedPrimitiveTypesString},
	{&structsValue, structsString},
	{&omitEmptyValue, omitEmptyString},
	{&snakeStructValue, snakeStructString},
	{&omitEmptyDefaultValue, omitEmptyDefaultString},
	{&optsValue, optsString},
	{&rawValue, rawString},
	{&stdMarshalerValue, stdMarshalerString},
	{&userMarshalerValue, userMarshalerString},
	{&unexportedStructValue, unexportedStructString},
	{&excludedFieldValue, excludedFieldString},
	{&sliceValue, sliceString},
	{&arrayValue, arrayString},
	{&mapsValue, mapsString},
	{&deepNestValue, deepNestString},
	{&IntsValue, IntsString},
	{&mapStringStringValue, mapStringStringString},
	{&namedTypeValue, namedTypeValueString},
	{&customMapKeyTypeValue, customMapKeyTypeValueString},
	{&embeddedTypeValue, embeddedTypeValueString},
	{&mapMyIntStringValue, mapMyIntStringValueString},
	{&mapIntStringValue, mapIntStringValueString},
	{&mapInt32StringValue, mapInt32StringValueString},
	{&mapInt64StringValue, mapInt64StringValueString},
	{&mapUintStringValue, mapUintStringValueString},
	{&mapUint32StringValue, mapUint32StringValueString},
	{&mapUint64StringValue, mapUint64StringValueString},
	{&mapUintptrStringValue, mapUintptrStringValueString},
	{&intKeyedMapStructValue, intKeyedMapStructValueString},
	{&intArrayStructValue, intArrayStructValueString},
	{&myUInt8SliceValue, myUInt8SliceString},
	{&myUInt8ArrayValue, myUInt8ArrayString},
	{&mapWithEncodingMarshaler, mapWithEncodingMarshalerString},
	{&myGenDeclaredValue, myGenDeclaredString},
	{&myGenDeclaredWithCommentValue, myGenDeclaredWithCommentString},
	{&myTypeDeclaredValue, myTypeDeclaredString},
	{&myTypeNotSkippedValue, myTypeNotSkippedString},
	{&intern, internString},
}

func TestMarshal(t *testing.T) {
	for i, test := range testCases {
		data, err := test.Decoded.MarshalJSON()
		if err != nil {
			t.Errorf("[%d, %T] MarshalJSON() error: %v", i, test.Decoded, err)
		}

		got := string(data)
		if got != test.Encoded {
			t.Errorf("[%d, %T] MarshalJSON(): got \n%v\n\t\t want \n%v", i, test.Decoded, got, test.Encoded)
		}
	}
}

func TestUnmarshal(t *testing.T) {
	for i, test := range testCases {
		v1 := reflect.New(reflect.TypeOf(test.Decoded).Elem()).Interface()
		v := v1.(testType)

		err := v.UnmarshalJSON([]byte(test.Encoded))
		if err != nil {
			t.Errorf("[%d, %T] UnmarshalJSON() error: %v", i, test.Decoded, err)
		}

		if !reflect.DeepEqual(v, test.Decoded) {
			t.Errorf("[%d, %T] UnmarshalJSON(): got \n%+v\n\t\t want \n%+v", i, test.Decoded, v, test.Decoded)
		}
	}
}

func TestRawMessageSTD(t *testing.T) {
	type T struct {
		F    easyjson.RawMessage
		Fnil easyjson.RawMessage
	}

	val := T{F: easyjson.RawMessage([]byte(`"test"`))}
	str := `{"F":"test","Fnil":null}`

	data, err := json.Marshal(val)
	if err != nil {
		t.Errorf("json.Marshal() error: %v", err)
	}
	got := string(data)
	if got != str {
		t.Errorf("json.Marshal() = %v; want %v", got, str)
	}

	wantV := T{F: easyjson.RawMessage([]byte(`"test"`)), Fnil: easyjson.RawMessage([]byte("null"))}
	var gotV T

	err = json.Unmarshal([]byte(str), &gotV)
	if err != nil {
		t.Errorf("json.Unmarshal() error: %v", err)
	}
	if !reflect.DeepEqual(gotV, wantV) {
		t.Errorf("json.Unmarshal() = %v; want %v", gotV, wantV)
	}
}

func TestParseNull(t *testing.T) {
	var got, want SubStruct
	if err := easyjson.Unmarshal([]byte("null"), &got); err != nil {
		t.Errorf("Unmarshal() error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Unmarshal() = %+v; want %+v", got, want)
	}
}

var testSpecialCases = []struct {
	EncodedString string
	Value         string
}{
	{`"Username \u003cuser@example.com\u003e"`, `Username <user@example.com>`},
	{`"Username\ufffd"`, "Username\xc5"},
	{`"тестzтест"`, "тестzтест"},
	{`"тест\ufffdтест"`, "тест\xc5тест"},
	{`"绿茶"`, "绿茶"},
	{`"绿\ufffd茶"`, "绿\xc5茶"},
	{`"тест\u2028"`, "тест\xE2\x80\xA8"},
	{`"\\\r\n\t\""`, "\\\r\n\t\""},
	{`"text\\\""`, "text\\\""},
	{`"ü"`, "ü"},
}

func TestSpecialCases(t *testing.T) {
	for i, test := range testSpecialCases {
		w := jwriter.Writer{}
		w.String(test.Value)
		got := string(w.Buffer.BuildBytes())
		if got != test.EncodedString {
			t.Errorf("[%d] Encoded() = %+v; want %+v", i, got, test.EncodedString)
		}
	}
}

func TestOverflowArray(t *testing.T) {
	var a Arrays
	err := easyjson.Unmarshal([]byte(arrayOverflowString), &a)
	if err != nil {
		t.Error(err)
	}
	if a != arrayValue {
		t.Errorf("Unmarshal(%v) = %+v; want %+v", arrayOverflowString, a, arrayValue)
	}
}

func TestUnderflowArray(t *testing.T) {
	var a Arrays
	err := easyjson.Unmarshal([]byte(arrayUnderflowString), &a)
	if err != nil {
		t.Error(err)
	}
	if a != arrayUnderflowValue {
		t.Errorf("Unmarshal(%v) = %+v; want %+v", arrayUnderflowString, a, arrayUnderflowValue)
	}
}

func TestEncodingFlags(t *testing.T) {
	for i, test := range []struct {
		Flags jwriter.Flags
		In    easyjson.Marshaler
		Want  string
	}{
		{0, EncodingFlagsTestMap{}, `{"F":null}`},
		{0, EncodingFlagsTestSlice{}, `{"F":null}`},
		{jwriter.NilMapAsEmpty, EncodingFlagsTestMap{}, `{"F":{}}`},
		{jwriter.NilSliceAsEmpty, EncodingFlagsTestSlice{}, `{"F":[]}`},
	} {
		w := &jwriter.Writer{Flags: test.Flags}
		test.In.MarshalEasyJSON(w)

		data, err := w.BuildBytes()
		if err != nil {
			t.Errorf("[%v] easyjson.Marshal(%+v) error: %v", i, test.In, err)
		}

		v := string(data)
		if v != test.Want {
			t.Errorf("[%v] easyjson.Marshal(%+v) = %v; want %v", i, test.In, v, test.Want)
		}
	}

}

func TestNestedEasyJsonMarshal(t *testing.T) {
	n := map[string]*NestedEasyMarshaler{
		"Value":  {},
		"Slice1": {},
		"Slice2": {},
		"Map1":   {},
		"Map2":   {},
	}

	ni := NestedInterfaces{
		Value: n["Value"],
		Slice: []interface{}{n["Slice1"], n["Slice2"]},
		Map:   map[string]interface{}{"1": n["Map1"], "2": n["Map2"]},
	}
	easyjson.Marshal(ni)

	for k, v := range n {
		if !v.EasilyMarshaled {
			t.Errorf("Nested interface %s wasn't easily marshaled", k)
		}
	}
}

func TestNestedMarshaler(t *testing.T) {
	s := NestedMarshaler{
		Value: &StructWithMarshaler{
			Value: 5,
		},
		Value2: 10,
	}

	data, err := s.MarshalJSON()
	if err != nil {
		t.Errorf("Can't marshal NestedMarshaler: %s", err)
	}

	s2 := NestedMarshaler {
		Value: &StructWithMarshaler{},
	}

	err = s2.UnmarshalJSON(data)
	if err != nil {
		t.Errorf("Can't unmarshal NestedMarshaler: %s", err)
	}

	if !reflect.DeepEqual(s2, s) {
		t.Errorf("easyjson.Unmarshal() = %#v; want %#v", s2, s)
	}
}

func TestUnmarshalStructWithEmbeddedPtrStruct(t *testing.T) {
	var s = StructWithInterface{Field2: &EmbeddedStruct{}}
	var err error
	err = easyjson.Unmarshal([]byte(structWithInterfaceString), &s)
	if err != nil {
		t.Errorf("easyjson.Unmarshal() error: %v", err)
	}
	if !reflect.DeepEqual(s, structWithInterfaceValueFilled) {
		t.Errorf("easyjson.Unmarshal() = %#v; want %#v", s, structWithInterfaceValueFilled)
	}
}

func TestDisallowUnknown(t *testing.T) {
	var d DisallowUnknown
	err := easyjson.Unmarshal([]byte(disallowUnknownString), &d)
	if err == nil {
		t.Error("want error, got nil")
	}
}

var testNotGeneratedTypeCases = []interface{}{
	TypeNotDeclared{},
	TypeSkipped{},
}

func TestMethodsNoGenerated(t *testing.T) {
	var ok bool
	for i, instance := range testNotGeneratedTypeCases {
		_, ok = instance.(json.Marshaler)
		if ok {
			t.Errorf("[%d, %T] Unexpected MarshalJSON()", i, instance)
		}

		_, ok = instance.(json.Unmarshaler)
		if ok {
			t.Errorf("[%d, %T] Unexpected Unmarshaler()", i, instance)
		}
	}
}

func TestNil(t *testing.T) {
	var p *PrimitiveTypes

	data, err := easyjson.Marshal(p)
	if err != nil {
		t.Errorf("easyjson.Marshal() error: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("Wanted null, got %q", string(data))
	}

	var b bytes.Buffer
	if n, err := easyjson.MarshalToWriter(p, &b); err != nil || n != 4 {
		t.Errorf("easyjson.MarshalToWriter() error: %v, written %d", err, n)
	}

	if s := b.String(); s != "null" {
		t.Errorf("Wanted null, got %q", s)
	}

	w := httptest.NewRecorder()
	started, written, err := easyjson.MarshalToHTTPResponseWriter(p, w)
	if !started || written != 4 || err != nil {
		t.Errorf("easyjson.MarshalToHTTPResponseWriter() error: %v, written %d, started %t",
			err, written, started)
	}

	if s := w.Body.String(); s != "null" {
		t.Errorf("Wanted null, got %q", s)
	}
}

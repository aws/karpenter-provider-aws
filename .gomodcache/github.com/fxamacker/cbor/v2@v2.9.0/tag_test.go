// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTagNewTypeWithBuiltinUnderlyingType(t *testing.T) {
	type myBool bool
	type myUint uint
	type myUint8 uint8
	type myUint16 uint16
	type myUint32 uint32
	type myUint64 uint64
	type myInt int
	type myInt8 int8
	type myInt16 int16
	type myInt32 int32
	type myInt64 int64
	type myFloat32 float32
	type myFloat64 float64
	type myString string
	type myByteSlice []byte
	type myIntSlice []int
	type myIntArray [4]int
	type myMapIntInt map[int]int

	types := []reflect.Type{
		reflect.TypeOf(myBool(false)),
		reflect.TypeOf(myUint(0)),
		reflect.TypeOf(myUint8(0)),
		reflect.TypeOf(myUint16(0)),
		reflect.TypeOf(myUint32(0)),
		reflect.TypeOf(myUint64(0)),
		reflect.TypeOf(myInt(0)),
		reflect.TypeOf(myInt8(0)),
		reflect.TypeOf(myInt16(0)),
		reflect.TypeOf(myInt32(0)),
		reflect.TypeOf(myInt64(0)),
		reflect.TypeOf(myFloat32(0)),
		reflect.TypeOf(myFloat64(0)),
		reflect.TypeOf(myString("")),
		reflect.TypeOf(myByteSlice([]byte{})),
		reflect.TypeOf(myIntSlice([]int{})),
		reflect.TypeOf(myIntArray([4]int{})),
		reflect.TypeOf(myMapIntInt(map[int]int{})),
	}

	tags := NewTagSet()
	for i, typ := range types {
		tagNum := uint64(100 + i)
		if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, typ, tagNum); err != nil {
			t.Fatalf("TagSet.Add(%s, %d) returned error %v", typ, tagNum, err)
		}
	}

	em, _ := EncOptions{Sort: SortCanonical}.EncModeWithTags(tags)
	dm, _ := DecOptions{}.DecModeWithTags(tags)

	testCases := []roundTripTest{
		{
			name:         "bool",
			obj:          myBool(true),
			wantCborData: mustHexDecode("d864f5"),
		},
		{
			name:         "uint",
			obj:          myUint(0),
			wantCborData: mustHexDecode("d86500"),
		},
		{
			name:         "uint8",
			obj:          myUint8(0),
			wantCborData: mustHexDecode("d86600"),
		},
		{
			name:         "uint16",
			obj:          myUint16(1000),
			wantCborData: mustHexDecode("d8671903e8"),
		},
		{
			name:         "uint32",
			obj:          myUint32(1000000),
			wantCborData: mustHexDecode("d8681a000f4240"),
		},
		{
			name:         "uint64",
			obj:          myUint64(1000000000000),
			wantCborData: mustHexDecode("d8691b000000e8d4a51000"),
		},
		{
			name:         "int",
			obj:          myInt(-1),
			wantCborData: mustHexDecode("d86a20"),
		},
		{
			name:         "int8",
			obj:          myInt8(-1),
			wantCborData: mustHexDecode("d86b20"),
		},
		{
			name:         "int16",
			obj:          myInt16(-1000),
			wantCborData: mustHexDecode("d86c3903e7"),
		},
		{
			name:         "int32",
			obj:          myInt32(-1000),
			wantCborData: mustHexDecode("d86d3903e7"),
		},
		{
			name:         "int64",
			obj:          myInt64(-1000),
			wantCborData: mustHexDecode("d86e3903e7"),
		},
		{
			name:         "float32",
			obj:          myFloat32(100000.0),
			wantCborData: mustHexDecode("d86ffa47c35000"),
		},
		{
			name:         "float64",
			obj:          myFloat64(1.1),
			wantCborData: mustHexDecode("d870fb3ff199999999999a"),
		},
		{
			name:         "string",
			obj:          myString("a"),
			wantCborData: mustHexDecode("d8716161"),
		},
		{
			name:         "[]byte",
			obj:          myByteSlice([]byte{1, 2, 3, 4}),
			wantCborData: mustHexDecode("d8724401020304"),
		},
		{
			name:         "[]int",
			obj:          myIntSlice([]int{1, 2, 3, 4}),
			wantCborData: mustHexDecode("d8738401020304"),
		},
		{
			name:         "[4]int",
			obj:          myIntArray([...]int{1, 2, 3, 4}),
			wantCborData: mustHexDecode("d8748401020304"),
		},
		{
			name:         "map[int]int",
			obj:          myMapIntInt(map[int]int{1: 2, 3: 4}),
			wantCborData: mustHexDecode("d875a201020304"),
		},
	}

	testRoundTrip(t, testCases, em, dm)
}

func TestTagBinaryMarshalerUnmarshaler(t *testing.T) {
	t1 := reflect.TypeOf((*number)(nil)) // Use *number for testing purpose
	t2 := reflect.TypeOf(stru{})

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, t1, 123); err != nil {
		t.Fatalf("TagSet.Add(%s, %d) returned error %v", t1, 123, err)
	}
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, t2, 124); err != nil {
		t.Fatalf("TagSet.Add(%s, %d) returned error %v", t2, 124, err)
	}

	em, _ := EncOptions{}.EncModeWithTags(tags)
	dm, _ := DecOptions{}.DecModeWithTags(tags)

	testCases := []roundTripTest{
		{
			name:         "primitive obj",
			obj:          number(1234567890),
			wantCborData: mustHexDecode("d87b4800000000499602d2"),
		},
		{
			name:         "struct obj",
			obj:          stru{a: "a", b: "b", c: "c"},
			wantCborData: mustHexDecode("d87c45612C622C63"),
		},
	}

	testRoundTrip(t, testCases, em, dm)
}

func TestTagStruct(t *testing.T) {
	type T struct {
		S string `cbor:"s,omitempty"`
	}

	t1 := reflect.TypeOf(T{})

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, t1, 100); err != nil {
		t.Fatalf("TagSet.Add(%s, %d) returned error %v", t1, 100, err)
	}

	em, _ := EncOptions{}.EncModeWithTags(tags)
	dm, _ := DecOptions{}.DecModeWithTags(tags)

	data := mustHexDecode("d864a0") // {}
	var v T
	if err := dm.Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal() returned error %v", err)
	}
	b, err := em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%+v) returned error %v", v, err)
	}
	if !bytes.Equal(b, data) {
		t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", v, b, data)
	}
}

func TestTagFixedLengthStruct(t *testing.T) {
	type T struct {
		S string `cbor:"s"`
	}

	t1 := reflect.TypeOf(T{})

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, t1, 100); err != nil {
		t.Fatalf("TagSet.Add(%s, %d) returned error %v", t1, 100, err)
	}

	em, _ := EncOptions{}.EncModeWithTags(tags)
	dm, _ := DecOptions{}.DecModeWithTags(tags)

	data := mustHexDecode("d864a1617360") // {"s":""}
	var v T
	if err := dm.Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal() returned error %v", err)
	}
	b, err := em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%+v) returned error %v", v, err)
	}
	if !bytes.Equal(b, data) {
		t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", v, b, data)
	}
}

func TestTagToArrayStruct(t *testing.T) {
	type coseHeader struct {
		Alg int    `cbor:"1,keyasint,omitempty"`
		Kid []byte `cbor:"4,keyasint,omitempty"`
		IV  []byte `cbor:"5,keyasint,omitempty"`
	}
	type signedCWT struct {
		_           struct{} `cbor:",toarray"`
		Protected   []byte
		Unprotected coseHeader
		Payload     []byte
		Signature   []byte
	}

	t1 := reflect.TypeOf(signedCWT{})

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, t1, 18); err != nil {
		t.Fatalf("TagSet.Add(%s, %d) returned error %v", t1, 18, err)
	}

	em, _ := EncOptions{}.EncModeWithTags(tags)
	dm, _ := DecOptions{}.DecModeWithTags(tags)

	// Data from https://tools.ietf.org/html/rfc8392#appendix-A section A.3
	data := mustHexDecode("d28443a10126a104524173796d6d657472696345434453413235365850a70175636f61703a2f2f61732e6578616d706c652e636f6d02656572696b77037818636f61703a2f2f6c696768742e6578616d706c652e636f6d041a5612aeb0051a5610d9f0061a5610d9f007420b7158405427c1ff28d23fbad1f29c4c7c6a555e601d6fa29f9179bc3d7438bacaca5acd08c8d4d4f96131680c429a01f85951ecee743a52b9b63632c57209120e1c9e30")
	var v signedCWT
	if err := dm.Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal() returned error %v", err)
	}
	b, err := em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%+v) returned error %v", v, err)
	}
	if !bytes.Equal(b, data) {
		t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", v, b, data)
	}
}

func TestNestedTagStruct(t *testing.T) {
	type coseHeader struct {
		Alg int    `cbor:"1,keyasint,omitempty"`
		Kid []byte `cbor:"4,keyasint,omitempty"`
		IV  []byte `cbor:"5,keyasint,omitempty"`
	}
	type macedCOSE struct {
		_           struct{} `cbor:",toarray"`
		Protected   []byte
		Unprotected coseHeader
		Payload     []byte
		Tag         []byte
	}

	t1 := reflect.TypeOf(macedCOSE{})

	// Register tag CBOR Web Token (CWT) 61 and COSE_Mac0 17 with macedCOSE type
	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, t1, 61, 17); err != nil {
		t.Fatalf("TagSet.Add(%s, %d, %v) returned error %v", t1, 61, 17, err)
	}

	em, _ := EncOptions{}.EncModeWithTags(tags)
	dm, _ := DecOptions{}.DecModeWithTags(tags)

	// Data from https://tools.ietf.org/html/rfc8392#appendix-A section A.4
	data := mustHexDecode("d83dd18443a10104a1044c53796d6d65747269633235365850a70175636f61703a2f2f61732e6578616d706c652e636f6d02656572696b77037818636f61703a2f2f6c696768742e6578616d706c652e636f6d041a5612aeb0051a5610d9f0061a5610d9f007420b7148093101ef6d789200")
	var v macedCOSE
	if err := dm.Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal() returned error %v", err)
	}
	b, err := em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%+v) returned error %v", v, err)
	}
	if !bytes.Equal(b, data) {
		t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", v, b, data)
	}
}

func TestAddTagError(t *testing.T) {
	type myInt int
	testCases := []struct {
		name         string
		typ          reflect.Type
		num          uint64
		opts         TagOptions
		wantErrorMsg string
	}{
		{
			name:         "nil type",
			typ:          nil,
			num:          100,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add nil content type to TagSet",
		},
		{
			name:         "DecTag is DecTagIgnored && EncTag is EncTagNone",
			typ:          reflect.TypeOf(myInt(0)),
			num:          100,
			opts:         TagOptions{DecTag: DecTagIgnored, EncTag: EncTagNone},
			wantErrorMsg: "cbor: cannot add tag with DecTagIgnored and EncTagNone options to TagSet",
		},
		{
			name:         "time.Time",
			typ:          reflect.TypeOf(time.Time{}),
			num:          101,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add time.Time to TagSet, use EncOptions.TimeTag and DecOptions.TimeTag instead",
		},
		{
			name:         "builtin type string",
			typ:          reflect.TypeOf(""),
			num:          102,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: can only add named types to TagSet, got string",
		},
		{
			name:         "unnamed type struct{}",
			typ:          reflect.TypeOf(struct{}{}),
			num:          103,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: can only add named types to TagSet, got struct {}",
		},
		{
			name:         "interface",
			typ:          reflect.TypeOf((*io.Reader)(nil)).Elem(),
			num:          104,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: can only add named types to TagSet, got io.Reader",
		},
		{
			name:         "cbor.Tag",
			typ:          reflect.TypeOf(Tag{}),
			num:          105,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add cbor.Tag to TagSet",
		},
		{
			name:         "cbor.RawTag",
			typ:          reflect.TypeOf(RawTag{}),
			num:          106,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add cbor.RawTag to TagSet",
		},
		{
			name:         "big.Int",
			typ:          reflect.TypeOf(big.Int{}),
			num:          107,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add big.Int to TagSet, it's built-in and supported automatically",
		},
		/*
			{
				name:         "cbor.Unmarshaler",
				typ:          reflect.TypeOf(number2(0)),
				num:          107,
				opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagNone},
				wantErrorMsg: "cbor: cannot add cbor.Unmarshaler to TagSet with DecTag != DecTagIgnored",
			},
			{
				name:         "cbor.Marshaler",
				typ:          reflect.TypeOf(number2(0)),
				num:          108,
				opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
				wantErrorMsg: "cbor: cannot add cbor.Marshaler to TagSet with EncTag != EncTagNone",
			},
		*/
		{
			name:         "tag number 0",
			typ:          reflect.TypeOf(myInt(0)),
			num:          0,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add tag number 0 or 1 to TagSet, use EncOptions.TimeTag and DecOptions.TimeTag instead",
		},
		{
			name:         "tag number 1",
			typ:          reflect.TypeOf(myInt(0)),
			num:          1,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add tag number 0 or 1 to TagSet, use EncOptions.TimeTag and DecOptions.TimeTag instead",
		},
		{
			name:         "tag number 2",
			typ:          reflect.TypeOf(myInt(0)),
			num:          2,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add tag number 2 or 3 to TagSet, it's built-in and supported automatically",
		},
		{
			name:         "tag number 3",
			typ:          reflect.TypeOf(myInt(0)),
			num:          3,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add tag number 2 or 3 to TagSet, it's built-in and supported automatically",
		},
		{
			name:         "tag number 55799",
			typ:          reflect.TypeOf(myInt(0)),
			num:          55799,
			opts:         TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired},
			wantErrorMsg: "cbor: cannot add tag number 55799 to TagSet, it's built-in and ignored automatically",
		},
	}
	tags := NewTagSet()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tags.Add(tc.opts, tc.typ, tc.num); err == nil {
				t.Errorf("TagSet.Add(%s, %d) didn't return an error", tc.typ.String(), tc.num)
			} else if err.Error() != tc.wantErrorMsg {
				var typeString string
				if tc.typ == nil {
					typeString = "nil"
				} else {
					typeString = tc.typ.String()
				}
				t.Errorf("TagSet.Add(%s, %d) returned error msg %q, want %q", typeString, tc.num, err, tc.wantErrorMsg)
			}
		})
	}
}

func TestAddDuplicateTagContentTypeError(t *testing.T) {
	type myInt int
	myIntType := reflect.TypeOf(myInt(0))
	wantErrorMsg := "cbor: content type cbor.myInt already exists in TagSet"

	tags := NewTagSet()
	// Add myIntType and 100 to tags
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, myIntType, 100); err != nil {
		t.Errorf("TagSet.Add(%s, %d) returned error %v", myIntType.String(), 100, err)
	}
	// Add myIntType and 101 to tags
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, myIntType, 101); err == nil {
		t.Errorf("TagSet.Add(%s, %d) didn't return an error", myIntType.String(), 101)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("TagSet.Add(%s, %d) returned error msg %q, want %q", myIntType, 101, err, wantErrorMsg)
	}
}

func TestAddDuplicateTagNumError(t *testing.T) {
	type myBool bool
	type myInt int
	myBoolType := reflect.TypeOf(myBool(false))
	myIntType := reflect.TypeOf(myInt(0))
	wantErrorMsg := "cbor: tag number [100] already exists in TagSet"

	tags := NewTagSet()

	// Add myIntType and 100 to tags
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, myIntType, 100); err != nil {
		t.Errorf("TagSet.Add(%s, %d) returned error %v", myIntType.String(), 100, err)
	}
	// Add myBoolType and 100 to tags
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, myBoolType, 100); err == nil {
		t.Errorf("TagSet.Add(%s, %d) didn't return an error", myBoolType.String(), 100)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("TagSet.Add(%s, %d) returned error msg %q, want %q", myBoolType, 100, err, wantErrorMsg)
	}
}

func TestAddDuplicateTagNumsError(t *testing.T) {
	type myBool bool
	type myInt int
	myBoolType := reflect.TypeOf(myBool(false))
	myIntType := reflect.TypeOf(myInt(0))
	wantErrorMsg := "cbor: tag number [100 101] already exists in TagSet"

	tags := NewTagSet()

	// Add myIntType and [100, 101] to tags
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, myIntType, 100, 101); err != nil {
		t.Errorf("TagSet.Add(%s, %d, %d) returned error %v", myIntType.String(), 100, 101, err)
	}
	// Add myBoolType and [100, 101] to tags
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, myBoolType, 100, 101); err == nil {
		t.Errorf("TagSet.Add(%s, %d, %d) didn't return an error", myBoolType.String(), 100, 101)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("TagSet.Add(%s, %d, %d) returned error msg %q, want %q", myBoolType, 100, 101, err, wantErrorMsg)
	}
}

func TestAddRemoveTag(t *testing.T) {
	type myInt int
	type myFloat float64
	myIntType := reflect.TypeOf(myInt(0))
	myFloatType := reflect.TypeOf(myFloat(0.0))
	pMyIntType := reflect.TypeOf((*myInt)(nil))
	pMyFloatType := reflect.TypeOf((*myFloat)(nil))

	tags := NewTagSet()
	stags := tags.(*syncTagSet)
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, myIntType, 100); err != nil {
		t.Errorf("TagSet.Add(%s, %d) returned error %v", myIntType.String(), 100, err)
	}
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, myFloatType, 101); err != nil {
		t.Errorf("TagSet.Add(%s, %d) returned error %v", myFloatType.String(), 101, err)
	}
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, pMyIntType, 102); err == nil {
		t.Errorf("TagSet.Add(%s, %d) didn't return an error", pMyIntType.String(), 102)
	}
	if err := tags.Add(TagOptions{DecTag: DecTagRequired, EncTag: EncTagRequired}, pMyFloatType, 103); err == nil {
		t.Errorf("TagSet.Add(%s, %d) didn't return an error", pMyFloatType.String(), 103)
	}
	if len(stags.t) != 2 {
		t.Errorf("TagSet len is %d, want %d", len(stags.t), 2)
	}
	tags.Remove(pMyIntType)
	if len(stags.t) != 1 {
		t.Errorf("TagSet len is %d, want %d", len(stags.t), 1)
	}
	tags.Remove(pMyFloatType)
	if len(stags.t) != 0 {
		t.Errorf("TagSet len is %d, want %d", len(stags.t), 0)
	}
	tags.Remove(myIntType)
	tags.Remove(myFloatType)
}

func TestAddTagTypeAliasError(t *testing.T) {
	type myBool = bool
	type myUint = uint
	type myUint8 = uint8
	type myUint16 = uint16
	type myUint32 = uint32
	type myUint64 = uint64
	type myInt = int
	type myInt8 = int8
	type myInt16 = int16
	type myInt32 = int32
	type myInt64 = int64
	type myFloat32 = float32
	type myFloat64 = float64
	type myString = string
	type myByteSlice = []byte
	type myIntSlice = []int
	type myIntArray = [4]int
	type myMapIntInt = map[int]int

	testCases := []struct {
		name         string
		typ          reflect.Type
		wantErrorMsg string
	}{
		{
			name:         "bool",
			typ:          reflect.TypeOf(myBool(false)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got bool",
		},
		{
			name:         "uint",
			typ:          reflect.TypeOf(myUint(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got uint",
		},
		{
			name:         "uint8",
			typ:          reflect.TypeOf(myUint8(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got uint8",
		},
		{
			name:         "uint16",
			typ:          reflect.TypeOf(myUint16(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got uint16",
		},
		{
			name:         "uint32",
			typ:          reflect.TypeOf(myUint32(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got uint32",
		},
		{
			name:         "uint64",
			typ:          reflect.TypeOf(myUint64(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got uint64",
		},
		{
			name:         "int",
			typ:          reflect.TypeOf(myInt(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got int",
		},
		{
			name:         "int8",
			typ:          reflect.TypeOf(myInt8(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got int8",
		},
		{
			name:         "int16",
			typ:          reflect.TypeOf(myInt16(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got int16",
		},
		{
			name:         "int32",
			typ:          reflect.TypeOf(myInt32(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got int32",
		},
		{
			name:         "int64",
			typ:          reflect.TypeOf(myInt64(0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got int64",
		},
		{
			name:         "float32",
			typ:          reflect.TypeOf(myFloat32(0.0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got float32",
		},
		{
			name:         "float64",
			typ:          reflect.TypeOf(myFloat64(0.0)),
			wantErrorMsg: "cbor: can only add named types to TagSet, got float64",
		},
		{
			name:         "string",
			typ:          reflect.TypeOf(myString("")),
			wantErrorMsg: "cbor: can only add named types to TagSet, got string",
		},
		{
			name:         "[]byte",
			typ:          reflect.TypeOf(myByteSlice([]byte{})), //nolint:unconvert
			wantErrorMsg: "cbor: can only add named types to TagSet, got []uint8",
		},
		{
			name:         "[]int",
			typ:          reflect.TypeOf(myIntSlice([]int{})), //nolint:unconvert
			wantErrorMsg: "cbor: can only add named types to TagSet, got []int",
		},
		{
			name:         "[4]int",
			typ:          reflect.TypeOf(myIntArray([4]int{})), //nolint:unconvert
			wantErrorMsg: "cbor: can only add named types to TagSet, got [4]int",
		},
		{
			name:         "map[int]int",
			typ:          reflect.TypeOf(myMapIntInt(map[int]int{})), //nolint:unconvert
			wantErrorMsg: "cbor: can only add named types to TagSet, got map[int]int",
		},
	}

	tags := NewTagSet()
	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, tc.typ, uint64(100+i)); err == nil {
				t.Errorf("TagSet.Add(%s, %d) didn't return an error", tc.typ.String(), 0)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("TagSet.Add(%s, %d) returned error msg %q, want %q", tc.typ.String(), 0, err, tc.wantErrorMsg)
			}
		})
	}
}

// TestDecodeTag decodes tag data with DecTagRequired/EncTagOptional/EncTagNone options.
func TestDecodeTagData(t *testing.T) {
	type myInt int
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
	}

	type tagInfo struct {
		t reflect.Type
		n []uint64
	}
	tagInfos := []tagInfo{
		{reflect.TypeOf((*number)(nil)), []uint64{123}}, // BinaryMarshaler *number
		{reflect.TypeOf(stru{}), []uint64{124}},         // BinaryMarshaler stru
		{reflect.TypeOf(myInt(0)), []uint64{125}},       // non-struct type
		{reflect.TypeOf(s{}), []uint64{126}},            // struct type
	}

	tagsDecRequired := NewTagSet()
	tagsDecOptional := NewTagSet()
	tagsDecIgnored := NewTagSet()
	for _, tag := range tagInfos {
		if err := tagsDecRequired.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, tag.t, tag.n[0], tag.n[1:]...); err != nil {
			t.Fatalf("TagSet.Add(%s, %v) returned error %v", tag.t, tag.n, err)
		}
		if err := tagsDecOptional.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagOptional}, tag.t, tag.n[0], tag.n[1:]...); err != nil {
			t.Fatalf("TagSet.Add(%s, %v) returned error %v", tag.t, tag.n, err)
		}
		if err := tagsDecIgnored.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagIgnored}, tag.t, tag.n[0], tag.n[1:]...); err != nil {
			t.Fatalf("TagSet.Add(%s, %v) returned error %v", tag.t, tag.n, err)
		}
	}

	type tag struct {
		name   string
		tagSet TagSet
	}
	tags := []tag{
		{"EncTagRequired_DecTagRequired", tagsDecRequired},
		{"EncTagRequired_DecTagOptional", tagsDecOptional},
		{"EncTagRequired_DecTagIgnored", tagsDecIgnored},
	}

	testCases := []roundTripTest{
		{
			name:         "BinaryMarshaler non-struct",
			obj:          number(1234567890),
			wantCborData: mustHexDecode("d87b4800000000499602d2"),
		},
		{
			name:         "BinaryMarshaler struct",
			obj:          stru{a: "a", b: "b", c: "c"},
			wantCborData: mustHexDecode("d87c45612C622C63"),
		},
		{
			name:         "non-struct",
			obj:          myInt(1),
			wantCborData: mustHexDecode("d87d01"),
		},
		{
			name:         "struct",
			obj:          s{A: "A", B: "B", C: "C"},
			wantCborData: mustHexDecode("d87ea3616161416162614261636143"), // {"a":"A", "b":"B", "c":"C"}
		},
	}
	for _, tag := range tags {
		t.Run(tag.name, func(t *testing.T) {
			em, _ := EncOptions{}.EncModeWithTags(tag.tagSet)
			dm, _ := DecOptions{}.DecModeWithTags(tag.tagSet)
			testRoundTrip(t, testCases, em, dm)
		})
	}
}

// TestDecodeNoTag decodes no-tag data with DecTagRequired/EncTagOptional/EncTagNone options
func TestDecodeNoTagData(t *testing.T) {
	type myInt int
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
	}

	type tagInfo struct {
		t reflect.Type
		n []uint64
	}
	tagInfos := []tagInfo{
		{reflect.TypeOf((*number)(nil)), []uint64{123}}, // BinaryMarshaler *number
		{reflect.TypeOf(stru{}), []uint64{124}},         // BinaryMarshaler stru
		{reflect.TypeOf(myInt(0)), []uint64{125}},       // non-struct type
		{reflect.TypeOf(s{}), []uint64{126}},            // struct type
	}

	tagsDecRequired := NewTagSet()
	tagsDecOptional := NewTagSet()
	for _, tag := range tagInfos {
		if err := tagsDecRequired.Add(TagOptions{EncTag: EncTagNone, DecTag: DecTagRequired}, tag.t, tag.n[0], tag.n[1:]...); err != nil {
			t.Fatalf("TagSet.Add(%s, %v) returned error %v", tag.t, tag.n, err)
		}
		if err := tagsDecOptional.Add(TagOptions{EncTag: EncTagNone, DecTag: DecTagOptional}, tag.t, tag.n[0], tag.n[1:]...); err != nil {
			t.Fatalf("TagSet.Add(%s, %v) returned error %v", tag.t, tag.n, err)
		}
	}

	type tag struct {
		name   string
		tagSet TagSet
	}
	tags := []tag{
		{"EncTagIgnored_DecTagOptional", tagsDecOptional},
	}

	testCases := []roundTripTest{
		{
			name:         "BinaryMarshaler non-struct",
			obj:          number(1234567890),
			wantCborData: mustHexDecode("4800000000499602d2"),
		},
		{
			name:         "BinaryMarshaler struct",
			obj:          stru{a: "a", b: "b", c: "c"},
			wantCborData: mustHexDecode("45612C622C63"),
		},
		{
			name:         "non-struct",
			obj:          myInt(1),
			wantCborData: mustHexDecode("01"),
		},
		{
			name:         "struct",
			obj:          s{A: "A", B: "B", C: "C"},
			wantCborData: mustHexDecode("a3616161416162614261636143"), // {"a":"A", "b":"B", "c":"C"}
		},
	}

	for _, tag := range tags {
		t.Run(tag.name, func(t *testing.T) {
			em, _ := EncOptions{}.EncModeWithTags(tag.tagSet)
			dm, _ := DecOptions{}.DecModeWithTags(tag.tagSet)
			testRoundTrip(t, testCases, em, dm)
		})
	}

	// Decode non-tag data with DecTagRequired option returns UnmarshalTypeError
	for _, tc := range testCases {
		name := "EncTagIgnored_DecTagRequired " + tc.name
		t.Run(name, func(t *testing.T) {
			dm, _ := DecOptions{}.DecModeWithTags(tagsDecRequired)
			v := reflect.New(reflect.TypeOf(tc.obj))
			if err := dm.Unmarshal(tc.wantCborData, v.Interface()); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error", tc.wantCborData)
			} else {
				if _, ok := err.(*UnmarshalTypeError); !ok {
					t.Errorf("Unmarshal(0x%x) returned wrong type of error %T, want (*UnmarshalTypeError)", tc.wantCborData, err)
				} else if !strings.Contains(err.Error(), "expect CBOR tag value") {
					t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", tc.wantCborData, err.Error(), "expect CBOR tag value")
				}
			}
		})
	}
}

// TestDecodeWrongTag decodes wrong tag data with DecTagRequired/EncTagOptional/EncTagNone options
func TestDecodeWrongTag(t *testing.T) {
	type myInt int
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
	}

	type tagInfo struct {
		t reflect.Type
		n []uint64
	}
	tagInfos := []tagInfo{
		{reflect.TypeOf((*number)(nil)), []uint64{123}}, // BinaryMarshaler *number
		{reflect.TypeOf(stru{}), []uint64{124}},         // BinaryMarshaler stru
		{reflect.TypeOf(myInt(0)), []uint64{100}},       // non-struct type
		{reflect.TypeOf(s{}), []uint64{101, 102}},       // struct type
	}

	tagsDecRequired := NewTagSet()
	tagsDecOptional := NewTagSet()
	tagsDecIgnored := NewTagSet()
	for _, tag := range tagInfos {
		if err := tagsDecRequired.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, tag.t, tag.n[0], tag.n[1:]...); err != nil {
			t.Fatalf("TagSet.Add(%s, %v) returned error %v", tag.t, tag.n, err)
		}
		if err := tagsDecOptional.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagOptional}, tag.t, tag.n[0], tag.n[1:]...); err != nil {
			t.Fatalf("TagSet.Add(%s, %v) returned error %v", tag.t, tag.n, err)
		}
		if err := tagsDecIgnored.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagIgnored}, tag.t, tag.n[0], tag.n[1:]...); err != nil {
			t.Fatalf("TagSet.Add(%s, %v) returned error %v", tag.t, tag.n, err)
		}
	}
	type tag struct {
		name   string
		tagSet TagSet
	}
	tags := []tag{
		{"EncTagRequired_DecTagRequired", tagsDecRequired},
		{"EncTagRequired_DecTagOptional", tagsDecOptional},
	}

	testCases := []struct {
		name         string
		obj          any
		data         []byte
		wantErrorMsg string
	}{
		{
			name:         "BinaryMarshaler non-struct",
			obj:          number(1234567890),
			data:         mustHexDecode("d87d4800000000499602d2"),
			wantErrorMsg: "cbor: wrong tag number for cbor.number, got [125], expected [123]",
		},
		{
			name:         "BinaryMarshaler struct",
			obj:          stru{a: "a", b: "b", c: "c"},
			data:         mustHexDecode("d87d45612C622C63"),
			wantErrorMsg: "cbor: wrong tag number for cbor.stru, got [125], expected [124]",
		},
		{
			name:         "non-struct",
			obj:          myInt(1),
			data:         mustHexDecode("d87d01"),
			wantErrorMsg: "cbor: wrong tag number for cbor.myInt, got [125], expected [100]",
		},
		{
			name:         "struct",
			obj:          s{A: "A", B: "B", C: "C"},
			data:         mustHexDecode("d87ea3616161416162614261636143"), // {"a":"A", "b":"B", "c":"C"}
			wantErrorMsg: "cbor: wrong tag number for cbor.s, got [126], expected [101 102]",
		},
	}

	for _, tc := range testCases {
		for _, tag := range tags {
			name := tag.name + " " + tc.name
			t.Run(name, func(t *testing.T) {
				dm, _ := DecOptions{}.DecModeWithTags(tag.tagSet)
				v := reflect.New(reflect.TypeOf(tc.obj))
				if err := dm.Unmarshal(tc.data, v.Interface()); err == nil {
					t.Errorf("Unmarshal(0x%x) didn't return an error", tc.data)
				} else {
					if _, ok := err.(*WrongTagError); !ok {
						t.Errorf("Unmarshal(0x%x) returned wrong type of error %T, want (*WrongTagError)", tc.data, err)
					} else if err.Error() != tc.wantErrorMsg {
						t.Errorf("Unmarshal(0x%x) returned error %q, want error %q", tc.data, err.Error(), tc.wantErrorMsg)
					}
				}
			})
		}
	}

	// Decode wrong tag data with DecTagIgnored option returns no error.
	for _, tc := range testCases {
		name := "EncTagRequired_DecTagIgnored " + tc.name
		t.Run(name, func(t *testing.T) {
			dm, _ := DecOptions{}.DecModeWithTags(tagsDecIgnored)
			v := reflect.New(reflect.TypeOf(tc.obj))
			if err := dm.Unmarshal(tc.data, v.Interface()); err != nil {
				t.Errorf("Unmarshal() returned error %v", err)
			}
			if !reflect.DeepEqual(tc.obj, v.Elem().Interface()) {
				t.Errorf("Marshal-Unmarshal returned different values: %v, %v", tc.obj, v.Elem().Interface())
			}
		})
	}
}

func TestEncodeSharedTag(t *testing.T) {
	type myInt int

	myIntType := reflect.TypeOf(myInt(0))

	sharedTagSet := NewTagSet()

	em, err := EncOptions{}.EncModeWithSharedTags(sharedTagSet)
	if err != nil {
		t.Errorf("EncModeWithSharedTags() returned error %v", err)
	}

	// Register myInt type with tag number 123
	if err = sharedTagSet.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, myIntType, 123); err != nil {
		t.Fatalf("TagSet.Add(%s, %v) returned error %v", myIntType, 100, err)
	}

	// Encode myInt with tag number 123
	v := myInt(1)
	wantCborData := mustHexDecode("d87b01")
	b, err := em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v, err)
	}
	if !bytes.Equal(b, wantCborData) {
		t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, b, wantCborData)
	}

	// Unregister myInt type
	sharedTagSet.Remove(myIntType)

	// Encode myInt without tag number 123
	v = myInt(2)
	wantCborData = mustHexDecode("02")
	b, err = em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v, err)
	}
	if !bytes.Equal(b, wantCborData) {
		t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, b, wantCborData)
	}

	// Register myInt type with tag number 234
	if err = sharedTagSet.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, myIntType, 234); err != nil {
		t.Fatalf("TagSet.Add(%s, %v) returned error %v", myIntType, 100, err)
	}

	// Encode myInt with tag number 234
	v = myInt(3)
	wantCborData = mustHexDecode("d8ea03")
	b, err = em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v, err)
	}
	if !bytes.Equal(b, wantCborData) {
		t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, b, wantCborData)
	}

}

func TestDecodeSharedTag(t *testing.T) {
	type myInt int

	myIntType := reflect.TypeOf(myInt(0))

	sharedTagSet := NewTagSet()

	dm, err := DecOptions{}.DecModeWithSharedTags(sharedTagSet)
	if err != nil {
		t.Errorf("DecModeWithSharedTags() returned error %v", err)
	}

	// Register myInt type with tag number 123
	if err = sharedTagSet.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, myIntType, 123); err != nil {
		t.Fatalf("TagSet.Add(%s, %v) returned error %v", myIntType, 100, err)
	}

	// Decode myInt with tag number 123
	var v myInt
	wantV := myInt(1)
	data := mustHexDecode("d87b01")
	if err = dm.Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(v, wantV) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantV, wantV)
	}

	// Unregister myInt type
	sharedTagSet.Remove(myIntType)

	// Decode myInt without tag number
	wantV = myInt(2)
	data = mustHexDecode("02")
	if err := dm.Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(v, wantV) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantV, wantV)
	}

	// Register myInt type with tag number 234
	if err := sharedTagSet.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, myIntType, 234); err != nil {
		t.Fatalf("TagSet.Add(%s, %v) returned error %v", myIntType, 100, err)
	}

	// Decode myInt with tag number 234
	wantV = myInt(3)
	data = mustHexDecode("d8ea03")
	if err := dm.Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(v, wantV) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantV, wantV)
	}
}

func TestDecModeWithTagsError(t *testing.T) {
	// Create DecMode with nil as TagSet
	wantErrorMsg := "cbor: cannot create DecMode with nil value as TagSet"
	dm, err := DecOptions{}.DecModeWithTags(nil)
	if dm != nil {
		t.Errorf("DecModeWithTags(nil) returned %v", dm)
	}
	if err.Error() != wantErrorMsg {
		t.Errorf("DecModeWithTags(nil) returned error %q, want %q", err.Error(), wantErrorMsg)
	}
	dm, err = DecOptions{}.DecModeWithSharedTags(nil)
	if dm != nil {
		t.Errorf("DecModeWithSharedTags(nil) returned %v", dm)
	}
	if err.Error() != wantErrorMsg {
		t.Errorf("DecModeWithSharedTags(nil) returned error %q, want %q", err.Error(), wantErrorMsg)
	}

	// Create DecMode with invalid EncOptions
	wantErrorMsg = "cbor: invalid TimeTag 100"
	dm, err = DecOptions{TimeTag: 100}.DecModeWithTags(NewTagSet())
	if dm != nil {
		t.Errorf("DecModeWithTags() returned %v", dm)
	}
	if err.Error() != wantErrorMsg {
		t.Errorf("DecModeWithTags() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
	dm, err = DecOptions{TimeTag: 100}.DecModeWithSharedTags(NewTagSet())
	if dm != nil {
		t.Errorf("DecModeWithSharedTags() returned %v", dm)
	}
	if err.Error() != wantErrorMsg {
		t.Errorf("DecModeWithSharedTags() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
}

func TestEncModeWithTagsError(t *testing.T) {
	// Create EncMode with nil as TagSet
	wantErrorMsg := "cbor: cannot create EncMode with nil value as TagSet"
	em, err := EncOptions{}.EncModeWithTags(nil)
	if em != nil {
		t.Errorf("EncModeWithTags(nil) returned %v", em)
	}
	if err.Error() != wantErrorMsg {
		t.Errorf("EncModeWithTags(nil) returned error %q, want %q", err.Error(), wantErrorMsg)
	}
	em, err = EncOptions{}.EncModeWithSharedTags(nil)
	if em != nil {
		t.Errorf("EncModeWithSharedTags(nil) returned %v", em)
	}
	if err.Error() != wantErrorMsg {
		t.Errorf("EncModeWithSharedTags(nil) returned error %q, want %q", err.Error(), wantErrorMsg)
	}

	// Create EncMode with invalid EncOptions
	wantErrorMsg = "cbor: invalid TimeTag 100"
	em, err = EncOptions{TimeTag: 100}.EncModeWithTags(NewTagSet())
	if em != nil {
		t.Errorf("EncModeWithTags() returned %v", em)
	}
	if err.Error() != wantErrorMsg {
		t.Errorf("EncModeWithTags() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
	em, err = EncOptions{TimeTag: 100}.EncModeWithSharedTags(NewTagSet())
	if em != nil {
		t.Errorf("EncModeWithSharedTags() returned %v", em)
	}
	if err.Error() != wantErrorMsg {
		t.Errorf("EncModeWithSharedTags() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
}

func TestNilRawTagUnmarshalCBORError(t *testing.T) {
	wantErrorMsg := "cbor.RawTag: UnmarshalCBOR on nil pointer"
	var tag *RawTag
	data := mustHexDecode("c249010000000000000000")
	if err := tag.UnmarshalCBOR(data); err == nil {
		t.Errorf("UnmarshalCBOR() didn't return error")
	} else if err.Error() != wantErrorMsg {
		t.Errorf("UnmarshalCBOR() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
}

func TestTagUnmarshalError(t *testing.T) {
	data := mustHexDecode("d87b61fe") // invalid UTF-8 string
	var tag Tag
	if err := Unmarshal(data, &tag); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return error", data)
	} else if err.Error() != invalidUTF8ErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), invalidUTF8ErrorMsg)
	}
}

func TestTagMarshalError(t *testing.T) {
	wantErrorMsg := "cbor: unsupported type: chan bool"
	tag := Tag{
		Number:  123,
		Content: make(chan bool),
	}
	if _, err := Marshal(tag); err == nil {
		t.Errorf("Marshal() didn't return error")
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Marshal() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
}

func TestMarshalUninitializedTag(t *testing.T) {
	var v Tag
	b, err := Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v, err)
	}
	if !bytes.Equal(b, cborNil) {
		t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, b, cborNil)
	}
}

func TestMarshalUninitializedRawTag(t *testing.T) {
	var v RawTag
	b, err := Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v, err)
	}
	if !bytes.Equal(b, cborNil) {
		t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, b, cborNil)
	}
}

func TestMarshalTagWithEmptyContent(t *testing.T) {
	v := Tag{Number: 100}           // Tag.Content is empty
	want := mustHexDecode("d864f6") // 100(null)
	b, err := Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v, err)
	}
	if !bytes.Equal(b, want) {
		t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, b, want)
	}
}

func TestMarshalRawTagWithEmptyContent(t *testing.T) {
	v := RawTag{Number: 100}        // RawTag.Content is empty
	want := mustHexDecode("d864f6") // 100(null)
	b, err := Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v, err)
	}
	if !bytes.Equal(b, want) {
		t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, b, want)
	}
}

func TestEncodeTag(t *testing.T) {
	m := make(map[any]bool)
	m[10] = true
	m[100] = true
	m[-1] = true
	m["z"] = true
	m["aa"] = true
	m[[1]int{100}] = true
	m[[1]int{-1}] = true
	m[false] = true

	v := Tag{100, m}

	lenFirstSortedCborData := mustHexDecode("d864a80af520f5f4f51864f5617af58120f5626161f5811864f5") // tag number: 100, value: map with sorted keys: 10, -1, false, 100, "z", [-1], "aa", [100]
	bytewiseSortedCborData := mustHexDecode("d864a80af51864f520f5617af5626161f5811864f58120f5f4f5") // tag number: 100, value: map with sorted keys: 10, 100, -1, "z", "aa", [100], [-1], false

	em, _ := EncOptions{Sort: SortLengthFirst}.EncMode()
	b, err := em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v, err)
	}
	if !bytes.Equal(b, lenFirstSortedCborData) {
		t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, b, lenFirstSortedCborData)
	}

	em, _ = EncOptions{Sort: SortBytewiseLexical}.EncMode()
	b, err = em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v, err)
	}
	if !bytes.Equal(b, bytewiseSortedCborData) {
		t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, b, bytewiseSortedCborData)
	}
}

func TestDecodeTagToEmptyIface(t *testing.T) {
	type myBool bool
	type myUint uint

	typeMyBool := reflect.TypeOf(myBool(false))
	typeMyUint := reflect.TypeOf(myUint(0))

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, typeMyBool, 100); err != nil {
		t.Fatalf("TagSet.Add(%s, %d) returned error %v", typeMyBool, 100, err)
	}
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, typeMyUint, 101, 102); err != nil {
		t.Fatalf("TagSet.Add(%s, %d, %d) returned error %v", typeMyUint, 101, 102, err)
	}

	dm, _ := DecOptions{}.DecModeWithTags(tags)
	dmSharedTags, _ := DecOptions{}.DecModeWithSharedTags(tags)

	testCases := []struct {
		name    string
		data    []byte
		wantObj any
	}{
		{
			name:    "registered myBool",
			data:    mustHexDecode("d864f5"), // 100(true)
			wantObj: myBool(true),
		},
		{
			name:    "registered myUint",
			data:    mustHexDecode("d865d86600"), // 101(102(0))
			wantObj: myUint(0),
		},
		{
			name:    "not registered bool",
			data:    mustHexDecode("d865f5"), // 101(true)
			wantObj: Tag{101, true},
		},
		{
			name:    "not registered uint",
			data:    mustHexDecode("d865d86700"), // 101(103(0))
			wantObj: Tag{101, Tag{103, uint64(0)}},
		},
		{
			name:    "not registered uint",
			data:    mustHexDecode("d865d866d86700"), // 101(102(103(0)))
			wantObj: Tag{101, Tag{102, Tag{103, uint64(0)}}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v1 any
			if err := dm.Unmarshal(tc.data, &v1); err != nil {
				t.Errorf("Unmarshal() returned error %v", err)
			}
			if !reflect.DeepEqual(tc.wantObj, v1) {
				t.Errorf("Unmarshal to interface{} returned different values: %v, %v", tc.wantObj, v1)
			}

			var v2 any
			if err := dmSharedTags.Unmarshal(tc.data, &v2); err != nil {
				t.Errorf("Unmarshal() returned error %v", err)
			}
			if !reflect.DeepEqual(tc.wantObj, v2) {
				t.Errorf("Unmarshal to interface{} returned different values: %v, %v", tc.wantObj, v2)
			}
		})
	}
}

func TestDecodeRegisteredTagToEmptyIfaceError(t *testing.T) {
	type myInt int

	typeMyInt := reflect.TypeOf(myInt(0))

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, typeMyInt, 101, 102); err != nil {
		t.Fatalf("TagSet.Add(%s, %d, %d) returned error %v", typeMyInt, 101, 102, err)
	}

	dm, _ := DecOptions{}.DecModeWithTags(tags)

	data := mustHexDecode("d865d8663bffffffffffffffff") // 101(102(-18446744073709551616))

	var v any
	if err := dm.Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), "cannot unmarshal")
	}
}

type number3 uint64

// MarshalCBOR marshals number3 to CBOR tagged map (tag number 100)
func (n number3) MarshalCBOR() (data []byte, err error) {
	m := map[string]uint64{"num": uint64(n)}
	return Marshal(Tag{100, m})
}

// UnmarshalCBOR unmarshals CBOR tagged map to number3
func (n *number3) UnmarshalCBOR(data []byte) (err error) {
	var rawTag RawTag
	if err := Unmarshal(data, &rawTag); err != nil {
		return err
	}

	if rawTag.Number != 100 {
		return fmt.Errorf("wrong tag number %d, want %d", rawTag.Number, 100)
	}

	if getType(rawTag.Content[0]) != cborTypeMap {
		return fmt.Errorf("wrong tag content type, want map")
	}

	var v map[string]uint64
	if err := Unmarshal(rawTag.Content, &v); err != nil {
		return err
	}
	*n = number3(v["num"])
	return nil
}

func TestDecodeRegisterTagForUnmarshaler(t *testing.T) {
	typ := reflect.TypeOf(number3(0))

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, typ, 100); err != nil {
		t.Fatalf("TagSet.Add(%s, %d) returned error %v", typ, 100, err)
	}

	data := mustHexDecode("d864a1636e756d01") // 100({"num": 1})
	wantObj := number3(1)

	dm, _ := DecOptions{}.DecModeWithTags(tags)
	em, _ := EncOptions{}.EncModeWithTags(tags)

	// Decode to empty interface.  Unmarshal() should return object of registered type.
	var v1 any
	if err := dm.Unmarshal(data, &v1); err != nil {
		t.Errorf("Unmarshal() returned error %v", err)
	}
	if !reflect.DeepEqual(wantObj, v1) {
		t.Errorf("Unmarshal() returned different values: %v (%T), %v (%T)", wantObj, wantObj, v1, v1)
	}
	b, err := em.Marshal(v1)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v1, err)
	} else if !bytes.Equal(b, data) {
		t.Errorf("Marshal(%v) returned %v, want %v", v1, b, data)
	}

	// Decode to registered type.
	var v2 number3
	if err = dm.Unmarshal(data, &v2); err != nil {
		t.Errorf("Unmarshal() returned error %v", err)
	}
	if !reflect.DeepEqual(wantObj, v2) {
		t.Errorf("Unmarshal() returned different values: %v, %v", wantObj, v2)
	}
	b, err = em.Marshal(v2)
	if err != nil {
		t.Errorf("Marshal(%v) returned error %v", v2, err)
	} else if !bytes.Equal(b, data) {
		t.Errorf("Marshal(%v) returned %v, want %v", v2, b, data)
	}
}

func TestMarshalRawTagContainingMalformedCBORData(t *testing.T) {
	testCases := []struct {
		name         string
		value        any
		wantErrorMsg string
	}{
		// Nil RawMessage and empty RawMessage are encoded as CBOR nil.
		{
			name:         "truncated data",
			value:        RawTag{Number: 100, Content: RawMessage{0xa6}},
			wantErrorMsg: "cbor: error calling MarshalCBOR for type cbor.RawTag: unexpected EOF",
		},
		{
			name:         "malformed data",
			value:        RawTag{Number: 100, Content: RawMessage{0x1f}},
			wantErrorMsg: "cbor: error calling MarshalCBOR for type cbor.RawTag: cbor: invalid additional information 31 for type positive integer",
		},
		{
			name:         "extraneous data",
			value:        RawTag{Number: 100, Content: RawMessage{0x01, 0x01}},
			wantErrorMsg: "cbor: error calling MarshalCBOR for type cbor.RawTag: cbor: 1 bytes of extraneous data starting at index 3",
		},
		{
			name:         "invalid builtin tag",
			value:        RawTag{Number: 0, Content: RawMessage{0x01}},
			wantErrorMsg: "cbor: error calling MarshalCBOR for type cbor.RawTag: cbor: tag number 0 must be followed by text string, got positive integer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := Marshal(tc.value)
			if err == nil {
				t.Errorf("Marshal(%v) didn't return an error, want error %q", tc.value, tc.wantErrorMsg)
			} else if _, ok := err.(*MarshalerError); !ok {
				t.Errorf("Marshal(%v) error type %T, want *MarshalerError", tc.value, err)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Marshal(%v) error %q, want %q", tc.value, err.Error(), tc.wantErrorMsg)
			}
			if b != nil {
				t.Errorf("Marshal(%v) = 0x%x, want nil", tc.value, b)
			}
		})
	}
}

// TestEncodeBuiltinTag tests that marshaling a value of type Tag "does the right thing" when
// marshaling the enclosed data item of a built-in tag number.
func TestEncodeBuiltinTag(t *testing.T) {
	for _, tc := range []struct {
		name string
		tag  Tag
		opts EncOptions
		want []byte
	}{
		{
			name: "unsigned bignum content not enclosed in expected encoding tag",
			tag:  Tag{Number: tagNumUnsignedBignum, Content: []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
			opts: EncOptions{ByteSliceLaterFormat: ByteSliceLaterFormatBase16},
			want: []byte{0xc2, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "negative bignum content not enclosed in expected encoding tag",
			tag:  Tag{Number: tagNumNegativeBignum, Content: []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
			opts: EncOptions{ByteSliceLaterFormat: ByteSliceLaterFormatBase16},
			want: []byte{0xc3, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "rfc 3339 content is not encoded as byte string",
			tag:  Tag{Number: tagNumRFC3339Time, Content: "2013-03-21T20:04:00Z"},
			opts: EncOptions{String: StringToByteString},
			want: mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			em, err := tc.opts.EncMode()
			if err != nil {
				t.Fatal(err)
			}

			got, err := em.Marshal(tc.tag)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(got, tc.want) {
				t.Errorf("unexpected difference\ngot: 0x%x\nwant: 0x%x", got, tc.want)
			}
		})
	}
}

func TestUnmarshalRawTagOnBadData(t *testing.T) {
	testCases := []struct {
		name   string
		data   []byte
		errMsg string
	}{
		// Empty data
		{
			name:   "nil data",
			data:   nil,
			errMsg: io.EOF.Error(),
		},
		{
			name:   "empty data",
			data:   []byte{},
			errMsg: io.EOF.Error(),
		},

		// Wrong CBOR types
		{
			name:   "uint type",
			data:   mustHexDecode("01"),
			errMsg: "cbor: cannot unmarshal positive integer into Go value of type cbor.RawTag",
		},
		{
			name:   "int type",
			data:   mustHexDecode("20"),
			errMsg: "cbor: cannot unmarshal negative integer into Go value of type cbor.RawTag",
		},
		{
			name:   "byte string type",
			data:   mustHexDecode("40"),
			errMsg: "cbor: cannot unmarshal byte string into Go value of type cbor.RawTag",
		},
		{
			name:   "string type",
			data:   mustHexDecode("60"),
			errMsg: "cbor: cannot unmarshal UTF-8 text string into Go value of type cbor.RawTag",
		},
		{
			name:   "array type",
			data:   mustHexDecode("80"),
			errMsg: "cbor: cannot unmarshal array into Go value of type cbor.RawTag",
		},
		{
			name:   "map type",
			data:   mustHexDecode("a0"),
			errMsg: "cbor: cannot unmarshal map into Go value of type cbor.RawTag",
		},
		{
			name:   "primitive type",
			data:   mustHexDecode("f4"),
			errMsg: "cbor: cannot unmarshal primitives into Go value of type cbor.RawTag",
		},
		{
			name:   "float type",
			data:   mustHexDecode("f90000"),
			errMsg: "cbor: cannot unmarshal primitives into Go value of type cbor.RawTag",
		},

		// Truncated CBOR data
		{
			name:   "truncated head",
			data:   mustHexDecode("18"),
			errMsg: io.ErrUnexpectedEOF.Error(),
		},

		// Truncated CBOR tag data
		{
			name:   "truncated tag number",
			data:   mustHexDecode("d8"),
			errMsg: io.ErrUnexpectedEOF.Error(),
		},
		{
			name:   "tag number not followed by tag content",
			data:   mustHexDecode("da"),
			errMsg: io.ErrUnexpectedEOF.Error(),
		},
		{
			name:   "truncated tag content",
			data:   mustHexDecode("c074323031332d30332d32315432303a30343a3030"),
			errMsg: io.ErrUnexpectedEOF.Error(),
		},

		// Extraneous CBOR data
		{
			name:   "extraneous data",
			data:   mustHexDecode("c074323031332d30332d32315432303a30343a30305a00"),
			errMsg: "cbor: 1 bytes of extraneous data starting at index 22",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test RawTag.UnmarshalCBOR(data)
			{
				var v RawTag

				err := v.UnmarshalCBOR(tc.data)
				if err == nil {
					t.Errorf("UnmarshalCBOR(%x) didn't return error", tc.data)
				}
				if !strings.HasPrefix(err.Error(), tc.errMsg) {
					t.Errorf("UnmarshalCBOR(%x) returned error %q, want %q", tc.data, err.Error(), tc.errMsg)
				}
			}
			// Test Unmarshal(data, *RawTag), which calls RawTag.unmarshalCBOR() under the hood
			{
				var v RawTag

				err := Unmarshal(tc.data, &v)
				if err == nil {
					t.Errorf("Unmarshal(%x) didn't return error", tc.data)
				}
				if !strings.HasPrefix(err.Error(), tc.errMsg) {
					t.Errorf("Unmarshal(%x) returned error %q, want %q", tc.data, err.Error(), tc.errMsg)
				}
			}
		})
	}
}

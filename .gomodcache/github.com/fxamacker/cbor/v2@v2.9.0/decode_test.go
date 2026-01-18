// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	typeBool            = reflect.TypeOf(true)
	typeUint8           = reflect.TypeOf(uint8(0))
	typeUint16          = reflect.TypeOf(uint16(0))
	typeUint32          = reflect.TypeOf(uint32(0))
	typeUint64          = reflect.TypeOf(uint64(0))
	typeInt8            = reflect.TypeOf(int8(0))
	typeInt16           = reflect.TypeOf(int16(0))
	typeInt32           = reflect.TypeOf(int32(0))
	typeInt64           = reflect.TypeOf(int64(0))
	typeFloat32         = reflect.TypeOf(float32(0))
	typeFloat64         = reflect.TypeOf(float64(0))
	typeByteArray       = reflect.TypeOf([5]byte{})
	typeIntSlice        = reflect.TypeOf([]int{})
	typeStringSlice     = reflect.TypeOf([]string{})
	typeMapIntfIntf     = reflect.TypeOf(map[any]any{})
	typeMapStringInt    = reflect.TypeOf(map[string]int{})
	typeMapStringString = reflect.TypeOf(map[string]string{})
	typeMapStringIntf   = reflect.TypeOf(map[string]any{})
)

type unmarshalTest struct {
	data               []byte
	wantInterfaceValue any
	wantValues         []any
	wrongTypes         []reflect.Type
}

var unmarshalTests = []unmarshalTest{
	// CBOR test data are from https://tools.ietf.org/html/rfc7049#appendix-A.

	// unsigned integer
	{
		data:               mustHexDecode("00"),
		wantInterfaceValue: uint64(0),
		wantValues: []any{
			uint8(0),
			uint16(0),
			uint32(0),
			uint64(0),
			uint(0),
			int8(0),
			int16(0),
			int32(0),
			int64(0),
			int(0),
			float32(0),
			float64(0),
			mustBigInt("0"),
		},
		wrongTypes: []reflect.Type{
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("01"),
		wantInterfaceValue: uint64(1),
		wantValues: []any{
			uint8(1),
			uint16(1),
			uint32(1),
			uint64(1),
			uint(1),
			int8(1),
			int16(1),
			int32(1),
			int64(1),
			int(1),
			float32(1),
			float64(1),
			mustBigInt("1"),
		},
		wrongTypes: []reflect.Type{
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("0a"),
		wantInterfaceValue: uint64(10),
		wantValues: []any{
			uint8(10),
			uint16(10),
			uint32(10),
			uint64(10),
			uint(10),
			int8(10),
			int16(10),
			int32(10),
			int64(10),
			int(10),
			float32(10),
			float64(10),
			mustBigInt("10"),
		},
		wrongTypes: []reflect.Type{
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("17"),
		wantInterfaceValue: uint64(23),
		wantValues: []any{
			uint8(23),
			uint16(23),
			uint32(23),
			uint64(23),
			uint(23),
			int8(23),
			int16(23),
			int32(23),
			int64(23),
			int(23),
			float32(23),
			float64(23),
			mustBigInt("23"),
		},
		wrongTypes: []reflect.Type{
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("1818"),
		wantInterfaceValue: uint64(24),
		wantValues: []any{
			uint8(24),
			uint16(24),
			uint32(24),
			uint64(24),
			uint(24),
			int8(24),
			int16(24),
			int32(24),
			int64(24),
			int(24),
			float32(24),
			float64(24),
			mustBigInt("24"),
		},
		wrongTypes: []reflect.Type{
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("1819"),
		wantInterfaceValue: uint64(25),
		wantValues: []any{
			uint8(25),
			uint16(25),
			uint32(25),
			uint64(25),
			uint(25),
			int8(25),
			int16(25),
			int32(25),
			int64(25),
			int(25),
			float32(25),
			float64(25),
			mustBigInt("25"),
		},
		wrongTypes: []reflect.Type{
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("1864"),
		wantInterfaceValue: uint64(100),
		wantValues: []any{
			uint8(100),
			uint16(100),
			uint32(100),
			uint64(100),
			uint(100),
			int8(100),
			int16(100),
			int32(100),
			int64(100),
			int(100),
			float32(100),
			float64(100),
			mustBigInt("100"),
		},
		wrongTypes: []reflect.Type{
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("1903e8"),
		wantInterfaceValue: uint64(1000),
		wantValues: []any{
			uint16(1000),
			uint32(1000),
			uint64(1000),
			uint(1000),
			int16(1000),
			int32(1000),
			int64(1000),
			int(1000),
			float32(1000),
			float64(1000),
			mustBigInt("1000"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeInt8,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("1a000f4240"),
		wantInterfaceValue: uint64(1000000),
		wantValues: []any{
			uint32(1000000),
			uint64(1000000),
			uint(1000000),
			int32(1000000),
			int64(1000000),
			int(1000000),
			float32(1000000),
			float64(1000000),
			mustBigInt("1000000"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeInt8,
			typeInt16,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("1b000000e8d4a51000"),
		wantInterfaceValue: uint64(1000000000000),
		wantValues: []any{
			uint64(1000000000000),
			uint(1000000000000),
			int64(1000000000000),
			int(1000000000000),
			float32(1000000000000),
			float64(1000000000000),
			mustBigInt("1000000000000"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeInt8,
			typeInt16,
			typeInt32,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("1bffffffffffffffff"),
		wantInterfaceValue: uint64(18446744073709551615),
		wantValues: []any{
			uint64(18446744073709551615),
			uint(18446744073709551615),
			float32(18446744073709551615),
			float64(18446744073709551615),
			mustBigInt("18446744073709551615"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeInt8,
			typeInt16,
			typeInt32,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},

	// negative integer
	{
		data:               mustHexDecode("20"),
		wantInterfaceValue: int64(-1),
		wantValues: []any{
			int8(-1),
			int16(-1),
			int32(-1),
			int64(-1),
			int(-1),
			float32(-1),
			float64(-1),
			mustBigInt("-1"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("29"),
		wantInterfaceValue: int64(-10),
		wantValues: []any{
			int8(-10),
			int16(-10),
			int32(-10),
			int64(-10),
			int(-10),
			float32(-10),
			float64(-10),
			mustBigInt("-10"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("3863"),
		wantInterfaceValue: int64(-100),
		wantValues: []any{
			int8(-100),
			int16(-100),
			int32(-100),
			int64(-100),
			int(-100),
			float32(-100),
			float64(-100),
			mustBigInt("-100"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("3903e7"),
		wantInterfaceValue: int64(-1000),
		wantValues: []any{
			int16(-1000),
			int32(-1000),
			int64(-1000),
			int(-1000),
			float32(-1000),
			float64(-1000),
			mustBigInt("-1000"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("3bffffffffffffffff"),
		wantInterfaceValue: mustBigInt("-18446744073709551616"),
		wantValues: []any{
			mustBigInt("-18446744073709551616"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	}, // CBOR value -18446744073709551616 overflows Go's int64, see TestNegIntOverflow

	// byte string
	{
		data:               mustHexDecode("40"),
		wantInterfaceValue: []byte{},
		wantValues: []any{
			[]byte{},
			[0]byte{},
			[1]byte{0},
			[5]byte{0, 0, 0, 0, 0},
			ByteString(""),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("4401020304"),
		wantInterfaceValue: []byte{1, 2, 3, 4},
		wantValues: []any{
			[]byte{1, 2, 3, 4},
			[0]byte{},
			[1]byte{1},
			[5]byte{1, 2, 3, 4, 0},
			ByteString("\x01\x02\x03\x04"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("5f42010243030405ff"),
		wantInterfaceValue: []byte{1, 2, 3, 4, 5},
		wantValues: []any{
			[]byte{1, 2, 3, 4, 5},
			[0]byte{},
			[1]byte{1},
			[5]byte{1, 2, 3, 4, 5},
			[6]byte{1, 2, 3, 4, 5, 0},
			ByteString("\x01\x02\x03\x04\x05"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeSimpleValue,
		},
	},

	// text string
	{
		data:               mustHexDecode("60"),
		wantInterfaceValue: "",
		wantValues:         []any{""},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("6161"),
		wantInterfaceValue: "a",
		wantValues:         []any{"a"},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("6449455446"),
		wantInterfaceValue: "IETF",
		wantValues:         []any{"IETF"},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("62225c"),
		wantInterfaceValue: "\"\\",
		wantValues:         []any{"\"\\"},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("62c3bc"),
		wantInterfaceValue: "ü",
		wantValues:         []any{"ü"},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("63e6b0b4"),
		wantInterfaceValue: "水",
		wantValues:         []any{"水"},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("64f0908591"),
		wantInterfaceValue: "𐅑",
		wantValues:         []any{"𐅑"},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("7f657374726561646d696e67ff"),
		wantInterfaceValue: "streaming",
		wantValues:         []any{"streaming"},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},

	// array
	{
		data:               mustHexDecode("80"),
		wantInterfaceValue: []any{},
		wantValues: []any{
			[]any{},
			[]byte{},
			[]string{},
			[]int{},
			[0]int{},
			[1]int{0},
			[5]int{0},
			[]float32{},
			[]float64{},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("83010203"),
		wantInterfaceValue: []any{uint64(1), uint64(2), uint64(3)},
		wantValues: []any{
			[]any{uint64(1), uint64(2), uint64(3)},
			[]byte{1, 2, 3},
			[]int{1, 2, 3},
			[]uint{1, 2, 3},
			[0]int{},
			[1]int{1},
			[3]int{1, 2, 3},
			[5]int{1, 2, 3, 0, 0},
			[]float32{1, 2, 3},
			[]float64{1, 2, 3},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("8301820203820405"),
		wantInterfaceValue: []any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		wantValues: []any{
			[]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
			[...]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("83018202039f0405ff"),
		wantInterfaceValue: []any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		wantValues: []any{
			[]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
			[...]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("83019f0203ff820405"),
		wantInterfaceValue: []any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		wantValues: []any{
			[]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
			[...]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("98190102030405060708090a0b0c0d0e0f101112131415161718181819"),
		wantInterfaceValue: []any{uint64(1), uint64(2), uint64(3), uint64(4), uint64(5), uint64(6), uint64(7), uint64(8), uint64(9), uint64(10), uint64(11), uint64(12), uint64(13), uint64(14), uint64(15), uint64(16), uint64(17), uint64(18), uint64(19), uint64(20), uint64(21), uint64(22), uint64(23), uint64(24), uint64(25)},
		wantValues: []any{
			[]any{uint64(1), uint64(2), uint64(3), uint64(4), uint64(5), uint64(6), uint64(7), uint64(8), uint64(9), uint64(10), uint64(11), uint64(12), uint64(13), uint64(14), uint64(15), uint64(16), uint64(17), uint64(18), uint64(19), uint64(20), uint64(21), uint64(22), uint64(23), uint64(24), uint64(25)},
			[]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[0]int{},
			[1]int{1},
			[...]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[30]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 0, 0, 0, 0, 0},
			[]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("9fff"),
		wantInterfaceValue: []any{},
		wantValues: []any{
			[]any{},
			[]byte{},
			[]string{},
			[]int{},
			[0]int{},
			[1]int{0},
			[5]int{0, 0, 0, 0, 0},
			[]float32{},
			[]float64{},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("9f018202039f0405ffff"),
		wantInterfaceValue: []any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		wantValues: []any{
			[]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
			[...]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("9f01820203820405ff"),
		wantInterfaceValue: []any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		wantValues: []any{
			[]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
			[...]any{uint64(1), []any{uint64(2), uint64(3)}, []any{uint64(4), uint64(5)}},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("9f0102030405060708090a0b0c0d0e0f101112131415161718181819ff"),
		wantInterfaceValue: []any{uint64(1), uint64(2), uint64(3), uint64(4), uint64(5), uint64(6), uint64(7), uint64(8), uint64(9), uint64(10), uint64(11), uint64(12), uint64(13), uint64(14), uint64(15), uint64(16), uint64(17), uint64(18), uint64(19), uint64(20), uint64(21), uint64(22), uint64(23), uint64(24), uint64(25)},
		wantValues: []any{
			[]any{uint64(1), uint64(2), uint64(3), uint64(4), uint64(5), uint64(6), uint64(7), uint64(8), uint64(9), uint64(10), uint64(11), uint64(12), uint64(13), uint64(14), uint64(15), uint64(16), uint64(17), uint64(18), uint64(19), uint64(20), uint64(21), uint64(22), uint64(23), uint64(24), uint64(25)},
			[]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]uint{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[0]int{},
			[1]int{1},
			[...]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[30]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 0, 0, 0, 0, 0},
			[]float32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			[]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("826161a161626163"),
		wantInterfaceValue: []any{"a", map[any]any{"b": "c"}},
		wantValues: []any{
			[]any{"a", map[any]any{"b": "c"}},
			[...]any{"a", map[any]any{"b": "c"}},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeByteArray,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("826161bf61626163ff"),
		wantInterfaceValue: []any{"a", map[any]any{"b": "c"}},
		wantValues: []any{
			[]any{"a", map[any]any{"b": "c"}},
			[...]any{"a", map[any]any{"b": "c"}},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeByteArray,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},

	// map
	{
		data:               mustHexDecode("a0"),
		wantInterfaceValue: map[any]any{},
		wantValues: []any{
			map[any]any{},
			map[string]bool{},
			map[string]int{},
			map[int]string{},
			map[int]bool{},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeByteSlice,
			typeByteArray,
			typeString,
			typeBool,
			typeIntSlice,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("a201020304"),
		wantInterfaceValue: map[any]any{uint64(1): uint64(2), uint64(3): uint64(4)},
		wantValues: []any{
			map[any]any{uint64(1): uint64(2), uint64(3): uint64(4)},
			map[uint]int{1: 2, 3: 4}, map[int]uint{1: 2, 3: 4},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeByteSlice,
			typeByteArray,
			typeString,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("a26161016162820203"),
		wantInterfaceValue: map[any]any{"a": uint64(1), "b": []any{uint64(2), uint64(3)}},
		wantValues: []any{
			map[any]any{"a": uint64(1), "b": []any{uint64(2), uint64(3)}},
			map[string]any{"a": uint64(1), "b": []any{uint64(2), uint64(3)}},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeByteSlice,
			typeByteArray,
			typeString,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("a56161614161626142616361436164614461656145"),
		wantInterfaceValue: map[any]any{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E"},
		wantValues: []any{
			map[any]any{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E"},
			map[string]any{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E"},
			map[string]string{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E"},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeByteSlice,
			typeByteArray,
			typeString,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("bf61610161629f0203ffff"),
		wantInterfaceValue: map[any]any{"a": uint64(1), "b": []any{uint64(2), uint64(3)}},
		wantValues: []any{
			map[any]any{"a": uint64(1), "b": []any{uint64(2), uint64(3)}},
			map[string]any{"a": uint64(1), "b": []any{uint64(2), uint64(3)}},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeByteSlice,
			typeByteArray,
			typeString,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("bf6346756ef563416d7421ff"),
		wantInterfaceValue: map[any]any{"Fun": true, "Amt": int64(-2)},
		wantValues: []any{
			map[any]any{"Fun": true, "Amt": int64(-2)},
			map[string]any{"Fun": true, "Amt": int64(-2)},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeByteSlice,
			typeByteArray,
			typeString,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},

	// tag
	{
		data:               mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
		wantInterfaceValue: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC), // 2013-03-21 20:04:00 +0000 UTC
		wantValues: []any{
			"2013-03-21T20:04:00Z",
			time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
			Tag{0, "2013-03-21T20:04:00Z"},
			RawTag{0, mustHexDecode("74323031332d30332d32315432303a30343a30305a")},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	}, // 0: standard date/time
	{
		data:               mustHexDecode("c11a514b67b0"),
		wantInterfaceValue: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC), // 2013-03-21 20:04:00 +0000 UTC
		wantValues: []any{
			uint32(1363896240),
			uint64(1363896240),
			int32(1363896240),
			int64(1363896240),
			float32(1363896240),
			float64(1363896240),
			time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
			Tag{1, uint64(1363896240)},
			RawTag{1, mustHexDecode("1a514b67b0")},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeInt8,
			typeInt16,
			typeByteSlice,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeByteString,
			typeSimpleValue,
		},
	}, // 1: epoch-based date/time
	{
		data:               mustHexDecode("c249010000000000000000"),
		wantInterfaceValue: mustBigInt("18446744073709551616"),
		wantValues: []any{
			// Decode to byte slice
			[]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			// Decode to array of various lengths
			[0]byte{},
			[1]byte{0x01},
			[3]byte{0x01, 0x00, 0x00},
			[...]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			[10]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			// Decode to Tag and RawTag
			Tag{2, []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
			RawTag{2, mustHexDecode("49010000000000000000")},
			// Decode to big.Int
			mustBigInt("18446744073709551616"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeByteString,
			typeSimpleValue,
		},
	}, // 2: positive bignum: 18446744073709551616
	{
		data:               mustHexDecode("c349010000000000000000"),
		wantInterfaceValue: mustBigInt("-18446744073709551617"),
		wantValues: []any{
			// Decode to byte slice
			[]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			// Decode to array of various lengths
			[0]byte{},
			[1]byte{0x01},
			[3]byte{0x01, 0x00, 0x00},
			[...]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			[10]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			// Decode to Tag and RawTag
			Tag{3, []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
			RawTag{3, mustHexDecode("49010000000000000000")},
			// Decode to big.Int
			mustBigInt("-18446744073709551617"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeByteString,
			typeSimpleValue,
		},
	}, // 3: negative bignum: -18446744073709551617
	{
		data:               mustHexDecode("c1fb41d452d9ec200000"),
		wantInterfaceValue: time.Date(2013, 3, 21, 20, 4, 0, 500000000, time.UTC), // 2013-03-21 20:04:00.5 +0000 UTC
		wantValues: []any{
			float32(1363896240.5),
			float64(1363896240.5),
			time.Date(2013, 3, 21, 20, 4, 0, 500000000, time.UTC),
			Tag{1, float64(1363896240.5)},
			RawTag{1, mustHexDecode("fb41d452d9ec200000")},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeByteSlice,
			typeByteArray,
			typeString,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	}, // 1: epoch-based date/time
	{
		data:               mustHexDecode("d74401020304"),
		wantInterfaceValue: Tag{23, []byte{0x01, 0x02, 0x03, 0x04}},
		wantValues: []any{
			[]byte{0x01, 0x02, 0x03, 0x04},
			[0]byte{},
			[1]byte{0x01},
			[3]byte{0x01, 0x02, 0x03},
			[...]byte{0x01, 0x02, 0x03, 0x04},
			[5]byte{0x01, 0x02, 0x03, 0x04, 0x00},
			Tag{23, []byte{0x01, 0x02, 0x03, 0x04}},
			RawTag{23, mustHexDecode("4401020304")},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	}, // 23: expected conversion to base16 encoding
	{
		data:               mustHexDecode("d818456449455446"),
		wantInterfaceValue: Tag{24, []byte{0x64, 0x49, 0x45, 0x54, 0x46}},
		wantValues: []any{
			[]byte{0x64, 0x49, 0x45, 0x54, 0x46},
			[0]byte{},
			[1]byte{0x64},
			[3]byte{0x64, 0x49, 0x45},
			[...]byte{0x64, 0x49, 0x45, 0x54, 0x46},
			[6]byte{0x64, 0x49, 0x45, 0x54, 0x46, 0x00},
			Tag{24, []byte{0x64, 0x49, 0x45, 0x54, 0x46}},
			RawTag{24, mustHexDecode("456449455446")},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	}, // 24: encoded cborBytes data item
	{
		data:               mustHexDecode("d82076687474703a2f2f7777772e6578616d706c652e636f6d"),
		wantInterfaceValue: Tag{32, "http://www.example.com"},
		wantValues: []any{
			"http://www.example.com",
			Tag{32, "http://www.example.com"},
			RawTag{32, mustHexDecode("76687474703a2f2f7777772e6578616d706c652e636f6d")},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	}, // 32: URI

	// primitives
	{
		data:               mustHexDecode("f4"),
		wantInterfaceValue: false,
		wantValues: []any{
			false,
			SimpleValue(20),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeByteArray,
			typeByteSlice,
			typeString,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
		},
	},
	{
		data:               mustHexDecode("f5"),
		wantInterfaceValue: true,
		wantValues: []any{
			true,
			SimpleValue(21),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeByteArray,
			typeByteSlice,
			typeString,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
		},
	},
	{
		data:               mustHexDecode("f6"),
		wantInterfaceValue: nil,
		wantValues: []any{
			SimpleValue(22),
			false,
			uint(0),
			uint8(0),
			uint16(0),
			uint32(0),
			uint64(0),
			int(0),
			int8(0),
			int16(0),
			int32(0),
			int64(0),
			float32(0.0),
			float64(0.0),
			"",
			[]byte(nil),
			[]int(nil),
			[]string(nil),
			map[string]int(nil),
			time.Time{},
			mustBigInt("0"),
			Tag{},
			RawTag{},
		},
		wrongTypes: nil,
	},
	{
		data:               mustHexDecode("f7"),
		wantInterfaceValue: nil,
		wantValues: []any{
			SimpleValue(23),
			false,
			uint(0),
			uint8(0),
			uint16(0),
			uint32(0),
			uint64(0),
			int(0),
			int8(0),
			int16(0),
			int32(0),
			int64(0),
			float32(0.0),
			float64(0.0),
			"",
			[]byte(nil),
			[]int(nil),
			[]string(nil),
			map[string]int(nil),
			time.Time{},
			mustBigInt("0"),
			Tag{},
			RawTag{},
		},
		wrongTypes: nil,
	},
	{
		data:               mustHexDecode("f0"),
		wantInterfaceValue: SimpleValue(16),
		wantValues: []any{
			SimpleValue(16),
			uint8(16),
			uint16(16),
			uint32(16),
			uint64(16),
			uint(16),
			int8(16),
			int16(16),
			int32(16),
			int64(16),
			int(16),
			float32(16),
			float64(16),
			mustBigInt("16"),
		},
		wrongTypes: []reflect.Type{
			typeByteSlice,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
		},
	},
	// This example is not well-formed because Simple value (with 5-bit value 24) must be >= 32.
	// See RFC 7049 section 2.3 for details, instead of the incorrect example in RFC 7049 Appendex A.
	// I reported an errata to RFC 7049 and Carsten Bormann confirmed at https://github.com/fxamacker/cbor/issues/46
	// {
	// data: hexDecode("f818"),
	// wantInterfaceValue: uint64(24),
	// wantValues: []interface{}{uint8(24), uint16(24), uint32(24), uint64(24), uint(24), int8(24), int16(24), int32(24), int64(24), int(24), float32(24), float64(24)},
	// wrongTypes: []reflect.Type{typeByteSlice, typeString, typeBool, typeIntSlice, typeMapStringInt},
	// },
	{
		data:               mustHexDecode("f820"),
		wantInterfaceValue: SimpleValue(32),
		wantValues: []any{
			SimpleValue(32),
			uint8(32),
			uint16(32),
			uint32(32),
			uint64(32),
			uint(32),
			int8(32),
			int16(32),
			int32(32),
			int64(32),
			int(32),
			float32(32),
			float64(32),
			mustBigInt("32"),
		},
		wrongTypes: []reflect.Type{
			typeByteSlice,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
		},
	},
	{
		data:               mustHexDecode("f8ff"),
		wantInterfaceValue: SimpleValue(255),
		wantValues: []any{
			SimpleValue(255),
			uint8(255),
			uint16(255),
			uint32(255),
			uint64(255),
			uint(255),
			int16(255),
			int32(255),
			int64(255),
			int(255),
			float32(255),
			float64(255),
			mustBigInt("255"),
		},
		wrongTypes: []reflect.Type{
			typeByteSlice,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeByteString,
		},
	},

	// More testcases not covered by https://tools.ietf.org/html/rfc7049#appendix-A.
	{
		data:               mustHexDecode("5fff"), // empty indefinite length byte string
		wantInterfaceValue: []byte{},
		wantValues: []any{
			[]byte{},
			[0]byte{},
			[1]byte{0x00},
			ByteString(""),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("7fff"), // empty indefinite length text string
		wantInterfaceValue: "",
		wantValues:         []any{""},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeTag,
			typeRawTag,
			typeBigInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("bfff"), // empty indefinite length map
		wantInterfaceValue: map[any]any{},
		wantValues: []any{
			map[any]any{},
			map[string]bool{},
			map[string]int{},
			map[int]string{},
			map[int]bool{},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeByteArray,
			typeByteSlice,
			typeString,
			typeBool,
			typeIntSlice,
			typeTag,
			typeRawTag,
			typeByteString,
			typeSimpleValue,
		},
	},

	// More test data with tags
	{
		data:               mustHexDecode("c13a0177f2cf"), // 1969-03-21T20:04:00Z, tag 1 with negative integer as epoch time
		wantInterfaceValue: time.Date(1969, 3, 21, 20, 4, 0, 0, time.UTC),
		wantValues: []any{
			int32(-24638160),
			int64(-24638160),
			int32(-24638160),
			int64(-24638160),
			float32(-24638160),
			float64(-24638160),
			time.Date(1969, 3, 21, 20, 4, 0, 0, time.UTC),
			Tag{1, int64(-24638160)},
			RawTag{1, mustHexDecode("3a0177f2cf")}, mustBigInt("-24638160"),
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeInt8,
			typeInt16,
			typeByteSlice,
			typeString,
			typeBool,
			typeByteArray,
			typeIntSlice,
			typeMapStringInt,
			typeByteString,
			typeSimpleValue,
		},
	},
	{
		data:               mustHexDecode("d83dd183010203"), // 61(17([1, 2, 3])), nested tags 61 and 17
		wantInterfaceValue: Tag{61, Tag{17, []any{uint64(1), uint64(2), uint64(3)}}},
		wantValues: []any{
			[]any{uint64(1), uint64(2), uint64(3)},
			[]byte{1, 2, 3},
			[0]byte{},
			[1]byte{1},
			[3]byte{1, 2, 3},
			[5]byte{1, 2, 3, 0, 0},
			[]int{1, 2, 3},
			[]uint{1, 2, 3},
			[...]int{1, 2, 3},
			[]float32{1, 2, 3},
			[]float64{1, 2, 3},
			Tag{61, Tag{17, []any{uint64(1), uint64(2), uint64(3)}}},
			RawTag{61, mustHexDecode("d183010203")},
		},
		wrongTypes: []reflect.Type{
			typeUint8,
			typeUint16,
			typeUint32,
			typeUint64,
			typeInt8,
			typeInt16,
			typeInt32,
			typeInt64,
			typeFloat32,
			typeFloat64,
			typeString,
			typeBool,
			typeStringSlice,
			typeMapStringInt,
			reflect.TypeOf([3]string{}),
			typeByteString,
			typeSimpleValue,
		},
	},
}

type unmarshalFloatTest struct {
	data               []byte
	wantInterfaceValue any
	wantValues         []any
	equalityThreshold  float64 // Not used for +inf, -inf, and NaN.
}

var unmarshalFloatWrongTypes = []reflect.Type{
	typeUint8,
	typeUint16,
	typeUint32,
	typeUint64,
	typeInt8,
	typeInt16,
	typeInt32,
	typeInt64,
	typeByteArray,
	typeByteSlice,
	typeString,
	typeBool,
	typeIntSlice,
	typeMapStringInt,
	typeTag,
	typeRawTag,
	typeBigInt,
	typeByteString,
	typeSimpleValue,
}

// unmarshalFloatTests includes test values for float16, float32, and float64.
// Note: the function for float16 to float32 conversion was tested with all
// 65536 values, which is too many to include here.
var unmarshalFloatTests = []unmarshalFloatTest{
	// CBOR test data are from https://tools.ietf.org/html/rfc7049#appendix-A.

	// float16
	{
		data:               mustHexDecode("f90000"),
		wantInterfaceValue: float64(0.0),
		wantValues:         []any{float32(0.0), float64(0.0)},
	},
	{
		data:               mustHexDecode("f98000"),
		wantInterfaceValue: float64(-0.0),                       //nolint:staticcheck // we know -0.0 is 0.0 in Go
		wantValues:         []any{float32(-0.0), float64(-0.0)}, //nolint:staticcheck // we know -0.0 is 0.0 in Go
	},
	{
		data:               mustHexDecode("f93c00"),
		wantInterfaceValue: float64(1.0),
		wantValues:         []any{float32(1.0), float64(1.0)},
	},
	{
		data:               mustHexDecode("f93e00"),
		wantInterfaceValue: float64(1.5),
		wantValues:         []any{float32(1.5), float64(1.5)},
	},
	{
		data:               mustHexDecode("f97bff"),
		wantInterfaceValue: float64(65504.0),
		wantValues:         []any{float32(65504.0), float64(65504.0)},
	},
	{
		data:               mustHexDecode("f90001"), // float16 subnormal value
		wantInterfaceValue: float64(5.960464477539063e-08),
		wantValues:         []any{float32(5.960464477539063e-08), float64(5.960464477539063e-08)},
		equalityThreshold:  1e-16,
	},
	{
		data:               mustHexDecode("f90400"),
		wantInterfaceValue: float64(6.103515625e-05),
		wantValues:         []any{float32(6.103515625e-05), float64(6.103515625e-05)},
		equalityThreshold:  1e-16,
	},
	{
		data:               mustHexDecode("f9c400"),
		wantInterfaceValue: float64(-4.0),
		wantValues:         []any{float32(-4.0), float64(-4.0)},
	},
	{
		data:               mustHexDecode("f97c00"),
		wantInterfaceValue: math.Inf(1),
		wantValues:         []any{math.Float32frombits(0x7f800000), math.Inf(1)},
	},
	{
		data:               mustHexDecode("f97e00"),
		wantInterfaceValue: math.NaN(),
		wantValues:         []any{math.Float32frombits(0x7fc00000), math.NaN()},
	},
	{
		data:               mustHexDecode("f9fc00"),
		wantInterfaceValue: math.Inf(-1),
		wantValues:         []any{math.Float32frombits(0xff800000), math.Inf(-1)},
	},

	// float32
	{
		data:               mustHexDecode("fa47c35000"),
		wantInterfaceValue: float64(100000.0),
		wantValues:         []any{float32(100000.0), float64(100000.0)},
	},
	{
		data:               mustHexDecode("fa7f7fffff"),
		wantInterfaceValue: float64(3.4028234663852886e+38),
		wantValues:         []any{float32(3.4028234663852886e+38), float64(3.4028234663852886e+38)},
		equalityThreshold:  1e-9,
	},
	{
		data:               mustHexDecode("fa7f800000"),
		wantInterfaceValue: math.Inf(1),
		wantValues:         []any{math.Float32frombits(0x7f800000), math.Inf(1)},
	},
	{
		data:               mustHexDecode("fa7fc00000"),
		wantInterfaceValue: math.NaN(),
		wantValues:         []any{math.Float32frombits(0x7fc00000), math.NaN()},
	},
	{
		data:               mustHexDecode("faff800000"),
		wantInterfaceValue: math.Inf(-1),
		wantValues:         []any{math.Float32frombits(0xff800000), math.Inf(-1)},
	},

	// float64
	{
		data:               mustHexDecode("fb3ff199999999999a"),
		wantInterfaceValue: float64(1.1),
		wantValues:         []any{float32(1.1), float64(1.1)},
	},
	{
		data:               mustHexDecode("fb7e37e43c8800759c"),
		wantInterfaceValue: float64(1.0e+300),
		wantValues:         []any{float64(1.0e+300)},
		equalityThreshold:  1e-9,
	},
	{
		data:               mustHexDecode("fbc010666666666666"),
		wantInterfaceValue: float64(-4.1),
		wantValues:         []any{float32(-4.1), float64(-4.1)},
	},
	{
		data:               mustHexDecode("fb7ff0000000000000"),
		wantInterfaceValue: math.Inf(1),
		wantValues:         []any{math.Float32frombits(0x7f800000), math.Inf(1)},
	},
	{
		data:               mustHexDecode("fb7ff8000000000000"),
		wantInterfaceValue: math.NaN(),
		wantValues:         []any{math.Float32frombits(0x7fc00000), math.NaN()},
	},
	{
		data:               mustHexDecode("fbfff0000000000000"),
		wantInterfaceValue: math.Inf(-1),
		wantValues:         []any{math.Float32frombits(0xff800000), math.Inf(-1)},
	},

	// float16 test data from https://en.wikipedia.org/wiki/Half-precision_floating-point_format
	{
		data:               mustHexDecode("f903ff"),
		wantInterfaceValue: float64(0.000060976),
		wantValues:         []any{float32(0.000060976), float64(0.000060976)},
		equalityThreshold:  1e-9,
	},
	{
		data:               mustHexDecode("f93bff"),
		wantInterfaceValue: float64(0.999511719),
		wantValues:         []any{float32(0.999511719), float64(0.999511719)},
		equalityThreshold:  1e-9,
	},
	{
		data:               mustHexDecode("f93c01"),
		wantInterfaceValue: float64(1.000976563),
		wantValues:         []any{float32(1.000976563), float64(1.000976563)},
		equalityThreshold:  1e-9,
	},
	{
		data:               mustHexDecode("f93555"),
		wantInterfaceValue: float64(0.333251953125),
		wantValues:         []any{float32(0.333251953125), float64(0.333251953125)},
		equalityThreshold:  1e-9,
	},

	// CBOR test data "canonNums" are from https://github.com/cbor-wg/cbor-test-vectors
	{
		data:               mustHexDecode("f9bd00"),
		wantInterfaceValue: float64(-1.25),
		wantValues:         []any{float32(-1.25), float64(-1.25)},
	},
	{
		data:               mustHexDecode("f93e00"),
		wantInterfaceValue: float64(1.5),
		wantValues:         []any{float32(1.5), float64(1.5)},
	},
	{
		data:               mustHexDecode("fb4024333333333333"),
		wantInterfaceValue: float64(10.1),
		wantValues:         []any{float32(10.1), float64(10.1)},
	},
	{
		data:               mustHexDecode("f90001"),
		wantInterfaceValue: float64(5.960464477539063e-8),
		wantValues:         []any{float32(5.960464477539063e-8), float64(5.960464477539063e-8)},
	},
	{
		data:               mustHexDecode("fa7f7fffff"),
		wantInterfaceValue: float64(3.4028234663852886e+38),
		wantValues:         []any{float32(3.4028234663852886e+38), float64(3.4028234663852886e+38)},
	},
	{
		data:               mustHexDecode("f90400"),
		wantInterfaceValue: float64(0.00006103515625),
		wantValues:         []any{float32(0.00006103515625), float64(0.00006103515625)},
	},
	{
		data:               mustHexDecode("f933ff"),
		wantInterfaceValue: float64(0.2498779296875),
		wantValues:         []any{float32(0.2498779296875), float64(0.2498779296875)},
	},
	{
		data:               mustHexDecode("fa33000000"),
		wantInterfaceValue: float64(2.9802322387695312e-8),
		wantValues:         []any{float32(2.9802322387695312e-8), float64(2.9802322387695312e-8)},
	},
	{
		data:               mustHexDecode("fa33333866"),
		wantInterfaceValue: float64(4.1727979294137185e-8),
		wantValues:         []any{float32(4.1727979294137185e-8), float64(4.1727979294137185e-8)},
	},
	{
		data:               mustHexDecode("fa37002000"),
		wantInterfaceValue: float64(0.000007636845111846924),
		wantValues:         []any{float32(0.000007636845111846924), float64(0.000007636845111846924)},
	},
}

const invalidUTF8ErrorMsg = "cbor: invalid UTF-8 string"

func mustHexDecode(s string) []byte {
	data, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return data
}

func mustBigInt(s string) big.Int {
	bi, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("failed to convert " + s + " to big.Int")
	}
	return *bi
}

func TestUnmarshalToEmptyInterface(t *testing.T) {
	for _, tc := range unmarshalTests {
		var v any
		if err := Unmarshal(tc.data, &v); err != nil {
			t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
			continue
		}
		compareNonFloats(t, tc.data, v, tc.wantInterfaceValue)
	}
}

func TestUnmarshalToRawMessage(t *testing.T) {
	for _, tc := range unmarshalTests {
		testUnmarshalToRawMessage(t, tc.data)
	}
}

func testUnmarshalToRawMessage(t *testing.T, data []byte) {
	cborNil := isCBORNil(data)

	// Decode to RawMessage
	var r RawMessage
	if err := Unmarshal(data, &r); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !bytes.Equal(r, data) {
		t.Errorf("Unmarshal(0x%x) returned RawMessage %v, want %v", data, r, data)
	}

	// Decode to *RawMesage (pr is nil)
	var pr *RawMessage
	if err := Unmarshal(data, &pr); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else {
		if cborNil {
			if pr != nil {
				t.Errorf("Unmarshal(0x%x) returned RawMessage %v, want nil *RawMessage", data, *pr)
			}
		} else {
			if !bytes.Equal(*pr, data) {
				t.Errorf("Unmarshal(0x%x) returned RawMessage %v, want %v", data, *pr, data)
			}
		}
	}

	// Decode to *RawMessage (pr is not nil)
	var ir RawMessage
	pr = &ir
	if err := Unmarshal(data, &pr); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else {
		if cborNil {
			if pr != nil {
				t.Errorf("Unmarshal(0x%x) returned RawMessage %v, want nil *RawMessage", data, *pr)
			}
		} else {
			if !bytes.Equal(*pr, data) {
				t.Errorf("Unmarshal(0x%x) returned RawMessage %v, want %v", data, *pr, data)
			}
		}
	}
}

func TestUnmarshalToCompatibleTypes(t *testing.T) {
	for _, tc := range unmarshalTests {
		for _, wantValue := range tc.wantValues {
			testUnmarshalToCompatibleType(t, tc.data, wantValue, func(gotValue any) {
				compareNonFloats(t, tc.data, gotValue, wantValue)
			})
		}
	}
}

func testUnmarshalToCompatibleType(t *testing.T, data []byte, wantValue any, compare func(gotValue any)) {
	var rv reflect.Value

	cborNil := isCBORNil(data)
	wantType := reflect.TypeOf(wantValue)

	// Decode to wantType, same as:
	//     var v wantType
	//     Unmarshal(tc.data, &v)

	rv = reflect.New(wantType)
	if err := Unmarshal(data, rv.Interface()); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		return
	}
	compare(rv.Elem().Interface())

	// Decode to *wantType (pv is nil), same as:
	//     var pv *wantType
	//     Unmarshal(tc.data, &pv)

	rv = reflect.New(reflect.PointerTo(wantType))
	if err := Unmarshal(data, rv.Interface()); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		return
	}
	if cborNil {
		if !rv.Elem().IsNil() {
			t.Errorf("Unmarshal(0x%x) = %v (%T), want nil", data, rv.Elem().Interface(), rv.Elem().Interface())
		}
	} else {
		compare(rv.Elem().Elem().Interface())
	}

	// Decode to *wantType (pv is not nil), same as:
	//     var v wantType
	//     pv := &v
	//     Unmarshal(tc.data, &pv)

	irv := reflect.New(wantType)
	rv = reflect.New(reflect.PointerTo(wantType))
	rv.Elem().Set(irv)
	if err := Unmarshal(data, rv.Interface()); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		return
	}
	if cborNil {
		if !rv.Elem().IsNil() {
			t.Errorf("Unmarshal(0x%x) = %v (%T), want nil", data, rv.Elem().Interface(), rv.Elem().Interface())
		}
	} else {
		compare(rv.Elem().Elem().Interface())
	}
}

func TestUnmarshalToIncompatibleTypes(t *testing.T) {
	for _, tc := range unmarshalTests {
		for _, wrongType := range tc.wrongTypes {
			testUnmarshalToIncompatibleType(t, tc.data, wrongType)
		}
	}
}

func testUnmarshalToIncompatibleType(t *testing.T, data []byte, wrongType reflect.Type) {
	var rv reflect.Value

	// Decode to wrongType, same as:
	//     var v wrongType
	//     Unmarshal(tc.data, &v)

	rv = reflect.New(wrongType)
	if err := Unmarshal(data, rv.Interface()); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	}

	// Decode to *wrongType (pv is nil), same as:
	//     var pv *wrongType
	//     Unmarshal(tc.data, &pv)

	rv = reflect.New(reflect.PointerTo(wrongType))
	if err := Unmarshal(data, rv.Interface()); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	}

	// Decode to *wrongType (pv is not nil), same as:
	//     var v wrongType
	//     pv := &v
	//     Unmarshal(tc.data, &pv)

	irv := reflect.New(wrongType)
	rv = reflect.New(reflect.PointerTo(wrongType))
	rv.Elem().Set(irv)

	if err := Unmarshal(data, rv.Interface()); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	}
}

func compareNonFloats(t *testing.T, data []byte, got any, want any) {
	switch tm := want.(type) {
	case time.Time:
		if vt, ok := got.(time.Time); !ok || !tm.Equal(vt) {
			t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, got, got, want, want)
		}

	default:
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, got, got, want, want)
		}
	}
}

func TestUnmarshalFloatToEmptyInterface(t *testing.T) {
	for _, tc := range unmarshalFloatTests {
		var v any
		if err := Unmarshal(tc.data, &v); err != nil {
			t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
			continue
		}
		compareFloats(t, tc.data, v, tc.wantInterfaceValue, tc.equalityThreshold)
	}
}

func TestUnmarshalFloatToRawMessage(t *testing.T) {
	for _, tc := range unmarshalFloatTests {
		testUnmarshalToRawMessage(t, tc.data)
	}
}

func TestUnmarshalFloatToCompatibleTypes(t *testing.T) {
	for _, tc := range unmarshalFloatTests {
		for _, wantValue := range tc.wantValues {
			testUnmarshalToCompatibleType(t, tc.data, wantValue, func(gotValue any) {
				compareFloats(t, tc.data, gotValue, wantValue, tc.equalityThreshold)
			})
		}
	}
}

func TestUnmarshalFloatToIncompatibleTypes(t *testing.T) {
	for _, tc := range unmarshalFloatTests {
		for _, wrongType := range unmarshalFloatWrongTypes {
			testUnmarshalToIncompatibleType(t, tc.data, wrongType)
		}
	}
}

func compareFloats(t *testing.T, data []byte, got any, want any, equalityThreshold float64) {
	var gotFloat64, wantFloat64 float64

	switch want := want.(type) {
	case float32:
		f, ok := got.(float32)
		if !ok {
			t.Errorf("Unmarshal(0x%x) returned value of type %T, want float32", data, f)
			return
		}
		gotFloat64, wantFloat64 = float64(f), float64(want)

	case float64:
		f, ok := got.(float64)
		if !ok {
			t.Errorf("Unmarshal(0x%x) returned value of type %T, want float64", data, f)
			return
		}
		gotFloat64, wantFloat64 = f, want
	}

	switch {
	case math.IsNaN(wantFloat64):
		if !math.IsNaN(gotFloat64) {
			t.Errorf("Unmarshal(0x%x) = %f, want NaN", data, gotFloat64)
		}

	case math.IsInf(wantFloat64, 0):
		if gotFloat64 != wantFloat64 {
			t.Errorf("Unmarshal(0x%x) = %f, want %f", data, gotFloat64, wantFloat64)
		}

	default:
		if math.Abs(gotFloat64-wantFloat64) > equalityThreshold {
			t.Errorf("Unmarshal(0x%x) = %.18f, want %.18f, diff %.18f > threshold %.18f", data, gotFloat64, wantFloat64, math.Abs(gotFloat64-wantFloat64), equalityThreshold)
		}
	}
}

func TestNegIntOverflow(t *testing.T) {
	data := mustHexDecode("3bffffffffffffffff") // -18446744073709551616

	// Decode CBOR neg int that overflows Go int64 to empty interface
	var v1 any
	wantObj := mustBigInt("-18446744073709551616")
	if err := Unmarshal(data, &v1); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %+v", data, err)
	} else if !reflect.DeepEqual(v1, wantObj) {
		t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", data, v1, v1, wantObj, wantObj)
	}

	// Decode CBOR neg int that overflows Go int64 to big.Int
	var v2 big.Int
	if err := Unmarshal(data, &v2); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %+v", data, err)
	} else if !reflect.DeepEqual(v2, wantObj) {
		t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", data, v2, v2, wantObj, wantObj)
	}

	// Decode CBOR neg int that overflows Go int64 to int64
	var v3 int64
	if err := Unmarshal(data, &v3); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), "cannot unmarshal")
	}
}

func TestUnmarshalIntoPtrPrimitives(t *testing.T) {
	cborDataInt := mustHexDecode("1818")                          // 24
	cborDataString := mustHexDecode("7f657374726561646d696e67ff") // "streaming"

	const wantInt = 24
	const wantString = "streaming"

	var p1 *int
	var p2 *string
	var p3 *RawMessage

	var i int
	pi := &i
	ppi := &pi

	var s string
	ps := &s
	pps := &ps

	var r RawMessage
	pr := &r
	ppr := &pr

	// Unmarshal CBOR integer into a non-nil pointer.
	if err := Unmarshal(cborDataInt, &ppi); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", cborDataInt, err)
	} else if i != wantInt {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %d", cborDataInt, i, i, wantInt)
	}
	// Unmarshal CBOR integer into a nil pointer.
	if err := Unmarshal(cborDataInt, &p1); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", cborDataInt, err)
	} else if *p1 != wantInt {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %d", cborDataInt, *pi, pi, wantInt)
	}

	// Unmarshal CBOR string into a non-nil pointer.
	if err := Unmarshal(cborDataString, &pps); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", cborDataString, err)
	} else if s != wantString {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", cborDataString, s, s, wantString)
	}
	// Unmarshal CBOR string into a nil pointer.
	if err := Unmarshal(cborDataString, &p2); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", cborDataString, err)
	} else if *p2 != wantString {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", cborDataString, *p2, p2, wantString)
	}

	// Unmarshal CBOR string into a non-nil RawMessage.
	if err := Unmarshal(cborDataString, &ppr); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", cborDataString, err)
	} else if !bytes.Equal(r, cborDataString) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", cborDataString, r, r, cborDataString)
	}
	// Unmarshal CBOR string into a nil pointer to RawMessage.
	if err := Unmarshal(cborDataString, &p3); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", cborDataString, err)
	} else if !bytes.Equal(*p3, cborDataString) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", cborDataString, *p3, p3, cborDataString)
	}
}

func TestUnmarshalIntoPtrArrayPtrElem(t *testing.T) {
	data := mustHexDecode("83010203") // []int{1, 2, 3}

	n1, n2, n3 := 1, 2, 3

	wantArray := []*int{&n1, &n2, &n3}

	var p *[]*int

	var slc []*int
	pslc := &slc
	ppslc := &pslc

	// Unmarshal CBOR array into a non-nil pointer.
	if err := Unmarshal(data, &ppslc); err != nil {
		t.Errorf("Unmarshal(0x%x, %s) returned error %v", data, reflect.TypeOf(ppslc), err)
	} else if !reflect.DeepEqual(slc, wantArray) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", data, slc, slc, wantArray)
	}
	// Unmarshal CBOR array into a nil pointer.
	if err := Unmarshal(data, &p); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(*p, wantArray) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", data, *p, p, wantArray)
	}
}

func TestUnmarshalIntoPtrMapPtrElem(t *testing.T) {
	data := mustHexDecode("a201020304") // {1: 2, 3: 4}

	n1, n2, n3, n4 := 1, 2, 3, 4

	wantMap := map[int]*int{n1: &n2, n3: &n4}

	var p *map[int]*int

	var m map[int]*int
	pm := &m
	ppm := &pm

	// Unmarshal CBOR map into a non-nil pointer.
	if err := Unmarshal(data, &ppm); err != nil {
		t.Errorf("Unmarshal(0x%x, %s) returned error %v", data, reflect.TypeOf(ppm), err)
	} else if !reflect.DeepEqual(m, wantMap) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", data, m, m, wantMap)
	}
	// Unmarshal CBOR map into a nil pointer.
	if err := Unmarshal(data, &p); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(*p, wantMap) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", data, *p, p, wantMap)
	}
}

func TestUnmarshalIntoPtrStructPtrElem(t *testing.T) {
	type s1 struct {
		A *string `cbor:"a"`
		B *string `cbor:"b"`
		C *string `cbor:"c"`
		D *string `cbor:"d"`
		E *string `cbor:"e"`
	}

	data := mustHexDecode("a56161614161626142616361436164614461656145") // map[string]string{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E"}

	a, b, c, d, e := "A", "B", "C", "D", "E"
	wantObj := s1{A: &a, B: &b, C: &c, D: &d, E: &e}

	var p *s1

	var s s1
	ps := &s
	pps := &ps

	// Unmarshal CBOR map into a non-nil pointer.
	if err := Unmarshal(data, &pps); err != nil {
		t.Errorf("Unmarshal(0x%x, %s) returned error %v", data, reflect.TypeOf(pps), err)
	} else if !reflect.DeepEqual(s, wantObj) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", data, s, s, wantObj)
	}
	// Unmarshal CBOR map into a nil pointer.
	if err := Unmarshal(data, &p); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(*p, wantObj) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v", data, *p, p, wantObj)
	}
}

func TestUnmarshalIntoArray(t *testing.T) {
	data := mustHexDecode("83010203") // []int{1, 2, 3}

	// Unmarshal CBOR array into Go array.
	var arr1 [3]int
	if err := Unmarshal(data, &arr1); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if arr1 != [3]int{1, 2, 3} {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want [3]int{1, 2, 3}", data, arr1, arr1)
	}

	// Unmarshal CBOR array into Go array with more elements.
	var arr2 [5]int
	if err := Unmarshal(data, &arr2); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if arr2 != [5]int{1, 2, 3, 0, 0} {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want [5]int{1, 2, 3, 0, 0}", data, arr2, arr2)
	}

	// Unmarshal CBOR array into Go array with less elements.
	var arr3 [1]int
	if err := Unmarshal(data, &arr3); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if arr3 != [1]int{1} {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want [0]int{1}", data, arr3, arr3)
	}
}

type nilUnmarshaler string

func (s *nilUnmarshaler) UnmarshalCBOR(data []byte) error {
	if len(data) == 1 && (data[0] == 0xf6 || data[0] == 0xf7) {
		*s = "null"
	} else {
		*s = nilUnmarshaler(data)
	}
	return nil
}

func TestUnmarshalNil(t *testing.T) {
	type T struct {
		I int
	}

	data := [][]byte{mustHexDecode("f6"), mustHexDecode("f7")} // CBOR null and undefined values

	testCases := []struct {
		name      string
		value     any
		wantValue any
	}{
		// Unmarshaling CBOR null to the following types is a no-op.
		{
			name:      "bool",
			value:     true,
			wantValue: true,
		},
		{
			name:      "int",
			value:     int(-1),
			wantValue: int(-1),
		},
		{
			name:      "int8",
			value:     int8(-2),
			wantValue: int8(-2),
		},
		{
			name:      "int16",
			value:     int16(-3),
			wantValue: int16(-3),
		},
		{
			name:      "int32",
			value:     int32(-4),
			wantValue: int32(-4),
		},
		{
			name:      "int64",
			value:     int64(-5),
			wantValue: int64(-5),
		},
		{
			name:      "uint",
			value:     uint(1),
			wantValue: uint(1),
		},
		{
			name:      "uint8",
			value:     uint8(2),
			wantValue: uint8(2),
		},
		{
			name:      "uint16",
			value:     uint16(3),
			wantValue: uint16(3),
		},
		{
			name:      "uint32",
			value:     uint32(4),
			wantValue: uint32(4),
		},
		{
			name:      "uint64",
			value:     uint64(5),
			wantValue: uint64(5),
		},
		{
			name:      "float32",
			value:     float32(1.23),
			wantValue: float32(1.23),
		},
		{
			name:      "float64",
			value:     float64(4.56),
			wantValue: float64(4.56),
		},
		{
			name:      "string",
			value:     "hello",
			wantValue: "hello",
		},
		{
			name:      "array",
			value:     [3]int{1, 2, 3},
			wantValue: [3]int{1, 2, 3},
		},

		// Unmarshaling CBOR null to slice/map sets Go values to nil.
		{
			name:      "[]byte",
			value:     []byte{1, 2, 3},
			wantValue: []byte(nil),
		},
		{
			name:      "slice",
			value:     []string{"hello", "world"},
			wantValue: []string(nil),
		},
		{
			name:      "map",
			value:     map[string]bool{"hello": true, "goodbye": false},
			wantValue: map[string]bool(nil),
		},

		// Unmarshaling CBOR null to ByteString (string wrapper for []byte) resets ByteString to empty string.
		{
			name:      "cbor.ByteString",
			value:     ByteString("\x01\x02\x03"),
			wantValue: ByteString(""),
		},

		// Unmarshaling CBOR null to time.Time is a no-op.
		{
			name:      "time.Time",
			value:     time.Date(2020, time.January, 2, 3, 4, 5, 6, time.UTC),
			wantValue: time.Date(2020, time.January, 2, 3, 4, 5, 6, time.UTC),
		},

		// Unmarshaling CBOR null to big.Int is a no-op.
		{
			name:      "big.Int",
			value:     mustBigInt("123"),
			wantValue: mustBigInt("123"),
		},

		// Unmarshaling CBOR null to user defined struct types is a no-op.
		{
			name:      "user defined struct",
			value:     T{I: 123},
			wantValue: T{I: 123},
		},

		// Unmarshaling CBOR null to cbor.Tag and cbor.RawTag is a no-op.
		{
			name:      "cbor.RawTag",
			value:     RawTag{123, []byte{4, 5, 6}},
			wantValue: RawTag{123, []byte{4, 5, 6}},
		},
		{
			name:      "cbor.Tag",
			value:     Tag{123, "hello world"},
			wantValue: Tag{123, "hello world"},
		},

		// Unmarshaling to cbor.RawMessage sets cbor.RawMessage to raw CBOR bytes (0xf6 or 0xf7).
		// It's tested in TestUnmarshal().

		// Unmarshaling to types implementing cbor.BinaryUnmarshaler is a no-op.
		{
			name:      "cbor.BinaryUnmarshaler",
			value:     number(456),
			wantValue: number(456),
		},

		// When unmarshaling to types implementing cbor.Unmarshaler,
		{
			name:      "cbor.Unmarshaler",
			value:     nilUnmarshaler("hello world"),
			wantValue: nilUnmarshaler("null"),
		},
	}

	// Unmarshaling to values of specified Go types.
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, data := range data {
				v := reflect.New(reflect.TypeOf(tc.value))
				v.Elem().Set(reflect.ValueOf(tc.value))

				if err := Unmarshal(data, v.Interface()); err != nil {
					t.Errorf("Unmarshal(0x%x) to %T returned error %v", data, v.Elem().Interface(), err)
				} else if !reflect.DeepEqual(v.Elem().Interface(), tc.wantValue) {
					t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v.Elem().Interface(), v.Elem().Interface(), tc.wantValue, tc.wantValue)
				}
			}
		})
	}
}

var invalidUnmarshalTests = []struct {
	name         string
	v            any
	wantErrorMsg string
}{
	{
		name:         "unmarshal into nil interface{}",
		v:            nil,
		wantErrorMsg: "cbor: Unmarshal(nil)",
	},
	{
		name:         "unmarshal into non-pointer value",
		v:            5,
		wantErrorMsg: "cbor: Unmarshal(non-pointer int)",
	},
	{
		name:         "unmarshal into nil pointer",
		v:            (*int)(nil),
		wantErrorMsg: "cbor: Unmarshal(nil *int)",
	},
}

func TestInvalidUnmarshal(t *testing.T) {
	data := []byte{0x00}

	for _, tc := range invalidUnmarshalTests {
		t.Run(tc.name, func(t *testing.T) {
			err := Unmarshal(data, tc.v)
			if err == nil {
				t.Errorf("Unmarshal(0x%x, %v) didn't return an error", data, tc.v)
			} else if _, ok := err.(*InvalidUnmarshalError); !ok {
				t.Errorf("Unmarshal(0x%x, %v) error type %T, want *InvalidUnmarshalError", data, tc.v, err)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Unmarshal(0x%x, %v) error %q, want %q", data, tc.v, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

var invalidCBORUnmarshalTests = []struct {
	name         string
	data         []byte
	wantErrorMsg string
}{
	{
		name:         "Nil data",
		data:         []byte(nil),
		wantErrorMsg: "EOF",
	},
	{
		name:         "Empty data",
		data:         []byte{},
		wantErrorMsg: "EOF",
	},
	{
		name:         "Tag number not followed by tag content",
		data:         []byte{0xc0},
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length byte string with tagged chunk",
		data:         mustHexDecode("5fc64401020304ff"),
		wantErrorMsg: "cbor: wrong element type tag for indefinite-length byte string",
	},
	{
		name:         "Indefinite length text string with tagged chunk",
		data:         mustHexDecode("7fc06161ff"),
		wantErrorMsg: "cbor: wrong element type tag for indefinite-length UTF-8 text string",
	},
	{
		name:         "Indefinite length strings with truncated text string",
		data:         mustHexDecode("7f61"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Invalid nested tag number",
		data:         mustHexDecode("d864dc1a514b67b0"),
		wantErrorMsg: "cbor: invalid additional information 28 for type tag",
	},
	// Data from 7049bis G.1
	// Premature end of the input
	{
		name:         "End of input in a head",
		data:         mustHexDecode("18"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("19"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("1a"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("1b"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("1901"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("1a0102"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("1b01020304050607"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("38"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("58"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("78"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("98"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("9a01ff00"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("b8"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("d8"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("f8"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("f900"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("fa0000"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "End of input in a head",
		data:         mustHexDecode("fb000000"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length strings with short data",
		data:         mustHexDecode("41"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length strings with short data",
		data:         mustHexDecode("61"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length strings with short data",
		data:         mustHexDecode("5affffffff00"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length strings with short data",
		data:         mustHexDecode("5bffffffffffffffff010203"),
		wantErrorMsg: "cbor: byte string length 18446744073709551615 is too large, causing integer overflow",
	},
	{
		name:         "Definite length strings with short data",
		data:         mustHexDecode("7affffffff00"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length strings with short data",
		data:         mustHexDecode("7b7fffffffffffffff010203"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length maps and arrays not closed with enough items",
		data:         mustHexDecode("81"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length maps and arrays not closed with enough items",
		data:         mustHexDecode("818181818181818181"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length maps and arrays not closed with enough items",
		data:         mustHexDecode("8200"),
		wantErrorMsg: "unexpected EOF",
	},

	{
		name:         "Definite length maps and arrays not closed with enough items",
		data:         mustHexDecode("a1"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length maps and arrays not closed with enough items",
		data:         mustHexDecode("a20102"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length maps and arrays not closed with enough items",
		data:         mustHexDecode("a100"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Definite length maps and arrays not closed with enough items",
		data:         mustHexDecode("a2000000"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length strings not closed by a break stop code",
		data:         mustHexDecode("5f4100"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length strings not closed by a break stop code",
		data:         mustHexDecode("7f6100"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length maps and arrays not closed by a break stop code",
		data:         mustHexDecode("9f"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length maps and arrays not closed by a break stop code",
		data:         mustHexDecode("9f0102"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length maps and arrays not closed by a break stop code",
		data:         mustHexDecode("bf"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length maps and arrays not closed by a break stop code",
		data:         mustHexDecode("bf01020102"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length maps and arrays not closed by a break stop code",
		data:         mustHexDecode("819f"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length maps and arrays not closed by a break stop code",
		data:         mustHexDecode("9f8000"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length maps and arrays not closed by a break stop code",
		data:         mustHexDecode("9f9f9f9f9fffffffff"),
		wantErrorMsg: "unexpected EOF",
	},
	{
		name:         "Indefinite length maps and arrays not closed by a break stop code",
		data:         mustHexDecode("9f819f819f9fffffff"),
		wantErrorMsg: "unexpected EOF",
	},
	// Five subkinds of well-formedness error kind 3 (syntax error)
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("3e"),
		wantErrorMsg: "cbor: invalid additional information 30 for type negative integer",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("5c"),
		wantErrorMsg: "cbor: invalid additional information 28 for type byte string",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("5d"),
		wantErrorMsg: "cbor: invalid additional information 29 for type byte string",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("5e"),
		wantErrorMsg: "cbor: invalid additional information 30 for type byte string",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("7c"),
		wantErrorMsg: "cbor: invalid additional information 28 for type UTF-8 text string",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("7d"),
		wantErrorMsg: "cbor: invalid additional information 29 for type UTF-8 text string",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("7e"),
		wantErrorMsg: "cbor: invalid additional information 30 for type UTF-8 text string",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("9c"),
		wantErrorMsg: "cbor: invalid additional information 28 for type array",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("9d"),
		wantErrorMsg: "cbor: invalid additional information 29 for type array",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("9e"),
		wantErrorMsg: "cbor: invalid additional information 30 for type array",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("bc"),
		wantErrorMsg: "cbor: invalid additional information 28 for type map",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("bd"),
		wantErrorMsg: "cbor: invalid additional information 29 for type map",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("be"),
		wantErrorMsg: "cbor: invalid additional information 30 for type map",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("dc"),
		wantErrorMsg: "cbor: invalid additional information 28 for type tag",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("dd"),
		wantErrorMsg: "cbor: invalid additional information 29 for type tag",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("de"),
		wantErrorMsg: "cbor: invalid additional information 30 for type tag",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("fc"),
		wantErrorMsg: "cbor: invalid additional information 28 for type primitives",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("fd"),
		wantErrorMsg: "cbor: invalid additional information 29 for type primitives",
	},
	{
		name:         "Reserved additional information values",
		data:         mustHexDecode("fe"),
		wantErrorMsg: "cbor: invalid additional information 30 for type primitives",
	},
	{
		name:         "Reserved two-byte encodings of simple types",
		data:         mustHexDecode("f800"),
		wantErrorMsg: "cbor: invalid simple value 0 for type primitives",
	},
	{
		name:         "Reserved two-byte encodings of simple types",
		data:         mustHexDecode("f801"),
		wantErrorMsg: "cbor: invalid simple value 1 for type primitives",
	},
	{
		name:         "Reserved two-byte encodings of simple types",
		data:         mustHexDecode("f818"),
		wantErrorMsg: "cbor: invalid simple value 24 for type primitives",
	},
	{
		name:         "Reserved two-byte encodings of simple types",
		data:         mustHexDecode("f81f"),
		wantErrorMsg: "cbor: invalid simple value 31 for type primitives",
	},
	{
		name:         "Indefinite length string chunks not of the correct type",
		data:         mustHexDecode("5f00ff"),
		wantErrorMsg: "cbor: wrong element type positive integer for indefinite-length byte string",
	},
	{
		name:         "Indefinite length string chunks not of the correct type",
		data:         mustHexDecode("5f21ff"),
		wantErrorMsg: "cbor: wrong element type negative integer for indefinite-length byte string",
	},
	{
		name:         "Indefinite length string chunks not of the correct type",
		data:         mustHexDecode("5f6100ff"),
		wantErrorMsg: "cbor: wrong element type UTF-8 text string for indefinite-length byte string",
	},
	{
		name:         "Indefinite length string chunks not of the correct type",
		data:         mustHexDecode("5f80ff"),
		wantErrorMsg: "cbor: wrong element type array for indefinite-length byte string",
	},
	{
		name:         "Indefinite length string chunks not of the correct type",
		data:         mustHexDecode("5fa0ff"),
		wantErrorMsg: "cbor: wrong element type map for indefinite-length byte string",
	},
	{
		name:         "Indefinite length string chunks not of the correct type",
		data:         mustHexDecode("5fc000ff"),
		wantErrorMsg: "cbor: wrong element type tag for indefinite-length byte string",
	},
	{
		name:         "Indefinite length string chunks not of the correct type",
		data:         mustHexDecode("5fe0ff"),
		wantErrorMsg: "cbor: wrong element type primitives for indefinite-length byte string",
	},
	{
		name:         "Indefinite length string chunks not of the correct type",
		data:         mustHexDecode("7f4100ff"),
		wantErrorMsg: "cbor: wrong element type byte string for indefinite-length UTF-8 text string",
	},
	{
		name:         "Indefinite length string chunks not definite length",
		data:         mustHexDecode("5f5f4100ffff"),
		wantErrorMsg: "cbor: indefinite-length byte string chunk is not definite-length",
	},
	{
		name:         "Indefinite length string chunks not definite length",
		data:         mustHexDecode("7f7f6100ffff"),
		wantErrorMsg: "cbor: indefinite-length UTF-8 text string chunk is not definite-length",
	},
	{
		name:         "Break occurring on its own outside of an indefinite length item",
		data:         mustHexDecode("ff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break occurring in a definite length array or map or a tag",
		data:         mustHexDecode("81ff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break occurring in a definite length array or map or a tag",
		data:         mustHexDecode("8200ff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break occurring in a definite length array or map or a tag",
		data:         mustHexDecode("a1ff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break occurring in a definite length array or map or a tag",
		data:         mustHexDecode("a1ff00"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break occurring in a definite length array or map or a tag",
		data:         mustHexDecode("a100ff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break occurring in a definite length array or map or a tag",
		data:         mustHexDecode("a20000ff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break occurring in a definite length array or map or a tag",
		data:         mustHexDecode("9f81ff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break occurring in a definite length array or map or a tag",
		data:         mustHexDecode("9f829f819f9fffffffff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break in indefinite length map would lead to odd number of items (break in a value position)",
		data:         mustHexDecode("bf00ff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Break in indefinite length map would lead to odd number of items (break in a value position)",
		data:         mustHexDecode("bf000000ff"),
		wantErrorMsg: "cbor: unexpected \"break\" code",
	},
	{
		name:         "Major type 0 with additional information 31",
		data:         mustHexDecode("1f"),
		wantErrorMsg: "cbor: invalid additional information 31 for type positive integer",
	},
	{
		name:         "Major type 1 with additional information 31",
		data:         mustHexDecode("3f"),
		wantErrorMsg: "cbor: invalid additional information 31 for type negative integer",
	},
	{
		name:         "Major type 6 with additional information 31",
		data:         mustHexDecode("df"),
		wantErrorMsg: "cbor: invalid additional information 31 for type tag",
	},
	// Extraneous data
	{
		name:         "Two ints",
		data:         mustHexDecode("0001"),
		wantErrorMsg: "cbor: 1 bytes of extraneous data starting at index 1",
	},
	{
		name:         "Two arrays",
		data:         mustHexDecode("830102038104"),
		wantErrorMsg: "cbor: 2 bytes of extraneous data starting at index 4",
	},
	{
		name:         "Int and partial array",
		data:         mustHexDecode("00830102"),
		wantErrorMsg: "cbor: 3 bytes of extraneous data starting at index 1",
	},
}

func TestInvalidCBORUnmarshal(t *testing.T) {
	for _, tc := range invalidCBORUnmarshalTests {
		t.Run(tc.name, func(t *testing.T) {
			var i any
			err := Unmarshal(tc.data, &i)
			if err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error", tc.data)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Unmarshal(0x%x) error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestValidUTF8String(t *testing.T) {
	dmRejectInvalidUTF8, err := DecOptions{UTF8: UTF8RejectInvalid}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}
	dmDecodeInvalidUTF8, err := DecOptions{UTF8: UTF8DecodeInvalid}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}

	testCases := []struct {
		name    string
		data    []byte
		dm      DecMode
		wantObj any
	}{
		{
			name:    "with UTF8RejectInvalid",
			data:    mustHexDecode("6973747265616d696e67"),
			dm:      dmRejectInvalidUTF8,
			wantObj: "streaming",
		},
		{
			name:    "with UTF8DecodeInvalid",
			data:    mustHexDecode("6973747265616d696e67"),
			dm:      dmDecodeInvalidUTF8,
			wantObj: "streaming",
		},
		{
			name:    "indef length with UTF8RejectInvalid",
			data:    mustHexDecode("7f657374726561646d696e67ff"),
			dm:      dmRejectInvalidUTF8,
			wantObj: "streaming",
		},
		{
			name:    "indef length with UTF8DecodeInvalid",
			data:    mustHexDecode("7f657374726561646d696e67ff"),
			dm:      dmDecodeInvalidUTF8,
			wantObj: "streaming",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Decode to empty interface
			var i any
			err = tc.dm.Unmarshal(tc.data, &i)
			if err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
			}
			if !reflect.DeepEqual(i, tc.wantObj) {
				t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, i, i, tc.wantObj, tc.wantObj)
			}

			// Decode to string
			var v string
			err = tc.dm.Unmarshal(tc.data, &v)
			if err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
			}
			if !reflect.DeepEqual(v, tc.wantObj) {
				t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, v, v, tc.wantObj, tc.wantObj)
			}
		})
	}
}

func TestInvalidUTF8String(t *testing.T) {
	dmRejectInvalidUTF8, err := DecOptions{UTF8: UTF8RejectInvalid}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}
	dmDecodeInvalidUTF8, err := DecOptions{UTF8: UTF8DecodeInvalid}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}

	testCases := []struct {
		name         string
		data         []byte
		dm           DecMode
		wantObj      any
		wantErrorMsg string
	}{
		{
			name:         "with UTF8RejectInvalid",
			data:         mustHexDecode("61fe"),
			dm:           dmRejectInvalidUTF8,
			wantErrorMsg: invalidUTF8ErrorMsg,
		},
		{
			name:    "with UTF8DecodeInvalid",
			data:    mustHexDecode("61fe"),
			dm:      dmDecodeInvalidUTF8,
			wantObj: string([]byte{0xfe}),
		},
		{
			name:         "indef length with UTF8RejectInvalid",
			data:         mustHexDecode("7f62e6b061b4ff"),
			dm:           dmRejectInvalidUTF8,
			wantErrorMsg: invalidUTF8ErrorMsg,
		},
		{
			name:    "indef length with UTF8DecodeInvalid",
			data:    mustHexDecode("7f62e6b061b4ff"),
			dm:      dmDecodeInvalidUTF8,
			wantObj: string([]byte{0xe6, 0xb0, 0xb4}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Decode to empty interface
			var v any
			err = tc.dm.Unmarshal(tc.data, &v)
			if tc.wantErrorMsg != "" {
				if err == nil {
					t.Errorf("Unmarshal(0x%x) didn't return error", tc.data)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
				}
				if !reflect.DeepEqual(v, tc.wantObj) {
					t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, v, v, tc.wantObj, tc.wantObj)
				}
			}

			// Decode to string
			var s string
			err = tc.dm.Unmarshal(tc.data, &s)
			if tc.wantErrorMsg != "" {
				if err == nil {
					t.Errorf("Unmarshal(0x%x) didn't return error", tc.data)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
				}
				if !reflect.DeepEqual(s, tc.wantObj) {
					t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, s, s, tc.wantObj, tc.wantObj)
				}
			}
		})
	}

	// Test decoding of mixed invalid text string and valid text string
	// with UTF8RejectInvalid option (default)
	data := mustHexDecode("7f62e6b061b4ff7f657374726561646d696e67ff")
	dec := NewDecoder(bytes.NewReader(data))
	var s string
	if err := dec.Decode(&s); err == nil {
		t.Errorf("Decode() didn't return an error")
	} else if s != "" {
		t.Errorf("Decode() returned %q, want %q", s, "")
	}
	if err := dec.Decode(&s); err != nil {
		t.Errorf("Decode() returned error %v", err)
	} else if s != "streaming" {
		t.Errorf("Decode() returned %q, want %q", s, "streaming")
	}

	// Test decoding of mixed invalid text string and valid text string
	// with UTF8DecodeInvalid option
	dec = dmDecodeInvalidUTF8.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&s); err != nil {
		t.Errorf("Decode() returned error %q", err)
	} else if s != string([]byte{0xe6, 0xb0, 0xb4}) {
		t.Errorf("Decode() returned %q, want %q", s, string([]byte{0xe6, 0xb0, 0xb4}))
	}
	if err := dec.Decode(&s); err != nil {
		t.Errorf("Decode() returned error %v", err)
	} else if s != "streaming" {
		t.Errorf("Decode() returned %q, want %q", s, "streaming")
	}
}

func TestUnmarshalStruct(t *testing.T) {
	want := outer{
		IntField:          123,
		FloatField:        100000.0,
		BoolField:         true,
		StringField:       "test",
		ByteStringField:   []byte{1, 3, 5},
		ArrayField:        []string{"hello", "world"},
		MapField:          map[string]bool{"morning": true, "afternoon": false},
		NestedStructField: &inner{X: 1000, Y: 1000000},
		unexportedField:   0,
	}

	tests := []struct {
		name string
		data []byte
		want any
	}{
		{
			name: "case-insensitive field name match",
			data: mustHexDecode("a868696e746669656c64187b6a666c6f61746669656c64fa47c3500069626f6f6c6669656c64f56b537472696e674669656c6464746573746f42797465537472696e674669656c64430103056a41727261794669656c64826568656c6c6f65776f726c64684d61704669656c64a2676d6f726e696e67f56961667465726e6f6f6ef4714e65737465645374727563744669656c64a261581903e861591a000f4240"),
			want: want,
		},
		{
			name: "exact field name match",
			data: mustHexDecode("a868496e744669656c64187b6a466c6f61744669656c64fa47c3500069426f6f6c4669656c64f56b537472696e674669656c6464746573746f42797465537472696e674669656c64430103056a41727261794669656c64826568656c6c6f65776f726c64684d61704669656c64a2676d6f726e696e67f56961667465726e6f6f6ef4714e65737465645374727563744669656c64a261581903e861591a000f4240"),
			want: want,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var v outer
			if err := Unmarshal(tc.data, &v); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
			} else if !reflect.DeepEqual(v, want) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.data, v, v, want, want)
			}
		})
	}
}

func TestUnmarshalStructError1(t *testing.T) {
	type outer2 struct {
		IntField          int
		FloatField        float32
		BoolField         bool
		StringField       string
		ByteStringField   []byte
		ArrayField        []int // wrong type
		MapField          map[string]bool
		NestedStructField map[int]string // wrong type
		unexportedField   int64
	}
	want := outer2{
		IntField:          123,
		FloatField:        100000.0,
		BoolField:         true,
		StringField:       "test",
		ByteStringField:   []byte{1, 3, 5},
		ArrayField:        []int{0, 0},
		MapField:          map[string]bool{"morning": true, "afternoon": false},
		NestedStructField: map[int]string{},
		unexportedField:   0,
	}

	data := mustHexDecode("a868496e744669656c64187b6a466c6f61744669656c64fa47c3500069426f6f6c4669656c64f56b537472696e674669656c6464746573746f42797465537472696e674669656c64430103056a41727261794669656c64826568656c6c6f65776f726c64684d61704669656c64a2676d6f726e696e67f56961667465726e6f6f6ef4714e65737465645374727563744669656c64a261581903e861591a000f4240")
	wantCBORType := "UTF-8 text string"
	wantGoType := "int"
	wantStructFieldName := "cbor.outer2.ArrayField"
	wantErrorMsg := "cannot unmarshal UTF-8 text string into Go struct field cbor.outer2.ArrayField of type int"

	var v outer2
	if err := Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else {
		if typeError, ok := err.(*UnmarshalTypeError); !ok {
			t.Errorf("Unmarshal(0x%x) returned wrong type of error %T, want (*UnmarshalTypeError)", data, err)
		} else {
			if typeError.CBORType != wantCBORType {
				t.Errorf("Unmarshal(0x%x) returned (*UnmarshalTypeError).CBORType %s, want %s", data, typeError.CBORType, wantCBORType)
			}
			if typeError.GoType != wantGoType {
				t.Errorf("Unmarshal(0x%x) returned (*UnmarshalTypeError).GoType %s, want %s", data, typeError.GoType, wantGoType)
			}
			if typeError.StructFieldName != wantStructFieldName {
				t.Errorf("Unmarshal(0x%x) returned (*UnmarshalTypeError).StructFieldName %s, want %s", data, typeError.StructFieldName, wantStructFieldName)
			}
			if !strings.Contains(err.Error(), wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
			}
		}
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, want, want)
	}
}

func TestUnmarshalStructError2(t *testing.T) {
	// Unmarshal integer and invalid UTF8 string as field name into struct
	type strc struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
	}
	want := strc{
		A: "A",
	}

	// Unmarshal returns first error encountered, which is *UnmarshalTypeError (failed to unmarshal int into Go string)
	data := mustHexDecode("a3fa47c35000026161614161fe6142") // {100000.0:2, "a":"A", 0xfe: B}
	wantCBORType := "primitives"
	wantGoType := "string"
	wantErrorMsg := "cannot unmarshal primitives into Go value of type string"

	v := strc{}
	if err := Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else {
		if typeError, ok := err.(*UnmarshalTypeError); !ok {
			t.Errorf("Unmarshal(0x%x) returned wrong type of error %T, want (*UnmarshalTypeError)", data, err)
		} else {
			if typeError.CBORType != wantCBORType {
				t.Errorf("Unmarshal(0x%x) returned (*UnmarshalTypeError).CBORType %s, want %s", data, typeError.CBORType, wantCBORType)
			}
			if typeError.GoType != wantGoType {
				t.Errorf("Unmarshal(0x%x) returned (*UnmarshalTypeError).GoType %s, want %s", data, typeError.GoType, wantGoType)
			}
			if !strings.Contains(err.Error(), wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
			}
		}
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, want, want)
	}

	// Unmarshal returns first error encountered, which is *cbor.SemanticError (invalid UTF8 string)
	data = mustHexDecode("a361fe6142010261616141") // {0xfe: B, 1:2, "a":"A"}
	v = strc{}
	if err := Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else {
		if _, ok := err.(*SemanticError); !ok {
			t.Errorf("Unmarshal(0x%x) returned wrong type of error %T, want (*SemanticError)", data, err)
		} else if err.Error() != invalidUTF8ErrorMsg {
			t.Errorf("Unmarshal(0x%x) returned error %q, want error %q", data, err.Error(), invalidUTF8ErrorMsg)
		}
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, want, want)
	}

	// Unmarshal returns first error encountered, which is *cbor.SemanticError (invalid UTF8 string)
	data = mustHexDecode("a3616261fe010261616141") // {"b": 0xfe, 1:2, "a":"A"}
	v = strc{}
	if err := Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else {
		if _, ok := err.(*SemanticError); !ok {
			t.Errorf("Unmarshal(0x%x) returned wrong type of error %T, want (*SemanticError)", data, err)
		} else if err.Error() != invalidUTF8ErrorMsg {
			t.Errorf("Unmarshal(0x%x) returned error %q, want error %q", data, err.Error(), invalidUTF8ErrorMsg)
		}
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, want, want)
	}
}

func TestUnmarshalPrefilledArray(t *testing.T) {
	prefilledArr := []int{1, 2, 3, 4, 5}
	want := []int{10, 11, 3, 4, 5}
	data := mustHexDecode("820a0b") // []int{10, 11}
	if err := Unmarshal(data, &prefilledArr); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if len(prefilledArr) != 2 || cap(prefilledArr) != 5 {
		t.Errorf("Unmarshal(0x%x) = %v (len %d, cap %d), want len == 2, cap == 5", data, prefilledArr, len(prefilledArr), cap(prefilledArr))
	}
	if !reflect.DeepEqual(prefilledArr[:cap(prefilledArr)], want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, prefilledArr, prefilledArr, want, want)
	}

	data = mustHexDecode("80") // empty array
	if err := Unmarshal(data, &prefilledArr); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if len(prefilledArr) != 0 || cap(prefilledArr) != 0 {
		t.Errorf("Unmarshal(0x%x) = %v (len %d, cap %d), want len == 0, cap == 0", data, prefilledArr, len(prefilledArr), cap(prefilledArr))
	}
}

func TestUnmarshalPrefilledMap(t *testing.T) {
	prefilledMap := map[string]string{"key": "value", "a": "1"}
	want := map[string]string{"key": "value", "a": "A", "b": "B", "c": "C", "d": "D", "e": "E"}
	data := mustHexDecode("a56161614161626142616361436164614461656145") // map[string]string{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E"}
	if err := Unmarshal(data, &prefilledMap); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(prefilledMap, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, prefilledMap, prefilledMap, want, want)
	}

	prefilledMap = map[string]string{"key": "value"}
	want = map[string]string{"key": "value"}
	data = mustHexDecode("a0") // map[string]string{}
	if err := Unmarshal(data, &prefilledMap); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(prefilledMap, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, prefilledMap, prefilledMap, want, want)
	}
}

func TestUnmarshalPrefilledStruct(t *testing.T) {
	type s struct {
		a int
		B []int
		C bool
	}
	prefilledStruct := s{a: 100, B: []int{200, 300, 400, 500}, C: true}
	want := s{a: 100, B: []int{2, 3}, C: true}
	data := mustHexDecode("a26161016162820203") // map[string]interface{} {"a": 1, "b": []int{2, 3}}
	if err := Unmarshal(data, &prefilledStruct); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(prefilledStruct, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, prefilledStruct, prefilledStruct, want, want)
	}
	if len(prefilledStruct.B) != 2 || cap(prefilledStruct.B) != 4 {
		t.Errorf("Unmarshal(0x%x) = %v (len %d, cap %d), want len == 2, cap == 5", data, prefilledStruct.B, len(prefilledStruct.B), cap(prefilledStruct.B))
	}
	if !reflect.DeepEqual(prefilledStruct.B[:cap(prefilledStruct.B)], []int{2, 3, 400, 500}) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, prefilledStruct.B, prefilledStruct.B, []int{2, 3, 400, 500}, []int{2, 3, 400, 500})
	}
}

func TestStructFieldNil(t *testing.T) {
	type TestStruct struct {
		I   int
		PI  *int
		PPI **int
	}
	var struc TestStruct
	data, err := Marshal(struc)
	if err != nil {
		t.Fatalf("Marshal(%+v) returned error %v", struc, err)
	}
	var struc2 TestStruct
	err = Unmarshal(data, &struc2)
	if err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(struc, struc2) {
		t.Errorf("Unmarshal(0x%x) returned %+v, want %+v", data, struc2, struc)
	}
}

func TestLengthOverflowsInt(t *testing.T) {
	// Data is generating by go-fuzz.
	// string/slice/map length in uint64 cast to int causes integer overflow.
	data := [][]byte{
		mustHexDecode("bbcf30303030303030cfd697829782"),
		mustHexDecode("5bcf30303030303030cfd697829782"),
	}
	wantErrorMsg := "is too large"
	for _, data := range data {
		var intf any
		if err := Unmarshal(data, &intf); err == nil {
			t.Errorf("Unmarshal(0x%x) didn't return an error, want error containing substring %q", data, wantErrorMsg)
		} else if !strings.Contains(err.Error(), wantErrorMsg) {
			t.Errorf("Unmarshal(0x%x) returned error %q, want error containing substring %q", data, err.Error(), wantErrorMsg)
		}
	}
}

func TestMapKeyUnhashable(t *testing.T) {
	testCases := []struct {
		name         string
		data         []byte
		wantErrorMsg string
	}{
		{
			name:         "slice as map key",
			data:         mustHexDecode("bf8030ff"),
			wantErrorMsg: "cbor: invalid map key type: []interface {}",
		}, // {[]: -17}
		{
			name:         "slice as map key",
			data:         mustHexDecode("a1813030"),
			wantErrorMsg: "cbor: invalid map key type: []interface {}",
		}, // {[-17]: -17}
		{
			name:         "slice as map key",
			data:         mustHexDecode("bfd1a388f730303030303030303030303030ff"),
			wantErrorMsg: "cbor: invalid map key type: []interface {}",
		}, // {17({[undefined, -17, -17, -17, -17, -17, -17, -17]: -17, -17: -17}): -17}}
		{
			name:         "map as map key",
			data:         mustHexDecode("bf30a1a030ff"),
			wantErrorMsg: "cbor: invalid map key type: map",
		}, // {-17: {{}: -17}}, empty map as map key
		{
			name:         "map as map key",
			data:         mustHexDecode("bfb0303030303030303030303030303030303030303030303030303030303030303030ff"),
			wantErrorMsg: "cbor: invalid map key type: map",
		}, // {{-17: -17}: -17}, map as key
		{
			name:         "big.Int as map key",
			data:         mustHexDecode("a13bbd3030303030303030"),
			wantErrorMsg: "cbor: invalid map key type: big.Int",
		}, // {-13632449055575519281: -17}
		{
			name:         "tagged big.Int as map key",
			data:         mustHexDecode("a1c24901000000000000000030"),
			wantErrorMsg: "cbor: invalid map key type: big.Int",
		}, // {18446744073709551616: -17}
		{
			name:         "tagged big.Int as map key",
			data:         mustHexDecode("a1c34901000000000000000030"),
			wantErrorMsg: "cbor: invalid map key type: big.Int",
		}, // {-18446744073709551617: -17}
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			if err := Unmarshal(tc.data, &v); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", tc.data, tc.wantErrorMsg)
			} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
			if _, ok := v.(map[any]any); ok {
				var v map[any]any
				if err := Unmarshal(tc.data, &v); err == nil {
					t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", tc.data, tc.wantErrorMsg)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
				}
			}
		})
	}
}

func TestMapKeyNaN(t *testing.T) {
	// Data is generating by go-fuzz.
	data := mustHexDecode("b0303030303030303030303030303030303038303030faffff30303030303030303030303030") // {-17: -17, NaN: -17}
	var intf any
	if err := Unmarshal(data, &intf); err != nil {
		t.Fatalf("Unmarshal(0x%x) returned error %v", data, err)
	}
	em, err := EncOptions{Sort: SortCanonical}.EncMode()
	if err != nil {
		t.Errorf("EncMode() returned an error %v", err)
	}
	if _, err := em.Marshal(intf); err != nil {
		t.Errorf("Marshal(%v) returned error %v", intf, err)
	}
}

func TestUnmarshalUndefinedElement(t *testing.T) {
	// Data is generating by go-fuzz.
	data := mustHexDecode("bfd1a388f730303030303030303030303030ff") // {17({[undefined, -17, -17, -17, -17, -17, -17, -17]: -17, -17: -17}): -17}
	var intf any
	wantErrorMsg := "invalid map key type"
	if err := Unmarshal(data, &intf); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error, want error containing substring %q", data, wantErrorMsg)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing substring %q", data, err.Error(), wantErrorMsg)
	}
}

func TestMapKeyNil(t *testing.T) {
	testData := [][]byte{
		mustHexDecode("a1f630"), // {null: -17}
	}
	want := map[any]any{nil: int64(-17)}
	for _, data := range testData {
		var intf any
		if err := Unmarshal(data, &intf); err != nil {
			t.Fatalf("Unmarshal(0x%x) returned error %v", data, err)
		} else if !reflect.DeepEqual(intf, want) {
			t.Errorf("Unmarshal(0x%x) returned %+v, want %+v", data, intf, want)
		}
		if _, err := Marshal(intf); err != nil {
			t.Errorf("Marshal(%v) returned error %v", intf, err)
		}

		var v map[any]any
		if err := Unmarshal(data, &v); err != nil {
			t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		} else if !reflect.DeepEqual(v, want) {
			t.Errorf("Unmarshal(0x%x) returned %+v, want %+v", data, v, want)
		}
		if _, err := Marshal(v); err != nil {
			t.Errorf("Marshal(%v) returned error %v", v, err)
		}
	}
}

func TestDecodeTime(t *testing.T) {
	unmodified := time.Now()

	testCases := []struct {
		name            string
		cborRFC3339Time []byte
		cborUnixTime    []byte
		wantTime        time.Time
	}{
		// Decoding untagged CBOR null/defined to time.Time is no-op.  See TestUnmarshalNil.
		{
			name:            "null within unrecognized tag", // no-op in DecTagIgnored
			cborRFC3339Time: mustHexDecode("dadeadbeeff6"),
			cborUnixTime:    mustHexDecode("dadeadbeeff6"),
			wantTime:        unmodified,
		},
		{
			name:            "undefined within unrecognized tag", // no-op in DecTagIgnored
			cborRFC3339Time: mustHexDecode("dadeadbeeff7"),
			cborUnixTime:    mustHexDecode("dadeadbeeff7"),
			wantTime:        unmodified,
		},
		{
			name:            "NaN",
			cborRFC3339Time: mustHexDecode("f97e00"),
			cborUnixTime:    mustHexDecode("f97e00"),
			wantTime:        time.Time{},
		},
		{
			name:            "positive infinity",
			cborRFC3339Time: mustHexDecode("f97c00"),
			cborUnixTime:    mustHexDecode("f97c00"),
			wantTime:        time.Time{},
		},
		{
			name:            "negative infinity",
			cborRFC3339Time: mustHexDecode("f9fc00"),
			cborUnixTime:    mustHexDecode("f9fc00"),
			wantTime:        time.Time{},
		},
		{
			name:            "time without fractional seconds", // positive integer
			cborRFC3339Time: mustHexDecode("74323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("1a514b67b0"),
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00Z"),
		},
		{
			name:            "time with fractional seconds", // float
			cborRFC3339Time: mustHexDecode("7819313937302d30312d30315432313a34363a34302d30363a3030"),
			cborUnixTime:    mustHexDecode("fa47c35000"),
			wantTime:        parseTime(time.RFC3339Nano, "1970-01-01T21:46:40-06:00"),
		},
		{
			name:            "time with fractional seconds", // float
			cborRFC3339Time: mustHexDecode("76323031332d30332d32315432303a30343a30302e355a"),
			cborUnixTime:    mustHexDecode("fb41d452d9ec200000"),
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00.5Z"),
		},
		{
			name:            "time before January 1, 1970 UTC without fractional seconds", // negative integer
			cborRFC3339Time: mustHexDecode("74313936392d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("3a0177f2cf"),
			wantTime:        parseTime(time.RFC3339Nano, "1969-03-21T20:04:00Z"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tm := unmodified
			if err := Unmarshal(tc.cborRFC3339Time, &tm); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.cborRFC3339Time, err)
			} else if !tc.wantTime.Equal(tm) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.cborRFC3339Time, tm, tm, tc.wantTime, tc.wantTime)
			}
			tm = unmodified
			if err := Unmarshal(tc.cborUnixTime, &tm); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.cborUnixTime, err)
			} else if !tc.wantTime.Equal(tm) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.cborUnixTime, tm, tm, tc.wantTime, tc.wantTime)
			}
		})
	}
}

func TestDecodeTimeWithTag(t *testing.T) {
	testCases := []struct {
		name            string
		cborRFC3339Time []byte
		cborUnixTime    []byte
		wantTime        time.Time
	}{
		{
			name:            "time without fractional seconds", // positive integer
			cborRFC3339Time: mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("c11a514b67b0"),
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00Z"),
		},
		{
			name:            "time with fractional seconds", // float
			cborRFC3339Time: mustHexDecode("c076323031332d30332d32315432303a30343a30302e355a"),
			cborUnixTime:    mustHexDecode("c1fb41d452d9ec200000"),
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00.5Z"),
		},
		{
			name:            "time before January 1, 1970 UTC without fractional seconds", // negative integer
			cborRFC3339Time: mustHexDecode("c074313936392d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("c13a0177f2cf"),
			wantTime:        parseTime(time.RFC3339Nano, "1969-03-21T20:04:00Z"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tm := time.Now()
			if err := Unmarshal(tc.cborRFC3339Time, &tm); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.cborRFC3339Time, err)
			} else if !tc.wantTime.Equal(tm) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.cborRFC3339Time, tm, tm, tc.wantTime, tc.wantTime)
			}
			tm = time.Now()
			if err := Unmarshal(tc.cborUnixTime, &tm); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.cborUnixTime, err)
			} else if !tc.wantTime.Equal(tm) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.cborUnixTime, tm, tm, tc.wantTime, tc.wantTime)
			}

			var v any
			if err := Unmarshal(tc.cborRFC3339Time, &v); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.cborRFC3339Time, err)
			} else if tm, ok := v.(time.Time); !ok || !tc.wantTime.Equal(tm) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.cborRFC3339Time, v, v, tc.wantTime, tc.wantTime)
			}
			v = nil
			if err := Unmarshal(tc.cborUnixTime, &v); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.cborUnixTime, err)
			} else if tm, ok := v.(time.Time); !ok || !tc.wantTime.Equal(tm) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.cborUnixTime, v, v, tc.wantTime, tc.wantTime)
			}
		})
	}
}

func TestDecodeTimeError(t *testing.T) {
	testCases := []struct {
		name         string
		opts         DecOptions
		data         []byte
		wantErrorMsg string
	}{
		{
			name:         "invalid RFC3339 time string",
			data:         mustHexDecode("7f657374726561646d696e67ff"),
			wantErrorMsg: "cbor: cannot set streaming for time.Time",
		},
		{
			name:         "byte string data cannot be decoded into time.Time",
			data:         mustHexDecode("4f013030303030303030e03031ed3030"),
			wantErrorMsg: "cbor: cannot unmarshal byte string into Go value of type time.Time",
		},
		{
			name:         "bool cannot be decoded into time.Time",
			data:         mustHexDecode("f4"),
			wantErrorMsg: "cbor: cannot unmarshal primitives into Go value of type time.Time",
		},
		{
			name:         "invalid UTF-8 string",
			data:         mustHexDecode("7f62e6b061b4ff"),
			wantErrorMsg: "cbor: invalid UTF-8 string",
		},
		{
			name:         "negative integer overflow",
			data:         mustHexDecode("3bffffffffffffffff"),
			wantErrorMsg: "cbor: cannot unmarshal negative integer into Go value of type time.Time",
		},
		{
			name: "untagged byte string content cannot be decoded into time.Time with DefaultByteStringType string",
			opts: DecOptions{
				TimeTag:               DecTagOptional,
				DefaultByteStringType: reflect.TypeOf(""),
			},
			data:         mustHexDecode("54323031332d30332d32315432303a30343a30305a"),
			wantErrorMsg: "cbor: cannot unmarshal byte string into Go value of type time.Time",
		},
		{
			name:         "time tag is validated when enclosed in unrecognized tag",
			data:         mustHexDecode("dadeadbeefc001"),
			wantErrorMsg: "cbor: tag number 0 must be followed by text string, got positive integer",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}
			tm := time.Now()
			if err := dm.Unmarshal(tc.data, &tm); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error, want error msg %q", tc.data, tc.wantErrorMsg)
			} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecodeInvalidTagTime(t *testing.T) {
	typeTimeSlice := reflect.TypeOf([]time.Time{})

	testCases := []struct {
		name          string
		data          []byte
		decodeToTypes []reflect.Type
		wantErrorMsg  string
	}{
		{
			name:          "Tag 0 with invalid RFC3339 time string",
			data:          mustHexDecode("c07f657374726561646d696e67ff"),
			decodeToTypes: []reflect.Type{typeIntf, typeTime},
			wantErrorMsg:  "cbor: cannot set streaming for time.Time",
		},
		{
			name:          "Tag 0 with invalid UTF-8 string",
			data:          mustHexDecode("c07f62e6b061b4ff"),
			decodeToTypes: []reflect.Type{typeIntf, typeTime},
			wantErrorMsg:  "cbor: invalid UTF-8 string",
		},
		{
			name:          "Tag 0 with integer content",
			data:          mustHexDecode("c01a514b67b0"),
			decodeToTypes: []reflect.Type{typeIntf, typeTime},
			wantErrorMsg:  "cbor: tag number 0 must be followed by text string, got positive integer",
		},
		{
			name:          "Tag 0 with byte string content",
			data:          mustHexDecode("c04f013030303030303030e03031ed3030"),
			decodeToTypes: []reflect.Type{typeIntf, typeTime},
			wantErrorMsg:  "cbor: tag number 0 must be followed by text string, got byte string",
		},
		{
			name:          "Tag 0 with integer content as array element",
			data:          mustHexDecode("81c01a514b67b0"),
			decodeToTypes: []reflect.Type{typeIntf, typeTimeSlice},
			wantErrorMsg:  "cbor: tag number 0 must be followed by text string, got positive integer",
		},
		{
			name:          "Tag 1 with negative integer overflow",
			data:          mustHexDecode("c13bffffffffffffffff"),
			decodeToTypes: []reflect.Type{typeIntf, typeTime},
			wantErrorMsg:  "cbor: cannot unmarshal negative integer into Go value of type time.Time (-18446744073709551616 overflows Go's int64)",
		},
		{
			name:          "Tag 1 with string content",
			data:          mustHexDecode("c174323031332d30332d32315432303a30343a30305a"),
			decodeToTypes: []reflect.Type{typeIntf, typeTime},
			wantErrorMsg:  "cbor: tag number 1 must be followed by integer or floating-point number, got UTF-8 text string",
		},
		{
			name:          "Tag 1 with simple value",
			data:          mustHexDecode("d801f6"), // 1(null)
			decodeToTypes: []reflect.Type{typeIntf, typeTime},
			wantErrorMsg:  "cbor: tag number 1 must be followed by integer or floating-point number, got primitive",
		},
		{
			name:          "Tag 1 with string content as array element",
			data:          mustHexDecode("81c174323031332d30332d32315432303a30343a30305a"),
			decodeToTypes: []reflect.Type{typeIntf, typeTimeSlice},
			wantErrorMsg:  "cbor: tag number 1 must be followed by integer or floating-point number, got UTF-8 text string",
		},
	}
	dm, _ := DecOptions{TimeTag: DecTagOptional}.DecMode()
	for _, tc := range testCases {
		for _, decodeToType := range tc.decodeToTypes {
			t.Run(tc.name+" decode to "+decodeToType.String(), func(t *testing.T) {
				v := reflect.New(decodeToType)
				if err := dm.Unmarshal(tc.data, v.Interface()); err == nil {
					t.Errorf("Unmarshal(0x%x) didn't return error, want error msg %q", tc.data, tc.wantErrorMsg)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err, tc.wantErrorMsg)
				}
			})
		}
	}
}

func TestDecodeTag0Error(t *testing.T) {
	data := mustHexDecode("c01a514b67b0") // 0(1363896240)
	wantErrorMsg := "cbor: tag number 0 must be followed by text string, got positive integer"

	timeTagIgnoredDM, _ := DecOptions{TimeTag: DecTagIgnored}.DecMode()
	timeTagOptionalDM, _ := DecOptions{TimeTag: DecTagOptional}.DecMode()
	timeTagRequiredDM, _ := DecOptions{TimeTag: DecTagRequired}.DecMode()

	testCases := []struct {
		name string
		dm   DecMode
	}{
		{name: "DecTagIgnored", dm: timeTagIgnoredDM},
		{name: "DecTagOptional", dm: timeTagOptionalDM},
		{name: "DecTagRequired", dm: timeTagRequiredDM},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Decode to interface{}
			var v any
			if err := tc.dm.Unmarshal(data, &v); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return error, want error msg %q", data, wantErrorMsg)
			} else if !strings.Contains(err.Error(), wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err, wantErrorMsg)
			}

			// Decode to time.Time
			var tm time.Time
			if err := tc.dm.Unmarshal(data, &tm); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return error, want error msg %q", data, wantErrorMsg)
			} else if !strings.Contains(err.Error(), wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err, wantErrorMsg)
			}

			// Decode to uint64
			var ui uint64
			if err := tc.dm.Unmarshal(data, &ui); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return error, want error msg %q", data, wantErrorMsg)
			} else if !strings.Contains(err.Error(), wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err, wantErrorMsg)
			}
		})
	}
}

func TestDecodeTag1Error(t *testing.T) {
	data := mustHexDecode("c174323031332d30332d32315432303a30343a30305a") // 1("2013-03-21T20:04:00Z")
	wantErrorMsg := "cbor: tag number 1 must be followed by integer or floating-point number, got UTF-8 text string"

	timeTagIgnoredDM, _ := DecOptions{TimeTag: DecTagIgnored}.DecMode()
	timeTagOptionalDM, _ := DecOptions{TimeTag: DecTagOptional}.DecMode()
	timeTagRequiredDM, _ := DecOptions{TimeTag: DecTagRequired}.DecMode()

	testCases := []struct {
		name string
		dm   DecMode
	}{
		{name: "DecTagIgnored", dm: timeTagIgnoredDM},
		{name: "DecTagOptional", dm: timeTagOptionalDM},
		{name: "DecTagRequired", dm: timeTagRequiredDM},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Decode to interface{}
			var v any
			if err := tc.dm.Unmarshal(data, &v); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return error, want error msg %q", data, wantErrorMsg)
			} else if !strings.Contains(err.Error(), wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err, wantErrorMsg)
			}

			// Decode to time.Time
			var tm time.Time
			if err := tc.dm.Unmarshal(data, &tm); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return error, want error msg %q", data, wantErrorMsg)
			} else if !strings.Contains(err.Error(), wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err, wantErrorMsg)
			}

			// Decode to string
			var s string
			if err := tc.dm.Unmarshal(data, &s); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return error, want error msg %q", data, wantErrorMsg)
			} else if !strings.Contains(err.Error(), wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err, wantErrorMsg)
			}
		})
	}
}

func TestDecodeTimeStreaming(t *testing.T) {
	// Decoder decodes from mixed invalid and valid time.
	testCases := []struct {
		data         []byte
		wantErrorMsg string
		wantObj      time.Time
	}{
		{
			data:         mustHexDecode("c07f62e6b061b4ff"),
			wantErrorMsg: "cbor: invalid UTF-8 string",
		},
		{
			data:    mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			wantObj: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			data:         mustHexDecode("c01a514b67b0"),
			wantErrorMsg: "cbor: tag number 0 must be followed by text string, got positive integer",
		},
		{
			data:    mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			wantObj: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			data:         mustHexDecode("c13bffffffffffffffff"),
			wantErrorMsg: "cbor: cannot unmarshal negative integer into Go value of type time.Time (-18446744073709551616 overflows Go's int64)",
		},
		{
			data:    mustHexDecode("c11a514b67b0"),
			wantObj: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			data:         mustHexDecode("c174323031332d30332d32315432303a30343a30305a"),
			wantErrorMsg: "tag number 1 must be followed by integer or floating-point number, got UTF-8 text string",
		},
		{
			data:    mustHexDecode("c11a514b67b0"),
			wantObj: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
	}
	// Data is a mixed stream of valid and invalid time data
	var data []byte
	for _, tc := range testCases {
		data = append(data, tc.data...)
	}
	dm, _ := DecOptions{TimeTag: DecTagOptional}.DecMode()
	dec := dm.NewDecoder(bytes.NewReader(data))
	for _, tc := range testCases {
		var v any
		err := dec.Decode(&v)
		if tc.wantErrorMsg != "" {
			if err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return error, want error msg %q", tc.data, tc.wantErrorMsg)
			} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error msg %q, want %q", tc.data, err, tc.wantErrorMsg)
			}
		} else {
			tm, ok := v.(time.Time)
			if !ok {
				t.Errorf("Unmarshal(0x%x) returned %s (%T), want time.Time", tc.data, v, v)
			}
			if !tc.wantObj.Equal(tm) {
				t.Errorf("Unmarshal(0x%x) returned %s, want %s", tc.data, tm, tc.wantObj)
			}
		}
	}
	dec = dm.NewDecoder(bytes.NewReader(data))
	for _, tc := range testCases {
		var tm time.Time
		err := dec.Decode(&tm)
		if tc.wantErrorMsg != "" {
			if err == nil {
				t.Errorf("Unmarshal(0x%x) did't return error, want error msg %q", tc.data, tc.wantErrorMsg)
			} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error msg %q, want %q", tc.data, err, tc.wantErrorMsg)
			}
		} else {
			if !tc.wantObj.Equal(tm) {
				t.Errorf("Unmarshal(0x%x) returned %s, want %s", tc.data, tm, tc.wantObj)
			}
		}
	}
}

func TestDecTimeTagOption(t *testing.T) {
	timeTagIgnoredDecMode, _ := DecOptions{TimeTag: DecTagIgnored}.DecMode()
	timeTagOptionalDecMode, _ := DecOptions{TimeTag: DecTagOptional}.DecMode()
	timeTagRequiredDecMode, _ := DecOptions{TimeTag: DecTagRequired}.DecMode()

	testCases := []struct {
		name            string
		cborRFC3339Time []byte
		cborUnixTime    []byte
		decMode         DecMode
		wantTime        time.Time
		wantErrorMsg    string
	}{
		// not-tagged time CBOR data
		{
			name:            "not-tagged data with DecTagIgnored option",
			cborRFC3339Time: mustHexDecode("74323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("1a514b67b0"),
			decMode:         timeTagIgnoredDecMode,
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00Z"),
		},
		{
			name:            "not-tagged data with timeTagOptionalDecMode option",
			cborRFC3339Time: mustHexDecode("74323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("1a514b67b0"),
			decMode:         timeTagOptionalDecMode,
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00Z"),
		},
		{
			name:            "not-tagged data with timeTagRequiredDecMode option",
			cborRFC3339Time: mustHexDecode("74323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("1a514b67b0"),
			decMode:         timeTagRequiredDecMode,
			wantErrorMsg:    "expect CBOR tag value",
		},
		// tagged time CBOR data
		{
			name:            "tagged data with timeTagIgnoredDecMode option",
			cborRFC3339Time: mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("c11a514b67b0"),
			decMode:         timeTagIgnoredDecMode,
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00Z"),
		},
		{
			name:            "tagged data with timeTagOptionalDecMode option",
			cborRFC3339Time: mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("c11a514b67b0"),
			decMode:         timeTagOptionalDecMode,
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00Z"),
		},
		{
			name:            "tagged data with timeTagRequiredDecMode option",
			cborRFC3339Time: mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("c11a514b67b0"),
			decMode:         timeTagRequiredDecMode,
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00Z"),
		},
		// mis-tagged time CBOR data
		{
			name:            "mis-tagged data with timeTagIgnoredDecMode option",
			cborRFC3339Time: mustHexDecode("c8c974323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("c8c91a514b67b0"),
			decMode:         timeTagIgnoredDecMode,
			wantTime:        parseTime(time.RFC3339Nano, "2013-03-21T20:04:00Z"),
		},
		{
			name:            "mis-tagged data with timeTagOptionalDecMode option",
			cborRFC3339Time: mustHexDecode("c8c974323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("c8c91a514b67b0"),
			decMode:         timeTagOptionalDecMode,
			wantErrorMsg:    "cbor: wrong tag number for time.Time, got 8, expect 0 or 1",
		},
		{
			name:            "mis-tagged data with timeTagRequiredDecMode option",
			cborRFC3339Time: mustHexDecode("c8c974323031332d30332d32315432303a30343a30305a"),
			cborUnixTime:    mustHexDecode("c8c91a514b67b0"),
			decMode:         timeTagRequiredDecMode,
			wantErrorMsg:    "cbor: wrong tag number for time.Time, got 8, expect 0 or 1",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tm := time.Now()
			err := tc.decMode.Unmarshal(tc.cborRFC3339Time, &tm)
			if tc.wantErrorMsg != "" {
				if err == nil {
					t.Errorf("Unmarshal(0x%x) didn't return error", tc.cborRFC3339Time)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", tc.cborRFC3339Time, err.Error(), tc.wantErrorMsg)
				}
			} else {
				if !tc.wantTime.Equal(tm) {
					t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.cborRFC3339Time, tm, tm, tc.wantTime, tc.wantTime)
				}
			}

			tm = time.Now()
			err = tc.decMode.Unmarshal(tc.cborUnixTime, &tm)
			if tc.wantErrorMsg != "" {
				if err == nil {
					t.Errorf("Unmarshal(0x%x) didn't return error", tc.cborRFC3339Time)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", tc.cborRFC3339Time, err.Error(), tc.wantErrorMsg)
				}
			} else {
				if !tc.wantTime.Equal(tm) {
					t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.cborRFC3339Time, tm, tm, tc.wantTime, tc.wantTime)
				}
			}
		})
	}
}

func TestUnmarshalStructTag1(t *testing.T) {
	type strc struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
	}
	want := strc{
		A: "A",
		B: "B",
		C: "C",
	}
	data := mustHexDecode("a3616161416162614261636143") // {"a":"A", "b":"B", "c":"C"}

	var v strc
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, want, want)
	}
}

func TestUnmarshalStructTag2(t *testing.T) {
	type strc struct {
		A string `json:"a"`
		B string `json:"b"`
		C string `json:"c"`
	}
	want := strc{
		A: "A",
		B: "B",
		C: "C",
	}
	data := mustHexDecode("a3616161416162614261636143") // {"a":"A", "b":"B", "c":"C"}

	var v strc
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, want, want)
	}
}

func TestUnmarshalStructTag3(t *testing.T) {
	type strc struct {
		A string `json:"x" cbor:"a"`
		B string `json:"y" cbor:"b"`
		C string `json:"z"`
	}
	want := strc{
		A: "A",
		B: "B",
		C: "C",
	}
	data := mustHexDecode("a36161614161626142617a6143") // {"a":"A", "b":"B", "z":"C"}

	var v strc
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, v, v, want, want)
	}
}

func TestUnmarshalStructTag4(t *testing.T) {
	type strc struct {
		A string `json:"x" cbor:"a"`
		B string `json:"y" cbor:"b"`
		C string `json:"-"`
	}
	want := strc{
		A: "A",
		B: "B",
	}
	data := mustHexDecode("a3616161416162614261636143") // {"a":"A", "b":"B", "c":"C"}

	var v strc
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, v, v, want, want)
	}
}

type number uint64

func (n number) MarshalBinary() (data []byte, err error) {
	if n == 0 {
		return []byte{}, nil
	}
	data = make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(n))
	return
}

func (n *number) UnmarshalBinary(data []byte) (err error) {
	if len(data) == 0 {
		*n = 0
		return nil
	}
	if len(data) != 8 {
		return errors.New("number:UnmarshalBinary: invalid length")
	}
	*n = number(binary.BigEndian.Uint64(data))
	return
}

type stru struct {
	a, b, c string
}

func (s *stru) MarshalBinary() ([]byte, error) {
	if s.a == "" && s.b == "" && s.c == "" {
		return []byte{}, nil
	}
	return fmt.Appendf(nil, "%s,%s,%s", s.a, s.b, s.c), nil
}

func (s *stru) UnmarshalBinary(data []byte) (err error) {
	if len(data) == 0 {
		s.a, s.b, s.c = "", "", ""
		return nil
	}
	ss := strings.Split(string(data), ",")
	if len(ss) != 3 {
		return errors.New("stru:UnmarshalBinary: invalid element count")
	}
	s.a, s.b, s.c = ss[0], ss[1], ss[2]
	return
}

type marshalBinaryError string

func (n marshalBinaryError) MarshalBinary() (data []byte, err error) {
	return nil, errors.New(string(n))
}

func TestBinaryMarshalerUnmarshaler(t *testing.T) {
	testCases := []roundTripTest{
		{
			name:         "primitive obj",
			obj:          number(1234567890),
			wantCborData: mustHexDecode("4800000000499602d2"),
		},
		{
			name:         "struct obj",
			obj:          stru{a: "a", b: "b", c: "c"},
			wantCborData: mustHexDecode("45612C622C63"),
		},
	}
	em, _ := EncOptions{}.EncMode()
	dm, _ := DecOptions{}.DecMode()
	testRoundTrip(t, testCases, em, dm)
}

func TestBinaryUnmarshalerError(t *testing.T) { //nolint:dupl
	testCases := []struct {
		name         string
		typ          reflect.Type
		data         []byte
		wantErrorMsg string
	}{
		{
			name:         "primitive type",
			typ:          reflect.TypeOf(number(0)),
			data:         mustHexDecode("44499602d2"),
			wantErrorMsg: "number:UnmarshalBinary: invalid length",
		},
		{
			name:         "struct type",
			typ:          reflect.TypeOf(stru{}),
			data:         mustHexDecode("47612C622C632C64"),
			wantErrorMsg: "stru:UnmarshalBinary: invalid element count",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v := reflect.New(tc.typ)
			if err := Unmarshal(tc.data, v.Interface()); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error, want error msg %q", tc.data, tc.wantErrorMsg)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestBinaryMarshalerError(t *testing.T) {
	wantErrorMsg := "MarshalBinary: error"
	v := marshalBinaryError(wantErrorMsg)
	if _, err := Marshal(v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error, want error msg %q", v, wantErrorMsg)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want %q", v, err.Error(), wantErrorMsg)
	}
}

type number2 uint64

func (n number2) MarshalCBOR() (data []byte, err error) {
	m := map[string]uint64{"num": uint64(n)}
	return Marshal(m)
}

func (n *number2) UnmarshalCBOR(data []byte) (err error) {
	var v map[string]uint64
	if err := Unmarshal(data, &v); err != nil {
		return err
	}
	*n = number2(v["num"])
	return nil
}

type stru2 struct {
	a, b, c string
}

func (s *stru2) MarshalCBOR() ([]byte, error) {
	v := []string{s.a, s.b, s.c}
	return Marshal(v)
}

func (s *stru2) UnmarshalCBOR(data []byte) (err error) {
	var v []string
	if err := Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) > 0 {
		s.a = v[0]
	}
	if len(v) > 1 {
		s.b = v[1]
	}
	if len(v) > 2 {
		s.c = v[2]
	}
	return nil
}

type marshalCBORError string

func (n marshalCBORError) MarshalCBOR() (data []byte, err error) {
	return nil, errors.New(string(n))
}

func TestMarshalerUnmarshaler(t *testing.T) {
	testCases := []roundTripTest{
		{
			name:         "primitive obj",
			obj:          number2(1),
			wantCborData: mustHexDecode("a1636e756d01"),
		},
		{
			name:         "struct obj",
			obj:          stru2{a: "a", b: "b", c: "c"},
			wantCborData: mustHexDecode("83616161626163"),
		},
	}
	em, _ := EncOptions{}.EncMode()
	dm, _ := DecOptions{}.DecMode()
	testRoundTrip(t, testCases, em, dm)
}

func TestUnmarshalerError(t *testing.T) { //nolint:dupl
	testCases := []struct {
		name         string
		typ          reflect.Type
		data         []byte
		wantErrorMsg string
	}{
		{
			name:         "primitive type",
			typ:          reflect.TypeOf(number2(0)),
			data:         mustHexDecode("44499602d2"),
			wantErrorMsg: "cbor: cannot unmarshal byte string into Go value of type map[string]uint64",
		},
		{
			name:         "struct type",
			typ:          reflect.TypeOf(stru2{}),
			data:         mustHexDecode("47612C622C632C64"),
			wantErrorMsg: "cbor: cannot unmarshal byte string into Go value of type []string",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v := reflect.New(tc.typ)
			if err := Unmarshal(tc.data, v.Interface()); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error, want error msg %q", tc.data, tc.wantErrorMsg)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestMarshalerError(t *testing.T) {
	wantErrorMsg := "MarshalCBOR: error"
	v := marshalCBORError(wantErrorMsg)
	if _, err := Marshal(v); err == nil {
		t.Errorf("Marshal(%+v) didn't return an error, want error msg %q", v, wantErrorMsg)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Marshal(%+v) returned error %q, want %q", v, err.Error(), wantErrorMsg)
	}
}

// Found at https://github.com/oasislabs/oasis-core/blob/master/go/common/cbor/cbor_test.go
func TestOutOfMem1(t *testing.T) {
	data := []byte("\x9b\x00\x00000000")
	var f []byte
	if err := Unmarshal(data, &f); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	}
}

// Found at https://github.com/oasislabs/oasis-core/blob/master/go/common/cbor/cbor_test.go
func TestOutOfMem2(t *testing.T) {
	data := []byte("\x9b\x00\x00\x81112233")
	var f []byte
	if err := Unmarshal(data, &f); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	}
}

// Found at https://github.com/cose-wg/Examples/tree/master/RFC8152
func TestCOSEExamples(t *testing.T) {
	data := [][]byte{
		mustHexDecode("D8608443A10101A1054C02D1F7E6F26C43D4868D87CE582464F84D913BA60A76070A9A48F26E97E863E2852948658F0811139868826E89218A75715B818440A101225818DBD43C4E9D719C27C6275C67D628D493F090593DB8218F11818344A1013818A220A401022001215820B2ADD44368EA6D641F9CA9AF308B4079AEB519F11E9B8A55A600B21233E86E6822F40458246D65726961646F632E6272616E64796275636B406275636B6C616E642E6578616D706C6540"),
		mustHexDecode("D8628440A054546869732069732074686520636F6E74656E742E818343A10126A1044231315840E2AEAFD40D69D19DFE6E52077C5D7FF4E408282CBEFB5D06CBF414AF2E19D982AC45AC98B8544C908B4507DE1E90B717C3D34816FE926A2B98F53AFD2FA0F30A"),
		mustHexDecode("D8628440A054546869732069732074686520636F6E74656E742E828343A10126A1044231315840E2AEAFD40D69D19DFE6E52077C5D7FF4E408282CBEFB5D06CBF414AF2E19D982AC45AC98B8544C908B4507DE1E90B717C3D34816FE926A2B98F53AFD2FA0F30A8344A1013823A104581E62696C626F2E62616767696E7340686F626269746F6E2E6578616D706C65588400A2D28A7C2BDB1587877420F65ADF7D0B9A06635DD1DE64BB62974C863F0B160DD2163734034E6AC003B01E8705524C5C4CA479A952F0247EE8CB0B4FB7397BA08D009E0C8BF482270CC5771AA143966E5A469A09F613488030C5B07EC6D722E3835ADB5B2D8C44E95FFB13877DD2582866883535DE3BB03D01753F83AB87BB4F7A0297"),
		mustHexDecode("D8628440A1078343A10126A10442313158405AC05E289D5D0E1B0A7F048A5D2B643813DED50BC9E49220F4F7278F85F19D4A77D655C9D3B51E805A74B099E1E085AACD97FC29D72F887E8802BB6650CCEB2C54546869732069732074686520636F6E74656E742E818343A10126A1044231315840E2AEAFD40D69D19DFE6E52077C5D7FF4E408282CBEFB5D06CBF414AF2E19D982AC45AC98B8544C908B4507DE1E90B717C3D34816FE926A2B98F53AFD2FA0F30A"),
		mustHexDecode("D8628456A2687265736572766564F40281687265736572766564A054546869732069732074686520636F6E74656E742E818343A10126A10442313158403FC54702AA56E1B2CB20284294C9106A63F91BAC658D69351210A031D8FC7C5FF3E4BE39445B1A3E83E1510D1ACA2F2E8A7C081C7645042B18ABA9D1FAD1BD9C"),
		mustHexDecode("D28443A10126A10442313154546869732069732074686520636F6E74656E742E58408EB33E4CA31D1C465AB05AAC34CC6B23D58FEF5C083106C4D25A91AEF0B0117E2AF9A291AA32E14AB834DC56ED2A223444547E01F11D3B0916E5A4C345CACB36"),
		mustHexDecode("D8608443A10101A1054CC9CF4DF2FE6C632BF788641358247ADBE2709CA818FB415F1E5DF66F4E1A51053BA6D65A1A0C52A357DA7A644B8070A151B0818344A1013818A220A40102200121582098F50A4FF6C05861C8860D13A638EA56C3F5AD7590BBFBF054E1C7B4D91D628022F50458246D65726961646F632E6272616E64796275636B406275636B6C616E642E6578616D706C6540"),
		mustHexDecode("D8608443A1010AA1054D89F52F65A1C580933B5261A76C581C753548A19B1307084CA7B2056924ED95F2E3B17006DFE931B687B847818343A10129A2335061616262636364646565666667676868044A6F75722D73656372657440"),
		mustHexDecode("D8608443A10101A2054CC9CF4DF2FE6C632BF7886413078344A1013823A104581E62696C626F2E62616767696E7340686F626269746F6E2E6578616D706C65588400929663C8789BB28177AE28467E66377DA12302D7F9594D2999AFA5DFA531294F8896F2B6CDF1740014F4C7F1A358E3A6CF57F4ED6FB02FCF8F7AA989F5DFD07F0700A3A7D8F3C604BA70FA9411BD10C2591B483E1D2C31DE003183E434D8FBA18F17A4C7E3DFA003AC1CF3D30D44D2533C4989D3AC38C38B71481CC3430C9D65E7DDFF58247ADBE2709CA818FB415F1E5DF66F4E1A51053BA6D65A1A0C52A357DA7A644B8070A151B0818344A1013818A220A40102200121582098F50A4FF6C05861C8860D13A638EA56C3F5AD7590BBFBF054E1C7B4D91D628022F50458246D65726961646F632E6272616E64796275636B406275636B6C616E642E6578616D706C6540"),
		mustHexDecode("D8608443A10101A1054C02D1F7E6F26C43D4868D87CE582464F84D913BA60A76070A9A48F26E97E863E28529D8F5335E5F0165EEE976B4A5F6C6F09D818344A101381FA3225821706572656772696E2E746F6F6B407475636B626F726F7567682E6578616D706C650458246D65726961646F632E6272616E64796275636B406275636B6C616E642E6578616D706C6535420101581841E0D76F579DBD0D936A662D54D8582037DE2E366FDE1C62"),
		mustHexDecode("D08343A1010AA1054D89F52F65A1C580933B5261A78C581C5974E1B99A3A4CC09A659AA2E9E7FFF161D38CE71CB45CE460FFB569"),
		mustHexDecode("D08343A1010AA1064261A7581C252A8911D465C125B6764739700F0141ED09192DE139E053BD09ABCA"),
		mustHexDecode("D8618543A1010FA054546869732069732074686520636F6E74656E742E489E1226BA1F81B848818340A20125044A6F75722D73656372657440"),
		mustHexDecode("D8618543A10105A054546869732069732074686520636F6E74656E742E582081A03448ACD3D305376EAA11FB3FE416A955BE2CBE7EC96F012C994BC3F16A41818344A101381AA3225821706572656772696E2E746F6F6B407475636B626F726F7567682E6578616D706C650458246D65726961646F632E6272616E64796275636B406275636B6C616E642E6578616D706C653558404D8553E7E74F3C6A3A9DD3EF286A8195CBF8A23D19558CCFEC7D34B824F42D92BD06BD2C7F0271F0214E141FB779AE2856ABF585A58368B017E7F2A9E5CE4DB540"),
		mustHexDecode("D8618543A1010EA054546869732069732074686520636F6E74656E742E4836F5AFAF0BAB5D43818340A2012404582430313863306165352D346439622D343731622D626664362D6565663331346263373033375818711AB0DC2FC4585DCE27EFFA6781C8093EBA906F227B6EB0"),
		mustHexDecode("D8618543A10105A054546869732069732074686520636F6E74656E742E5820BF48235E809B5C42E995F2B7D5FA13620E7ED834E337F6AA43DF161E49E9323E828344A101381CA220A4010220032158420043B12669ACAC3FD27898FFBA0BCD2E6C366D53BC4DB71F909A759304ACFB5E18CDC7BA0B13FF8C7636271A6924B1AC63C02688075B55EF2D613574E7DC242F79C322F504581E62696C626F2E62616767696E7340686F626269746F6E2E6578616D706C655828339BC4F79984CDC6B3E6CE5F315A4C7D2B0AC466FCEA69E8C07DFBCA5BB1F661BC5F8E0DF9E3EFF58340A2012404582430313863306165352D346439622D343731622D626664362D65656633313462633730333758280B2C7CFCE04E98276342D6476A7723C090DFDD15F9A518E7736549E998370695E6D6A83B4AE507BB"),
		mustHexDecode("D18443A1010FA054546869732069732074686520636F6E74656E742E48726043745027214F"),
	}
	for _, d := range data {
		var v any
		if err := Unmarshal(d, &v); err != nil {
			t.Errorf("Unmarshal(0x%x) returned error %v", d, err)
		}
	}
}

func TestUnmarshalStructKeyAsIntError(t *testing.T) {
	type T1 struct {
		F1 int `cbor:"1,keyasint"`
	}
	data := mustHexDecode("a13bffffffffffffffff01") // {1: -18446744073709551616}
	var v T1
	if err := Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), "cannot unmarshal")
	}
}

func TestUnmarshalArrayToStruct(t *testing.T) {
	type T struct {
		_ struct{} `cbor:",toarray"`
		A int
		B int
		C int
	}
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "definite length array",
			data: mustHexDecode("83010203"),
		},
		{
			name: "indefinite length array",
			data: mustHexDecode("9f010203ff"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v T
			if err := Unmarshal(tc.data, &v); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
			}
		})
	}
}

func TestUnmarshalArrayToStructNoToArrayOptionError(t *testing.T) {
	type T struct {
		A int
		B int
		C int
	}
	data := mustHexDecode("8301020383010203")
	var v1 T
	wantT := T{}
	dec := NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&v1); err == nil {
		t.Errorf("Decode(%+v) didn't return an error", v1)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Decode(%+v) returned wrong error type %T, want (*UnmarshalTypeError)", v1, err)
	} else if !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("Decode(%+v) returned error %q, want error containing %q", err.Error(), v1, "cannot unmarshal")
	}
	if !reflect.DeepEqual(v1, wantT) {
		t.Errorf("Decode() = %+v (%T), want %+v (%T)", v1, v1, wantT, wantT)
	}
	var v2 []int
	want := []int{1, 2, 3}
	if err := dec.Decode(&v2); err != nil {
		t.Errorf("Decode() returned error %v", err)
	}
	if !reflect.DeepEqual(v2, want) {
		t.Errorf("Decode() = %+v (%T), want %+v (%T)", v2, v2, want, want)
	}
}

func TestUnmarshalNonArrayDataToStructToArray(t *testing.T) {
	type T struct {
		_ struct{} `cbor:",toarray"`
		A int
		B int
		C int
	}
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "CBOR positive int",
			data: mustHexDecode("00"),
		}, // 0
		{
			name: "CBOR negative int",
			data: mustHexDecode("20"),
		}, // -1
		{
			name: "CBOR byte string",
			data: mustHexDecode("4401020304"),
		}, // h`01020304`
		{
			name: "CBOR text string",
			data: mustHexDecode("7f657374726561646d696e67ff"),
		}, // streaming
		{
			name: "CBOR map",
			data: mustHexDecode("a3614101614202614303"),
		}, // {"A": 1, "B": 2, "C": 3}
		{
			name: "CBOR bool",
			data: mustHexDecode("f5"),
		}, // true
		{
			name: "CBOR float",
			data: mustHexDecode("fa7f7fffff"),
		}, // 3.4028234663852886e+38
	}
	wantT := T{}
	wantErrorMsg := "cannot unmarshal"
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v T
			if err := Unmarshal(tc.data, &v); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error", tc.data)
			} else if _, ok := err.(*UnmarshalTypeError); !ok {
				t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", tc.data, err)
			} else if !strings.Contains(err.Error(), wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", tc.data, err.Error(), wantErrorMsg)
			}
			if !reflect.DeepEqual(v, wantT) {
				t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", tc.data, v, v, wantT, wantT)
			}
		})
	}
}

func TestUnmarshalArrayToStructWrongSizeError(t *testing.T) {
	type T struct {
		_ struct{} `cbor:",toarray"`
		A int
		B int
	}
	data := mustHexDecode("8301020383010203")
	var v1 T
	wantT := T{}
	dec := NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&v1); err == nil {
		t.Errorf("Decode(%+v) didn't return an error", v1)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Decode(%+v) returned wrong error type %T, want (*UnmarshalTypeError)", v1, err)
	} else if !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("Decode(%+v) returned error %q, want error containing %q", v1, err.Error(), "cannot unmarshal")
	}
	if !reflect.DeepEqual(v1, wantT) {
		t.Errorf("Decode() = %+v (%T), want %+v (%T)", v1, v1, wantT, wantT)
	}
	var v2 []int
	want := []int{1, 2, 3}
	if err := dec.Decode(&v2); err != nil {
		t.Errorf("Decode() returned error %v", err)
	}
	if !reflect.DeepEqual(v2, want) {
		t.Errorf("Decode() = %+v (%T), want %+v (%T)", v2, v2, want, want)
	}
}

func TestUnmarshalArrayToStructWrongFieldTypeError(t *testing.T) {
	type T struct {
		_ struct{} `cbor:",toarray"`
		A int
		B string
		C int
	}
	testCases := []struct {
		name         string
		data         []byte
		wantErrorMsg string
		wantV        any
	}{
		// [1, 2, 3]
		{
			name:         "wrong field type",
			data:         mustHexDecode("83010203"),
			wantErrorMsg: "cannot unmarshal",
			wantV:        T{A: 1, C: 3},
		},
		// [1, 0xfe, 3]
		{
			name:         "invalid UTF-8 string",
			data:         mustHexDecode("830161fe03"),
			wantErrorMsg: invalidUTF8ErrorMsg,
			wantV:        T{A: 1, C: 3},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v T
			if err := Unmarshal(tc.data, &v); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error", tc.data)
			} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
			if !reflect.DeepEqual(v, tc.wantV) {
				t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", tc.data, v, v, tc.wantV, tc.wantV)
			}
		})
	}
}

func TestUnmarshalArrayToStructCannotSetEmbeddedPointerError(t *testing.T) {
	type (
		s1 struct {
			x int //nolint:unused,structcheck
			X int
		}
		S2 struct {
			y int //nolint:unused,structcheck
			Y int
		}
		S struct {
			_ struct{} `cbor:",toarray"`
			*s1
			*S2
		}
	)
	data := []byte{0x82, 0x02, 0x04} // [2, 4]
	const wantErrorMsg = "cannot set embedded pointer to unexported struct"
	wantV := S{S2: &S2{Y: 4}}
	var v S
	err := Unmarshal(data, &v)
	if err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error, want error %q", data, wantErrorMsg)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(v, wantV) {
		t.Errorf("Decode() = %+v (%T), want %+v (%T)", v, v, wantV, wantV)
	}
}

func TestUnmarshalIntoSliceError(t *testing.T) {
	data := []byte{0x83, 0x61, 0x61, 0x61, 0xfe, 0x61, 0x62} // ["a", 0xfe, "b"]
	wantErrorMsg := invalidUTF8ErrorMsg
	var want any

	// Unmarshal CBOR array into Go empty interface.
	var v1 any
	want = []any{"a", any(nil), "b"}
	if err := Unmarshal(data, &v1); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", data, wantErrorMsg)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(v1, want) {
		t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v1, want)
	}

	// Unmarshal CBOR array into Go slice.
	var v2 []string
	want = []string{"a", "", "b"}
	if err := Unmarshal(data, &v2); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", data, wantErrorMsg)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(v2, want) {
		t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v2, want)
	}

	// Unmarshal CBOR array into Go array.
	var v3 [3]string
	want = [3]string{"a", "", "b"}
	if err := Unmarshal(data, &v3); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", data, wantErrorMsg)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(v3, want) {
		t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v3, want)
	}

	// Unmarshal CBOR array into populated Go slice.
	v4 := []string{"hello", "to", "you"}
	want = []string{"a", "to", "b"}
	if err := Unmarshal(data, &v4); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", data, wantErrorMsg)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(v4, want) {
		t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v4, want)
	}
}

func TestUnmarshalIntoMapError(t *testing.T) {
	data := [][]byte{
		{0xa3, 0x61, 0x61, 0x61, 0x41, 0x61, 0xfe, 0x61, 0x43, 0x61, 0x62, 0x61, 0x42}, // {"a":"A", 0xfe: "C", "b":"B"}
		{0xa3, 0x61, 0x61, 0x61, 0x41, 0x61, 0x63, 0x61, 0xfe, 0x61, 0x62, 0x61, 0x42}, // {"a":"A", "c": 0xfe, "b":"B"}
	}
	wantErrorMsg := invalidUTF8ErrorMsg
	var want any

	for _, data := range data {
		// Unmarshal CBOR map into Go empty interface.
		var v1 any
		want = map[any]any{"a": "A", "b": "B"}
		if err := Unmarshal(data, &v1); err == nil {
			t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", data, wantErrorMsg)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(v1, want) {
			t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v1, want)
		}

		// Unmarshal CBOR map into Go map[interface{}]interface{}.
		var v2 map[any]any
		want = map[any]any{"a": "A", "b": "B"}
		if err := Unmarshal(data, &v2); err == nil {
			t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", data, wantErrorMsg)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(v2, want) {
			t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v2, want)
		}

		// Unmarshal CBOR array into Go map[string]string.
		var v3 map[string]string
		want = map[string]string{"a": "A", "b": "B"}
		if err := Unmarshal(data, &v3); err == nil {
			t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", data, wantErrorMsg)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(v3, want) {
			t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v3, want)
		}

		// Unmarshal CBOR array into populated Go map[string]string.
		v4 := map[string]string{"c": "D"}
		want = map[string]string{"a": "A", "b": "B", "c": "D"}
		if err := Unmarshal(data, &v4); err == nil {
			t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", data, wantErrorMsg)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(v4, want) {
			t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v4, want)
		}
	}
}

func TestUnmarshalDeepNesting(t *testing.T) {
	// Construct this object rather than embed such a large constant in the code
	type TestNode struct {
		Value int
		Child *TestNode
	}
	n := &TestNode{Value: 0}
	root := n
	for i := 0; i < 65534; i++ {
		child := &TestNode{Value: i}
		n.Child = child
		n = child
	}
	em, err := EncOptions{}.EncMode()
	if err != nil {
		t.Errorf("EncMode() returned error %v", err)
	}
	data, err := em.Marshal(root)
	if err != nil {
		t.Errorf("Marshal() deeply nested object returned error %v", err)
	}

	// Try unmarshal it
	dm, err := DecOptions{MaxNestedLevels: 65535}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned error %v", err)
	}
	var readback TestNode
	err = dm.Unmarshal(data, &readback)
	if err != nil {
		t.Errorf("Unmarshal() of deeply nested object returned error: %v", err)
	}
	if !reflect.DeepEqual(root, &readback) {
		t.Errorf("Unmarshal() of deeply nested object did not match\nGot: %#v\n Want: %#v\n",
			&readback, root)
	}
}

func TestStructToArrayError(t *testing.T) {
	type coseHeader struct {
		Alg int    `cbor:"1,keyasint,omitempty"`
		Kid []byte `cbor:"4,keyasint,omitempty"`
		IV  []byte `cbor:"5,keyasint,omitempty"`
	}
	type nestedCWT struct {
		_           struct{} `cbor:",toarray"`
		Protected   []byte
		Unprotected coseHeader
		Ciphertext  []byte
	}
	for _, tc := range []struct {
		data         []byte
		wantErrorMsg string
	}{
		// [-17, [-17, -17], -17]
		{mustHexDecode("9f3082303030ff"), "cbor: cannot unmarshal negative integer into Go struct field cbor.nestedCWT.Protected of type []uint8"},
		// [[], [], ["\x930000", -17]]
		{mustHexDecode("9f9fff9fff9f65933030303030ffff"), "cbor: cannot unmarshal array into Go struct field cbor.nestedCWT.Unprotected of type cbor.coseHeader (cannot decode CBOR array to struct without toarray option)"},
	} {
		var v nestedCWT
		if err := Unmarshal(tc.data, &v); err == nil {
			t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", tc.data, tc.wantErrorMsg)
		} else if err.Error() != tc.wantErrorMsg {
			t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
		}
	}
}

func TestStructKeyAsIntError(t *testing.T) {
	type claims struct {
		Iss string  `cbor:"1,keyasint"`
		Sub string  `cbor:"2,keyasint"`
		Aud string  `cbor:"3,keyasint"`
		Exp float64 `cbor:"4,keyasint"`
		Nbf float64 `cbor:"5,keyasint"`
		Iat float64 `cbor:"6,keyasint"`
		Cti []byte  `cbor:"7,keyasint"`
	}
	data := mustHexDecode("bf0783e662f03030ff") // {7: [simple(6), "\xF00", -17]}
	wantErrorMsg := invalidUTF8ErrorMsg
	wantV := claims{Cti: []byte{6, 0, 0}}
	var v claims
	if err := Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", data, wantErrorMsg)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(v, wantV) {
		t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v, wantV)
	}
}

func TestUnmarshalToNotNilInterface(t *testing.T) {
	data := mustHexDecode("83010203") // []uint64{1, 2, 3}
	s := "hello"                      //nolint:goconst
	var v any = s                     // Unmarshal() sees v as type interface{} and sets CBOR data as default Go type.  s is unmodified.  Same behavior as encoding/json.
	wantV := []any{uint64(1), uint64(2), uint64(3)}
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(v, wantV) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantV, wantV)
	} else if s != "hello" {
		t.Errorf("Unmarshal(0x%x) modified s %q", data, s)
	}
}

func TestDecOptions(t *testing.T) {
	simpleValues, err := NewSimpleValueRegistryFromDefaults(WithRejectedSimpleValue(255))
	if err != nil {
		t.Fatal(err)
	}

	opts1 := DecOptions{
		DupMapKey:                 DupMapKeyEnforcedAPF,
		TimeTag:                   DecTagRequired,
		MaxNestedLevels:           100,
		MaxArrayElements:          102,
		MaxMapPairs:               101,
		IndefLength:               IndefLengthForbidden,
		TagsMd:                    TagsForbidden,
		IntDec:                    IntDecConvertSigned,
		MapKeyByteString:          MapKeyByteStringForbidden,
		ExtraReturnErrors:         ExtraDecErrorUnknownField,
		DefaultMapType:            reflect.TypeOf(map[string]any(nil)),
		UTF8:                      UTF8DecodeInvalid,
		FieldNameMatching:         FieldNameMatchingCaseSensitive,
		BigIntDec:                 BigIntDecodePointer,
		DefaultByteStringType:     reflect.TypeOf(""),
		ByteStringToString:        ByteStringToStringAllowed,
		FieldNameByteString:       FieldNameByteStringAllowed,
		UnrecognizedTagToAny:      UnrecognizedTagContentToAny,
		TimeTagToAny:              TimeTagToRFC3339,
		SimpleValues:              simpleValues,
		NaN:                       NaNDecodeForbidden,
		Inf:                       InfDecodeForbidden,
		ByteStringToTime:          ByteStringToTimeAllowed,
		ByteStringExpectedFormat:  ByteStringExpectedBase64URL,
		BignumTag:                 BignumTagForbidden,
		BinaryUnmarshaler:         BinaryUnmarshalerNone,
		TextUnmarshaler:           TextUnmarshalerTextString,
		JSONUnmarshalerTranscoder: stubTranscoder{},
	}
	ov := reflect.ValueOf(opts1)
	for i := 0; i < ov.NumField(); i++ {
		fv := ov.Field(i)
		if fv.IsZero() {
			t.Errorf("options field %q is unset or set to the zero value for its type", ov.Type().Field(i).Name)
		}
	}
	dm, err := opts1.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %v", err)
	} else {
		opts2 := dm.DecOptions()
		if !reflect.DeepEqual(opts1, opts2) {
			t.Errorf("DecOptions->DecMode->DecOptions returned different values: %#v, %#v", opts1, opts2)
		}
	}
}

type roundTripTest struct {
	name         string
	obj          any
	wantCborData []byte
}

func testRoundTrip(t *testing.T, testCases []roundTripTest, em EncMode, dm DecMode) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := em.Marshal(tc.obj)
			if err != nil {
				t.Errorf("Marshal(%+v) returned error %v", tc.obj, err)
			}
			if !bytes.Equal(b, tc.wantCborData) {
				t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", tc.obj, b, tc.wantCborData)
			}
			v := reflect.New(reflect.TypeOf(tc.obj))
			if err := dm.Unmarshal(b, v.Interface()); err != nil {
				t.Errorf("Unmarshal() returned error %v", err)
			}
			if !reflect.DeepEqual(tc.obj, v.Elem().Interface()) {
				t.Errorf("Marshal-Unmarshal returned different values: %v, %v", tc.obj, v.Elem().Interface())
			}
		})
	}
}

func TestDecModeInvalidTimeTag(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{TimeTag: -1},
			wantErrorMsg: "cbor: invalid TimeTag -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{TimeTag: 101},
			wantErrorMsg: "cbor: invalid TimeTag 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecModeInvalidDuplicateMapKey(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{DupMapKey: -1},
			wantErrorMsg: "cbor: invalid DupMapKey -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{DupMapKey: 101},
			wantErrorMsg: "cbor: invalid DupMapKey 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecModeDefaultMaxNestedLevel(t *testing.T) {
	dm, err := DecOptions{}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned error %v", err)
	} else {
		maxNestedLevels := dm.DecOptions().MaxNestedLevels
		if maxNestedLevels != 32 {
			t.Errorf("DecOptions().MaxNestedLevels = %d, want %v", maxNestedLevels, 32)
		}
	}
}

func TestDecModeInvalidMaxNestedLevel(t *testing.T) {
	testCases := []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "MaxNestedLevels < 4",
			opts:         DecOptions{MaxNestedLevels: 1},
			wantErrorMsg: "cbor: invalid MaxNestedLevels 1 (range is [4, 65535])",
		},
		{
			name:         "MaxNestedLevels > 65535",
			opts:         DecOptions{MaxNestedLevels: 65536},
			wantErrorMsg: "cbor: invalid MaxNestedLevels 65536 (range is [4, 65535])",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecModeDefaultMaxMapPairs(t *testing.T) {
	dm, err := DecOptions{}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned error %v", err)
	} else {
		maxMapPairs := dm.DecOptions().MaxMapPairs
		if maxMapPairs != defaultMaxMapPairs {
			t.Errorf("DecOptions().MaxMapPairs = %d, want %v", maxMapPairs, defaultMaxMapPairs)
		}
	}
}

func TestDecModeInvalidMaxMapPairs(t *testing.T) {
	testCases := []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "MaxMapPairs < 16",
			opts:         DecOptions{MaxMapPairs: 1},
			wantErrorMsg: "cbor: invalid MaxMapPairs 1 (range is [16, 2147483647])",
		},
		{
			name:         "MaxMapPairs > 2147483647",
			opts:         DecOptions{MaxMapPairs: 2147483648},
			wantErrorMsg: "cbor: invalid MaxMapPairs 2147483648 (range is [16, 2147483647])",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecModeDefaultMaxArrayElements(t *testing.T) {
	dm, err := DecOptions{}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned error %v", err)
	} else {
		maxArrayElements := dm.DecOptions().MaxArrayElements
		if maxArrayElements != defaultMaxArrayElements {
			t.Errorf("DecOptions().MaxArrayElementsr = %d, want %v", maxArrayElements, defaultMaxArrayElements)
		}
	}
}

func TestDecModeInvalidMaxArrayElements(t *testing.T) {
	testCases := []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "MaxArrayElements < 16",
			opts:         DecOptions{MaxArrayElements: 1},
			wantErrorMsg: "cbor: invalid MaxArrayElements 1 (range is [16, 2147483647])",
		},
		{
			name:         "MaxArrayElements > 2147483647",
			opts:         DecOptions{MaxArrayElements: 2147483648},
			wantErrorMsg: "cbor: invalid MaxArrayElements 2147483648 (range is [16, 2147483647])",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecModeInvalidIndefiniteLengthMode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{IndefLength: -1},
			wantErrorMsg: "cbor: invalid IndefLength -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{IndefLength: 101},
			wantErrorMsg: "cbor: invalid IndefLength 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecModeInvalidTagsMode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{TagsMd: -1},
			wantErrorMsg: "cbor: invalid TagsMd -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{TagsMd: 101},
			wantErrorMsg: "cbor: invalid TagsMd 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestUnmarshalStructKeyAsIntNumError(t *testing.T) {
	type T1 struct {
		F1 int `cbor:"a,keyasint"`
	}
	type T2 struct {
		F1 int `cbor:"-18446744073709551616,keyasint"`
	}
	testCases := []struct {
		name         string
		data         []byte
		obj          any
		wantErrorMsg string
	}{
		{
			name:         "string as key",
			data:         mustHexDecode("a1616101"),
			obj:          T1{},
			wantErrorMsg: "cbor: failed to parse field name \"a\" to int",
		},
		{
			name:         "out of range int as key",
			data:         mustHexDecode("a13bffffffffffffffff01"),
			obj:          T2{},
			wantErrorMsg: "cbor: failed to parse field name \"-18446744073709551616\" to int",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v := reflect.New(reflect.TypeOf(tc.obj))
			err := Unmarshal(tc.data, v.Interface())
			if err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error, want error %q", tc.data, tc.wantErrorMsg)
			} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
				t.Errorf("Unmarshal(0x%x) error %v, want %v", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestUnmarshalEmptyMapWithDupMapKeyOpt(t *testing.T) {
	testCases := []struct {
		name  string
		data  []byte
		wantV any
	}{
		{
			name:  "empty map",
			data:  mustHexDecode("a0"),
			wantV: map[any]any{},
		},
		{
			name:  "indefinite empty map",
			data:  mustHexDecode("bfff"),
			wantV: map[any]any{},
		},
	}

	dm, err := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned error %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			if err := dm.Unmarshal(tc.data, &v); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
			}
			if !reflect.DeepEqual(v, tc.wantV) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.data, v, v, tc.wantV, tc.wantV)
			}
		})
	}
}

func TestUnmarshalDupMapKeyToEmptyInterface(t *testing.T) {
	data := mustHexDecode("a6616161416162614261636143616161466164614461656145") // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}

	// Duplicate key overwrites previous value (default).
	wantV := map[any]any{"a": "F", "b": "B", "c": "C", "d": "D", "e": "E"}
	var v any
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(v, wantV) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantV, wantV)
	}

	// Duplicate key triggers error.
	wantV = map[any]any{"a": nil, "b": "B", "c": "C"}
	wantErrorMsg := "cbor: found duplicate map key \"a\" at map element index 3"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var v2 any
	if err := dm.Unmarshal(data, &v2); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*DupMapKeyError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*DupMapKeyError)", data, err)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(v2, wantV) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v2, v2, wantV, wantV)
	}
}

func TestStreamDupMapKeyToEmptyInterface(t *testing.T) {
	data := mustHexDecode("a6616161416162614261636143616161466164614461656145") // map with duplicate key "c": {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}

	var b []byte
	for i := 0; i < 3; i++ {
		b = append(b, data...)
	}

	// Duplicate key overwrites previous value (default).
	wantV := map[any]any{"a": "F", "b": "B", "c": "C", "d": "D", "e": "E"}
	dec := NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var v1 any
		if err := dec.Decode(&v1); err != nil {
			t.Errorf("Decode() returned error %v", err)
		}
		if !reflect.DeepEqual(v1, wantV) {
			t.Errorf("Decode() = %v (%T), want %v (%T)", v1, v1, wantV, wantV)
		}
	}
	var v any
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}

	// Duplicate key triggers error.
	wantV = map[any]any{"a": nil, "b": "B", "c": "C"}
	wantErrorMsg := "cbor: found duplicate map key \"a\" at map element index 3"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	dec = dm.NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var v2 any
		if err := dec.Decode(&v2); err == nil {
			t.Errorf("Decode() didn't return an error")
		} else if _, ok := err.(*DupMapKeyError); !ok {
			t.Errorf("Decode() returned wrong error type %T, want (*DupMapKeyError)", err)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Decode() returned error %q, want error containing %q", err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(v2, wantV) {
			t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v2, v2, wantV, wantV)
		}
	}
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}
}

func TestUnmarshalDupMapKeyToEmptyMap(t *testing.T) {
	data := mustHexDecode("a6616161416162614261636143616161466164614461656145") // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}

	// Duplicate key overwrites previous value (default).
	wantM := map[string]string{"a": "F", "b": "B", "c": "C", "d": "D", "e": "E"}
	var m map[string]string
	if err := Unmarshal(data, &m); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(m, wantM) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, m, m, wantM, wantM)
	}

	// Duplicate key triggers error.
	wantM = map[string]string{"a": "", "b": "B", "c": "C"}
	wantErrorMsg := "cbor: found duplicate map key \"a\" at map element index 3"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var m2 map[string]string
	if err := dm.Unmarshal(data, &m2); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*DupMapKeyError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*DupMapKeyError)", data, err)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(m2, wantM) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, m2, m2, wantM, wantM)
	}
}

func TestStreamDupMapKeyToEmptyMap(t *testing.T) {
	data := mustHexDecode("a6616161416162614261636143616161466164614461656145") // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}

	var b []byte
	for i := 0; i < 3; i++ {
		b = append(b, data...)
	}

	// Duplicate key overwrites previous value (default).
	wantM := map[string]string{"a": "F", "b": "B", "c": "C", "d": "D", "e": "E"}
	dec := NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var m1 map[string]string
		if err := dec.Decode(&m1); err != nil {
			t.Errorf("Decode() returned error %v", err)
		}
		if !reflect.DeepEqual(m1, wantM) {
			t.Errorf("Decode() = %v (%T), want %v (%T)", m1, m1, wantM, wantM)
		}
	}
	var v any
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}

	// Duplicate key triggers error.
	wantM = map[string]string{"a": "", "b": "B", "c": "C"}
	wantErrorMsg := "cbor: found duplicate map key \"a\" at map element index 3"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	dec = dm.NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var m2 map[string]string
		if err := dec.Decode(&m2); err == nil {
			t.Errorf("Decode() didn't return an error")
		} else if _, ok := err.(*DupMapKeyError); !ok {
			t.Errorf("Decode() returned wrong error type %T, want (*DupMapKeyError)", err)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Decode() returned error %q, want error containing %q", err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(m2, wantM) {
			t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, m2, m2, wantM, wantM)
		}
	}
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}
}

func TestUnmarshalDupMapKeyToNotEmptyMap(t *testing.T) {
	data := mustHexDecode("a6616161416162614261636143616161466164614461656145") // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}

	// Duplicate key overwrites previous value (default).
	m := map[string]string{"a": "Z", "b": "Z", "c": "Z", "d": "Z", "e": "Z", "f": "Z"}
	wantM := map[string]string{"a": "F", "b": "B", "c": "C", "d": "D", "e": "E", "f": "Z"}
	if err := Unmarshal(data, &m); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(m, wantM) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, m, m, wantM, wantM)
	}

	// Duplicate key triggers error.
	m2 := map[string]string{"a": "Z", "b": "Z", "c": "Z", "d": "Z", "e": "Z", "f": "Z"}
	wantM = map[string]string{"a": "", "b": "B", "c": "C", "d": "Z", "e": "Z", "f": "Z"}
	wantErrorMsg := "cbor: found duplicate map key \"a\" at map element index 3"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	if err := dm.Unmarshal(data, &m2); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*DupMapKeyError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*DupMapKeyError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(m2, wantM) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, m2, m2, wantM, wantM)
	}
}

func TestStreamDupMapKeyToNotEmptyMap(t *testing.T) {
	data := mustHexDecode("a6616161416162614261636143616161466164614461656145") // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}

	var b []byte
	for i := 0; i < 3; i++ {
		b = append(b, data...)
	}

	// Duplicate key overwrites previous value (default).
	wantM := map[string]string{"a": "F", "b": "B", "c": "C", "d": "D", "e": "E", "f": "Z"}
	dec := NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		m1 := map[string]string{"a": "Z", "b": "Z", "c": "Z", "d": "Z", "e": "Z", "f": "Z"}
		if err := dec.Decode(&m1); err != nil {
			t.Errorf("Decode() returned error %v", err)
		}
		if !reflect.DeepEqual(m1, wantM) {
			t.Errorf("Decode() = %v (%T), want %v (%T)", m1, m1, wantM, wantM)
		}
	}
	var v any
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}

	// Duplicate key triggers error.
	wantM = map[string]string{"a": "", "b": "B", "c": "C", "d": "Z", "e": "Z", "f": "Z"}
	wantErrorMsg := "cbor: found duplicate map key \"a\" at map element index 3"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	dec = dm.NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		m2 := map[string]string{"a": "Z", "b": "Z", "c": "Z", "d": "Z", "e": "Z", "f": "Z"}
		if err := dec.Decode(&m2); err == nil {
			t.Errorf("Decode() didn't return an error")
		} else if _, ok := err.(*DupMapKeyError); !ok {
			t.Errorf("Decode() returned wrong error type %T, want (*DupMapKeyError)", err)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Decode() returned error %q, want error containing %q", err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(m2, wantM) {
			t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, m2, m2, wantM, wantM)
		}
	}
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}
}

func TestUnmarshalDupMapKeyToStruct(t *testing.T) {
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`

		I string `cbor:"1,keyasint"`
	}

	for _, tc := range []struct {
		name    string
		opts    DecOptions
		data    []byte
		want    s
		wantErr *DupMapKeyError
	}{
		{
			name: "duplicate key does not overwrite previous value",
			data: mustHexDecode("a6616161416162614261636143616161466164614461656145"), // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}
			want: s{A: "A", B: "B", C: "C", D: "D", E: "E"},
		},
		{
			name:    "duplicate key triggers error",
			opts:    DecOptions{DupMapKey: DupMapKeyEnforcedAPF},
			data:    mustHexDecode("a6616161416162614261636143616161466164614461656145"), // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}
			want:    s{A: "A", B: "B", C: "C"},
			wantErr: &DupMapKeyError{Key: "a", Index: 3},
		},
		{
			name:    "duplicate keys of comparable but disallowed cbor types skips remaining entries and returns error",
			opts:    DecOptions{DupMapKey: DupMapKeyEnforcedAPF},
			data:    mustHexDecode("a7616161416162614261636143d903e70100d903e701016164614461656145"), // {"a": "A", "b": "B", "c": "C", 999(1): 0, 999(1): 1, "d": "D", "e": "E"}
			want:    s{A: "A", B: "B", C: "C"},
			wantErr: &DupMapKeyError{Key: Tag{Number: 999, Content: uint64(1)}, Index: 4},
		},
		{
			name: "mixed-case duplicate key does not overwrite previous value",
			data: mustHexDecode("a6616161416162614261636143614161466164614461656145"), // {"a": "A", "b": "B", "c": "C", "A": "F", "d": "D", "e": "E"}
			want: s{A: "A", B: "B", C: "C", D: "D", E: "E"},
		},
		{
			name:    "mixed-case duplicate key triggers error",
			opts:    DecOptions{DupMapKey: DupMapKeyEnforcedAPF},
			data:    mustHexDecode("a6616161416162614261636143614161466164614461656145"), // {"a": "A", "b": "B", "c": "C", "A": "F", "d": "D", "e": "E"}
			want:    s{A: "A", B: "B", C: "C"},
			wantErr: &DupMapKeyError{Key: "A", Index: 3},
		},
		{
			name: "keyasint duplicate key does not overwrite previous value",
			data: mustHexDecode("a36131616901614961616141"), // {"1": "i", 1: "I", "a": "A"}
			want: s{I: "i", A: "A"},
		},
		{
			name:    "keyasint duplicate key triggers error",
			opts:    DecOptions{DupMapKey: DupMapKeyEnforcedAPF},
			data:    mustHexDecode("a36131616901614961616141"), // {"1": "i", 1: "I", "a": "A"}
			want:    s{I: "i"},
			wantErr: &DupMapKeyError{Key: int64(1), Index: 1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			var s1 s
			if err := dm.Unmarshal(tc.data, &s1); err != nil {
				if !reflect.DeepEqual(err, tc.wantErr) {
					t.Errorf("got error: %v, wanted: %v", err, tc.wantErr)
				}
			} else {
				if tc.wantErr != nil {
					t.Errorf("got nil error, wanted: %v", tc.wantErr)
				}
			}

			if !reflect.DeepEqual(s1, tc.want) {
				t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", tc.data, s1, s1, tc.want, tc.want)
			}
		})
	}
}

func TestStreamDupMapKeyToStruct(t *testing.T) {
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`
	}
	data := mustHexDecode("a6616161416162614261636143616161466164614461656145") // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}

	var b []byte
	for i := 0; i < 3; i++ {
		b = append(b, data...)
	}

	// Duplicate key overwrites previous value (default).
	wantS := s{A: "A", B: "B", C: "C", D: "D", E: "E"}
	dec := NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s1 s
		if err := dec.Decode(&s1); err != nil {
			t.Errorf("Decode() returned error %v", err)
		}
		if !reflect.DeepEqual(s1, wantS) {
			t.Errorf("Decode() = %v (%T), want %v (%T)", s1, s1, wantS, wantS)
		}
	}
	var v any
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}

	// Duplicate key triggers error.
	wantS = s{A: "A", B: "B", C: "C"}
	wantErrorMsg := "cbor: found duplicate map key \"a\" at map element index 3"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	dec = dm.NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s2 s
		if err := dec.Decode(&s2); err == nil {
			t.Errorf("Decode() didn't return an error")
		} else if _, ok := err.(*DupMapKeyError); !ok {
			t.Errorf("Decode() returned wrong error type %T, want (*DupMapKeyError)", err)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Decode() returned error %q, want error containing %q", err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(s2, wantS) {
			t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
		}
	}
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}
}

// dupl map key is a struct field
func TestUnmarshalDupMapKeyToStructKeyAsInt(t *testing.T) {
	type s struct {
		A int `cbor:"1,keyasint"`
		B int `cbor:"3,keyasint"`
		C int `cbor:"5,keyasint"`
	}
	data := mustHexDecode("a40102030401030506") // {1:2, 3:4, 1:3, 5:6}

	// Duplicate key doesn't overwrite previous value (default).
	wantS := s{A: 2, B: 4, C: 6}
	var s1 s
	if err := Unmarshal(data, &s1); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(s1, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
	}

	// Duplicate key triggers error.
	wantS = s{A: 2, B: 4}
	wantErrorMsg := "cbor: found duplicate map key 1 at map element index 2"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var s2 s
	if err := dm.Unmarshal(data, &s2); err == nil {
		t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, reflect.TypeOf(s2))
	} else if _, ok := err.(*DupMapKeyError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*DupMapKeyError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s2, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
	}
}

func TestStreamDupMapKeyToStructKeyAsInt(t *testing.T) {
	type s struct {
		A int `cbor:"1,keyasint"`
		B int `cbor:"3,keyasint"`
		C int `cbor:"5,keyasint"`
	}
	data := mustHexDecode("a40102030401030506") // {1:2, 3:4, 1:3, 5:6}

	var b []byte
	for i := 0; i < 3; i++ {
		b = append(b, data...)
	}

	// Duplicate key overwrites previous value (default).
	wantS := s{A: 2, B: 4, C: 6}
	dec := NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s1 s
		if err := dec.Decode(&s1); err != nil {
			t.Errorf("Decode() returned error %v", err)
		}
		if !reflect.DeepEqual(s1, wantS) {
			t.Errorf("Decode() = %v (%T), want %v (%T)", s1, s1, wantS, wantS)
		}
	}
	var v any
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}

	// Duplicate key triggers error.
	wantS = s{A: 2, B: 4}
	wantErrorMsg := "cbor: found duplicate map key 1 at map element index 2"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	dec = dm.NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s2 s
		if err := dec.Decode(&s2); err == nil {
			t.Errorf("Decode() didn't return an error")
		} else if _, ok := err.(*DupMapKeyError); !ok {
			t.Errorf("Decode() returned wrong error type %T, want (*DupMapKeyError)", err)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Decode() returned error %q, want error containing %q", err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(s2, wantS) {
			t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
		}
	}
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}
}

func TestUnmarshalDupMapKeyToStructNoMatchingField(t *testing.T) {
	type s struct {
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`
	}
	data := mustHexDecode("a6616161416162614261636143616161466164614461656145") // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}

	wantS := s{B: "B", C: "C", D: "D", E: "E"}
	var s1 s
	if err := Unmarshal(data, &s1); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(s1, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
	}

	// Duplicate key triggers error even though map key "a" doesn't have a corresponding struct field.
	wantS = s{B: "B", C: "C"}
	wantErrorMsg := "cbor: found duplicate map key \"a\" at map element index 3"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var s2 s
	if err := dm.Unmarshal(data, &s2); err == nil {
		t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, reflect.TypeOf(s2))
	} else if _, ok := err.(*DupMapKeyError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*DupMapKeyError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s2, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
	}
}

func TestStreamDupMapKeyToStructNoMatchingField(t *testing.T) {
	type s struct {
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`
	}
	data := mustHexDecode("a6616161416162614261636143616161466164614461656145") // {"a": "A", "b": "B", "c": "C", "a": "F", "d": "D", "e": "E"}

	var b []byte
	for i := 0; i < 3; i++ {
		b = append(b, data...)
	}

	// Duplicate key overwrites previous value (default).
	wantS := s{B: "B", C: "C", D: "D", E: "E"}
	dec := NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s1 s
		if err := dec.Decode(&s1); err != nil {
			t.Errorf("Decode() returned error %v", err)
		}
		if !reflect.DeepEqual(s1, wantS) {
			t.Errorf("Decode() = %v (%T), want %v (%T)", s1, s1, wantS, wantS)
		}
	}
	var v any
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}

	// Duplicate key triggers error.
	wantS = s{B: "B", C: "C"}
	wantErrorMsg := "cbor: found duplicate map key \"a\" at map element index 3"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	dec = dm.NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s2 s
		if err := dec.Decode(&s2); err == nil {
			t.Errorf("Decode() didn't return an error")
		} else if _, ok := err.(*DupMapKeyError); !ok {
			t.Errorf("Decode() returned wrong error type %T, want (*DupMapKeyError)", err)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Decode() returned error %q, want error containing %q", err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(s2, wantS) {
			t.Errorf("Decode() = %v (%T), want %v (%T)", s2, s2, wantS, wantS)
		}
	}
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}
}

func TestUnmarshalDupMapKeyToStructKeyAsIntNoMatchingField(t *testing.T) {
	type s struct {
		B int `cbor:"3,keyasint"`
		C int `cbor:"5,keyasint"`
	}
	data := mustHexDecode("a40102030401030506") // {1:2, 3:4, 1:3, 5:6}

	wantS := s{B: 4, C: 6}
	var s1 s
	if err := Unmarshal(data, &s1); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(s1, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
	}

	// Duplicate key triggers error even though map key "a" doesn't have a corresponding struct field.
	wantS = s{B: 4}
	wantErrorMsg := "cbor: found duplicate map key 1 at map element index 2"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var s2 s
	if err := dm.Unmarshal(data, &s2); err == nil {
		t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, reflect.TypeOf(s2))
	} else if _, ok := err.(*DupMapKeyError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*DupMapKeyError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s2, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
	}
}

func TestStreamDupMapKeyToStructKeyAsIntNoMatchingField(t *testing.T) {
	type s struct {
		B int `cbor:"3,keyasint"`
		C int `cbor:"5,keyasint"`
	}
	data := mustHexDecode("a40102030401030506") // {1:2, 3:4, 1:3, 5:6}

	var b []byte
	for i := 0; i < 3; i++ {
		b = append(b, data...)
	}

	// Duplicate key overwrites previous value (default).
	wantS := s{B: 4, C: 6}
	dec := NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s1 s
		if err := dec.Decode(&s1); err != nil {
			t.Errorf("Decode() returned error %v", err)
		}
		if !reflect.DeepEqual(s1, wantS) {
			t.Errorf("Decode() = %v (%T), want %v (%T)", s1, s1, wantS, wantS)
		}
	}
	var v any
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}

	// Duplicate key triggers error.
	wantS = s{B: 4}
	wantErrorMsg := "cbor: found duplicate map key 1 at map element index 2"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	dec = dm.NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s2 s
		if err := dec.Decode(&s2); err == nil {
			t.Errorf("Decode() didn't return an error")
		} else if _, ok := err.(*DupMapKeyError); !ok {
			t.Errorf("Decode() returned wrong error type %T, want (*DupMapKeyError)", err)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Decode() returned error %q, want error containing %q", err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(s2, wantS) {
			t.Errorf("Decode() = %v (%T), want %v (%T)", s2, s2, wantS, wantS)
		}
	}
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}
}

func TestUnmarshalDupMapKeyToStructWrongType(t *testing.T) {
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`
	}
	data := mustHexDecode("a861616141fa47c35000026162614261636143fa47c3500003616161466164614461656145") // {"a": "A", 100000.0:2, "b": "B", "c": "C", 100000.0:3, "a": "F", "d": "D", "e": "E"}

	var s1 s
	wantS := s{A: "A", B: "B", C: "C", D: "D", E: "E"}
	wantErrorMsg := "cbor: cannot unmarshal"
	if err := Unmarshal(data, &s1); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s1, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
	}

	wantS = s{A: "A", B: "B", C: "C"}
	wantErrorMsg = "cbor: found duplicate map key 100000 at map element index 4"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var s2 s
	if err := dm.Unmarshal(data, &s2); err == nil {
		t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, reflect.TypeOf(s2))
	} else if _, ok := err.(*DupMapKeyError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*DupMapKeyError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s2, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
	}
}

func TestStreamDupMapKeyToStructWrongType(t *testing.T) {
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`
	}
	data := mustHexDecode("a861616141fa47c35000026162614261636143fa47c3500003616161466164614461656145") // {"a": "A", 100000.0:2, "b": "B", "c": "C", 100000.0:3, "a": "F", "d": "D", "e": "E"}

	var b []byte
	for i := 0; i < 3; i++ {
		b = append(b, data...)
	}

	wantS := s{A: "A", B: "B", C: "C", D: "D", E: "E"}
	wantErrorMsg := "cbor: cannot unmarshal"
	dec := NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s1 s
		if err := dec.Decode(&s1); err == nil {
			t.Errorf("Unmarshal(0x%x) didn't return an error", data)
		} else if _, ok := err.(*UnmarshalTypeError); !ok {
			t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
		} else if !strings.Contains(err.Error(), wantErrorMsg) {
			t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(s1, wantS) {
			t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
		}
	}
	var v any
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}

	// Duplicate key triggers error.
	wantS = s{A: "A", B: "B", C: "C"}
	wantErrorMsg = "cbor: found duplicate map key 100000 at map element index 4"
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	dec = dm.NewDecoder(bytes.NewReader(b))
	for i := 0; i < 3; i++ {
		var s2 s
		if err := dec.Decode(&s2); err == nil {
			t.Errorf("Decode() didn't return an error")
		} else if _, ok := err.(*DupMapKeyError); !ok {
			t.Errorf("Decode() returned wrong error type %T, want (*DupMapKeyError)", err)
		} else if err.Error() != wantErrorMsg {
			t.Errorf("Decode() returned error %q, want error containing %q", err.Error(), wantErrorMsg)
		}
		if !reflect.DeepEqual(s2, wantS) {
			t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
		}
	}
	if err := dec.Decode(&v); err != io.EOF {
		t.Errorf("Decode() returned error %v, want %v", err, io.EOF)
	}
}

func TestUnmarshalDupMapKeyToStructStringParseError(t *testing.T) {
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`
	}
	data := mustHexDecode("a661fe6141616261426163614361fe61466164614461656145") // {"\xFE": "A", "b": "B", "c": "C", "\xFE": "F", "d": "D", "e": "E"}
	wantS := s{A: "", B: "B", C: "C", D: "D", E: "E"}
	wantErrorMsg := "cbor: invalid UTF-8 string"

	// Duplicate key doesn't overwrite previous value (default).
	var s1 s
	if err := Unmarshal(data, &s1); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*SemanticError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*SemanticError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s1, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
	}

	// Duplicate key triggers error.
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var s2 s
	if err := dm.Unmarshal(data, &s2); err == nil {
		t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, reflect.TypeOf(s2))
	} else if _, ok := err.(*SemanticError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*SemanticError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s2, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
	}
}

func TestUnmarshalDupMapKeyToStructIntParseError(t *testing.T) {
	type s struct {
		A int `cbor:"1,keyasint"`
		B int `cbor:"3,keyasint"`
		C int `cbor:"5,keyasint"`
	}
	data := mustHexDecode("a43bffffffffffffffff0203043bffffffffffffffff030506") // {-18446744073709551616:2, 3:4, -18446744073709551616:3, 5:6}

	// Duplicate key doesn't overwrite previous value (default).
	wantS := s{B: 4, C: 6}
	wantErrorMsg := "cbor: cannot unmarshal"
	var s1 s
	if err := Unmarshal(data, &s1); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s1, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
	}

	// Duplicate key triggers error.
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var s2 s
	if err := dm.Unmarshal(data, &s2); err == nil {
		t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, reflect.TypeOf(s2))
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s2, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
	}
}

func TestUnmarshalDupMapKeyToStructWrongTypeParseError(t *testing.T) {
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`
	}
	data := mustHexDecode("a68161fe614161626142616361438161fe61466164614461656145") // {["\xFE"]: "A", "b": "B", "c": "C", ["\xFE"]: "F", "d": "D", "e": "E"}

	// Duplicate key doesn't overwrite previous value (default).
	wantS := s{A: "", B: "B", C: "C", D: "D", E: "E"}
	wantErrorMsg := "cbor: cannot unmarshal"
	var s1 s
	if err := Unmarshal(data, &s1); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s1, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
	}

	// Duplicate key triggers error.
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var s2 s
	if err := dm.Unmarshal(data, &s2); err == nil {
		t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, reflect.TypeOf(s2))
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s2, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
	}
}

func TestUnmarshalDupMapKeyToStructWrongTypeUnhashableError(t *testing.T) {
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`
	}
	data := mustHexDecode("a6810061416162614261636143810061466164614461656145") // {[0]: "A", "b": "B", "c": "C", [0]: "F", "d": "D", "e": "E"}
	wantS := s{A: "", B: "B", C: "C", D: "D", E: "E"}

	// Duplicate key doesn't overwrite previous value (default).
	wantErrorMsg := "cbor: cannot unmarshal"
	var s1 s
	if err := Unmarshal(data, &s1); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s1, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
	}

	// Duplicate key triggers error.
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var s2 s
	if err := dm.Unmarshal(data, &s2); err == nil {
		t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, reflect.TypeOf(s2))
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s2, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
	}
}

func TestUnmarshalDupMapKeyToStructTagTypeError(t *testing.T) {
	type s struct {
		A string `cbor:"a"`
		B string `cbor:"b"`
		C string `cbor:"c"`
		D string `cbor:"d"`
		E string `cbor:"e"`
	}
	data := mustHexDecode("a6c24901000000000000000061416162614261636143c24901000000000000000061466164614461656145") // {bignum(18446744073709551616): "A", "b": "B", "c": "C", bignum(18446744073709551616): "F", "d": "D", "e": "E"}
	wantS := s{A: "", B: "B", C: "C", D: "D", E: "E"}

	// Duplicate key doesn't overwrite previous value (default).
	wantErrorMsg := "cbor: cannot unmarshal"
	var s1 s
	if err := Unmarshal(data, &s1); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s1, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s1, s1, wantS, wantS)
	}

	// Duplicate key triggers error.
	dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
	var s2 s
	if err := dm.Unmarshal(data, &s2); err == nil {
		t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, reflect.TypeOf(s2))
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), wantErrorMsg)
	}
	if !reflect.DeepEqual(s2, wantS) {
		t.Errorf("Unmarshal(0x%x) = %+v (%T), want %+v (%T)", data, s2, s2, wantS, wantS)
	}
}

func TestIndefiniteLengthArrayToArray(t *testing.T) {
	testCases := []struct {
		name  string
		data  []byte
		wantV any
	}{
		{
			name:  "CBOR empty array to Go 5 elem array",
			data:  mustHexDecode("9fff"),
			wantV: [5]byte{},
		},
		{
			name:  "CBOR 3 elem array to Go 5 elem array",
			data:  mustHexDecode("9f010203ff"),
			wantV: [5]byte{1, 2, 3, 0, 0},
		},
		{
			name:  "CBOR 10 elem array to Go 5 elem array",
			data:  mustHexDecode("9f0102030405060708090aff"),
			wantV: [5]byte{1, 2, 3, 4, 5},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v := reflect.New(reflect.TypeOf(tc.wantV))
			if err := Unmarshal(tc.data, v.Interface()); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
			}
			if !reflect.DeepEqual(v.Elem().Interface(), tc.wantV) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.data, v.Elem().Interface(), v.Elem().Interface(), tc.wantV, tc.wantV)
			}
		})
	}
}

func TestExceedMaxArrayElements(t *testing.T) {
	testCases := []struct {
		name         string
		opts         DecOptions
		data         []byte
		wantErrorMsg string
	}{
		{
			name:         "array",
			opts:         DecOptions{MaxArrayElements: 16},
			data:         mustHexDecode("910101010101010101010101010101010101"),
			wantErrorMsg: "cbor: exceeded max number of elements 16 for CBOR array",
		},
		{
			name:         "indefinite length array",
			opts:         DecOptions{MaxArrayElements: 16},
			data:         mustHexDecode("9f0101010101010101010101010101010101ff"),
			wantErrorMsg: "cbor: exceeded max number of elements 16 for CBOR array",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dm, _ := tc.opts.DecMode()
			var v any
			if err := dm.Unmarshal(tc.data, &v); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error", tc.data)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestExceedMaxMapPairs(t *testing.T) {
	testCases := []struct {
		name         string
		opts         DecOptions
		data         []byte
		wantErrorMsg string
	}{
		{
			name:         "array",
			opts:         DecOptions{MaxMapPairs: 16},
			data:         mustHexDecode("b101010101010101010101010101010101010101010101010101010101010101010101"),
			wantErrorMsg: "cbor: exceeded max number of key-value pairs 16 for CBOR map",
		},
		{
			name:         "indefinite length array",
			opts:         DecOptions{MaxMapPairs: 16},
			data:         mustHexDecode("bf01010101010101010101010101010101010101010101010101010101010101010101ff"),
			wantErrorMsg: "cbor: exceeded max number of key-value pairs 16 for CBOR map",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dm, _ := tc.opts.DecMode()
			var v any
			if err := dm.Unmarshal(tc.data, &v); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error", tc.data)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecIndefiniteLengthOption(t *testing.T) {
	testCases := []struct {
		name         string
		opts         DecOptions
		data         []byte
		wantErrorMsg string
	}{
		{
			name:         "byte string",
			opts:         DecOptions{IndefLength: IndefLengthForbidden},
			data:         mustHexDecode("5fff"),
			wantErrorMsg: "cbor: indefinite-length byte string isn't allowed",
		},
		{
			name:         "text string",
			opts:         DecOptions{IndefLength: IndefLengthForbidden},
			data:         mustHexDecode("7fff"),
			wantErrorMsg: "cbor: indefinite-length UTF-8 text string isn't allowed",
		},
		{
			name:         "array",
			opts:         DecOptions{IndefLength: IndefLengthForbidden},
			data:         mustHexDecode("9fff"),
			wantErrorMsg: "cbor: indefinite-length array isn't allowed",
		},
		{
			name:         "indefinite length array",
			opts:         DecOptions{IndefLength: IndefLengthForbidden},
			data:         mustHexDecode("bfff"),
			wantErrorMsg: "cbor: indefinite-length map isn't allowed",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Default option allows indefinite length items
			var v any
			if err := Unmarshal(tc.data, &v); err != nil {
				t.Errorf("Unmarshal(0x%x) returned an error %v", tc.data, err)
			}

			dm, _ := tc.opts.DecMode()
			if err := dm.Unmarshal(tc.data, &v); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error", tc.data)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecTagsMdOption(t *testing.T) {
	data := mustHexDecode("c074323031332d30332d32315432303a30343a30305a")
	wantErrorMsg := "cbor: CBOR tag isn't allowed"

	// Default option allows CBOR tags
	var v any
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned an error %v", data, err)
	}

	// Decoding CBOR tags with TagsForbidden option returns error
	dm, _ := DecOptions{TagsMd: TagsForbidden}.DecMode()
	if err := dm.Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if err.Error() != wantErrorMsg {
		t.Errorf("Unmarshal(0x%x) returned error %q, want %q", data, err.Error(), wantErrorMsg)
	}

	// Create DecMode with TagSet and TagsForbidden option returns error
	wantErrorMsg = "cbor: cannot create DecMode with TagSet when TagsMd is TagsForbidden"
	tags := NewTagSet()
	_, err := DecOptions{TagsMd: TagsForbidden}.DecModeWithTags(tags)
	if err == nil {
		t.Errorf("DecModeWithTags() didn't return an error")
	} else if err.Error() != wantErrorMsg {
		t.Errorf("DecModeWithTags() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
	_, err = DecOptions{TagsMd: TagsForbidden}.DecModeWithSharedTags(tags)
	if err == nil {
		t.Errorf("DecModeWithSharedTags() didn't return an error")
	} else if err.Error() != wantErrorMsg {
		t.Errorf("DecModeWithSharedTags() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
}

func TestDecModeInvalidIntDec(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{IntDec: -1},
			wantErrorMsg: "cbor: invalid IntDec -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{IntDec: 101},
			wantErrorMsg: "cbor: invalid IntDec 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestIntDecConvertNone(t *testing.T) {
	dm, err := DecOptions{
		IntDec:    IntDecConvertNone,
		BigIntDec: BigIntDecodePointer,
	}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}

	testCases := []struct {
		name    string
		data    []byte
		wantObj any
	}{
		{
			name:    "CBOR pos int",
			data:    mustHexDecode("1a000f4240"),
			wantObj: uint64(1000000),
		},
		{
			name:    "CBOR pos int overflows int64",
			data:    mustHexDecode("1b8000000000000000"), // math.MaxInt64+1
			wantObj: uint64(math.MaxInt64 + 1),
		},
		{
			name:    "CBOR neg int",
			data:    mustHexDecode("3903e7"),
			wantObj: int64(-1000),
		},
		{
			name:    "CBOR neg int overflows int64",
			data:    mustHexDecode("3b8000000000000000"), // math.MinInt64-1
			wantObj: new(big.Int).Sub(big.NewInt(math.MinInt64), big.NewInt(1)),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			err := dm.Unmarshal(tc.data, &v)
			if err == nil {
				if !reflect.DeepEqual(v, tc.wantObj) {
					t.Errorf("Unmarshal(0x%x) return %v (%T), want %v (%T)", tc.data, v, v, tc.wantObj, tc.wantObj)
				}
			} else {
				t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
			}
		})
	}
}

func TestIntDecConvertSigned(t *testing.T) {
	dm, err := DecOptions{
		IntDec:    IntDecConvertSigned,
		BigIntDec: BigIntDecodePointer,
	}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}

	testCases := []struct {
		name         string
		data         []byte
		wantObj      any
		wantErrorMsg string
	}{
		{
			name:    "CBOR pos int",
			data:    mustHexDecode("1a000f4240"),
			wantObj: int64(1000000),
		},
		{
			name:         "CBOR pos int overflows int64",
			data:         mustHexDecode("1b8000000000000000"), // math.MaxInt64+1
			wantErrorMsg: "9223372036854775808 overflows Go's int64",
		},
		{
			name:    "CBOR neg int",
			data:    mustHexDecode("3903e7"),
			wantObj: int64(-1000),
		},
		{
			name:    "CBOR neg int overflows int64",
			data:    mustHexDecode("3b8000000000000000"), // math.MinInt64-1
			wantObj: new(big.Int).Sub(big.NewInt(math.MinInt64), big.NewInt(1)),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			err := dm.Unmarshal(tc.data, &v)
			if err == nil {
				if tc.wantErrorMsg != "" {
					t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", tc.data, tc.wantErrorMsg)
				} else if !reflect.DeepEqual(v, tc.wantObj) {
					t.Errorf("Unmarshal(0x%x) return %v (%T), want %v (%T)", tc.data, v, v, tc.wantObj, tc.wantObj)
				}
			} else {
				if tc.wantErrorMsg == "" {
					t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
				}
			}
		})
	}
}

func TestIntDecConvertSignedOrBigInt(t *testing.T) {
	dm, err := DecOptions{
		IntDec:    IntDecConvertSignedOrBigInt,
		BigIntDec: BigIntDecodePointer,
	}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}

	testCases := []struct {
		name    string
		data    []byte
		wantObj any
	}{
		{
			name:    "CBOR pos int",
			data:    mustHexDecode("1a000f4240"),
			wantObj: int64(1000000),
		},
		{
			name:    "CBOR pos int overflows int64",
			data:    mustHexDecode("1b8000000000000000"),
			wantObj: new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(1)),
		},
		{
			name:    "CBOR neg int",
			data:    mustHexDecode("3903e7"),
			wantObj: int64(-1000),
		},
		{
			name:    "CBOR neg int overflows int64",
			data:    mustHexDecode("3b8000000000000000"),
			wantObj: new(big.Int).Sub(big.NewInt(math.MinInt64), big.NewInt(1)),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			err := dm.Unmarshal(tc.data, &v)
			if err == nil {
				if !reflect.DeepEqual(v, tc.wantObj) {
					t.Errorf("Unmarshal(0x%x) return %v (%T), want %v (%T)", tc.data, v, v, tc.wantObj, tc.wantObj)
				}
			} else {
				t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
			}
		})
	}
}

func TestIntDecConvertSignedOrError(t *testing.T) {
	dm, err := DecOptions{
		IntDec:    IntDecConvertSignedOrFail,
		BigIntDec: BigIntDecodePointer,
	}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}

	testCases := []struct {
		name         string
		data         []byte
		wantObj      any
		wantErrorMsg string
	}{
		{
			name:    "CBOR pos int",
			data:    mustHexDecode("1a000f4240"),
			wantObj: int64(1000000),
		},
		{
			name:         "CBOR pos int overflows int64",
			data:         mustHexDecode("1b8000000000000000"), // math.MaxInt64+1
			wantErrorMsg: "9223372036854775808 overflows Go's int64",
		},
		{
			name:    "CBOR neg int",
			data:    mustHexDecode("3903e7"),
			wantObj: int64(-1000),
		},
		{
			name:         "CBOR neg int overflows int64",
			data:         mustHexDecode("3b8000000000000000"), // math.MinInt64-1
			wantErrorMsg: "-9223372036854775809 overflows Go's int64",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			err := dm.Unmarshal(tc.data, &v)
			if err == nil {
				if tc.wantErrorMsg != "" {
					t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", tc.data, tc.wantErrorMsg)
				} else if !reflect.DeepEqual(v, tc.wantObj) {
					t.Errorf("Unmarshal(0x%x) return %v (%T), want %v (%T)", tc.data, v, v, tc.wantObj, tc.wantObj)
				}
			} else {
				if tc.wantErrorMsg == "" {
					t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
				}
			}
		})
	}
}

func TestDecModeInvalidMapKeyByteString(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{MapKeyByteString: -1},
			wantErrorMsg: "cbor: invalid MapKeyByteString -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{MapKeyByteString: 101},
			wantErrorMsg: "cbor: invalid MapKeyByteString 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestMapKeyByteString(t *testing.T) {
	bsForbiddenMode, err := DecOptions{MapKeyByteString: MapKeyByteStringForbidden}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}

	bsAllowedMode, err := DecOptions{MapKeyByteString: MapKeyByteStringAllowed}.DecMode()
	if err != nil {
		t.Errorf("DecMode() returned an error %+v", err)
	}

	testCases := []struct {
		name         string
		data         []byte
		wantObj      any
		wantErrorMsg string
		dm           DecMode
	}{
		{
			name:         "byte string map key with MapKeyByteStringForbidden",
			data:         mustHexDecode("a143abcdef187b"),
			wantErrorMsg: "cbor: invalid map key type: []uint8",
			dm:           bsForbiddenMode,
		},
		{
			name:         "tagged byte string map key with MapKeyByteStringForbidden",
			data:         mustHexDecode("a1d86443abcdef187b"),
			wantErrorMsg: "cbor: invalid map key type: cbor.Tag",
			dm:           bsForbiddenMode,
		},
		{
			name:         "nested tagged byte string map key with MapKeyByteStringForbidden",
			data:         mustHexDecode("a1d865d86443abcdef187b"),
			wantErrorMsg: "cbor: invalid map key type: cbor.Tag",
			dm:           bsForbiddenMode,
		},
		{
			name: "byte string map key with MapKeyByteStringAllowed",
			data: mustHexDecode("a143abcdef187b"),
			wantObj: map[any]any{
				ByteString("\xab\xcd\xef"): uint64(123),
			},
			dm: bsAllowedMode,
		},
		{
			name: "tagged byte string map key with MapKeyByteStringAllowed",
			data: mustHexDecode("a1d86443abcdef187b"),
			wantObj: map[any]any{
				Tag{Number: 100, Content: ByteString("\xab\xcd\xef")}: uint64(123),
			},
			dm: bsAllowedMode,
		},
		{
			name: "nested tagged byte string map key with MapKeyByteStringAllowed",
			data: mustHexDecode("a1d865d86443abcdef187b"),
			wantObj: map[any]any{
				Tag{Number: 101, Content: Tag{Number: 100, Content: ByteString("\xab\xcd\xef")}}: uint64(123),
			},
			dm: bsAllowedMode,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, typ := range []reflect.Type{typeIntf, typeMapIntfIntf} {
				v := reflect.New(typ)
				vPtr := v.Interface()
				err = tc.dm.Unmarshal(tc.data, vPtr)
				if err == nil {
					if tc.wantErrorMsg != "" {
						t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", tc.data, tc.wantErrorMsg)
					} else if !reflect.DeepEqual(v.Elem().Interface(), tc.wantObj) {
						t.Errorf("Unmarshal(0x%x) return %v (%T), want %v (%T)", tc.data, v.Elem().Interface(), v.Elem().Interface(), tc.wantObj, tc.wantObj)
					}
				} else {
					if tc.wantErrorMsg == "" {
						t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
					} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
						t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
					}
				}
			}
		})
	}
}

func TestDecModeInvalidExtraError(t *testing.T) {
	wantErrorMsg := "cbor: invalid ExtraReturnErrors 3"
	_, err := DecOptions{ExtraReturnErrors: 3}.DecMode()
	if err == nil {
		t.Errorf("DecMode() didn't return an error")
	} else if err.Error() != wantErrorMsg {
		t.Errorf("DecMode() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
}

func TestExtraErrorCondUnknownField(t *testing.T) {
	type s struct {
		A string
		B string
		C string
	}

	dm, _ := DecOptions{}.DecMode()
	dmUnknownFieldError, _ := DecOptions{ExtraReturnErrors: ExtraDecErrorUnknownField}.DecMode()

	testCases := []struct {
		name         string
		data         []byte
		dm           DecMode
		wantObj      any
		wantErrorMsg string
	}{
		{
			name:    "field by field match",
			data:    mustHexDecode("a3614161616142616261436163"), // map[string]string{"A": "a", "B": "b", "C": "c"}
			dm:      dm,
			wantObj: s{A: "a", B: "b", C: "c"},
		},
		{
			name:    "field by field match with ExtraDecErrorUnknownField",
			data:    mustHexDecode("a3614161616142616261436163"), // map[string]string{"A": "a", "B": "b", "C": "c"}
			dm:      dmUnknownFieldError,
			wantObj: s{A: "a", B: "b", C: "c"},
		},
		{
			name:    "CBOR map less field",
			data:    mustHexDecode("a26141616161426162"), // map[string]string{"A": "a", "B": "b"}
			dm:      dm,
			wantObj: s{A: "a", B: "b", C: ""},
		},
		{
			name:    "CBOR map less field with ExtraDecErrorUnknownField",
			data:    mustHexDecode("a26141616161426162"), // map[string]string{"A": "a", "B": "b"}
			dm:      dmUnknownFieldError,
			wantObj: s{A: "a", B: "b", C: ""},
		},
		{
			name:    "duplicate map keys matching known field with ExtraDecErrorUnknownField",
			data:    mustHexDecode("a26141616161416141"), // map[string]string{"A": "a", "A": "A"}
			dm:      dmUnknownFieldError,
			wantObj: s{A: "a"},
		},
		{
			name:    "CBOR map unknown field",
			data:    mustHexDecode("a461416161614261626143616361446164"), // map[string]string{"A": "a", "B": "b", "C": "c", "D": "d"}
			dm:      dm,
			wantObj: s{A: "a", B: "b", C: "c"},
		},
		{
			name:         "CBOR map unknown field with ExtraDecErrorUnknownField",
			data:         mustHexDecode("a461416161614261626143616361446164"), // map[string]string{"A": "a", "B": "b", "C": "c", "D": "d"}
			dm:           dmUnknownFieldError,
			wantErrorMsg: "cbor: found unknown field at map element index 3",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v s
			err := tc.dm.Unmarshal(tc.data, &v)
			if err == nil {
				if tc.wantErrorMsg != "" {
					t.Errorf("Unmarshal(0x%x) didn't return an error, want %q", tc.data, tc.wantErrorMsg)
				} else if !reflect.DeepEqual(v, tc.wantObj) {
					t.Errorf("Unmarshal(0x%x) return %v (%T), want %v (%T)", tc.data, v, v, tc.wantObj, tc.wantObj)
				}
			} else {
				if tc.wantErrorMsg == "" {
					t.Errorf("Unmarshal(0x%x) returned error %q", tc.data, err)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
				}
			}
		})
	}
}

func TestInvalidUTF8Mode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{UTF8: -1},
			wantErrorMsg: "cbor: invalid UTF8 -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{UTF8: 101},
			wantErrorMsg: "cbor: invalid UTF8 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestStreamExtraErrorCondUnknownField(t *testing.T) {
	type s struct {
		A string
		B string
		C string
	}

	data := mustHexDecode("a461416161614461646142616261436163a3614161616142616261436163") // map[string]string{"A": "a", "D": "d", "B": "b", "C": "c"}, map[string]string{"A": "a", "B": "b", "C": "c"}
	wantErrorMsg := "cbor: found unknown field at map element index 1"
	wantObj := s{A: "a", B: "b", C: "c"}

	dmUnknownFieldError, _ := DecOptions{ExtraReturnErrors: ExtraDecErrorUnknownField}.DecMode()
	dec := dmUnknownFieldError.NewDecoder(bytes.NewReader(data))

	var v1 s
	err := dec.Decode(&v1)
	if err == nil {
		t.Errorf("Decode() didn't return an error, want %q", wantErrorMsg)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Decode() returned error %q, want %q", err.Error(), wantErrorMsg)
	}

	var v2 s
	err = dec.Decode(&v2)
	if err != nil {
		t.Errorf("Decode() returned an error %v", err)
	} else if !reflect.DeepEqual(v2, wantObj) {
		t.Errorf("Decode() return %v (%T), want %v (%T)", v2, v2, wantObj, wantObj)
	}
}

// TestUnmarshalTagNum55799 is identical to TestUnmarshal,
// except that CBOR test data is prefixed with tag number 55799 (0xd9d9f7).
func TestUnmarshalTagNum55799(t *testing.T) {
	tagNum55799 := mustHexDecode("d9d9f7")

	for _, tc := range unmarshalTests {
		// Prefix tag number 55799 to CBOR test data
		data := make([]byte, len(tc.data)+6)
		copy(data, tagNum55799)
		copy(data[3:], tagNum55799)
		copy(data[6:], tc.data)

		// Test unmarshaling CBOR into empty interface.
		var v any
		if err := Unmarshal(data, &v); err != nil {
			t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		} else {
			if tm, ok := tc.wantInterfaceValue.(time.Time); ok {
				if vt, ok := v.(time.Time); !ok || !tm.Equal(vt) {
					t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
				}
			} else if !reflect.DeepEqual(v, tc.wantInterfaceValue) {
				t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
			}
		}

		// Test unmarshaling CBOR into RawMessage.
		var r RawMessage
		if err := Unmarshal(data, &r); err != nil {
			t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		} else if !bytes.Equal(r, tc.data) {
			t.Errorf("Unmarshal(0x%x) returned RawMessage %v, want %v", data, r, tc.data)
		}

		// Test unmarshaling CBOR into compatible data types.
		for _, value := range tc.wantValues {
			v := reflect.New(reflect.TypeOf(value))
			vPtr := v.Interface()
			if err := Unmarshal(data, vPtr); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
			} else {
				if tm, ok := value.(time.Time); ok {
					if vt, ok := v.Elem().Interface().(time.Time); !ok || !tm.Equal(vt) {
						t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v.Elem().Interface(), v.Elem().Interface(), value, value)
					}
				} else if !reflect.DeepEqual(v.Elem().Interface(), value) {
					t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v.Elem().Interface(), v.Elem().Interface(), value, value)
				}
			}
		}

		// Test unmarshaling CBOR into incompatible data types.
		for _, typ := range tc.wrongTypes {
			v := reflect.New(typ)
			vPtr := v.Interface()
			if err := Unmarshal(data, vPtr); err == nil {
				t.Errorf("Unmarshal(0x%x, %s) didn't return an error", data, typ.String())
			} else if _, ok := err.(*UnmarshalTypeError); !ok {
				t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", data, err)
			} else if !strings.Contains(err.Error(), "cannot unmarshal") {
				t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", data, err.Error(), "cannot unmarshal")
			}
		}
	}
}

// TestUnmarshalFloatWithTagNum55799 is identical to TestUnmarshalFloat,
// except that CBOR test data is prefixed with tag number 55799 (0xd9d9f7).
func TestUnmarshalFloatWithTagNum55799(t *testing.T) {
	tagNum55799 := mustHexDecode("d9d9f7")

	for _, tc := range unmarshalFloatTests {
		// Prefix tag number 55799 to CBOR test data
		data := make([]byte, len(tc.data)+3)
		copy(data, tagNum55799)
		copy(data[3:], tc.data)

		// Test unmarshaling CBOR into empty interface.
		var v any
		if err := Unmarshal(tc.data, &v); err != nil {
			t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
		} else {
			compareFloats(t, tc.data, v, tc.wantInterfaceValue, tc.equalityThreshold)
		}

		// Test unmarshaling CBOR into RawMessage.
		var r RawMessage
		if err := Unmarshal(tc.data, &r); err != nil {
			t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
		} else if !bytes.Equal(r, tc.data) {
			t.Errorf("Unmarshal(0x%x) returned RawMessage %v, want %v", tc.data, r, tc.data)
		}

		// Test unmarshaling CBOR into compatible data types.
		for _, value := range tc.wantValues {
			v := reflect.New(reflect.TypeOf(value))
			vPtr := v.Interface()
			if err := Unmarshal(tc.data, vPtr); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
			} else {
				compareFloats(t, tc.data, v.Elem().Interface(), value, tc.equalityThreshold)
			}
		}

		// Test unmarshaling CBOR into incompatible data types.
		for _, typ := range unmarshalFloatWrongTypes {
			v := reflect.New(typ)
			vPtr := v.Interface()
			if err := Unmarshal(tc.data, vPtr); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error", tc.data)
			} else if _, ok := err.(*UnmarshalTypeError); !ok {
				t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", tc.data, err)
			} else if !strings.Contains(err.Error(), "cannot unmarshal") {
				t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", tc.data, err.Error(), "cannot unmarshal")
			}
		}
	}
}

func TestUnmarshalTagNum55799AsElement(t *testing.T) {
	testCases := []struct {
		name                string
		data                []byte
		emptyInterfaceValue any
		values              []any
		wrongTypes          []reflect.Type
	}{
		{
			name:                "array",
			data:                mustHexDecode("d9d9f783d9d9f701d9d9f702d9d9f703"), // 55799([55799(1), 55799(2), 55799(3)])
			emptyInterfaceValue: []any{uint64(1), uint64(2), uint64(3)},
			values:              []any{[]any{uint64(1), uint64(2), uint64(3)}, []byte{1, 2, 3}, []int{1, 2, 3}, []uint{1, 2, 3}, [0]int{}, [1]int{1}, [3]int{1, 2, 3}, [5]int{1, 2, 3, 0, 0}, []float32{1, 2, 3}, []float64{1, 2, 3}},
			wrongTypes:          []reflect.Type{typeUint8, typeUint16, typeUint32, typeUint64, typeInt8, typeInt16, typeInt32, typeInt64, typeFloat32, typeFloat64, typeString, typeBool, typeStringSlice, typeMapStringInt, reflect.TypeOf([3]string{}), typeTag, typeRawTag},
		},
		{
			name:                "map",
			data:                mustHexDecode("d9d9f7a2d9d9f701d9d9f702d9d9f703d9d9f704"), // 55799({55799(1): 55799(2), 55799(3): 55799(4)})
			emptyInterfaceValue: map[any]any{uint64(1): uint64(2), uint64(3): uint64(4)},
			values:              []any{map[any]any{uint64(1): uint64(2), uint64(3): uint64(4)}, map[uint]int{1: 2, 3: 4}, map[int]uint{1: 2, 3: 4}},
			wrongTypes:          []reflect.Type{typeUint8, typeUint16, typeUint32, typeUint64, typeInt8, typeInt16, typeInt32, typeInt64, typeFloat32, typeFloat64, typeByteSlice, typeByteArray, typeString, typeBool, typeIntSlice, typeMapStringInt, typeTag, typeRawTag},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test unmarshaling CBOR into empty interface.
			var v any
			if err := Unmarshal(tc.data, &v); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
			} else {
				if tm, ok := tc.emptyInterfaceValue.(time.Time); ok {
					if vt, ok := v.(time.Time); !ok || !tm.Equal(vt) {
						t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.data, v, v, tc.emptyInterfaceValue, tc.emptyInterfaceValue)
					}
				} else if !reflect.DeepEqual(v, tc.emptyInterfaceValue) {
					t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.data, v, v, tc.emptyInterfaceValue, tc.emptyInterfaceValue)
				}
			}

			// Test unmarshaling CBOR into compatible data types.
			for _, value := range tc.values {
				v := reflect.New(reflect.TypeOf(value))
				vPtr := v.Interface()
				if err := Unmarshal(tc.data, vPtr); err != nil {
					t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
				} else {
					if tm, ok := value.(time.Time); ok {
						if vt, ok := v.Elem().Interface().(time.Time); !ok || !tm.Equal(vt) {
							t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.data, v.Elem().Interface(), v.Elem().Interface(), value, value)
						}
					} else if !reflect.DeepEqual(v.Elem().Interface(), value) {
						t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.data, v.Elem().Interface(), v.Elem().Interface(), value, value)
					}
				}
			}

			// Test unmarshaling CBOR into incompatible data types.
			for _, typ := range tc.wrongTypes {
				v := reflect.New(typ)
				vPtr := v.Interface()
				if err := Unmarshal(tc.data, vPtr); err == nil {
					t.Errorf("Unmarshal(0x%x, %s) didn't return an error", tc.data, typ.String())
				} else if _, ok := err.(*UnmarshalTypeError); !ok {
					t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*UnmarshalTypeError)", tc.data, err)
				} else if !strings.Contains(err.Error(), "cannot unmarshal") {
					t.Errorf("Unmarshal(0x%x) returned error %q, want error containing %q", tc.data, err.Error(), "cannot unmarshal")
				}
			}
		})
	}
}

func TestUnmarshalTagNum55799ToBinaryUnmarshaler(t *testing.T) {
	data := mustHexDecode("d9d9f74800000000499602d2") // 55799(h'00000000499602D2')
	wantObj := number(1234567890)

	var v number
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(v, wantObj) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantObj, wantObj)
	}
}

func TestUnmarshalTagNum55799ToUnmarshaler(t *testing.T) {
	data := mustHexDecode("d9d9f7d864a1636e756d01") // 55799(100({"num": 1}))
	wantObj := number3(1)

	var v number3
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(v, wantObj) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantObj, wantObj)
	}
}

func TestUnmarshalTagNum55799ToRegisteredGoType(t *testing.T) {
	type myInt int
	typ := reflect.TypeOf(myInt(0))

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, typ, 125); err != nil {
		t.Fatalf("TagSet.Add(%s, %v) returned error %v", typ, 125, err)
	}

	dm, _ := DecOptions{}.DecModeWithTags(tags)

	data := mustHexDecode("d9d9f7d87d01") // 55799(125(1))
	wantObj := myInt(1)

	var v myInt
	if err := dm.Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(v, wantObj) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantObj, wantObj)
	}
}

// TODO: wait for clarification from 7049bis https://github.com/cbor-wg/CBORbis/issues/183
// Nested tag number 55799 may be stripeed as well depending on 7049bis clarification.
func TestUnmarshalNestedTagNum55799ToEmptyInterface(t *testing.T) {
	data := mustHexDecode("d864d9d9f701") // 100(55799(1))
	wantObj := Tag{100, Tag{55799, uint64(1)}}

	var v any
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(v, wantObj) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantObj, wantObj)
	}
}

func TestUnmarshalNestedTagNum55799ToValue(t *testing.T) {
	data := mustHexDecode("d864d9d9f701") // 100(55799(1))
	wantObj := 1

	var v int
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(v, wantObj) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantObj, wantObj)
	}
}

func TestUnmarshalNestedTagNum55799ToTag(t *testing.T) {
	data := mustHexDecode("d864d9d9f701") // 100(55799(1))
	wantObj := Tag{100, Tag{55799, uint64(1)}}

	var v Tag
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(v, wantObj) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantObj, wantObj)
	}
}

func TestUnmarshalNestedTagNum55799ToTime(t *testing.T) {
	data := mustHexDecode("c0d9d9f774323031332d30332d32315432303a30343a30305a") // 0(55799("2013-03-21T20:04:00Z"))
	wantErrorMsg := "tag number 0 must be followed by text string, got tag"

	var v time.Time
	if err := Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return error", data)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %s, want %s", data, err.Error(), wantErrorMsg)
	}
}

func TestUnmarshalNestedTagNum55799ToBinaryUnmarshaler(t *testing.T) {
	data := mustHexDecode("d864d9d9f74800000000499602d2") // 100(55799(h'00000000499602D2'))
	wantObj := number(1234567890)

	var v number
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(v, wantObj) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, wantObj, wantObj)
	}
}

func TestUnmarshalNestedTagNum55799ToUnmarshaler(t *testing.T) {
	data := mustHexDecode("d864d9d9f7a1636e756d01") // 100(55799({"num": 1}))
	wantErrorMsg := "wrong tag content type"

	var v number3
	if err := Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return error", data)
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %s, want %s", data, err.Error(), wantErrorMsg)
	}
}

func TestUnmarshalNestedTagNum55799ToRegisteredGoType(t *testing.T) {
	type myInt int
	typ := reflect.TypeOf(myInt(0))

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, typ, 125); err != nil {
		t.Fatalf("TagSet.Add(%s, %v) returned error %v", typ, 125, err)
	}

	dm, _ := DecOptions{}.DecModeWithTags(tags)

	data := mustHexDecode("d87dd9d9f701") // 125(55799(1))
	wantErrorMsg := "cbor: wrong tag number for cbor.myInt, got [125 55799], expected [125]"

	var v myInt
	if err := dm.Unmarshal(data, &v); err == nil {
		t.Errorf("Unmarshal() didn't return error")
	} else if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Unmarshal(0x%x) returned error %s, want %s", data, err.Error(), wantErrorMsg)
	}
}

func TestUnmarshalPosIntToBigInt(t *testing.T) {
	data := mustHexDecode("1bffffffffffffffff") // 18446744073709551615
	wantEmptyInterfaceValue := uint64(18446744073709551615)
	wantBigIntValue := mustBigInt("18446744073709551615")

	var v1 any
	if err := Unmarshal(data, &v1); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %+v", data, err)
	} else if !reflect.DeepEqual(v1, wantEmptyInterfaceValue) {
		t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", data, v1, v1, wantEmptyInterfaceValue, wantEmptyInterfaceValue)
	}

	var v2 big.Int
	if err := Unmarshal(data, &v2); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %+v", data, err)
	} else if !reflect.DeepEqual(v2, wantBigIntValue) {
		t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", data, v2, v2, wantBigIntValue, wantBigIntValue)
	}
}

func TestUnmarshalNegIntToBigInt(t *testing.T) {
	testCases := []struct {
		name                    string
		data                    []byte
		wantEmptyInterfaceValue any
		wantBigIntValue         big.Int
	}{
		{
			name:                    "fit Go int64",
			data:                    mustHexDecode("3b7fffffffffffffff"), // -9223372036854775808
			wantEmptyInterfaceValue: int64(-9223372036854775808),
			wantBigIntValue:         mustBigInt("-9223372036854775808"),
		},
		{
			name:                    "overflow Go int64",
			data:                    mustHexDecode("3b8000000000000000"), // -9223372036854775809
			wantEmptyInterfaceValue: mustBigInt("-9223372036854775809"),
			wantBigIntValue:         mustBigInt("-9223372036854775809"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v1 any
			if err := Unmarshal(tc.data, &v1); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %+v", tc.data, err)
			} else if !reflect.DeepEqual(v1, tc.wantEmptyInterfaceValue) {
				t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, v1, v1, tc.wantEmptyInterfaceValue, tc.wantEmptyInterfaceValue)
			}

			var v2 big.Int
			if err := Unmarshal(tc.data, &v2); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %+v", tc.data, err)
			} else if !reflect.DeepEqual(v2, tc.wantBigIntValue) {
				t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, v2, v2, tc.wantBigIntValue, tc.wantBigIntValue)
			}
		})
	}
}

func TestUnmarshalTag2(t *testing.T) {
	testCases := []struct {
		name                    string
		data                    []byte
		wantEmptyInterfaceValue any
		wantValues              []any
	}{
		{
			name:                    "fit Go int64",
			data:                    mustHexDecode("c2430f4240"), // 2(1000000)
			wantEmptyInterfaceValue: mustBigInt("1000000"),
			wantValues: []any{
				int64(1000000),
				uint64(1000000),
				float32(1000000),
				float64(1000000),
				mustBigInt("1000000"),
			},
		},
		{
			name:                    "fit Go uint64",
			data:                    mustHexDecode("c248ffffffffffffffff"), // 2(18446744073709551615)
			wantEmptyInterfaceValue: mustBigInt("18446744073709551615"),
			wantValues: []any{
				uint64(18446744073709551615),
				float32(18446744073709551615),
				float64(18446744073709551615),
				mustBigInt("18446744073709551615"),
			},
		},
		{
			name:                    "fit Go uint64 with leading zeros",
			data:                    mustHexDecode("c24900ffffffffffffffff"), // 2(18446744073709551615)
			wantEmptyInterfaceValue: mustBigInt("18446744073709551615"),
			wantValues: []any{
				uint64(18446744073709551615),
				float32(18446744073709551615),
				float64(18446744073709551615),
				mustBigInt("18446744073709551615"),
			},
		},
		{
			name:                    "overflow Go uint64",
			data:                    mustHexDecode("c249010000000000000000"), // 2(18446744073709551616)
			wantEmptyInterfaceValue: mustBigInt("18446744073709551616"),
			wantValues: []any{
				mustBigInt("18446744073709551616"),
			},
		},
		{
			name:                    "overflow Go uint64 with leading zeros",
			data:                    mustHexDecode("c24b0000010000000000000000"), // 2(18446744073709551616)
			wantEmptyInterfaceValue: mustBigInt("18446744073709551616"),
			wantValues: []any{
				mustBigInt("18446744073709551616"),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v1 any
			if err := Unmarshal(tc.data, &v1); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %+v", tc.data, err)
			} else if !reflect.DeepEqual(v1, tc.wantEmptyInterfaceValue) {
				t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, v1, v1, tc.wantEmptyInterfaceValue, tc.wantEmptyInterfaceValue)
			}

			for _, wantValue := range tc.wantValues {
				v := reflect.New(reflect.TypeOf(wantValue))
				if err := Unmarshal(tc.data, v.Interface()); err != nil {
					t.Errorf("Unmarshal(0x%x) returned error %+v", tc.data, err)
				} else if !reflect.DeepEqual(v.Elem().Interface(), wantValue) {
					t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, v.Elem().Interface(), v.Elem().Interface(), wantValue, wantValue)
				}
			}
		})
	}
}

func TestUnmarshalTag3(t *testing.T) {
	testCases := []struct {
		name                    string
		data                    []byte
		wantEmptyInterfaceValue any
		wantValues              []any
	}{
		{
			name:                    "fit Go int64",
			data:                    mustHexDecode("c3487fffffffffffffff"), // 3(-9223372036854775808)
			wantEmptyInterfaceValue: mustBigInt("-9223372036854775808"),
			wantValues: []any{
				int64(-9223372036854775808),
				float32(-9223372036854775808),
				float64(-9223372036854775808),
				mustBigInt("-9223372036854775808"),
			},
		},
		{
			name:                    "fit Go int64 with leading zeros",
			data:                    mustHexDecode("c349007fffffffffffffff"), // 3(-9223372036854775808)
			wantEmptyInterfaceValue: mustBigInt("-9223372036854775808"),
			wantValues: []any{
				int64(-9223372036854775808),
				float32(-9223372036854775808),
				float64(-9223372036854775808),
				mustBigInt("-9223372036854775808"),
			},
		},
		{
			name:                    "overflow Go int64",
			data:                    mustHexDecode("c349010000000000000000"), // 3(-18446744073709551617)
			wantEmptyInterfaceValue: mustBigInt("-18446744073709551617"),
			wantValues: []any{
				mustBigInt("-18446744073709551617"),
			},
		},
		{
			name:                    "overflow Go int64 with leading zeros",
			data:                    mustHexDecode("c34b0000010000000000000000"), // 3(-18446744073709551617)
			wantEmptyInterfaceValue: mustBigInt("-18446744073709551617"),
			wantValues: []any{
				mustBigInt("-18446744073709551617"),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var v1 any
			if err := Unmarshal(tc.data, &v1); err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %+v", tc.data, err)
			} else if !reflect.DeepEqual(v1, tc.wantEmptyInterfaceValue) {
				t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, v1, v1, tc.wantEmptyInterfaceValue, tc.wantEmptyInterfaceValue)
			}

			for _, wantValue := range tc.wantValues {
				v := reflect.New(reflect.TypeOf(wantValue))
				if err := Unmarshal(tc.data, v.Interface()); err != nil {
					t.Errorf("Unmarshal(0x%x) returned error %+v", tc.data, err)
				} else if !reflect.DeepEqual(v.Elem().Interface(), wantValue) {
					t.Errorf("Unmarshal(0x%x) returned %v (%T), want %v (%T)", tc.data, v.Elem().Interface(), v.Elem().Interface(), wantValue, wantValue)
				}
			}
		})
	}
}

func TestUnmarshalInvalidTagBignum(t *testing.T) {
	typeBigIntSlice := reflect.TypeOf([]big.Int{})

	testCases := []struct {
		name          string
		data          []byte
		decodeToTypes []reflect.Type
		wantErrorMsg  string
	}{
		{
			name:          "Tag 2 with string",
			data:          mustHexDecode("c27f657374726561646d696e67ff"),
			decodeToTypes: []reflect.Type{typeIntf, typeBigInt},
			wantErrorMsg:  "cbor: tag number 2 or 3 must be followed by byte string, got UTF-8 text string",
		},
		{
			name:          "Tag 3 with string",
			data:          mustHexDecode("c37f657374726561646d696e67ff"),
			decodeToTypes: []reflect.Type{typeIntf, typeBigInt},
			wantErrorMsg:  "cbor: tag number 2 or 3 must be followed by byte string, got UTF-8 text string",
		},
		{
			name:          "Tag 3 with negavtive int",
			data:          mustHexDecode("81C330"), // [3(-17)]
			decodeToTypes: []reflect.Type{typeIntf, typeBigIntSlice},
			wantErrorMsg:  "cbor: tag number 2 or 3 must be followed by byte string, got negative integer",
		},
	}
	for _, tc := range testCases {
		for _, decodeToType := range tc.decodeToTypes {
			t.Run(tc.name+" decode to "+decodeToType.String(), func(t *testing.T) {
				v := reflect.New(decodeToType)
				if err := Unmarshal(tc.data, v.Interface()); err == nil {
					t.Errorf("Unmarshal(0x%x) didn't return error, want error msg %q", tc.data, tc.wantErrorMsg)
				} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
					t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.data, err, tc.wantErrorMsg)
				}
			})
		}
	}
}

type Foo interface {
	Foo() string
}

type UintFoo uint

func (f *UintFoo) Foo() string {
	return fmt.Sprint(f)
}

type IntFoo int

func (f *IntFoo) Foo() string {
	return fmt.Sprint(*f)
}

type ByteFoo []byte

func (f *ByteFoo) Foo() string {
	return fmt.Sprint(*f)
}

type StringFoo string

func (f *StringFoo) Foo() string {
	return string(*f)
}

type ArrayFoo []int

func (f *ArrayFoo) Foo() string {
	return fmt.Sprint(*f)
}

type MapFoo map[int]int

func (f *MapFoo) Foo() string {
	return fmt.Sprint(*f)
}

type StructFoo struct {
	Value int `cbor:"1,keyasint"`
}

func (f *StructFoo) Foo() string {
	return fmt.Sprint(*f)
}

type TestExample struct {
	Message string `cbor:"1,keyasint"`
	Foo     Foo    `cbor:"2,keyasint"`
}

func TestUnmarshalToInterface(t *testing.T) {

	uintFoo, uintFoo123 := UintFoo(0), UintFoo(123)
	intFoo, intFooNeg1 := IntFoo(0), IntFoo(-1)
	byteFoo, byteFoo123 := ByteFoo(nil), ByteFoo([]byte{1, 2, 3})
	stringFoo, stringFoo123 := StringFoo(""), StringFoo("123")
	arrayFoo, arrayFoo123 := ArrayFoo(nil), ArrayFoo([]int{1, 2, 3})
	mapFoo, mapFoo123 := MapFoo(nil), MapFoo(map[int]int{1: 1, 2: 2, 3: 3})

	em, _ := EncOptions{Sort: SortCanonical}.EncMode()

	testCases := []struct {
		name           string
		data           []byte
		v              *TestExample
		unmarshalToObj *TestExample
	}{
		{
			name: "uint",
			data: mustHexDecode("a2016c736f6d65206d65737361676502187b"), // {1: "some message", 2: 123}
			v: &TestExample{
				Message: "some message",
				Foo:     &uintFoo123,
			},
			unmarshalToObj: &TestExample{Foo: &uintFoo},
		},
		{
			name: "int",
			data: mustHexDecode("a2016c736f6d65206d6573736167650220"), // {1: "some message", 2: -1}
			v: &TestExample{
				Message: "some message",
				Foo:     &intFooNeg1,
			},
			unmarshalToObj: &TestExample{Foo: &intFoo},
		},
		{
			name: "bytes",
			data: mustHexDecode("a2016c736f6d65206d6573736167650243010203"), // {1: "some message", 2: [1,2,3]}
			v: &TestExample{
				Message: "some message",
				Foo:     &byteFoo123,
			},
			unmarshalToObj: &TestExample{Foo: &byteFoo},
		},
		{
			name: "string",
			data: mustHexDecode("a2016c736f6d65206d6573736167650263313233"), // {1: "some message", 2: "123"}
			v: &TestExample{
				Message: "some message",
				Foo:     &stringFoo123,
			},
			unmarshalToObj: &TestExample{Foo: &stringFoo},
		},
		{
			name: "array",
			data: mustHexDecode("a2016c736f6d65206d6573736167650283010203"), // {1: "some message", 2: []int{1,2,3}}
			v: &TestExample{
				Message: "some message",
				Foo:     &arrayFoo123,
			},
			unmarshalToObj: &TestExample{Foo: &arrayFoo},
		},
		{
			name: "map",
			data: mustHexDecode("a2016c736f6d65206d65737361676502a3010102020303"), // {1: "some message", 2: map[int]int{1:1,2:2,3:3}}
			v: &TestExample{
				Message: "some message",
				Foo:     &mapFoo123,
			},
			unmarshalToObj: &TestExample{Foo: &mapFoo},
		},
		{
			name: "struct",
			data: mustHexDecode("a2016c736f6d65206d65737361676502a1011901c8"), // {1: "some message", 2: {1: 456}}
			v: &TestExample{
				Message: "some message",
				Foo:     &StructFoo{Value: 456},
			},
			unmarshalToObj: &TestExample{Foo: &StructFoo{}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			data, err := em.Marshal(tc.v)
			if err != nil {
				t.Errorf("Marshal(%+v) returned error %v", tc.v, err)
			} else if !bytes.Equal(data, tc.data) {
				t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", tc.v, data, tc.data)
			}

			// Unmarshal to empty interface
			var einterface TestExample
			if err = Unmarshal(data, &einterface); err == nil {
				t.Errorf("Unmarshal(0x%x) didn't return an error, want error (*UnmarshalTypeError)", data)
			} else if _, ok := err.(*UnmarshalTypeError); !ok {
				t.Errorf("Unmarshal(0x%x) returned wrong type of error %T, want (*UnmarshalTypeError)", data, err)
			}

			// Unmarshal to interface value
			err = Unmarshal(data, tc.unmarshalToObj)
			if err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
			} else if !reflect.DeepEqual(tc.unmarshalToObj, tc.v) {
				t.Errorf("Unmarshal(0x%x) = %v, want %v", data, tc.unmarshalToObj, tc.v)
			}
		})
	}
}

type Bar struct {
	I int
}

func (b *Bar) Foo() string {
	return fmt.Sprint(*b)
}

type FooStruct struct {
	Foos []Foo
}

func TestUnmarshalTaggedDataToInterface(t *testing.T) {

	var tags = NewTagSet()
	err := tags.Add(
		TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired},
		reflect.TypeOf(&Bar{}),
		4,
	)
	if err != nil {
		t.Error(err)
	}

	v := &FooStruct{
		Foos: []Foo{&Bar{1}},
	}

	want := mustHexDecode("a164466f6f7381c4a1614901") // {"Foos": [4({"I": 1})]}

	em, _ := EncOptions{}.EncModeWithTags(tags)
	data, err := em.Marshal(v)
	if err != nil {
		t.Errorf("Marshal(%+v) returned error %v", v, err)
	} else if !bytes.Equal(data, want) {
		t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", v, data, want)
	}

	dm, _ := DecOptions{}.DecModeWithTags(tags)

	// Unmarshal to empty interface
	var v1 Bar
	if err = dm.Unmarshal(data, &v1); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error, want error (*UnmarshalTypeError)", data)
	} else if _, ok := err.(*UnmarshalTypeError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong type of error %T, want (*UnmarshalTypeError)", data, err)
	}

	// Unmarshal to interface value
	v2 := &FooStruct{
		Foos: []Foo{&Bar{}},
	}
	err = dm.Unmarshal(data, v2)
	if err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
	} else if !reflect.DeepEqual(v2, v) {
		t.Errorf("Unmarshal(0x%x) = %v, want %v", data, v2, v)
	}
}

type B interface {
	Foo()
}

type C struct {
	Field int
}

func (c *C) Foo() {}

type D struct {
	Field string
}

func (d *D) Foo() {}

type A1 struct {
	Field B
}

type A2 struct {
	Fields []B
}

func TestUnmarshalRegisteredTagToInterface(t *testing.T) {
	var err error
	tags := NewTagSet()
	err = tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, reflect.TypeOf(C{}), 279)
	if err != nil {
		t.Error(err)
	}
	err = tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, reflect.TypeOf(D{}), 280)
	if err != nil {
		t.Error(err)
	}

	encMode, _ := PreferredUnsortedEncOptions().EncModeWithTags(tags)
	decMode, _ := DecOptions{}.DecModeWithTags(tags)

	v1 := A1{Field: &C{Field: 5}}
	data1, err := encMode.Marshal(v1)
	if err != nil {
		t.Fatalf("Marshal(%+v) returned error %v", v1, err)
	}

	v2 := A2{Fields: []B{&C{Field: 5}, &D{Field: "a"}}}
	data2, err := encMode.Marshal(v2)
	if err != nil {
		t.Fatalf("Marshal(%+v) returned error %v", v2, err)
	}

	testCases := []struct {
		name           string
		data           []byte
		unmarshalToObj any
		wantValue      any
	}{
		{
			name:           "interface type",
			data:           data1,
			unmarshalToObj: &A1{},
			wantValue:      &v1,
		},
		{
			name:           "concrete type",
			data:           data1,
			unmarshalToObj: &A1{Field: &C{}},
			wantValue:      &v1,
		},
		{
			name:           "slice of interface type",
			data:           data2,
			unmarshalToObj: &A2{},
			wantValue:      &v2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err = decMode.Unmarshal(tc.data, tc.unmarshalToObj)
			if err != nil {
				t.Errorf("Unmarshal(0x%x) returned error %v", tc.data, err)
			}
			if !reflect.DeepEqual(tc.unmarshalToObj, tc.wantValue) {
				t.Errorf("Unmarshal(0x%x) = %v, want %v", tc.data, tc.unmarshalToObj, tc.wantValue)
			}
		})
	}
}

func TestDecModeInvalidDefaultMapType(t *testing.T) {
	testCases := []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "byte slice",
			opts:         DecOptions{DefaultMapType: reflect.TypeOf([]byte(nil))},
			wantErrorMsg: "cbor: invalid DefaultMapType []uint8",
		},
		{
			name:         "int slice",
			opts:         DecOptions{DefaultMapType: reflect.TypeOf([]int(nil))},
			wantErrorMsg: "cbor: invalid DefaultMapType []int",
		},
		{
			name:         "string",
			opts:         DecOptions{DefaultMapType: reflect.TypeOf("")},
			wantErrorMsg: "cbor: invalid DefaultMapType string",
		},
		{
			name:         "unnamed struct type",
			opts:         DecOptions{DefaultMapType: reflect.TypeOf(struct{}{})},
			wantErrorMsg: "cbor: invalid DefaultMapType struct {}",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestUnmarshalToDefaultMapType(t *testing.T) {

	cborDataMapIntInt := mustHexDecode("a201020304")                                             // {1: 2, 3: 4}
	cborDataMapStringInt := mustHexDecode("a2616101616202")                                      // {"a": 1, "b": 2}
	cborDataArrayOfMapStringint := mustHexDecode("82a2616101616202a2616303616404")               // [{"a": 1, "b": 2}, {"c": 3, "d": 4}]
	cborDataNestedMap := mustHexDecode("a268496e744669656c6401684d61704669656c64a2616101616202") // {"IntField": 1, "MapField": {"a": 1, "b": 2}}

	decOptionsDefault := DecOptions{}
	decOptionsMapIntfIntfType := DecOptions{DefaultMapType: reflect.TypeOf(map[any]any(nil))}
	decOptionsMapStringIntType := DecOptions{DefaultMapType: reflect.TypeOf(map[string]int(nil))}
	decOptionsMapStringIntfType := DecOptions{DefaultMapType: reflect.TypeOf(map[string]any(nil))}

	testCases := []struct {
		name         string
		opts         DecOptions
		data         []byte
		wantValue    any
		wantErrorMsg string
	}{
		// Decode CBOR map to map[interface{}]interface{} using default options
		{
			name:      "decode CBOR map[int]int to Go map[interface{}]interface{} (default)",
			opts:      decOptionsDefault,
			data:      cborDataMapIntInt,
			wantValue: map[any]any{uint64(1): uint64(2), uint64(3): uint64(4)},
		},
		{
			name:      "decode CBOR map[string]int to Go map[interface{}]interface{} (default)",
			opts:      decOptionsDefault,
			data:      cborDataMapStringInt,
			wantValue: map[any]any{"a": uint64(1), "b": uint64(2)},
		},
		{
			name: "decode CBOR array of map[string]int to Go []map[interface{}]interface{} (default)",
			opts: decOptionsDefault,
			data: cborDataArrayOfMapStringint,
			wantValue: []any{
				map[any]any{"a": uint64(1), "b": uint64(2)},
				map[any]any{"c": uint64(3), "d": uint64(4)},
			},
		},
		{
			name: "decode CBOR nested map to Go map[interface{}]interface{} (default)",
			opts: decOptionsDefault,
			data: cborDataNestedMap,
			wantValue: map[any]any{
				"IntField": uint64(1),
				"MapField": map[any]any{"a": uint64(1), "b": uint64(2)},
			},
		},
		// Decode CBOR map to map[interface{}]interface{} using default map type option
		{
			name:      "decode CBOR map[int]int to Go map[interface{}]interface{}",
			opts:      decOptionsMapIntfIntfType,
			data:      cborDataMapIntInt,
			wantValue: map[any]any{uint64(1): uint64(2), uint64(3): uint64(4)},
		},
		{
			name:      "decode CBOR map[string]int to Go map[interface{}]interface{}",
			opts:      decOptionsMapIntfIntfType,
			data:      cborDataMapStringInt,
			wantValue: map[any]any{"a": uint64(1), "b": uint64(2)},
		},
		{
			name: "decode CBOR array of map[string]int to Go []map[interface{}]interface{}",
			opts: decOptionsMapIntfIntfType,
			data: cborDataArrayOfMapStringint,
			wantValue: []any{
				map[any]any{"a": uint64(1), "b": uint64(2)},
				map[any]any{"c": uint64(3), "d": uint64(4)},
			},
		},
		{
			name: "decode CBOR nested map to Go map[interface{}]interface{}",
			opts: decOptionsMapIntfIntfType,
			data: cborDataNestedMap,
			wantValue: map[any]any{
				"IntField": uint64(1),
				"MapField": map[any]any{"a": uint64(1), "b": uint64(2)},
			},
		},
		// Decode CBOR map to map[string]interface{} using default map type option
		{
			name:         "decode CBOR map[int]int to Go map[string]interface{}",
			opts:         decOptionsMapStringIntfType,
			data:         cborDataMapIntInt,
			wantErrorMsg: "cbor: cannot unmarshal positive integer into Go value of type string",
		},
		{
			name:      "decode CBOR map[string]int to Go map[string]interface{}",
			opts:      decOptionsMapStringIntfType,
			data:      cborDataMapStringInt,
			wantValue: map[string]any{"a": uint64(1), "b": uint64(2)},
		},
		{
			name: "decode CBOR array of map[string]int to Go []map[string]interface{}",
			opts: decOptionsMapStringIntfType,
			data: cborDataArrayOfMapStringint,
			wantValue: []any{
				map[string]any{"a": uint64(1), "b": uint64(2)},
				map[string]any{"c": uint64(3), "d": uint64(4)},
			},
		},
		{
			name: "decode CBOR nested map to Go map[string]interface{}",
			opts: decOptionsMapStringIntfType,
			data: cborDataNestedMap,
			wantValue: map[string]any{
				"IntField": uint64(1),
				"MapField": map[string]any{"a": uint64(1), "b": uint64(2)},
			},
		},
		// Decode CBOR map to map[string]int using default map type option
		{
			name:         "decode CBOR map[int]int to Go map[string]int",
			opts:         decOptionsMapStringIntType,
			data:         cborDataMapIntInt,
			wantErrorMsg: "cbor: cannot unmarshal positive integer into Go value of type string",
		},
		{
			name:      "decode CBOR map[string]int to Go map[string]int",
			opts:      decOptionsMapStringIntType,
			data:      cborDataMapStringInt,
			wantValue: map[string]int{"a": 1, "b": 2},
		},
		{
			name: "decode CBOR array of map[string]int to Go []map[string]int",
			opts: decOptionsMapStringIntType,
			data: cborDataArrayOfMapStringint,
			wantValue: []any{
				map[string]int{"a": 1, "b": 2},
				map[string]int{"c": 3, "d": 4},
			},
		},
		{
			name:         "decode CBOR nested map to Go map[string]int",
			opts:         decOptionsMapStringIntType,
			data:         cborDataNestedMap,
			wantErrorMsg: "cbor: cannot unmarshal map into Go value of type int",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decMode, _ := tc.opts.DecMode()

			var v any
			err := decMode.Unmarshal(tc.data, &v)
			if err != nil {
				if tc.wantErrorMsg == "" {
					t.Errorf("Unmarshal(0x%x) to empty interface returned error %v", tc.data, err)
				} else if tc.wantErrorMsg != err.Error() {
					t.Errorf("Unmarshal(0x%x) error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
				}
			} else {
				if tc.wantValue == nil {
					t.Errorf("Unmarshal(0x%x) = %v (%T), want error %q", tc.data, v, v, tc.wantErrorMsg)
				} else if !reflect.DeepEqual(v, tc.wantValue) {
					t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.data, v, v, tc.wantValue, tc.wantValue)
				}
			}
		})
	}
}

func TestUnmarshalFirstNoTrailing(t *testing.T) {
	for _, tc := range unmarshalTests {
		var v any
		if rest, err := UnmarshalFirst(tc.data, &v); err != nil {
			t.Errorf("UnmarshalFirst(0x%x) returned error %v", tc.data, err)
		} else {
			if len(rest) != 0 {
				t.Errorf("UnmarshalFirst(0x%x) returned rest %x (want [])", tc.data, rest)
			}
			// Check the value as well, although this is covered by other tests
			if tm, ok := tc.wantInterfaceValue.(time.Time); ok {
				if vt, ok := v.(time.Time); !ok || !tm.Equal(vt) {
					t.Errorf("UnmarshalFirst(0x%x) = %v (%T), want %v (%T)", tc.data, v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
				}
			} else if !reflect.DeepEqual(v, tc.wantInterfaceValue) {
				t.Errorf("UnmarshalFirst(0x%x) = %v (%T), want %v (%T)", tc.data, v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
			}
		}
	}
}

func TestUnmarshalfirstTrailing(t *testing.T) {
	// Random trailing data
	trailingData := mustHexDecode("4a6b0f4718c73f391091ea1c")
	for _, tc := range unmarshalTests {
		data := make([]byte, 0, len(tc.data)+len(trailingData))
		data = append(data, tc.data...)
		data = append(data, trailingData...)
		var v any
		if rest, err := UnmarshalFirst(data, &v); err != nil {
			t.Errorf("UnmarshalFirst(0x%x) returned error %v", data, err)
		} else {
			if !bytes.Equal(trailingData, rest) {
				t.Errorf("UnmarshalFirst(0x%x) returned rest %x (want %x)", data, rest, trailingData)
			}
			// Check the value as well, although this is covered by other tests
			if tm, ok := tc.wantInterfaceValue.(time.Time); ok {
				if vt, ok := v.(time.Time); !ok || !tm.Equal(vt) {
					t.Errorf("UnmarshalFirst(0x%x) = %v (%T), want %v (%T)", data, v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
				}
			} else if !reflect.DeepEqual(v, tc.wantInterfaceValue) {
				t.Errorf("UnmarshalFirst(0x%x) = %v (%T), want %v (%T)", data, v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
			}
		}
	}
}

func TestUnmarshalFirstInvalidItem(t *testing.T) {
	// UnmarshalFirst should not return "rest" if the item was not well-formed
	invalidCBOR := mustHexDecode("83FF20030102")
	var v any
	rest, err := UnmarshalFirst(invalidCBOR, &v)
	if rest != nil || err == nil {
		t.Errorf("UnmarshalFirst(0x%x) = (%x, %v), want (nil, err)", invalidCBOR, rest, err)
	}
}

func TestDecModeInvalidFieldNameMatchingMode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{FieldNameMatching: -1},
			wantErrorMsg: "cbor: invalid FieldNameMatching -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{FieldNameMatching: 101},
			wantErrorMsg: "cbor: invalid FieldNameMatching 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecodeFieldNameMatching(t *testing.T) {
	type s struct {
		LowerA int `cbor:"a"`
		UpperB int `cbor:"B"`
		LowerB int `cbor:"b"`
	}

	testCases := []struct {
		name      string
		opts      DecOptions
		data      []byte
		wantValue s
	}{
		{
			name:      "case-insensitive match",
			data:      mustHexDecode("a1614101"), // {"A": 1}
			wantValue: s{LowerA: 1},
		},
		{
			name:      "ignore case-insensitive match",
			opts:      DecOptions{FieldNameMatching: FieldNameMatchingCaseSensitive},
			data:      mustHexDecode("a1614101"), // {"A": 1}
			wantValue: s{},
		},
		{
			name:      "exact match before case-insensitive match",
			data:      mustHexDecode("a2616101614102"), // {"a": 1, "A": 2}
			wantValue: s{LowerA: 1},
		},
		{
			name:      "case-insensitive match before exact match",
			data:      mustHexDecode("a2614101616102"), // {"A": 1, "a": 2}
			wantValue: s{LowerA: 1},
		},
		{
			name:      "ignore case-insensitive match before exact match",
			opts:      DecOptions{FieldNameMatching: FieldNameMatchingCaseSensitive},
			data:      mustHexDecode("a2614101616102"), // {"A": 1, "a": 2}
			wantValue: s{LowerA: 2},
		},
		{
			name:      "earliest exact match wins",
			opts:      DecOptions{FieldNameMatching: FieldNameMatchingCaseSensitive},
			data:      mustHexDecode("a2616101616102"), // {"a": 1, "a": 2} (invalid)
			wantValue: s{LowerA: 1},
		},
		{
			// the field tags themselves are case-insensitive matches for each other
			name:      "duplicate key does not fall back to case-insensitive match",
			data:      mustHexDecode("a2614201614202"), // {"B": 1, "B": 2} (invalid)
			wantValue: s{UpperB: 1},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decMode, _ := tc.opts.DecMode()

			var dst s
			err := decMode.Unmarshal(tc.data, &dst)
			if err != nil {
				t.Fatalf("Unmarshal(0x%x) returned unexpected error %v", tc.data, err)
			}

			if !reflect.DeepEqual(dst, tc.wantValue) {
				t.Errorf("Unmarshal(0x%x) = %#v, want %#v", tc.data, dst, tc.wantValue)
			}
		})
	}
}

func TestInvalidBigIntDecMode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{BigIntDec: -1},
			wantErrorMsg: "cbor: invalid BigIntDec -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{BigIntDec: 101},
			wantErrorMsg: "cbor: invalid BigIntDec 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecodeBignumToEmptyInterface(t *testing.T) {
	decOptionsDecodeToBigIntValue := DecOptions{BigIntDec: BigIntDecodeValue}
	decOptionsDecodeToBigIntPointer := DecOptions{BigIntDec: BigIntDecodePointer}

	cborDataPositiveBignum := mustHexDecode("c249010000000000000000") // positive bignum: 18446744073709551616
	pbn, _ := new(big.Int).SetString("18446744073709551616", 10)

	cborDataNegativeBignum := mustHexDecode("c349010000000000000000") // negative bignum: -18446744073709551617
	nbn, _ := new(big.Int).SetString("-18446744073709551617", 10)

	cborDataLargeNegativeInt := mustHexDecode("3bffffffffffffffff") // -18446744073709551616
	ni, _ := new(big.Int).SetString("-18446744073709551616", 10)

	testCases := []struct {
		name      string
		opts      DecOptions
		data      []byte
		wantValue any
	}{
		{
			name:      "decode positive bignum to big.Int",
			opts:      decOptionsDecodeToBigIntValue,
			data:      cborDataPositiveBignum,
			wantValue: *pbn,
		},
		{
			name:      "decode negative bignum to big.Int",
			opts:      decOptionsDecodeToBigIntValue,
			data:      cborDataNegativeBignum,
			wantValue: *nbn,
		},
		{
			name:      "decode large negative int to big.Int",
			opts:      decOptionsDecodeToBigIntValue,
			data:      cborDataLargeNegativeInt,
			wantValue: *ni,
		},
		{
			name:      "decode positive bignum to *big.Int",
			opts:      decOptionsDecodeToBigIntPointer,
			data:      cborDataPositiveBignum,
			wantValue: pbn,
		},
		{
			name:      "decode negative bignum to *big.Int",
			opts:      decOptionsDecodeToBigIntPointer,
			data:      cborDataNegativeBignum,
			wantValue: nbn,
		},
		{
			name:      "decode large negative int to *big.Int",
			opts:      decOptionsDecodeToBigIntPointer,
			data:      cborDataLargeNegativeInt,
			wantValue: ni,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decMode, _ := tc.opts.DecMode()

			var v any
			err := decMode.Unmarshal(tc.data, &v)
			if err != nil {
				t.Errorf("Unmarshal(0x%x) to empty interface returned error %v", tc.data, err)
			} else { //nolint:gocritic // ignore elseif
				if !reflect.DeepEqual(v, tc.wantValue) {
					t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", tc.data, v, v, tc.wantValue, tc.wantValue)
				}
			}
		})
	}
}

func TestDecModeInvalidDefaultByteStringType(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "neither slice nor string",
			opts:         DecOptions{DefaultByteStringType: reflect.TypeOf(int(42))},
			wantErrorMsg: "cbor: invalid DefaultByteStringType: int is not of kind string or []uint8",
		},
		{
			name:         "slice of non-byte",
			opts:         DecOptions{DefaultByteStringType: reflect.TypeOf([]int{})},
			wantErrorMsg: "cbor: invalid DefaultByteStringType: []int is not of kind string or []uint8",
		},
		{
			name:         "pointer to byte array",
			opts:         DecOptions{DefaultByteStringType: reflect.TypeOf(&[42]byte{})},
			wantErrorMsg: "cbor: invalid DefaultByteStringType: *[42]uint8 is not of kind string or []uint8",
		},
		{
			name:         "byte array",
			opts:         DecOptions{DefaultByteStringType: reflect.TypeOf([42]byte{})},
			wantErrorMsg: "cbor: invalid DefaultByteStringType: [42]uint8 is not of kind string or []uint8",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestUnmarshalDefaultByteStringType(t *testing.T) {
	type namedByteSliceType []byte

	for _, tc := range []struct {
		name string
		opts DecOptions
		in   []byte
		want any
	}{
		{
			name: "default to []byte",
			opts: DecOptions{},
			in:   mustHexDecode("43414243"),
			want: []byte("ABC"),
		},
		{
			name: "explicitly []byte",
			opts: DecOptions{DefaultByteStringType: reflect.TypeOf([]byte(nil))},
			in:   mustHexDecode("43414243"),
			want: []byte("ABC"),
		},
		{
			name: "string",
			opts: DecOptions{DefaultByteStringType: reflect.TypeOf("")},
			in:   mustHexDecode("43414243"),
			want: "ABC",
		},
		{
			name: "ByteString",
			opts: DecOptions{DefaultByteStringType: reflect.TypeOf(ByteString(""))},
			in:   mustHexDecode("43414243"),
			want: ByteString("ABC"),
		},
		{
			name: "named []byte type",
			opts: DecOptions{DefaultByteStringType: reflect.TypeOf(namedByteSliceType(nil))},
			in:   mustHexDecode("43414243"),
			want: namedByteSliceType("ABC"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			var got any
			if err := dm.Unmarshal(tc.in, &got); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(tc.want, got) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestDecModeInvalidByteStringToStringMode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{ByteStringToString: -1},
			wantErrorMsg: "cbor: invalid ByteStringToString -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{ByteStringToString: 101},
			wantErrorMsg: "cbor: invalid ByteStringToString 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestUnmarshalByteStringToString(t *testing.T) {
	var s string

	derror, err := DecOptions{ByteStringToString: ByteStringToStringForbidden}.DecMode()
	if err != nil {
		t.Fatal(err)
	}

	if err = derror.Unmarshal(mustHexDecode("43414243"), &s); err == nil {
		t.Error("expected non-nil error from Unmarshal")
	}

	if s != "" {
		t.Errorf("expected destination string to be empty, got %q", s)
	}

	dallow, err := DecOptions{ByteStringToString: ByteStringToStringAllowed}.DecMode()
	if err != nil {
		t.Fatal(err)
	}

	if err = dallow.Unmarshal(mustHexDecode("43414243"), &s); err != nil {
		t.Errorf("expected nil error from Unmarshal, got: %v", err)
	}

	if s != "ABC" {
		t.Errorf("expected destination string to be \"ABC\", got %q", s)
	}
}

func TestDecModeInvalidFieldNameByteStringMode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{FieldNameByteString: -1},
			wantErrorMsg: "cbor: invalid FieldNameByteString -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{FieldNameByteString: 101},
			wantErrorMsg: "cbor: invalid FieldNameByteString 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestUnmarshalFieldNameByteString(t *testing.T) {
	allowed, err := DecOptions{
		FieldNameByteString: FieldNameByteStringAllowed,
	}.DecMode()
	if err != nil {
		t.Fatal(err)
	}

	var s struct {
		F int64 `json:"f"`
	}

	err = allowed.Unmarshal(mustHexDecode("a1414601"), &s) // {h'46': 1}
	if err != nil {
		t.Fatal(err)
	}

	if s.F != 1 {
		t.Errorf("expected field F to be set to 1, got %d", s.F)
	}

	forbidden, err := DecOptions{
		FieldNameByteString: FieldNameByteStringForbidden,
	}.DecMode()
	if err != nil {
		t.Fatal(err)
	}

	const wantMsg = "cbor: cannot unmarshal byte string into Go value of type string (map key is of type byte string and cannot be used to match struct field name)"
	if err := forbidden.Unmarshal(mustHexDecode("a1414601"), &s); err == nil {
		t.Errorf("expected non-nil error")
	} else if gotMsg := err.Error(); gotMsg != wantMsg {
		t.Errorf("expected error %q, got %q", wantMsg, gotMsg)
	}
}

func TestDecModeInvalidReturnTypeForEmptyInterface(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{UnrecognizedTagToAny: -1},
			wantErrorMsg: "cbor: invalid UnrecognizedTagToAnyMode -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{UnrecognizedTagToAny: 101},
			wantErrorMsg: "cbor: invalid UnrecognizedTagToAnyMode 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestUnmarshalWithUnrecognizedTagToAnyMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts DecOptions
		in   []byte
		want any
	}{
		{
			name: "default to value of type Tag",
			opts: DecOptions{},
			in:   mustHexDecode("d8ff00"),
			want: Tag{Number: uint64(255), Content: uint64(0)},
		},
		{
			name: "Tag's content",
			opts: DecOptions{UnrecognizedTagToAny: UnrecognizedTagContentToAny},
			in:   mustHexDecode("d8ff00"),
			want: uint64(0),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			var got any
			if err := dm.Unmarshal(tc.in, &got); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tc.want != got {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestUnmarshalWithUnrecognizedTagToAnyModeForSupportedTags(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts DecOptions
		in   []byte
		want any
	}{
		{
			name: "Unmarshal with tag number 0 when UnrecognizedTagContentToAny option is not set",
			opts: DecOptions{},
			in:   mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			want: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			name: "Unmarshal with tag number 0 when UnrecognizedTagContentToAny option is set",
			opts: DecOptions{UnrecognizedTagToAny: UnrecognizedTagContentToAny},
			in:   mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			want: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			name: "Unmarshal with tag number 1 when UnrecognizedTagContentToAny option is not set",
			opts: DecOptions{},
			in:   mustHexDecode("c11a514b67b0"),
			want: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			name: "Unmarshal with tag number 1 when UnrecognizedTagContentToAny option is set",
			opts: DecOptions{UnrecognizedTagToAny: UnrecognizedTagContentToAny},
			in:   mustHexDecode("c11a514b67b0"),
			want: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			name: "Unmarshal with tag number 2 when UnrecognizedTagContentToAny option is not set",
			opts: DecOptions{},
			in:   mustHexDecode("c249010000000000000000"),
			want: mustBigInt("18446744073709551616"),
		},
		{
			name: "Unmarshal with tag number 2 when UnrecognizedTagContentToAny option is set",
			opts: DecOptions{UnrecognizedTagToAny: UnrecognizedTagContentToAny},
			in:   mustHexDecode("c249010000000000000000"),
			want: mustBigInt("18446744073709551616"),
		},
		{
			name: "Unmarshal with tag number 3 when UnrecognizedTagContentToAny option is not set",
			opts: DecOptions{},
			in:   mustHexDecode("c349010000000000000000"),
			want: mustBigInt("-18446744073709551617"),
		},
		{
			name: "Unmarshal with tag number 3 when UnrecognizedTagContentToAny option is set",
			opts: DecOptions{UnrecognizedTagToAny: UnrecognizedTagContentToAny},
			in:   mustHexDecode("c349010000000000000000"),
			want: mustBigInt("-18446744073709551617"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			var got any
			if err := dm.Unmarshal(tc.in, &got); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			compareNonFloats(t, tc.in, got, tc.want)

		})
	}
}

func TestUnmarshalWithUnrecognizedTagToAnyModeForSharedTag(t *testing.T) {

	type myInt int
	typ := reflect.TypeOf(myInt(0))

	tags := NewTagSet()
	if err := tags.Add(TagOptions{EncTag: EncTagRequired, DecTag: DecTagRequired}, typ, 125); err != nil {
		t.Fatalf("TagSet.Add(%s, %v) returned error %v", typ, 125, err)
	}

	for _, tc := range []struct {
		name string
		opts DecOptions
		in   []byte
		want any
	}{
		{
			name: "Unmarshal with a shared tag when UnrecognizedTagContentToAny option is not set",
			opts: DecOptions{},
			in:   mustHexDecode("d9d9f7d87d01"), // 55799(125(1))
			want: myInt(1),
		},
		{
			name: "Unmarshal with a shared tag when UnrecognizedTagContentToAny option is set",
			opts: DecOptions{UnrecognizedTagToAny: UnrecognizedTagContentToAny},
			in:   mustHexDecode("d9d9f7d87d01"), // 55799(125(1))
			want: myInt(1),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecModeWithTags(tags)
			if err != nil {
				t.Fatal(err)
			}

			var got any

			if err := dm.Unmarshal(tc.in, &got); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			compareNonFloats(t, tc.in, got, tc.want)

		})
	}
}

func TestNewSimpleValueRegistry(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         []func(*SimpleValueRegistry) error
		wantErrorMsg string
	}{
		{
			name:         "min reserved",
			opts:         []func(*SimpleValueRegistry) error{WithRejectedSimpleValue(24)},
			wantErrorMsg: "cbor: cannot set analog for reserved simple value 24",
		},
		{
			name:         "max reserved",
			opts:         []func(*SimpleValueRegistry) error{WithRejectedSimpleValue(31)},
			wantErrorMsg: "cbor: cannot set analog for reserved simple value 31",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewSimpleValueRegistryFromDefaults(tc.opts...)
			if err == nil {
				t.Fatalf("got nil error, want: %s", tc.wantErrorMsg)
			}
			if got := err.Error(); got != tc.wantErrorMsg {
				t.Errorf("want: %s, got: %s", tc.wantErrorMsg, got)
			}
		})
	}
}

func TestUnmarshalSimpleValues(t *testing.T) {
	assertNilError := func(t *testing.T, e error) {
		if e != nil {
			t.Errorf("expected nil error, got: %v", e)
		}
	}

	assertExactError := func(want error) func(*testing.T, error) {
		return func(t *testing.T, got error) {
			targetErr := reflect.New(reflect.TypeOf(want)).Interface()
			if !errors.As(got, &targetErr) ||
				got.Error() != want.Error() {
				t.Errorf("want %#v, got %#v", want, got)
			}
		}
	}

	for _, tc := range []struct {
		name          string
		fns           []func(*SimpleValueRegistry) error
		in            []byte
		into          reflect.Type
		want          any
		assertOnError func(t *testing.T, e error)
	}{
		{
			name:          "default false into interface{}",
			fns:           nil,
			in:            []byte{0xf4},
			into:          typeIntf,
			want:          false,
			assertOnError: assertNilError,
		},
		{
			name:          "default false into bool",
			fns:           nil,
			in:            []byte{0xf4},
			into:          typeBool,
			want:          false,
			assertOnError: assertNilError,
		},
		{
			name:          "default true into interface{}",
			fns:           nil,
			in:            []byte{0xf5},
			into:          typeIntf,
			want:          true,
			assertOnError: assertNilError,
		},
		{
			name:          "default true into bool",
			fns:           nil,
			in:            []byte{0xf5},
			into:          typeBool,
			want:          true,
			assertOnError: assertNilError,
		},
		{
			name:          "default null into interface{}",
			fns:           nil,
			in:            []byte{0xf6},
			into:          typeIntf,
			want:          nil,
			assertOnError: assertNilError,
		},
		{
			name:          "default undefined into interface{}",
			fns:           nil,
			in:            []byte{0xf7},
			into:          typeIntf,
			want:          nil,
			assertOnError: assertNilError,
		},
		{
			name: "reject undefined into interface{}",
			fns:  []func(*SimpleValueRegistry) error{WithRejectedSimpleValue(23)},
			in:   []byte{0xf7},
			into: typeIntf,
			want: nil,
			assertOnError: assertExactError(&UnacceptableDataItemError{
				CBORType: "primitives",
				Message:  "simple value 23 is not recognized",
			}),
		},
		{
			name: "reject true into bool",
			fns:  []func(*SimpleValueRegistry) error{WithRejectedSimpleValue(21)},
			in:   []byte{0xf5},
			into: typeBool,
			want: false,
			assertOnError: assertExactError(&UnacceptableDataItemError{
				CBORType: "primitives",
				Message:  "simple value 21 is not recognized",
			}),
		},
		{
			name:          "default unrecognized into uint64",
			fns:           nil,
			in:            []byte{0xf8, 0xc8},
			into:          typeUint64,
			want:          uint64(200),
			assertOnError: assertNilError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewSimpleValueRegistryFromDefaults(tc.fns...)
			if err != nil {
				t.Fatal(err)
			}

			decMode, err := DecOptions{SimpleValues: r}.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			dst := reflect.New(tc.into)
			err = decMode.Unmarshal(tc.in, dst.Interface())
			tc.assertOnError(t, err)

			if got := dst.Elem().Interface(); !reflect.DeepEqual(tc.want, got) {
				t.Errorf("got: %#v\nwant: %#v\n", got, tc.want)
			}
		})
	}
}

func isCBORNil(data []byte) bool {
	return len(data) > 0 && (data[0] == 0xf6 || data[0] == 0xf7)
}

func TestDecModeInvalidTimeTagToAnyMode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{TimeTagToAny: -1},
			wantErrorMsg: "cbor: invalid TimeTagToAny -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{TimeTagToAny: 4},
			wantErrorMsg: "cbor: invalid TimeTagToAny 4",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("Expected non nil error from DecMode()")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Expected error: %q, want: %q \n", tc.wantErrorMsg, err.Error())
			}
		})
	}
}

func TestDecModeTimeTagToAny(t *testing.T) {
	for _, tc := range []struct {
		name           string
		opts           DecOptions
		in             []byte
		want           any
		wantErrMessage string
	}{
		{
			name: "Unmarshal tag 0 data to time.Time when TimeTagToAny is not set",
			opts: DecOptions{},
			in:   mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			want: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			name: "Unmarshal tag 1 data to time.Time when TimeTagToAny is not set",
			opts: DecOptions{},
			in:   mustHexDecode("c11a514b67b0"),
			want: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			name: "Unmarshal tag 0 data to RFC3339 string when TimeTagToAny is set",
			opts: DecOptions{TimeTagToAny: TimeTagToRFC3339},
			in:   mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			want: "2013-03-21T20:04:00Z",
		},
		{
			name: "Unmarshal tag 1 data to RFC3339 string when TimeTagToAny is set",
			opts: DecOptions{TimeTagToAny: TimeTagToRFC3339},
			in:   mustHexDecode("c11a514b67b0"),
			want: "2013-03-21T20:04:00Z",
		},
		{
			name: "Unmarshal tag 0 data to RFC3339Nano string when TimeTagToAny is set",
			opts: DecOptions{TimeTagToAny: TimeTagToRFC3339Nano},
			in:   mustHexDecode("c076323031332d30332d32315432303a30343a30302e355a"),
			want: "2013-03-21T20:04:00.5Z",
		},
		{
			name: "Unmarshal tag 1 data to RFC3339Nano string when TimeTagToAny is set",
			opts: DecOptions{TimeTagToAny: TimeTagToRFC3339Nano},
			in:   mustHexDecode("c1fb41d452d9ec200000"),
			want: "2013-03-21T20:04:00.5Z",
		},
		{
			name:           "error under TimeTagToRFC3339 when tag 0 contains an invalid RFC3339 timestamp",
			opts:           DecOptions{TimeTagToAny: TimeTagToRFC3339},
			in:             mustHexDecode("c07731303030302D30332D32315432303A30343A30302E355A"), // 0("10000-03-21T20:04:00.5Z")
			wantErrMessage: `cbor: cannot set 10000-03-21T20:04:00.5Z for time.Time: parsing time "10000-03-21T20:04:00.5Z" as "2006-01-02T15:04:05Z07:00": cannot parse "0-03-21T20:04:00.5Z" as "-"`,
		},
		{
			name:           "error under TimeTagToRFC3339Nano when tag 0 contains an invalid RFC3339 timestamp",
			opts:           DecOptions{TimeTagToAny: TimeTagToRFC3339Nano},
			in:             mustHexDecode("c07731303030302D30332D32315432303A30343A30302E355A"), // 0("10000-03-21T20:04:00.5Z")
			wantErrMessage: `cbor: cannot set 10000-03-21T20:04:00.5Z for time.Time: parsing time "10000-03-21T20:04:00.5Z" as "2006-01-02T15:04:05Z07:00": cannot parse "0-03-21T20:04:00.5Z" as "-"`,
		},
		{
			name:           "error under TimeTagToRFC3339 when tag 1 represents a time that can't be represented by valid RFC3339",
			opts:           DecOptions{TimeTagToAny: TimeTagToRFC3339},
			in:             mustHexDecode("c11b0000003afff44181"), // 1(253402300801)
			wantErrMessage: "cbor: decoded time cannot be represented in RFC3339 format: Time.MarshalText: year outside of range [0,9999]",
		},
		{
			name:           "error under TimeTagToRFC3339Nano when tag 1 represents a time that can't be represented by valid RFC3339",
			opts:           DecOptions{TimeTagToAny: TimeTagToRFC3339Nano},
			in:             mustHexDecode("c11b0000003afff44181"), // 1(253402300801)
			wantErrMessage: "cbor: decoded time cannot be represented in RFC3339 format with sub-second precision: Time.MarshalText: year outside of range [0,9999]",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			var got any
			if err := dm.Unmarshal(tc.in, &got); err != nil {
				if tc.wantErrMessage == "" {
					t.Fatalf("unexpected error: %v", err)
				} else if gotErrMessage := err.Error(); tc.wantErrMessage != gotErrMessage {
					t.Fatalf("want error %q, got %q", tc.wantErrMessage, gotErrMessage)
				}
			} else if tc.wantErrMessage != "" {
				t.Fatalf("got nil error, want %q", tc.wantErrMessage)
			}

			compareNonFloats(t, tc.in, got, tc.want)

		})
	}
}

func TestDecModeInvalidNaNDec(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{NaN: -1},
			wantErrorMsg: "cbor: invalid NaNDec -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{NaN: 101},
			wantErrorMsg: "cbor: invalid NaNDec 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestNaNDecMode(t *testing.T) {
	for _, tc := range []struct {
		opt    NaNMode
		src    []byte
		dst    any
		reject bool
	}{
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("197e00"),
			dst:    new(any),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("1a7fc00000"),
			dst:    new(any),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("1b7ff8000000000000"),
			dst:    new(any),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("f90000"), // 0.0
			dst:    new(any),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("f90000"), // 0.0
			dst:    new(float32),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("f90000"), // 0.0
			dst:    new(float64),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("f90000"), // 0.0
			dst:    new(time.Time),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fa47c35000"), // 100000.0
			dst:    new(any),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fa47c35000"), // 100000.0
			dst:    new(float32),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fa47c35000"), // 100000.0
			dst:    new(float64),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fa47c35000"), // 100000.0
			dst:    new(time.Time),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fb3ff199999999999a"), // 1.1
			dst:    new(any),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fb3ff199999999999a"), // 1.1
			dst:    new(float32),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fb3ff199999999999a"), // 1.1
			dst:    new(float64),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fb3ff199999999999a"), // 1.1
			dst:    new(time.Time),
			reject: false,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("f97e00"),
			dst:    new(any),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("f97e00"),
			dst:    new(float32),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("f97e00"),
			dst:    new(float64),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("f97e00"),
			dst:    new(time.Time),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fa7fc00000"),
			dst:    new(any),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fa7fc00000"),
			dst:    new(float32),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fa7fc00000"),
			dst:    new(float64),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fa7fc00000"),
			dst:    new(time.Time),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fb7ff8000000000000"),
			dst:    new(any),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fb7ff8000000000000"),
			dst:    new(float32),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fb7ff8000000000000"),
			dst:    new(float64),
			reject: true,
		},
		{
			opt:    NaNDecodeForbidden,
			src:    mustHexDecode("fb7ff8000000000000"),
			dst:    new(time.Time),
			reject: true,
		},
	} {
		t.Run(fmt.Sprintf("mode=%d/0x%x into %s", tc.opt, tc.src, reflect.TypeOf(tc.dst).String()), func(t *testing.T) {
			dm, err := DecOptions{NaN: tc.opt}.DecMode()
			if err != nil {
				t.Fatal(err)
			}
			want := &UnacceptableDataItemError{
				CBORType: cborTypePrimitives.String(),
				Message:  "floating-point NaN",
			}
			if got := dm.Unmarshal(tc.src, tc.dst); got != nil {
				if tc.reject {
					if !reflect.DeepEqual(want, got) {
						t.Errorf("want error: %v, got error: %v", want, got)
					}
				} else {
					t.Errorf("unexpected error: %v", got)
				}
			} else if tc.reject {
				t.Error("unexpected nil error")
			}
		})
	}
}

func TestDecModeInvalidInfDec(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{Inf: -1},
			wantErrorMsg: "cbor: invalid InfDec -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{Inf: 101},
			wantErrorMsg: "cbor: invalid InfDec 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestInfDecMode(t *testing.T) {
	for _, tc := range []struct {
		opt    InfMode
		src    []byte
		dst    any
		reject bool
	}{
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("197c00"),
			dst:    new(any),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("1a7f800000"), // Infinity
			dst:    new(any),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("1b7ff0000000000000"), // Infinity
			dst:    new(any),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f90000"), // 0.0
			dst:    new(any),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f90000"), // 0.0
			dst:    new(float32),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f90000"), // 0.0
			dst:    new(float64),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f90000"), // 0.0
			dst:    new(time.Time),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fa47c35000"), // 100000.0
			dst:    new(any),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fa47c35000"), // 100000.0
			dst:    new(float32),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fa47c35000"), // 100000.0
			dst:    new(float64),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fa47c35000"), // 100000.0
			dst:    new(time.Time),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fb3ff199999999999a"), // 1.1
			dst:    new(any),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fb3ff199999999999a"), // 1.1
			dst:    new(float32),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fb3ff199999999999a"), // 1.1
			dst:    new(float64),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fb3ff199999999999a"), // 1.1
			dst:    new(time.Time),
			reject: false,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f97c00"), // Infinity
			dst:    new(any),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f97c00"), // Infinity
			dst:    new(float32),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f97c00"), // Infinity
			dst:    new(float64),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f97c00"), // Infinity
			dst:    new(time.Time),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f9fc00"), // -Infinity
			dst:    new(any),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f9fc00"), // -Infinity
			dst:    new(float32),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f9fc00"), // -Infinity
			dst:    new(float64),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("f9fc00"), // -Infinity
			dst:    new(time.Time),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fa7f800000"), // Infinity
			dst:    new(any),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fa7f800000"), // Infinity
			dst:    new(float32),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fa7f800000"), // Infinity
			dst:    new(float64),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fa7f800000"), // Infinity
			dst:    new(time.Time),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("faff800000"), // -Infinity
			dst:    new(any),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("faff800000"), // -Infinity
			dst:    new(float32),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("faff800000"), // -Infinity
			dst:    new(float64),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("faff800000"), // -Infinity
			dst:    new(time.Time),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fb7ff0000000000000"), // Infinity
			dst:    new(any),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fb7ff0000000000000"), // Infinity
			dst:    new(float32),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fb7ff0000000000000"), // Infinity
			dst:    new(float64),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fb7ff0000000000000"), // Infinity
			dst:    new(time.Time),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fbfff0000000000000"), // -Infinity
			dst:    new(any),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fbfff0000000000000"), // -Infinity
			dst:    new(float32),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fbfff0000000000000"), // -Infinity
			dst:    new(float64),
			reject: true,
		},
		{
			opt:    InfDecodeForbidden,
			src:    mustHexDecode("fbfff0000000000000"), // -Infinity
			dst:    new(time.Time),
			reject: true,
		},
	} {
		t.Run(fmt.Sprintf("mode=%d/0x%x into %s", tc.opt, tc.src, tc.dst), func(t *testing.T) {
			dm, err := DecOptions{Inf: tc.opt}.DecMode()
			if err != nil {
				t.Fatal(err)
			}
			want := &UnacceptableDataItemError{
				CBORType: cborTypePrimitives.String(),
				Message:  "floating-point infinity",
			}
			if got := dm.Unmarshal(tc.src, tc.dst); got != nil {
				if tc.reject {
					if !reflect.DeepEqual(want, got) {
						t.Errorf("want error: %v, got error: %v", want, got)
					}
				} else {
					t.Errorf("unexpected error: %v", got)
				}
			} else if tc.reject {
				t.Error("unexpected nil error")
			}
		})
	}
}

func TestDecModeInvalidByteStringToTimeMode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{ByteStringToTime: -1},
			wantErrorMsg: "cbor: invalid ByteStringToTime -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{ByteStringToTime: 4},
			wantErrorMsg: "cbor: invalid ByteStringToTime 4",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("Expected non nil error from DecMode()")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Expected error: %q, want: %q \n", tc.wantErrorMsg, err.Error())
			}
		})
	}
}

func TestDecModeByteStringToTime(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		in           []byte
		want         time.Time
		wantErrorMsg string
	}{
		{
			name:         "Unmarshal byte string to time.Time when ByteStringToTime is not set",
			opts:         DecOptions{},
			in:           mustHexDecode("54323031332D30332D32315432303A30343A30305A"), // '2013-03-21T20:04:00Z'
			wantErrorMsg: "cbor: cannot unmarshal byte string into Go value of type time.Time",
		},
		{
			name: "Unmarshal byte string to time.Time when ByteStringToTime is set to ByteStringToTimeAllowed",
			opts: DecOptions{ByteStringToTime: ByteStringToTimeAllowed},
			in:   mustHexDecode("54323031332D30332D32315432303A30343A30305A"), // '2013-03-21T20:04:00Z'
			want: time.Date(2013, 3, 21, 20, 4, 0, 0, time.UTC),
		},
		{
			name: "Unmarshal byte string to time.Time with nano when ByteStringToTime is set to ByteStringToTimeAllowed",
			opts: DecOptions{ByteStringToTime: ByteStringToTimeAllowed},
			in:   mustHexDecode("56323031332D30332D32315432303A30343A30302E355A"), // '2013-03-21T20:04:00.5Z'
			want: time.Date(2013, 3, 21, 20, 4, 0, 500000000, time.UTC),
		},
		{
			name:         "Unmarshal a byte string that is not a valid RFC3339 timestamp to time.Time when ByteStringToTime is set to ByteStringToTimeAllowed",
			opts:         DecOptions{ByteStringToTime: ByteStringToTimeAllowed},
			in:           mustHexDecode("4B696E76616C696454657874"), // 'invalidText'
			wantErrorMsg: `cbor: cannot set "invalidText" for time.Time: parsing time "invalidText" as "2006-01-02T15:04:05Z07:00": cannot parse "invalidText" as "2006"`,
		},
		{
			name:         "Unmarshal a byte string that is not a valid utf8 sequence to time.Time when ByteStringToTime is set to ByteStringToTimeAllowed",
			opts:         DecOptions{ByteStringToTime: ByteStringToTimeAllowed},
			in:           mustHexDecode("54323031338030332D32315432303A30343A30305A"), // "2013\x8003-21T20:04:00Z" -- the first hyphen of a valid RFC3339 string is replaced by a continuation byte
			wantErrorMsg: `cbor: cannot set "2013\x8003-21T20:04:00Z" for time.Time: parsing time "2013\x8003-21T20:04:00Z" as "2006-01-02T15:04:05Z07:00": cannot parse "\x8003-21T20:04:00Z" as "-"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			var got time.Time
			if err := dm.Unmarshal(tc.in, &got); err != nil {
				if tc.wantErrorMsg != err.Error() {
					t.Errorf("unexpected error: got %v want %v", err, tc.wantErrorMsg)
				}
			} else {
				compareNonFloats(t, tc.in, got, tc.want)
			}
		})
	}
}

func TestInvalidByteSliceExpectedEncodingMode(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{ByteStringExpectedFormat: -1},
			wantErrorMsg: "cbor: invalid ByteStringExpectedFormat -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{ByteStringExpectedFormat: 101},
			wantErrorMsg: "cbor: invalid ByteStringExpectedFormat 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestDecOptionsConflictWithRegisteredTags(t *testing.T) {
	type empty struct{}

	for _, tc := range []struct {
		name    string
		opts    DecOptions
		tags    func(TagSet) error
		wantErr string
	}{
		{
			name: "base64url encoding tag ignored by default",
			opts: DecOptions{},
			tags: func(tags TagSet) error {
				return tags.Add(TagOptions{DecTag: DecTagOptional}, reflect.TypeOf(empty{}), 21)
			},
			wantErr: "",
		},
		{
			name: "base64url encoding tag conflicts in ByteStringToStringAllowedWithExpectedLaterEncoding mode",
			opts: DecOptions{ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding},
			tags: func(tags TagSet) error {
				return tags.Add(TagOptions{DecTag: DecTagOptional}, reflect.TypeOf(empty{}), 21)
			},
			wantErr: "cbor: DecMode with non-default StringExpectedEncoding or ByteSliceExpectedEncoding treats tag 21 as built-in and conflicts with the provided TagSet's registration of cbor.empty",
		},
		{
			name: "base64url encoding tag conflicts with non-default ByteSliceExpectedEncoding option",
			opts: DecOptions{ByteStringExpectedFormat: ByteStringExpectedBase16},
			tags: func(tags TagSet) error {
				return tags.Add(TagOptions{DecTag: DecTagOptional}, reflect.TypeOf(empty{}), 21)
			},
			wantErr: "cbor: DecMode with non-default StringExpectedEncoding or ByteSliceExpectedEncoding treats tag 21 as built-in and conflicts with the provided TagSet's registration of cbor.empty",
		},
		{
			name: "base64 encoding tag ignored by default",
			opts: DecOptions{},
			tags: func(tags TagSet) error {
				return tags.Add(TagOptions{DecTag: DecTagOptional}, reflect.TypeOf(empty{}), 22)
			},
			wantErr: "",
		},
		{
			name: "base64 encoding tag conflicts in ByteStringToStringAllowedWithExpectedLaterEncoding mode",
			opts: DecOptions{ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding},
			tags: func(tags TagSet) error {
				return tags.Add(TagOptions{DecTag: DecTagOptional}, reflect.TypeOf(empty{}), 22)
			},
			wantErr: "cbor: DecMode with non-default StringExpectedEncoding or ByteSliceExpectedEncoding treats tag 22 as built-in and conflicts with the provided TagSet's registration of cbor.empty",
		},
		{
			name: "base64 encoding tag conflicts with non-default ByteSliceExpectedEncoding option",
			opts: DecOptions{ByteStringExpectedFormat: ByteStringExpectedBase16},
			tags: func(tags TagSet) error {
				return tags.Add(TagOptions{DecTag: DecTagOptional}, reflect.TypeOf(empty{}), 22)
			},
			wantErr: "cbor: DecMode with non-default StringExpectedEncoding or ByteSliceExpectedEncoding treats tag 22 as built-in and conflicts with the provided TagSet's registration of cbor.empty",
		},
		{
			name: "base16 encoding tag ignored by default",
			opts: DecOptions{},
			tags: func(tags TagSet) error {
				return tags.Add(TagOptions{DecTag: DecTagOptional}, reflect.TypeOf(empty{}), 23)
			},
			wantErr: "",
		},
		{
			name: "base16 encoding tag conflicts in ByteStringToStringAllowedWithExpectedLaterEncoding mode",
			opts: DecOptions{ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding},
			tags: func(tags TagSet) error {
				return tags.Add(TagOptions{DecTag: DecTagOptional}, reflect.TypeOf(empty{}), 23)
			},
			wantErr: "cbor: DecMode with non-default StringExpectedEncoding or ByteSliceExpectedEncoding treats tag 23 as built-in and conflicts with the provided TagSet's registration of cbor.empty",
		},
		{
			name: "base16 encoding tag conflicts with non-default ByteSliceExpectedEncoding option",
			opts: DecOptions{ByteStringExpectedFormat: ByteStringExpectedBase16},
			tags: func(tags TagSet) error {
				return tags.Add(TagOptions{DecTag: DecTagOptional}, reflect.TypeOf(empty{}), 23)
			},
			wantErr: "cbor: DecMode with non-default StringExpectedEncoding or ByteSliceExpectedEncoding treats tag 23 as built-in and conflicts with the provided TagSet's registration of cbor.empty",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tags := NewTagSet()
			if err := tc.tags(tags); err != nil {
				t.Fatal(err)
			}

			if _, err := tc.opts.DecModeWithTags(tags); err == nil {
				if tc.wantErr != "" {
					t.Errorf("got nil error from DecModeWithTags, want %q", tc.wantErr)
				}
			} else if got := err.Error(); got != tc.wantErr {
				if tc.wantErr != "" {
					t.Errorf("unexpected error from DecModeWithTags, got %q want %q", got, tc.wantErr)
				} else {
					t.Errorf("want nil error from DecModeWithTags, got %q", got)
				}
			}

			if _, err := tc.opts.DecModeWithSharedTags(tags); err == nil {
				if tc.wantErr != "" {
					t.Errorf("got nil error from DecModeWithSharedTags, want %q", tc.wantErr)
				}
			} else if got := err.Error(); got != tc.wantErr {
				if tc.wantErr != "" {
					t.Errorf("unexpected error from DecModeWithSharedTags, got %q want %q", got, tc.wantErr)
				} else {
					t.Errorf("want nil error from DecModeWithSharedTags, got %q", got)
				}
			}
		})
	}
}

func TestUnmarshalByteStringTextConversionError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    DecOptions
		dstType reflect.Type
		in      []byte
		wantErr string
	}{
		{
			name:    "reject untagged byte string containing invalid base64url",
			opts:    DecOptions{ByteStringExpectedFormat: ByteStringExpectedBase64URL},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0x41, 0x00},
			wantErr: "cbor: failed to decode base64url from byte string: illegal base64 data at input byte 0",
		},
		{
			name:    "reject untagged byte string containing invalid base64url",
			opts:    DecOptions{ByteStringExpectedFormat: ByteStringExpectedBase64},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0x41, 0x00},
			wantErr: "cbor: failed to decode base64 from byte string: illegal base64 data at input byte 0",
		},
		{
			name:    "reject untagged byte string containing invalid base16",
			opts:    DecOptions{ByteStringExpectedFormat: ByteStringExpectedBase16},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0x41, 0x00},
			wantErr: "cbor: failed to decode hex from byte string: encoding/hex: invalid byte: U+0000",
		},
		{
			name:    "accept tagged byte string containing invalid base64url",
			opts:    DecOptions{ByteStringExpectedFormat: ByteStringExpectedBase64URL},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0xd5, 0x41, 0x00},
			wantErr: "",
		},
		{
			name:    "accept tagged byte string containing invalid base64url",
			opts:    DecOptions{ByteStringExpectedFormat: ByteStringExpectedBase64},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0xd5, 0x41, 0x00},
			wantErr: "",
		},
		{
			name:    "accept tagged byte string containing invalid base16",
			opts:    DecOptions{ByteStringExpectedFormat: ByteStringExpectedBase16},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0xd5, 0x41, 0x00},
			wantErr: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			if err := dm.Unmarshal(tc.in, reflect.New(tc.dstType).Interface()); err == nil {
				if tc.wantErr != "" {
					t.Errorf("got nil error, want %q", tc.wantErr)
				}
			} else if got := err.Error(); got != tc.wantErr {
				if tc.wantErr == "" {
					t.Errorf("expected nil error, got %q", got)
				} else {
					t.Errorf("unexpected error, got %q want %q", got, tc.wantErr)
				}
			}
		})
	}
}

func TestUnmarshalByteStringTextConversion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    DecOptions
		dstType reflect.Type
		in      []byte
		want    any
	}{
		{
			name: "untagged into string",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0x41, 0xff}, // h'ff'
			want:    "\xff",
		},
		{
			name: "tagged base64url into string",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0xd5, 0x41, 0xff}, // 21(h'ff')
			want:    "_w",
		},
		{
			name: "indirectly tagged base64url into string",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0xd5, 0xd9, 0xd9, 0xf7, 0x41, 0xff}, // 21(55799(h'ff'))
			want:    "_w",
		},
		{
			name: "tagged base64url into string tags ignored",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowed,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0xd5, 0x41, 0xff}, // 21(h'ff')
			want:    "\xff",
		},
		{
			name: "tagged into []byte with default encoding base64url",
			opts: DecOptions{
				ByteStringExpectedFormat: ByteStringExpectedBase64URL,
			},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0xd5, 0x41, 0xff}, // 21(h'ff')
			want:    []byte{0xff},
		},
		{
			name: "indirectly tagged into []byte with default encoding base64url",
			opts: DecOptions{
				ByteStringExpectedFormat: ByteStringExpectedBase64URL,
			},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0xd5, 0xd9, 0xd9, 0xf7, 0x41, 0xff}, // 21(55799(h'ff'))
			want:    []byte{0xff},
		},
		{
			name: "untagged base64url into []byte with default encoding base64url",
			opts: DecOptions{
				ByteStringExpectedFormat: ByteStringExpectedBase64URL,
			},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0x42, '_', 'w'}, // '_w'
			want:    []byte{0xff},
		},
		{
			name: "tagged base64 into string",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0xd6, 0x41, 0xff}, // 22(h'ff')
			want:    "/w==",
		},
		{
			name: "indirectly tagged base64 into string",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0xd6, 0xd9, 0xd9, 0xf7, 0x41, 0xff}, // 22(55799(h'ff'))
			want:    "/w==",
		},
		{
			name: "tagged base64 into string tags ignored",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowed,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0xd6, 0x41, 0xff}, // 22(h'ff')
			want:    "\xff",
		},
		{
			name: "tagged into []byte with default encoding base64",
			opts: DecOptions{
				ByteStringExpectedFormat: ByteStringExpectedBase64,
			},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0xd6, 0x41, 0xff}, // 22(h'ff')
			want:    []byte{0xff},
		},
		{
			name: "indirectly tagged into []byte with default encoding base64",
			opts: DecOptions{
				ByteStringExpectedFormat: ByteStringExpectedBase64,
			},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0xd6, 0xd9, 0xd9, 0xf7, 0x41, 0xff}, // 22(55799(h'ff'))
			want:    []byte{0xff},
		},
		{
			name: "untagged base64 into []byte with default encoding base64",
			opts: DecOptions{
				ByteStringExpectedFormat: ByteStringExpectedBase64,
			},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0x44, '/', 'w', '=', '='}, // '/w=='
			want:    []byte{0xff},
		},
		{
			name: "tagged base16 into string",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0xd7, 0x41, 0xff}, // 23(h'ff')
			want:    "ff",
		},
		{
			name: "indirectly tagged base16 into string",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowedWithExpectedLaterEncoding,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0xd7, 0xd9, 0xd9, 0xf7, 0x41, 0xff}, // 23(55799(h'ff'))
			want:    "ff",
		},
		{
			name: "tagged base16 into string tags ignored",
			opts: DecOptions{
				ByteStringToString: ByteStringToStringAllowed,
			},
			dstType: reflect.TypeOf(""),
			in:      []byte{0xd7, 0x41, 0xff}, // 23(h'ff')
			want:    "\xff",
		},
		{
			name: "tagged into []byte with default encoding base16",
			opts: DecOptions{
				ByteStringExpectedFormat: ByteStringExpectedBase16,
			},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0xd7, 0x41, 0xff}, // 23(h'ff')
			want:    []byte{0xff},
		},
		{
			name: "indirectly tagged into []byte with default encoding base16",
			opts: DecOptions{
				ByteStringExpectedFormat: ByteStringExpectedBase16,
			},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0xd7, 0xd9, 0xd9, 0xf7, 0x41, 0xff}, // 23(55799(h'ff'))
			want:    []byte{0xff},
		},
		{
			name: "untagged base16 into []byte with default encoding base16",
			opts: DecOptions{
				ByteStringExpectedFormat: ByteStringExpectedBase16,
			},
			dstType: reflect.TypeOf([]byte{}),
			in:      []byte{0x42, 'f', 'f'},
			want:    []byte{0xff},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			dstVal := reflect.New(tc.dstType)
			if err := dm.Unmarshal(tc.in, dstVal.Interface()); err != nil {
				t.Fatal(err)
			}

			if dst := dstVal.Elem().Interface(); !reflect.DeepEqual(dst, tc.want) {
				t.Errorf("got: %#v, want %#v", dst, tc.want)
			}
		})
	}
}

func TestDecModeInvalidBinaryUnmarshaler(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{BinaryUnmarshaler: -1},
			wantErrorMsg: "cbor: invalid BinaryUnmarshaler -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{BinaryUnmarshaler: 101},
			wantErrorMsg: "cbor: invalid BinaryUnmarshaler 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

type testBinaryUnmarshaler []byte

func (bu *testBinaryUnmarshaler) UnmarshalBinary(_ []byte) error {
	*bu = []byte("UnmarshalBinary")
	return nil
}

func TestBinaryUnmarshalerMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts DecOptions
		in   []byte
		want any
	}{
		{
			name: "UnmarshalBinary is called by default",
			opts: DecOptions{},
			in:   []byte("\x45hello"), // 'hello'
			want: testBinaryUnmarshaler("UnmarshalBinary"),
		},
		{
			name: "UnmarshalBinary is called with BinaryUnmarshalerByteString",
			opts: DecOptions{BinaryUnmarshaler: BinaryUnmarshalerByteString},
			in:   []byte("\x45hello"), // 'hello'
			want: testBinaryUnmarshaler("UnmarshalBinary"),
		},
		{
			name: "default byte slice unmarshaling behavior is used with BinaryUnmarshalerNone",
			opts: DecOptions{BinaryUnmarshaler: BinaryUnmarshalerNone},
			in:   []byte("\x45hello"), // 'hello'
			want: testBinaryUnmarshaler("hello"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			gotrv := reflect.New(reflect.TypeOf(tc.want))
			if err := dm.Unmarshal(tc.in, gotrv.Interface()); err != nil {
				t.Fatal(err)
			}

			got := gotrv.Elem().Interface()
			if !reflect.DeepEqual(tc.want, got) {
				t.Errorf("want: %v, got: %v", tc.want, got)
			}
		})
	}
}

func TestDecModeInvalidBignumTag(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{BignumTag: -1},
			wantErrorMsg: "cbor: invalid BignumTag -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{BignumTag: 101},
			wantErrorMsg: "cbor: invalid BignumTag 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("Expected non nil error from DecMode()")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Expected error: %q, want: %q \n", tc.wantErrorMsg, err.Error())
			}
		})
	}
}

func TestBignumTagMode(t *testing.T) {
	for _, tc := range []struct {
		name           string
		opt            DecOptions
		input          []byte
		wantErrMessage string // if "" then expect nil error
	}{
		{
			name:  "default options decode unsigned bignum without error",
			opt:   DecOptions{},
			input: mustHexDecode("c240"), // 2(0) i.e. unsigned bignum 0
		},
		{
			name:  "default options decode negative bignum without error",
			opt:   DecOptions{},
			input: mustHexDecode("c340"), // 3(0) i.e. negative bignum -1
		},
		{
			name:           "BignumTagForbidden returns UnacceptableDataItemError on unsigned bignum",
			opt:            DecOptions{BignumTag: BignumTagForbidden},
			input:          mustHexDecode("c240"), // 2(0) i.e. unsigned bignum 0
			wantErrMessage: "cbor: data item of cbor type tag is not accepted by protocol: bignum",
		},
		{
			name:           "BignumTagForbidden returns UnacceptableDataItemError on negative bignum",
			opt:            DecOptions{BignumTag: BignumTagForbidden},
			input:          mustHexDecode("c340"), // 3(0) i.e. negative bignum -1
			wantErrMessage: "cbor: data item of cbor type tag is not accepted by protocol: bignum",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opt.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			for _, dstType := range []reflect.Type{
				typeInt64,
				typeFloat64,
				typeByteArray,
				typeByteSlice,
				typeBigInt,
				typeIntf,
			} {
				t.Run(dstType.String(), func(t *testing.T) {
					dstv := reflect.New(dstType)
					err = dm.Unmarshal(tc.input, dstv.Interface())
					if err != nil {
						if tc.wantErrMessage == "" {
							t.Errorf("want nil error, got: %v", err)
						} else if gotErrMessage := err.Error(); gotErrMessage != tc.wantErrMessage {
							t.Errorf("want error: %q, got error: %q", tc.wantErrMessage, gotErrMessage)
						}
					} else {
						if tc.wantErrMessage != "" {
							t.Errorf("got nil error, want: %s", tc.wantErrMessage)
						}
					}
				})
			}
		})
	}
}

func TestDecModeInvalidTextUnmarshaler(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "below range of valid modes",
			opts:         DecOptions{TextUnmarshaler: -1},
			wantErrorMsg: "cbor: invalid TextUnmarshaler -1",
		},
		{
			name:         "above range of valid modes",
			opts:         DecOptions{TextUnmarshaler: 101},
			wantErrorMsg: "cbor: invalid TextUnmarshaler 101",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.opts.DecMode()
			if err == nil {
				t.Errorf("DecMode() didn't return an error")
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("DecMode() returned error %q, want %q", err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

type testTextUnmarshaler string

func (tu *testTextUnmarshaler) UnmarshalText(_ []byte) error {
	*tu = "UnmarshalText"
	return nil
}

func TestTextUnmarshalerMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts DecOptions
		in   []byte
		want any
	}{
		{
			name: "UnmarshalText is not called by default",
			opts: DecOptions{},
			in:   []byte("\x65hello"), // "hello"
			want: testTextUnmarshaler("hello"),
		},
		{
			name: "UnmarshalText is called with TextUnmarshalerTextString",
			opts: DecOptions{TextUnmarshaler: TextUnmarshalerTextString},
			in:   []byte("\x65hello"), // "hello"
			want: testTextUnmarshaler("UnmarshalText"),
		},
		{
			name: "default text string unmarshaling behavior is used with TextUnmarshalerNone",
			opts: DecOptions{TextUnmarshaler: TextUnmarshalerNone},
			in:   []byte("\x65hello"), // "hello"
			want: testTextUnmarshaler("hello"),
		},
		{
			name: "UnmarshalText is called for byte string with TextUnmarshalerTextString and ByteStringToStringAllowed",
			opts: DecOptions{
				TextUnmarshaler:    TextUnmarshalerTextString,
				ByteStringToString: ByteStringToStringAllowed,
			},
			in:   []byte("\x45hello"), // 'hello'
			want: testTextUnmarshaler("UnmarshalText"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dm, err := tc.opts.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			gotrv := reflect.New(reflect.TypeOf(tc.want))
			if err := dm.Unmarshal(tc.in, gotrv.Interface()); err != nil {
				t.Fatal(err)
			}

			got := gotrv.Elem().Interface()
			if !reflect.DeepEqual(tc.want, got) {
				t.Errorf("want: %v, got: %v", tc.want, got)
			}
		})
	}
}

type errorTextUnmarshaler struct{}

func (u *errorTextUnmarshaler) UnmarshalText([]byte) error {
	return errors.New("test")
}

func TestTextUnmarshalerModeError(t *testing.T) {
	dec, err := DecOptions{TextUnmarshaler: TextUnmarshalerTextString}.DecMode()
	if err != nil {
		t.Fatal(err)
	}

	err = dec.Unmarshal([]byte{0x61, 'a'}, new(errorTextUnmarshaler))
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	if got, want := err.Error(), "cbor: cannot unmarshal text for *cbor.errorTextUnmarshaler: test"; got != want {
		t.Errorf("want: %q, got: %q", want, got)
	}
}

func TestJSONUnmarshalerTranscoder(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte

		transcodeInput  []byte
		transcodeOutput []byte
		transcodeError  error

		want         any
		wantErrorMsg string
	}{
		{
			name: "successful transcode",
			in:   []byte{0xf5},

			transcodeInput:  []byte{0xf5},
			transcodeOutput: []byte("true"),

			want: json.RawMessage("true"),
		},
		{
			name: "transcode returns non-nil error",
			in:   []byte{0xf5},

			transcodeInput: []byte{0xf5},
			transcodeError: errors.New("test"),

			want: json.RawMessage("true"),
			wantErrorMsg: TranscodeError{
				err:          errors.New("test"),
				rtype:        reflect.TypeOf((*json.RawMessage)(nil)),
				sourceFormat: "cbor",
				targetFormat: "json",
			}.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec, err := DecOptions{
				JSONUnmarshalerTranscoder: transcodeFunc(func(w io.Writer, r io.Reader) error {
					source, err := io.ReadAll(r)
					if err != nil {
						t.Fatal(err)
					}
					if got := string(source); got != string(tc.transcodeInput) {
						t.Errorf("transcoder got input %q, want %q", got, string(tc.transcodeInput))
					}

					if tc.transcodeError != nil {
						return tc.transcodeError
					}

					_, err = w.Write(tc.transcodeOutput)
					return err

				}),
			}.DecMode()
			if err != nil {
				t.Fatal(err)
			}

			gotrv := reflect.New(reflect.TypeOf(tc.want))
			err = dec.Unmarshal(tc.in, gotrv.Interface())
			if tc.wantErrorMsg != "" {
				if err == nil {
					t.Errorf("Unmarshal(0x%x) didn't return an error, want error %q", tc.in, tc.wantErrorMsg)
				} else if gotErrorMsg := err.Error(); gotErrorMsg != tc.wantErrorMsg {
					t.Errorf("Unmarshal(0x%x) returned error %q, want %q", tc.in, gotErrorMsg, tc.wantErrorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Unmarshal(0x%x) returned non-nil error %v", tc.in, err)
				} else if got := gotrv.Elem().Interface(); !reflect.DeepEqual(tc.want, got) {
					t.Errorf("Unmarshal(0x%x): %v, want %v", tc.in, got, tc.want)
				}
			}

		})
	}
}

// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor

import (
	"bytes"
	"testing"
)

func TestValid1(t *testing.T) {
	for _, mt := range marshalTests {
		if err := Wellformed(mt.wantData); err != nil {
			t.Errorf("Wellformed() returned error %v", err)
		}
	}
}

func TestValid2(t *testing.T) {
	for _, mt := range marshalTests {
		dm, _ := DecOptions{DupMapKey: DupMapKeyEnforcedAPF}.DecMode()
		if err := dm.Wellformed(mt.wantData); err != nil {
			t.Errorf("Wellformed() returned error %v", err)
		}
	}
}

func TestValidExtraneousData(t *testing.T) {
	testCases := []struct {
		name                     string
		data                     []byte
		extraneousDataNumOfBytes int
		extraneousDataIndex      int
	}{
		{
			name:                     "two numbers",
			data:                     []byte{0x00, 0x01},
			extraneousDataNumOfBytes: 1,
			extraneousDataIndex:      1,
		}, // 0, 1
		{
			name:                     "bytestring and int",
			data:                     []byte{0x44, 0x01, 0x02, 0x03, 0x04, 0x00},
			extraneousDataNumOfBytes: 1,
			extraneousDataIndex:      5,
		}, // h'01020304', 0
		{
			name:                     "int and partial array",
			data:                     []byte{0x00, 0x83, 0x01, 0x02},
			extraneousDataNumOfBytes: 3,
			extraneousDataIndex:      1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := Wellformed(tc.data)
			if err == nil {
				t.Errorf("Wellformed(0x%x) didn't return an error", tc.data)
			} else {
				ederr, ok := err.(*ExtraneousDataError)
				if !ok {
					t.Errorf("Wellformed(0x%x) error type %T, want *ExtraneousDataError", tc.data, err)
				} else if ederr.numOfBytes != tc.extraneousDataNumOfBytes {
					t.Errorf("Wellformed(0x%x) returned %d bytes of extraneous data, want %d", tc.data, ederr.numOfBytes, tc.extraneousDataNumOfBytes)
				} else if ederr.index != tc.extraneousDataIndex {
					t.Errorf("Wellformed(0x%x) returned extraneous data index %d, want %d", tc.data, ederr.index, tc.extraneousDataIndex)
				}
			}
		})
	}
}

func TestValidOnStreamingData(t *testing.T) {
	var buf bytes.Buffer
	for _, t := range marshalTests {
		buf.Write(t.wantData)
	}
	d := decoder{data: buf.Bytes(), dm: defaultDecMode}
	for i := 0; i < len(marshalTests); i++ {
		if err := d.wellformed(true, false); err != nil {
			t.Errorf("wellformed() returned error %v", err)
		}
	}
}

func TestDepth(t *testing.T) {
	testCases := []struct {
		name      string
		data      []byte
		wantDepth int
	}{
		{
			name:      "uint",
			data:      mustHexDecode("00"),
			wantDepth: 0,
		}, // 0
		{
			name:      "int",
			data:      mustHexDecode("20"),
			wantDepth: 0,
		}, // -1
		{
			name:      "bool",
			data:      mustHexDecode("f4"),
			wantDepth: 0,
		}, // false
		{
			name:      "nil",
			data:      mustHexDecode("f6"),
			wantDepth: 0,
		}, // nil
		{
			name:      "float",
			data:      mustHexDecode("fa47c35000"),
			wantDepth: 0,
		}, // 100000.0
		{
			name:      "byte string",
			data:      mustHexDecode("40"),
			wantDepth: 0,
		}, // []byte{}
		{
			name:      "indefinite length byte string",
			data:      mustHexDecode("5f42010243030405ff"),
			wantDepth: 0,
		}, // []byte{1, 2, 3, 4, 5}
		{
			name:      "text string",
			data:      mustHexDecode("60"),
			wantDepth: 0,
		}, // ""
		{
			name:      "indefinite length text string",
			data:      mustHexDecode("7f657374726561646d696e67ff"),
			wantDepth: 0,
		}, // "streaming"
		{
			name:      "empty array",
			data:      mustHexDecode("80"),
			wantDepth: 1,
		}, // []
		{
			name:      "indefinite length empty array",
			data:      mustHexDecode("9fff"),
			wantDepth: 1,
		}, // []
		{
			name:      "array",
			data:      mustHexDecode("98190102030405060708090a0b0c0d0e0f101112131415161718181819"),
			wantDepth: 1,
		}, // [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]
		{
			name:      "indefinite length array",
			data:      mustHexDecode("9f0102030405060708090a0b0c0d0e0f101112131415161718181819ff"),
			wantDepth: 1,
		}, // [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]
		{
			name:      "nested array",
			data:      mustHexDecode("8301820203820405"),
			wantDepth: 2,
		}, // [1,[2,3],[4,5]]
		{
			name:      "indefinite length nested array",
			data:      mustHexDecode("83018202039f0405ff"),
			wantDepth: 2,
		}, // [1,[2,3],[4,5]]
		{
			name:      "array and map",
			data:      mustHexDecode("826161a161626163"),
			wantDepth: 2,
		}, // [a", {"b": "c"}]
		{
			name:      "indefinite length array and map",
			data:      mustHexDecode("826161bf61626163ff"),
			wantDepth: 2,
		}, // [a", {"b": "c"}]
		{
			name:      "empty map",
			data:      mustHexDecode("a0"),
			wantDepth: 1,
		}, // {}
		{
			name:      "indefinite length empty map",
			data:      mustHexDecode("bfff"),
			wantDepth: 1,
		}, // {}
		{
			name:      "map",
			data:      mustHexDecode("a201020304"),
			wantDepth: 1,
		}, // {1:2, 3:4}
		{
			name:      "nested map",
			data:      mustHexDecode("a26161016162820203"),
			wantDepth: 2,
		}, // {"a": 1, "b": [2, 3]}
		{
			name:      "indefinite length nested map",
			data:      mustHexDecode("bf61610161629f0203ffff"),
			wantDepth: 2,
		}, // {"a": 1, "b": [2, 3]}
		{
			name:      "tag",
			data:      mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			wantDepth: 0,
		}, // 0("2013-03-21T20:04:00Z")
		{
			name:      "tagged map",
			data:      mustHexDecode("d864a26161016162820203"),
			wantDepth: 2,
		}, // 100({"a": 1, "b": [2, 3]})
		{
			name:      "tagged map and array",
			data:      mustHexDecode("d864a26161016162d865820203"),
			wantDepth: 2,
		}, // 100({"a": 1, "b": 101([2, 3])})
		{
			name:      "tagged map and array",
			data:      mustHexDecode("d864a26161016162d865d866820203"),
			wantDepth: 3,
		}, // 100({"a": 1, "b": 101(102([2, 3]))})
		{
			name:      "nested tag",
			data:      mustHexDecode("d864d865d86674323031332d30332d32315432303a30343a30305a"),
			wantDepth: 2,
		}, // 100(101(102("2013-03-21T20:04:00Z")))
		{
			name:      "32-level array",
			data:      mustHexDecode("82018181818181818181818181818181818181818181818181818181818181818101"),
			wantDepth: 32,
		},
		{
			name:      "32-level indefinite length array",
			data:      mustHexDecode("9f018181818181818181818181818181818181818181818181818181818181818101ff"),
			wantDepth: 32,
		},
		{
			name:      "32-level map",
			data:      mustHexDecode("a1018181818181818181818181818181818181818181818181818181818181818101"),
			wantDepth: 32,
		},
		{
			name:      "32-level indefinite length map",
			data:      mustHexDecode("bf018181818181818181818181818181818181818181818181818181818181818101ff"),
			wantDepth: 32,
		},
		{
			name:      "32-level tag",
			data:      mustHexDecode("d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d86474323031332d30332d32315432303a30343a30305a"),
			wantDepth: 32,
		}, // 100(100(...("2013-03-21T20:04:00Z")))
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := decoder{data: tc.data, dm: defaultDecMode}
			depth, err := d.wellformedInternal(0, false)
			if err != nil {
				t.Errorf("wellformed(0x%x) returned error %v", tc.data, err)
			}
			if depth != tc.wantDepth {
				t.Errorf("wellformed(0x%x) returned depth %d, want %d", tc.data, depth, tc.wantDepth)
			}
		})
	}
}

func TestDepthError(t *testing.T) {
	testCases := []struct {
		name         string
		data         []byte
		opts         DecOptions
		wantErrorMsg string
	}{
		{
			name:         "33-level array",
			data:         mustHexDecode("82018181818181818181818181818181818181818181818181818181818181818101"),
			opts:         DecOptions{MaxNestedLevels: 4},
			wantErrorMsg: "cbor: exceeded max nested level 4",
		},
		{
			name:         "33-level array",
			data:         mustHexDecode("82018181818181818181818181818181818181818181818181818181818181818101"),
			opts:         DecOptions{MaxNestedLevels: 10},
			wantErrorMsg: "cbor: exceeded max nested level 10",
		},
		{
			name:         "33-level array",
			data:         mustHexDecode("8201818181818181818181818181818181818181818181818181818181818181818101"),
			opts:         DecOptions{},
			wantErrorMsg: "cbor: exceeded max nested level 32",
		},
		{
			name:         "33-level indefinite length array",
			data:         mustHexDecode("9f01818181818181818181818181818181818181818181818181818181818181818101ff"),
			opts:         DecOptions{},
			wantErrorMsg: "cbor: exceeded max nested level 32",
		},
		{
			name:         "33-level map",
			data:         mustHexDecode("a101818181818181818181818181818181818181818181818181818181818181818101"),
			opts:         DecOptions{},
			wantErrorMsg: "cbor: exceeded max nested level 32",
		},
		{
			name:         "33-level indefinite length map",
			data:         mustHexDecode("bf01818181818181818181818181818181818181818181818181818181818181818101ff"),
			opts:         DecOptions{},
			wantErrorMsg: "cbor: exceeded max nested level 32",
		},
		{
			name:         "33-level tag",
			data:         mustHexDecode("d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d864d86474323031332d30332d32315432303a30343a30305a"),
			opts:         DecOptions{},
			wantErrorMsg: "cbor: exceeded max nested level 32",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dm, _ := tc.opts.decMode()
			d := decoder{data: tc.data, dm: dm}
			if _, err := d.wellformedInternal(0, false); err == nil {
				t.Errorf("wellformed(0x%x) didn't return an error", tc.data)
			} else if _, ok := err.(*MaxNestedLevelError); !ok {
				t.Errorf("wellformed(0x%x) returned wrong error type %T, want (*MaxNestedLevelError)", tc.data, err)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("wellformed(0x%x) returned error %q, want error %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

func TestValidBuiltinTagTest(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{
			name: "tag 0",
			data: mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
		},
		{
			name: "tag 1",
			data: mustHexDecode("c11a514b67b0"),
		},
		{
			name: "tag 2",
			data: mustHexDecode("c249010000000000000000"),
		},
		{
			name: "tag 3",
			data: mustHexDecode("c349010000000000000000"),
		},
		{
			name: "nested tag 0",
			data: mustHexDecode("d9d9f7c074323031332d30332d32315432303a30343a30305a"),
		},
		{
			name: "nested tag 1",
			data: mustHexDecode("d9d9f7c11a514b67b0"),
		},
		{
			name: "nested tag 2",
			data: mustHexDecode("d9d9f7c249010000000000000000"),
		},
		{
			name: "nested tag 3",
			data: mustHexDecode("d9d9f7c349010000000000000000"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := decoder{data: tc.data, dm: defaultDecMode}
			if err := d.wellformed(true, true); err != nil {
				t.Errorf("wellformed(0x%x) returned error %v", tc.data, err)
			}
		})
	}
}

func TestInvalidBuiltinTagTest(t *testing.T) {
	testCases := []struct {
		name         string
		data         []byte
		wantErrorMsg string
	}{
		{
			name:         "tag 0",
			data:         mustHexDecode("c01a514b67b0"),
			wantErrorMsg: "cbor: tag number 0 must be followed by text string, got positive integer",
		},
		{
			name:         "tag 1",
			data:         mustHexDecode("c174323031332d30332d32315432303a30343a30305a"),
			wantErrorMsg: "cbor: tag number 1 must be followed by integer or floating-point number, got UTF-8 text string",
		},
		{
			name:         "tag 2",
			data:         mustHexDecode("c269010000000000000000"),
			wantErrorMsg: "cbor: tag number 2 or 3 must be followed by byte string, got UTF-8 text string",
		},
		{
			name:         "tag 3",
			data:         mustHexDecode("c300"),
			wantErrorMsg: "cbor: tag number 2 or 3 must be followed by byte string, got positive integer",
		},
		{
			name:         "nested tag 0",
			data:         mustHexDecode("d9d9f7c01a514b67b0"),
			wantErrorMsg: "cbor: tag number 0 must be followed by text string, got positive integer",
		},
		{
			name:         "nested tag 1",
			data:         mustHexDecode("d9d9f7c174323031332d30332d32315432303a30343a30305a"),
			wantErrorMsg: "cbor: tag number 1 must be followed by integer or floating-point number, got UTF-8 text string",
		},
		{
			name:         "nested tag 2",
			data:         mustHexDecode("d9d9f7c269010000000000000000"),
			wantErrorMsg: "cbor: tag number 2 or 3 must be followed by byte string, got UTF-8 text string",
		},
		{
			name:         "nested tag 3",
			data:         mustHexDecode("d9d9f7c300"),
			wantErrorMsg: "cbor: tag number 2 or 3 must be followed by byte string, got positive integer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := decoder{data: tc.data, dm: defaultDecMode}
			err := d.wellformed(true, true)
			if err == nil {
				t.Errorf("wellformed(0x%x) didn't return an error", tc.data)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("wellformed(0x%x) error %q, want %q", tc.data, err.Error(), tc.wantErrorMsg)
			}
		})
	}
}

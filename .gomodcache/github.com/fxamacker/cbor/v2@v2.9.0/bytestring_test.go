// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor

import (
	"io"
	"strings"
	"testing"
)

func TestByteString(t *testing.T) {
	type s1 struct {
		A ByteString `cbor:"a"`
	}
	type s2 struct {
		A *ByteString `cbor:"a"`
	}
	type s3 struct {
		A ByteString `cbor:"a,omitempty"`
	}
	type s4 struct {
		A *ByteString `cbor:"a,omitempty"`
	}

	emptybs := ByteString("")
	bs := ByteString("\x01\x02\x03\x04")

	testCases := []roundTripTest{
		{
			name:         "empty",
			obj:          emptybs,
			wantCborData: mustHexDecode("40"),
		},
		{
			name:         "not empty",
			obj:          bs,
			wantCborData: mustHexDecode("4401020304"),
		},
		{
			name:         "array",
			obj:          []ByteString{bs},
			wantCborData: mustHexDecode("814401020304"),
		},
		{
			name:         "map with ByteString key",
			obj:          map[ByteString]bool{bs: true},
			wantCborData: mustHexDecode("a14401020304f5"),
		},
		{
			name:         "empty ByteString field",
			obj:          s1{},
			wantCborData: mustHexDecode("a1616140"),
		},
		{
			name:         "not empty ByteString field",
			obj:          s1{A: bs},
			wantCborData: mustHexDecode("a161614401020304"),
		},
		{
			name:         "nil *ByteString field",
			obj:          s2{},
			wantCborData: mustHexDecode("a16161f6"),
		},
		{
			name:         "empty *ByteString field",
			obj:          s2{A: &emptybs},
			wantCborData: mustHexDecode("a1616140"),
		},
		{
			name:         "not empty *ByteString field",
			obj:          s2{A: &bs},
			wantCborData: mustHexDecode("a161614401020304"),
		},
		{
			name:         "empty ByteString field with omitempty option",
			obj:          s3{},
			wantCborData: mustHexDecode("a0"),
		},
		{
			name:         "not empty ByteString field with omitempty option",
			obj:          s3{A: bs},
			wantCborData: mustHexDecode("a161614401020304"),
		},
		{
			name:         "nil *ByteString field with omitempty option",
			obj:          s4{},
			wantCborData: mustHexDecode("a0"),
		},
		{
			name:         "empty *ByteString field with omitempty option",
			obj:          s4{A: &emptybs},
			wantCborData: mustHexDecode("a1616140"),
		},
		{
			name:         "not empty *ByteString field with omitempty option",
			obj:          s4{A: &bs},
			wantCborData: mustHexDecode("a161614401020304"),
		},
	}

	em, _ := EncOptions{}.EncMode()
	dm, _ := DecOptions{}.DecMode()
	testRoundTrip(t, testCases, em, dm)
}

func TestUnmarshalByteStringOnBadData(t *testing.T) {
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
			errMsg: "cbor: cannot unmarshal positive integer into Go value of type cbor.ByteString",
		},
		{
			name:   "int type",
			data:   mustHexDecode("20"),
			errMsg: "cbor: cannot unmarshal negative integer into Go value of type cbor.ByteString",
		},
		{
			name:   "string type",
			data:   mustHexDecode("60"),
			errMsg: "cbor: cannot unmarshal UTF-8 text string into Go value of type cbor.ByteString",
		},
		{
			name:   "array type",
			data:   mustHexDecode("80"),
			errMsg: "cbor: cannot unmarshal array into Go value of type cbor.ByteString",
		},
		{
			name:   "map type",
			data:   mustHexDecode("a0"),
			errMsg: "cbor: cannot unmarshal map into Go value of type cbor.ByteString",
		},
		{
			name:   "tag type",
			data:   mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			errMsg: "cbor: cannot unmarshal tag into Go value of type cbor.ByteString",
		},
		{
			name:   "float type",
			data:   mustHexDecode("f90000"),
			errMsg: "cbor: cannot unmarshal primitives into Go value of type cbor.ByteString",
		},

		// Truncated CBOR data
		{
			name:   "truncated head",
			data:   mustHexDecode("18"),
			errMsg: io.ErrUnexpectedEOF.Error(),
		},

		// Truncated CBOR byte string
		{
			name:   "truncated byte string",
			data:   mustHexDecode("44010203"),
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
			// Test ByteString.UnmarshalCBOR(data)
			{
				var v ByteString

				err := v.UnmarshalCBOR(tc.data)
				if err == nil {
					t.Errorf("UnmarshalCBOR(%x) didn't return error", tc.data)
				}
				if !strings.HasPrefix(err.Error(), tc.errMsg) {
					t.Errorf("UnmarshalCBOR(%x) returned error %q, want %q", tc.data, err.Error(), tc.errMsg)
				}
			}
			// Test Unmarshal(data, *ByteString), which calls ByteString.unmarshalCBOR() under the hood
			{
				var v ByteString

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

// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestUnmarshalSimpleValue(t *testing.T) {
	t.Run("0..23", func(t *testing.T) {
		for i := 0; i <= 23; i++ {
			data := []byte{byte(cborTypePrimitives) | byte(i)}
			want := SimpleValue(i)

			switch i {
			case 20: // false
				testUnmarshalSimpleValueToEmptyInterface(t, data, false)
			case 21: // true
				testUnmarshalSimpleValueToEmptyInterface(t, data, true)
			case 22: // null
				testUnmarshalSimpleValueToEmptyInterface(t, data, nil)
			case 23: // undefined
				testUnmarshalSimpleValueToEmptyInterface(t, data, nil)
			default:
				testUnmarshalSimpleValueToEmptyInterface(t, data, want)
			}

			testUnmarshalSimpleValue(t, data, want)
		}
	})

	t.Run("24..31", func(t *testing.T) {
		for i := 24; i <= 31; i++ {
			data := []byte{byte(cborTypePrimitives) | byte(24), byte(i)}

			testUnmarshalInvalidSimpleValueToEmptyInterface(t, data)
			testUnmarshalInvalidSimpleValue(t, data)
		}
	})

	t.Run("32..255", func(t *testing.T) {
		for i := 32; i <= 255; i++ {
			data := []byte{byte(cborTypePrimitives) | byte(24), byte(i)}
			want := SimpleValue(i)
			testUnmarshalSimpleValueToEmptyInterface(t, data, want)
			testUnmarshalSimpleValue(t, data, want)
		}
	})
}

func TestUnmarshalSimpleValueOnBadData(t *testing.T) {
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
			errMsg: "cbor: cannot unmarshal positive integer into Go value of type SimpleValue",
		},
		{
			name:   "int type",
			data:   mustHexDecode("20"),
			errMsg: "cbor: cannot unmarshal negative integer into Go value of type SimpleValue",
		},
		{
			name:   "byte string type",
			data:   mustHexDecode("40"),
			errMsg: "cbor: cannot unmarshal byte string into Go value of type SimpleValue",
		},
		{
			name:   "string type",
			data:   mustHexDecode("60"),
			errMsg: "cbor: cannot unmarshal UTF-8 text string into Go value of type SimpleValue",
		},
		{
			name:   "array type",
			data:   mustHexDecode("80"),
			errMsg: "cbor: cannot unmarshal array into Go value of type SimpleValue",
		},
		{
			name:   "map type",
			data:   mustHexDecode("a0"),
			errMsg: "cbor: cannot unmarshal map into Go value of type SimpleValue",
		},
		{
			name:   "tag type",
			data:   mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			errMsg: "cbor: cannot unmarshal tag into Go value of type SimpleValue",
		},
		{
			name:   "float type",
			data:   mustHexDecode("f90000"),
			errMsg: "cbor: cannot unmarshal primitives into Go value of type SimpleValue",
		},

		// Truncated CBOR data
		{
			name:   "truncated head",
			data:   mustHexDecode("18"),
			errMsg: io.ErrUnexpectedEOF.Error(),
		},

		// Truncated CBOR simple value
		{
			name:   "truncated simple value",
			data:   mustHexDecode("f8"),
			errMsg: io.ErrUnexpectedEOF.Error(),
		},

		// Invalid simple value
		{
			name:   "invalid simple value",
			data:   mustHexDecode("f800"),
			errMsg: "cbor: invalid simple value 0 for type primitives",
		},

		// Extraneous CBOR data
		{
			name:   "extraneous data",
			data:   mustHexDecode("f4f5"),
			errMsg: "cbor: 1 bytes of extraneous data starting at index 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test SimpleValue.UnmarshalCBOR(data)
			{
				var v SimpleValue

				err := v.UnmarshalCBOR(tc.data)
				if err == nil {
					t.Errorf("UnmarshalCBOR(%x) didn't return error", tc.data)
				}
				if !strings.HasPrefix(err.Error(), tc.errMsg) {
					t.Errorf("UnmarshalCBOR(%x) returned error %q, want %q", tc.data, err.Error(), tc.errMsg)
				}
			}
			// Test Unmarshal(data, *SimpleValue), which calls SimpleValue.unmarshalCBOR() under the hood
			{
				var v SimpleValue

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

func testUnmarshalInvalidSimpleValueToEmptyInterface(t *testing.T, data []byte) {
	var v any
	if err := Unmarshal(data, v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*SyntaxError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*SyntaxError)", data, err)
	}
}

func testUnmarshalInvalidSimpleValue(t *testing.T, data []byte) {
	var v SimpleValue
	if err := Unmarshal(data, v); err == nil {
		t.Errorf("Unmarshal(0x%x) didn't return an error", data)
	} else if _, ok := err.(*SyntaxError); !ok {
		t.Errorf("Unmarshal(0x%x) returned wrong error type %T, want (*SyntaxError)", data, err)
	}
}

func testUnmarshalSimpleValueToEmptyInterface(t *testing.T, data []byte, want any) {
	var v any
	if err := Unmarshal(data, &v); err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		return
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, want, want)
	}
}

func testUnmarshalSimpleValue(t *testing.T, data []byte, want SimpleValue) {
	cborNil := isCBORNil(data)

	// Decode to SimpleValue
	var v SimpleValue
	err := Unmarshal(data, &v)
	if err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		return
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, want, want)
	}

	// Decode to uninitialized *SimpleValue
	var pv *SimpleValue
	err = Unmarshal(data, &pv)
	if err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		return
	}
	if cborNil {
		if pv != nil {
			t.Errorf("Unmarshal(0x%x) returned %v, want nil *SimpleValue", data, *pv)
		}
	} else {
		if !reflect.DeepEqual(*pv, want) {
			t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, *pv, *pv, want, want)
		}
	}

	// Decode to initialized *SimpleValue
	v = SimpleValue(0)
	pv = &v
	err = Unmarshal(data, &pv)
	if err != nil {
		t.Errorf("Unmarshal(0x%x) returned error %v", data, err)
		return
	}
	if cborNil {
		if pv != nil {
			t.Errorf("Unmarshal(0x%x) returned %v, want nil *SimpleValue", data, *pv)
		}
	} else {
		if !reflect.DeepEqual(v, want) {
			t.Errorf("Unmarshal(0x%x) = %v (%T), want %v (%T)", data, v, v, want, want)
		}
	}
}

func TestMarshalSimpleValue(t *testing.T) {
	t.Run("0..23", func(t *testing.T) {
		for i := 0; i <= 23; i++ {
			wantData := []byte{byte(cborTypePrimitives) | byte(i)}
			v := SimpleValue(i)

			data, err := Marshal(v)
			if err != nil {
				t.Errorf("Marshal(%v) returned error %v", v, err)
				continue
			}
			if !bytes.Equal(data, wantData) {
				t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, data, wantData)
			}
		}
	})

	t.Run("24..31", func(t *testing.T) {
		for i := 24; i <= 31; i++ {
			v := SimpleValue(i)

			if data, err := Marshal(v); err == nil {
				t.Errorf("Marshal(%v) didn't return an error", data)
			} else if _, ok := err.(*UnsupportedValueError); !ok {
				t.Errorf("Marshal(%v) returned wrong error type %T, want (*UnsupportedValueError)", data, err)
			}
		}
	})

	t.Run("32..255", func(t *testing.T) {
		for i := 32; i <= 255; i++ {
			wantData := []byte{byte(cborTypePrimitives) | byte(24), byte(i)}
			v := SimpleValue(i)

			data, err := Marshal(v)
			if err != nil {
				t.Errorf("Marshal(%v) returned error %v", v, err)
				continue
			}
			if !bytes.Equal(data, wantData) {
				t.Errorf("Marshal(%v) = 0x%x, want 0x%x", v, data, wantData)
			}
		}
	})
}

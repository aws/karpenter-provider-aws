// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDecoder(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 5; i++ {
		for _, tc := range unmarshalTests {
			buf.Write(tc.data)
		}
	}

	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "bytes.Buffer",
			reader: &buf,
		},
		{
			name:   "1 byte reader",
			reader: newNBytesReader(buf.Bytes(), 1),
		},
		{
			name:   "toggled reader",
			reader: newToggledReader(buf.Bytes(), 1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(tc.reader)
			bytesRead := 0
			for i := 0; i < 5; i++ {
				for _, tc := range unmarshalTests {
					var v any
					if err := decoder.Decode(&v); err != nil {
						t.Fatalf("Decode() returned error %v", err)
					}
					if tm, ok := tc.wantInterfaceValue.(time.Time); ok {
						if vt, ok := v.(time.Time); !ok || !tm.Equal(vt) {
							t.Errorf("Decode() = %v (%T), want %v (%T)", v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
						}
					} else if !reflect.DeepEqual(v, tc.wantInterfaceValue) {
						t.Errorf("Decode() = %v (%T), want %v (%T)", v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
					}
					bytesRead += len(tc.data)
					if decoder.NumBytesRead() != bytesRead {
						t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), bytesRead)
					}
				}
			}
			for i := 0; i < 2; i++ {
				// no more data
				var v any
				err := decoder.Decode(&v)
				if v != nil {
					t.Errorf("Decode() = %v (%T), want nil (no more data)", v, v)
				}
				if err != io.EOF {
					t.Errorf("Decode() returned error %v, want io.EOF (no more data)", err)
				}
			}
		})
	}
}

func TestDecoderUnmarshalTypeError(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 5; i++ {
		for _, tc := range unmarshalTests {
			for j := 0; j < len(tc.wrongTypes)*2; j++ {
				buf.Write(tc.data)
			}
		}
	}

	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "bytes.Buffer",
			reader: &buf,
		},
		{
			name:   "1 byte reader",
			reader: newNBytesReader(buf.Bytes(), 1),
		},
		{
			name:   "toggled reader",
			reader: newToggledReader(buf.Bytes(), 1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(tc.reader)
			bytesRead := 0
			for i := 0; i < 5; i++ {
				for _, tc := range unmarshalTests {
					for _, typ := range tc.wrongTypes {
						v := reflect.New(typ)
						if err := decoder.Decode(v.Interface()); err == nil {
							t.Errorf("Decode(0x%x) didn't return an error, want UnmarshalTypeError", tc.data)
						} else if _, ok := err.(*UnmarshalTypeError); !ok {
							t.Errorf("Decode(0x%x) returned wrong error type %T, want UnmarshalTypeError", tc.data, err)
						}
						bytesRead += len(tc.data)
						if decoder.NumBytesRead() != bytesRead {
							t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), bytesRead)
						}

						var vi any
						if err := decoder.Decode(&vi); err != nil {
							t.Errorf("Decode() returned error %v", err)
						}
						if tm, ok := tc.wantInterfaceValue.(time.Time); ok {
							if vt, ok := vi.(time.Time); !ok || !tm.Equal(vt) {
								t.Errorf("Decode() = %v (%T), want %v (%T)", vi, vi, tc.wantInterfaceValue, tc.wantInterfaceValue)
							}
						} else if !reflect.DeepEqual(vi, tc.wantInterfaceValue) {
							t.Errorf("Decode() = %v (%T), want %v (%T)", vi, vi, tc.wantInterfaceValue, tc.wantInterfaceValue)
						}
						bytesRead += len(tc.data)
						if decoder.NumBytesRead() != bytesRead {
							t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), bytesRead)
						}
					}
				}
			}
			for i := 0; i < 2; i++ {
				// no more data
				var v any
				err := decoder.Decode(&v)
				if v != nil {
					t.Errorf("Decode() = %v (%T), want nil (no more data)", v, v)
				}
				if err != io.EOF {
					t.Errorf("Decode() returned error %v, want io.EOF (no more data)", err)
				}
			}
		})
	}
}

func TestDecoderUnexpectedEOFError(t *testing.T) {
	var buf bytes.Buffer
	for _, tc := range unmarshalTests {
		buf.Write(tc.data)
	}
	buf.Truncate(buf.Len() - 1)

	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "bytes.Buffer",
			reader: &buf,
		},
		{
			name:   "1 byte reader",
			reader: newNBytesReader(buf.Bytes(), 1),
		},
		{
			name:   "toggled reader",
			reader: newToggledReader(buf.Bytes(), 1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			decoder := NewDecoder(tc.reader)
			bytesRead := 0
			for i := 0; i < len(unmarshalTests)-1; i++ {
				tc := unmarshalTests[i]
				var v any
				if err := decoder.Decode(&v); err != nil {
					t.Fatalf("Decode() returned error %v", err)
				}
				if tm, ok := tc.wantInterfaceValue.(time.Time); ok {
					if vt, ok := v.(time.Time); !ok || !tm.Equal(vt) {
						t.Errorf("Decode() = %v (%T), want %v (%T)", v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
					}
				} else if !reflect.DeepEqual(v, tc.wantInterfaceValue) {
					t.Errorf("Decode() = %v (%T), want %v (%T)", v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
				}
				bytesRead += len(tc.data)
				if decoder.NumBytesRead() != bytesRead {
					t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), bytesRead)
				}
			}
			for i := 0; i < 2; i++ {
				// truncated data
				var v any
				err := decoder.Decode(&v)
				if v != nil {
					t.Errorf("Decode() = %v (%T), want nil (no more data)", v, v)
				}
				if err != io.ErrUnexpectedEOF {
					t.Errorf("Decode() returned error %v, want io.UnexpectedEOF (truncated data)", err)
				}
			}
		})
	}
}

func TestDecoderReadError(t *testing.T) {
	var buf bytes.Buffer
	for _, tc := range unmarshalTests {
		buf.Write(tc.data)
	}
	buf.Truncate(buf.Len() - 1)

	readerErr := errors.New("reader error")

	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "byte reader",
			reader: newNBytesReaderWithError(buf.Bytes(), 512, readerErr),
		},
		{
			name:   "1 byte reader",
			reader: newNBytesReaderWithError(buf.Bytes(), 1, readerErr),
		},
		{
			name:   "toggled reader",
			reader: newToggledReaderWithError(buf.Bytes(), 1, readerErr),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(tc.reader)
			bytesRead := 0
			for i := 0; i < len(unmarshalTests)-1; i++ {
				tc := unmarshalTests[i]
				var v any
				if err := decoder.Decode(&v); err != nil {
					t.Fatalf("Decode() returned error %v", err)
				}
				if tm, ok := tc.wantInterfaceValue.(time.Time); ok {
					if vt, ok := v.(time.Time); !ok || !tm.Equal(vt) {
						t.Errorf("Decode() = %v (%T), want %v (%T)", v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
					}
				} else if !reflect.DeepEqual(v, tc.wantInterfaceValue) {
					t.Errorf("Decode() = %v (%T), want %v (%T)", v, v, tc.wantInterfaceValue, tc.wantInterfaceValue)
				}
				bytesRead += len(tc.data)
				if decoder.NumBytesRead() != bytesRead {
					t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), bytesRead)
				}
			}
			for i := 0; i < 2; i++ {
				// truncated data because Reader returned error
				var v any
				err := decoder.Decode(&v)
				if v != nil {
					t.Errorf("Decode() = %v (%T), want nil (no more data)", v, v)
				}
				if err != readerErr {
					t.Errorf("Decode() returned error %v, want reader error", err)
				}
			}
		})
	}
}

func TestDecoderNoData(t *testing.T) {
	readerErr := errors.New("reader error")

	testCases := []struct {
		name    string
		reader  io.Reader
		wantErr error
	}{
		{
			name:    "byte.Buffer",
			reader:  new(bytes.Buffer),
			wantErr: io.EOF,
		},
		{
			name:    "1 byte reader",
			reader:  newNBytesReaderWithError(nil, 0, readerErr),
			wantErr: readerErr,
		},
		{
			name:    "toggled reader",
			reader:  newToggledReaderWithError(nil, 0, readerErr),
			wantErr: readerErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(tc.reader)
			for i := 0; i < 2; i++ {
				var v any
				err := decoder.Decode(&v)
				if v != nil {
					t.Errorf("Decode() = %v (%T), want nil", v, v)
				}
				if err != tc.wantErr {
					t.Errorf("Decode() returned error %v, want error %v", err, tc.wantErr)
				}
			}
		})
	}
}

func TestDecoderRecoverableReadError(t *testing.T) {
	data := mustHexDecode("83010203") // [1,2,3]
	wantValue := []any{uint64(1), uint64(2), uint64(3)}
	recoverableReaderErr := errors.New("recoverable reader error")

	decoder := NewDecoder(newRecoverableReader(data, 1, recoverableReaderErr))

	var v any
	err := decoder.Decode(&v)
	if err != recoverableReaderErr {
		t.Fatalf("Decode() returned error %v, want error %v", err, recoverableReaderErr)
	}

	err = decoder.Decode(&v)
	if err != nil {
		t.Fatalf("Decode() returned error %v", err)
	}
	if !reflect.DeepEqual(v, wantValue) {
		t.Errorf("Decode() = %v (%T), want %v (%T)", v, v, wantValue, wantValue)
	}
	if decoder.NumBytesRead() != len(data) {
		t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), len(data))
	}

	// no more data
	v = any(nil)
	err = decoder.Decode(&v)
	if v != nil {
		t.Errorf("Decode() = %v (%T), want nil (no more data)", v, v)
	}
	if err != io.EOF {
		t.Errorf("Decode() returned error %v, want io.EOF (no more data)", err)
	}
}

func TestDecoderInvalidData(t *testing.T) {
	data := []byte{0x01, 0x3e}
	decoder := NewDecoder(bytes.NewReader(data))

	var v1 any
	err := decoder.Decode(&v1)
	if err != nil {
		t.Errorf("Decode() returned error %v when decoding valid data item", err)
	}

	var v2 any
	err = decoder.Decode(&v2)
	if err == nil {
		t.Errorf("Decode() didn't return error when decoding invalid data item")
	} else if !strings.Contains(err.Error(), "cbor: invalid additional information") {
		t.Errorf("Decode() error %q, want \"cbor: invalid additional information\"", err)
	}
}

func TestDecoderSkip(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 5; i++ {
		for _, tc := range unmarshalTests {
			buf.Write(tc.data)
		}
	}

	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "bytes.Buffer",
			reader: &buf,
		},
		{
			name:   "1 byte reader",
			reader: newNBytesReader(buf.Bytes(), 1),
		},
		{
			name:   "toggled reader",
			reader: newToggledReader(buf.Bytes(), 1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(tc.reader)
			bytesRead := 0
			for i := 0; i < 5; i++ {
				for _, tc := range unmarshalTests {
					if err := decoder.Skip(); err != nil {
						t.Fatalf("Skip() returned error %v", err)
					}
					bytesRead += len(tc.data)
					if decoder.NumBytesRead() != bytesRead {
						t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), bytesRead)
					}
				}
			}
			for i := 0; i < 2; i++ {
				// no more data
				err := decoder.Skip()
				if err != io.EOF {
					t.Errorf("Skip() returned error %v, want io.EOF (no more data)", err)
				}
			}
		})
	}
}

func TestDecoderSkipInvalidDataError(t *testing.T) {
	var buf bytes.Buffer
	for _, tc := range unmarshalTests {
		buf.Write(tc.data)
	}
	buf.WriteByte(0x3e)

	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "bytes.Buffer",
			reader: &buf,
		},
		{
			name:   "1 byte reader",
			reader: newNBytesReader(buf.Bytes(), 1),
		},
		{
			name:   "toggled reader",
			reader: newToggledReader(buf.Bytes(), 1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(tc.reader)
			bytesRead := 0
			for i := 0; i < len(unmarshalTests); i++ {
				tc := unmarshalTests[i]
				if err := decoder.Skip(); err != nil {
					t.Fatalf("Skip() returned error %v", err)
				}
				bytesRead += len(tc.data)
				if decoder.NumBytesRead() != bytesRead {
					t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), bytesRead)
				}
			}
			for i := 0; i < 2; i++ {
				// last data item is invalid
				err := decoder.Skip()
				if err == nil {
					t.Fatalf("Skip() didn't return error")
				} else if !strings.Contains(err.Error(), "cbor: invalid additional information") {
					t.Errorf("Skip() error %q, want \"cbor: invalid additional information\"", err)
				}
			}
		})
	}
}

func TestDecoderSkipUnexpectedEOFError(t *testing.T) {
	var buf bytes.Buffer
	for _, tc := range unmarshalTests {
		buf.Write(tc.data)
	}
	buf.Truncate(buf.Len() - 1)

	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "bytes.Buffer",
			reader: &buf,
		},
		{
			name:   "1 byte reader",
			reader: newNBytesReader(buf.Bytes(), 1),
		},
		{
			name:   "toggled reader",
			reader: newToggledReader(buf.Bytes(), 1),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(tc.reader)
			bytesRead := 0
			for i := 0; i < len(unmarshalTests)-1; i++ {
				tc := unmarshalTests[i]
				if err := decoder.Skip(); err != nil {
					t.Fatalf("Skip() returned error %v", err)
				}
				bytesRead += len(tc.data)
				if decoder.NumBytesRead() != bytesRead {
					t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), bytesRead)
				}
			}
			for i := 0; i < 2; i++ {
				// last data item is invalid
				err := decoder.Skip()
				if err != io.ErrUnexpectedEOF {
					t.Errorf("Skip() returned error %v, want io.ErrUnexpectedEOF (truncated data)", err)
				}
			}
		})
	}
}

func TestDecoderSkipReadError(t *testing.T) {
	var buf bytes.Buffer
	for _, tc := range unmarshalTests {
		buf.Write(tc.data)
	}
	buf.Truncate(buf.Len() - 1)

	readerErr := errors.New("reader error")

	testCases := []struct {
		name   string
		reader io.Reader
	}{
		{
			name:   "byte reader",
			reader: newNBytesReaderWithError(buf.Bytes(), 512, readerErr),
		},
		{
			name:   "1 byte reader",
			reader: newNBytesReaderWithError(buf.Bytes(), 1, readerErr),
		},
		{
			name:   "toggled reader",
			reader: newToggledReaderWithError(buf.Bytes(), 1, readerErr),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(tc.reader)
			bytesRead := 0
			for i := 0; i < len(unmarshalTests)-1; i++ {
				tc := unmarshalTests[i]
				if err := decoder.Skip(); err != nil {
					t.Fatalf("Skip() returned error %v", err)
				}
				bytesRead += len(tc.data)
				if decoder.NumBytesRead() != bytesRead {
					t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), bytesRead)
				}
			}
			for i := 0; i < 2; i++ {
				// truncated data because Reader returned error
				err := decoder.Skip()
				if err != readerErr {
					t.Errorf("Skip() returned error %v, want reader error", err)
				}
			}
		})
	}
}

func TestDecoderSkipNoData(t *testing.T) {
	readerErr := errors.New("reader error")

	testCases := []struct {
		name    string
		reader  io.Reader
		wantErr error
	}{
		{
			name:    "byte.Buffer",
			reader:  new(bytes.Buffer),
			wantErr: io.EOF,
		},
		{
			name:    "1 byte reader",
			reader:  newNBytesReaderWithError(nil, 0, readerErr),
			wantErr: readerErr,
		},
		{
			name:    "toggled reader",
			reader:  newToggledReaderWithError(nil, 0, readerErr),
			wantErr: readerErr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := NewDecoder(tc.reader)
			for i := 0; i < 2; i++ {
				err := decoder.Skip()
				if err != tc.wantErr {
					t.Errorf("Decode() returned error %v, want error %v", err, tc.wantErr)
				}
			}
		})
	}
}

func TestDecoderSkipRecoverableReadError(t *testing.T) {
	data := mustHexDecode("83010203") // [1,2,3]
	recoverableReaderErr := errors.New("recoverable reader error")

	decoder := NewDecoder(newRecoverableReader(data, 1, recoverableReaderErr))

	err := decoder.Skip()
	if err != recoverableReaderErr {
		t.Fatalf("Skip() returned error %v, want error %v", err, recoverableReaderErr)
	}

	err = decoder.Skip()
	if err != nil {
		t.Fatalf("Skip() returned error %v", err)
	}
	if decoder.NumBytesRead() != len(data) {
		t.Errorf("NumBytesRead() = %v, want %v", decoder.NumBytesRead(), len(data))
	}

	// no more data
	err = decoder.Skip()
	if err != io.EOF {
		t.Errorf("Skip() returned error %v, want io.EOF (no more data)", err)
	}
}

func TestDecoderStructTag(t *testing.T) {
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
	dec := NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&v); err != nil {
		t.Errorf("Decode() returned error %v", err)
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Decode() = %+v (%T), want %+v (%T)", v, v, want, want)
	}
}

func TestDecoderBuffered(t *testing.T) {
	testCases := []struct {
		name      string
		data      []byte
		buffered  []byte
		decodeErr error
	}{
		{
			name:      "empty",
			data:      []byte{},
			buffered:  []byte{},
			decodeErr: io.EOF,
		},
		{
			name:      "malformed CBOR data item",
			data:      []byte{0xc0},
			buffered:  []byte{0xc0},
			decodeErr: io.ErrUnexpectedEOF,
		},
		{
			name:     "1 CBOR data item",
			data:     []byte{0xc2, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			buffered: []byte{},
		},
		{
			name: "2 CBOR data items",
			data: []byte{
				// First CBOR data item
				0xc2, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				// Second CBOR data item
				0xc3, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			buffered: []byte{
				// Second CBOR data item
				0xc3, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			name: "1 CBOR data item followed by non-CBOR data",
			data: []byte{
				// CBOR data item
				0xc2, 0x49, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				// Extraneous non-CBOR data ("abc")
				0x61, 0x62, 0x63,
			},
			buffered: []byte{
				// non-CBOR data ("abc")
				0x61, 0x62, 0x63,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := bytes.NewReader(tc.data)

			decoder := NewDecoder(r)

			// Decoder's buffer has no data yet.
			br := decoder.Buffered()
			buffered, err := io.ReadAll(br)
			if err != nil {
				t.Errorf("failed to read from reader returned by Buffered(): %v", err)
			}
			if len(buffered) > 0 {
				t.Errorf("Buffered() = 0x%x (%d bytes), want 0 bytes", buffered, len(buffered))
			}

			var v any
			err = decoder.Decode(&v)
			if err != tc.decodeErr {
				t.Errorf("Decode() returned error %v, want %v", err, tc.decodeErr)
			}

			br = decoder.Buffered()
			buffered, err = io.ReadAll(br)
			if err != nil {
				t.Errorf("failed to read from reader returned by Buffered(): %v", err)
			}
			if !bytes.Equal(tc.buffered, buffered) {
				t.Errorf("Buffered() = 0x%x (%d bytes), want 0x%x (%d bytes)", buffered, len(buffered), tc.buffered, len(tc.buffered))
			}
		})
	}
}

func TestEncoder(t *testing.T) {
	var want bytes.Buffer
	var w bytes.Buffer
	em, err := CanonicalEncOptions().EncMode()
	if err != nil {
		t.Errorf("EncMode() returned an error %v", err)
	}
	encoder := em.NewEncoder(&w)
	for _, tc := range marshalTests {
		for _, value := range tc.values {
			want.Write(tc.wantData)

			if err := encoder.Encode(value); err != nil {
				t.Fatalf("Encode() returned error %v", err)
			}
		}
	}
	if !bytes.Equal(w.Bytes(), want.Bytes()) {
		t.Errorf("Encoding mismatch: got %v, want %v", w.Bytes(), want.Bytes())
	}
}

func TestEncoderError(t *testing.T) {
	testcases := []struct {
		name         string
		value        any
		wantErrorMsg string
	}{
		{
			name:         "channel cannot be marshaled",
			value:        make(chan bool),
			wantErrorMsg: "cbor: unsupported type: chan bool",
		},
		{
			name:         "function cannot be marshaled",
			value:        func(i int) int { return i * i },
			wantErrorMsg: "cbor: unsupported type: func(int) int",
		},
		{
			name:         "complex cannot be marshaled",
			value:        complex(100, 8),
			wantErrorMsg: "cbor: unsupported type: complex128",
		},
	}
	var w bytes.Buffer
	encoder := NewEncoder(&w)
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.value
			err := encoder.Encode(&v)
			if err == nil {
				t.Errorf("Encode(%v) didn't return an error, want error %q", tc.value, tc.wantErrorMsg)
			} else if _, ok := err.(*UnsupportedTypeError); !ok {
				t.Errorf("Encode(%v) error type %T, want *UnsupportedTypeError", tc.value, err)
			} else if err.Error() != tc.wantErrorMsg {
				t.Errorf("Encode(%v) error %q, want %q", tc.value, err.Error(), tc.wantErrorMsg)
			}
		})
	}
	if w.Len() != 0 {
		t.Errorf("Encoder's writer has %d bytes of data, want empty data", w.Len())
	}
}

func TestIndefiniteByteString(t *testing.T) {
	want := mustHexDecode("5f42010243030405ff")
	var w bytes.Buffer
	encoder := NewEncoder(&w)
	if err := encoder.StartIndefiniteByteString(); err != nil {
		t.Fatalf("StartIndefiniteByteString() returned error %v", err)
	}
	if err := encoder.Encode([]byte{1, 2}); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.Encode([3]byte{3, 4, 5}); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.EndIndefinite(); err != nil {
		t.Fatalf("EndIndefinite() returned error %v", err)
	}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("Encoding mismatch: got %v, want %v", w.Bytes(), want)
	}
}

func TestIndefiniteByteStringError(t *testing.T) {
	var w bytes.Buffer
	encoder := NewEncoder(&w)
	if err := encoder.StartIndefiniteByteString(); err != nil {
		t.Fatalf("StartIndefiniteByteString() returned error %v", err)
	}
	if err := encoder.Encode([]int{1, 2}); err == nil {
		t.Errorf("Encode() didn't return an error")
	} else if err.Error() != "cbor: cannot encode item type slice for indefinite-length byte string" {
		t.Errorf("Encode() returned error %q, want %q", err.Error(), "cbor: cannot encode item type slice for indefinite-length byte string")
	}
	if err := encoder.Encode("hello"); err == nil {
		t.Errorf("Encode() didn't return an error")
	} else if err.Error() != "cbor: cannot encode item type string for indefinite-length byte string" {
		t.Errorf("Encode() returned error %q, want %q", err.Error(), "cbor: cannot encode item type string for indefinite-length byte string")
	}
}

func TestIndefiniteTextString(t *testing.T) {
	want := mustHexDecode("7f657374726561646d696e67ff")
	var w bytes.Buffer
	encoder := NewEncoder(&w)
	if err := encoder.StartIndefiniteTextString(); err != nil {
		t.Fatalf("StartIndefiniteTextString() returned error %v", err)
	}
	if err := encoder.Encode("strea"); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.Encode("ming"); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.EndIndefinite(); err != nil {
		t.Fatalf("EndIndefinite() returned error %v", err)
	}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("Encoding mismatch: got %v, want %v", w.Bytes(), want)
	}
}

func TestIndefiniteTextStringError(t *testing.T) {
	var w bytes.Buffer
	encoder := NewEncoder(&w)
	if err := encoder.StartIndefiniteTextString(); err != nil {
		t.Fatalf("StartIndefiniteTextString() returned error %v", err)
	}
	if err := encoder.Encode([]byte{1, 2}); err == nil {
		t.Errorf("Encode() didn't return an error")
	} else if err.Error() != "cbor: cannot encode item type slice for indefinite-length text string" {
		t.Errorf("Encode() returned error %q, want %q", err.Error(), "cbor: cannot encode item type slice for indefinite-length text string")
	}
}

func TestIndefiniteArray(t *testing.T) {
	want := mustHexDecode("9f018202039f0405ffff")
	var w bytes.Buffer
	encoder := NewEncoder(&w)
	if err := encoder.StartIndefiniteArray(); err != nil {
		t.Fatalf("StartIndefiniteArray() returned error %v", err)
	}
	if err := encoder.Encode(1); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.Encode([]int{2, 3}); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.StartIndefiniteArray(); err != nil {
		t.Fatalf("StartIndefiniteArray() returned error %v", err)
	}
	if err := encoder.Encode(4); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.Encode(5); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.EndIndefinite(); err != nil {
		t.Fatalf("EndIndefinite() returned error %v", err)
	}
	if err := encoder.EndIndefinite(); err != nil {
		t.Fatalf("EndIndefinite() returned error %v", err)
	}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("Encoding mismatch: got %v, want %v", w.Bytes(), want)
	}
}

func TestIndefiniteMap(t *testing.T) {
	want := mustHexDecode("bf61610161629f0203ffff")
	var w bytes.Buffer
	em, err := EncOptions{Sort: SortCanonical}.EncMode()
	if err != nil {
		t.Errorf("EncMode() returned an error %v", err)
	}
	encoder := em.NewEncoder(&w)
	if err := encoder.StartIndefiniteMap(); err != nil {
		t.Fatalf("StartIndefiniteMap() returned error %v", err)
	}
	if err := encoder.Encode("a"); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.Encode(1); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.Encode("b"); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.StartIndefiniteArray(); err != nil {
		t.Fatalf("StartIndefiniteArray() returned error %v", err)
	}
	if err := encoder.Encode(2); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.Encode(3); err != nil {
		t.Fatalf("Encode() returned error %v", err)
	}
	if err := encoder.EndIndefinite(); err != nil {
		t.Fatalf("EndIndefinite() returned error %v", err)
	}
	if err := encoder.EndIndefinite(); err != nil {
		t.Fatalf("EndIndefinite() returned error %v", err)
	}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("Encoding mismatch: got %v, want %v", w.Bytes(), want)
	}
}

func TestIndefiniteLengthError(t *testing.T) {
	var w bytes.Buffer
	encoder := NewEncoder(&w)
	if err := encoder.StartIndefiniteByteString(); err != nil {
		t.Fatalf("StartIndefiniteByteString() returned error %v", err)
	}
	if err := encoder.EndIndefinite(); err != nil {
		t.Fatalf("EndIndefinite() returned error %v", err)
	}
	if err := encoder.EndIndefinite(); err == nil {
		t.Fatalf("EndIndefinite() didn't return an error")
	}
}

func TestEncoderStructTag(t *testing.T) {
	type strc struct {
		A string `json:"x" cbor:"a"`
		B string `json:"y" cbor:"b"`
		C string `json:"z"`
	}
	v := strc{
		A: "A",
		B: "B",
		C: "C",
	}
	want := mustHexDecode("a36161614161626142617a6143") // {"a":"A", "b":"B", "z":"C"}

	var w bytes.Buffer
	encoder := NewEncoder(&w)
	if err := encoder.Encode(v); err != nil {
		t.Errorf("Encode(%+v) returned error %v", v, err)
	}
	if !bytes.Equal(w.Bytes(), want) {
		t.Errorf("Encoding mismatch: got %v, want %v", w.Bytes(), want)
	}
}

func TestRawMessage(t *testing.T) {
	type strc struct {
		A RawMessage  `cbor:"a"`
		B *RawMessage `cbor:"b"`
		C *RawMessage `cbor:"c"`
	}
	data := mustHexDecode("a361610161628202036163f6") // {"a": 1, "b": [2, 3], "c": nil},
	r := RawMessage(mustHexDecode("820203"))
	want := strc{
		A: RawMessage([]byte{0x01}),
		B: &r,
	}
	var v strc
	if err := Unmarshal(data, &v); err != nil {
		t.Fatalf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal(0x%x) returned v %v, want %v", data, v, want)
	}
	b, err := Marshal(v)
	if err != nil {
		t.Fatalf("Marshal(%+v) returned error %v", v, err)
	}
	if !bytes.Equal(b, data) {
		t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", v, b, data)
	}

	address := fmt.Sprintf("%p", *v.B)
	if err := Unmarshal(v.A, v.B); err != nil {
		t.Fatalf("Unmarshal(0x%x) returned error %v", v.A, err)
	}
	if address != fmt.Sprintf("%p", *v.B) {
		t.Fatalf("Unmarshal RawMessage should reuse underlying array if it has sufficient capacity")
	}
	if err := Unmarshal(data, v.B); err != nil {
		t.Fatalf("Unmarshal(0x%x) returned error %v", data, err)
	}
	if address == fmt.Sprintf("%p", *v.B) {
		t.Fatalf("Unmarshal RawMessage should allocate a new underlying array if it does not have sufficient capacity")
	}
}

func TestNullRawMessage(t *testing.T) {
	r := RawMessage(nil)
	wantCborData := []byte{0xf6}
	b, err := Marshal(r)
	if err != nil {
		t.Errorf("Marshal(%+v) returned error %v", r, err)
	}
	if !bytes.Equal(b, wantCborData) {
		t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", r, b, wantCborData)
	}
}

func TestEmptyRawMessage(t *testing.T) {
	var r RawMessage
	wantCborData := []byte{0xf6}
	b, err := Marshal(r)
	if err != nil {
		t.Errorf("Marshal(%+v) returned error %v", r, err)
	}
	if !bytes.Equal(b, wantCborData) {
		t.Errorf("Marshal(%+v) = 0x%x, want 0x%x", r, b, wantCborData)
	}
}

func TestNilRawMessageUnmarshalCBORError(t *testing.T) {
	wantErrorMsg := "cbor.RawMessage: UnmarshalCBOR on nil pointer"
	var r *RawMessage
	data := mustHexDecode("01")
	if err := r.UnmarshalCBOR(data); err == nil {
		t.Errorf("UnmarshalCBOR() didn't return error")
	} else if err.Error() != wantErrorMsg {
		t.Errorf("UnmarshalCBOR() returned error %q, want %q", err.Error(), wantErrorMsg)
	}
}

// nBytesReader reads at most maxBytesPerRead into b.  It also returns error at the last read.
type nBytesReader struct {
	data            []byte
	maxBytesPerRead int
	off             int
	err             error
}

func newNBytesReader(data []byte, maxBytesPerRead int) *nBytesReader {
	return &nBytesReader{
		data:            append([]byte{}, data...),
		maxBytesPerRead: maxBytesPerRead,
		err:             io.EOF,
	}
}

func newNBytesReaderWithError(data []byte, maxBytesPerRead int, err error) *nBytesReader {
	return &nBytesReader{
		data:            append([]byte{}, data...),
		maxBytesPerRead: maxBytesPerRead,
		err:             err,
	}
}

func (r *nBytesReader) Read(b []byte) (int, error) {
	var n int
	if r.off < len(r.data) {
		numOfBytesToRead := len(r.data) - r.off
		if numOfBytesToRead > r.maxBytesPerRead {
			numOfBytesToRead = r.maxBytesPerRead
		}
		n = copy(b, r.data[r.off:r.off+numOfBytesToRead])
		r.off += n
	}
	if r.off == len(r.data) {
		return n, r.err
	}
	return n, nil
}

// toggledReader returns (0, nil) for every other read to mimic non-blocking read for stream reader.
type toggledReader struct {
	nBytesReader
	toggle bool
}

func newToggledReader(data []byte, maxBytesPerRead int) *toggledReader {
	return &toggledReader{
		nBytesReader: nBytesReader{
			data:            append([]byte{}, data...),
			maxBytesPerRead: maxBytesPerRead,
			err:             io.EOF,
		},
		toggle: true, // first read returns (0, nil)
	}
}

func newToggledReaderWithError(data []byte, maxBytesPerRead int, err error) *toggledReader {
	return &toggledReader{
		nBytesReader: nBytesReader{
			data:            append([]byte{}, data...),
			maxBytesPerRead: maxBytesPerRead,
			err:             err,
		},
		toggle: true, // first read returns (0, nil)
	}
}

func (r *toggledReader) Read(b []byte) (int, error) {
	defer func() {
		r.toggle = !r.toggle
	}()
	if r.toggle {
		return 0, nil
	}
	return r.nBytesReader.Read(b)
}

// recoverableReader returns a recoverable error at first read operation.
type recoverableReader struct {
	nBytesReader
	recoverableErr error
	first          bool
}

func newRecoverableReader(data []byte, maxBytesPerRead int, err error) *recoverableReader {
	return &recoverableReader{
		nBytesReader: nBytesReader{
			data:            append([]byte{}, data...),
			maxBytesPerRead: maxBytesPerRead,
			err:             io.EOF,
		},
		recoverableErr: err,
		first:          true,
	}
}

func (r *recoverableReader) Read(b []byte) (int, error) {
	if r.first {
		r.first = false
		return 0, r.recoverableErr
	}
	return r.nBytesReader.Read(b)
}

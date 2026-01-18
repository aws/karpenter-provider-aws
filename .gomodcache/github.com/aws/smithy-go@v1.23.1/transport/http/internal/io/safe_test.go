package io

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"
)

type mockReadCloser struct {
	ReadFn  func([]byte) (int, error)
	CloseFn func() error
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.ReadFn == nil {
		return len(p), nil
	}
	return m.ReadFn(p)
}

func (m *mockReadCloser) Close() error {
	if m.CloseFn == nil {
		return nil
	}
	return m.CloseFn()
}

type mockWriteTo struct {
	mockReadCloser
	WriteToFn func(io.Writer) (int64, error)
}

func (m *mockWriteTo) WriteTo(w io.Writer) (int64, error) {
	if m.WriteToFn == nil {
		return 0, nil
	}
	return m.WriteToFn(w)
}

func TestNewSafeReadCloser(t *testing.T) {
	cases := map[string]struct {
		ReadCloser io.ReadCloser
		ReadTest   func(*testing.T, *safeReadCloser)
		CloseTest  func(*testing.T, *safeReadCloser)
	}{
		"success read and close": {
			ReadCloser: &mockReadCloser{
				ReadFn: func(bytes []byte) (int, error) {
					bytes[0], bytes[1], bytes[2] = 'f', 'o', 'o'
					return 3, nil
				},
			},
			ReadTest: func(t *testing.T, closer *safeReadCloser) {
				t.Helper()
				bs := make([]byte, 3)
				read, err := closer.Read(bs)
				if err != nil {
					t.Errorf("expect no error, got %v", err)
				}
				if e, a := "foo", string(bs[:read]); e != a {
					t.Errorf("expect %v, got %v", e, a)
				}
			},
			CloseTest: func(t *testing.T, closer *safeReadCloser) {
				t.Helper()
				if err := closer.Close(); err != nil {
					t.Errorf("expect no error, got %v", err)
				}
			},
		},
		"error read": {
			ReadCloser: &mockReadCloser{
				ReadFn: func(bytes []byte) (int, error) {
					return 0, io.ErrUnexpectedEOF
				},
			},
			ReadTest: func(t *testing.T, closer *safeReadCloser) {
				t.Helper()
				_, err := closer.Read([]byte{})
				if err == nil {
					t.Errorf("expect error, got nil")
				}
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Errorf("expect error to be type %T, got %T", io.ErrUnexpectedEOF, err)
				}
			},
		},
		"error close": {
			ReadCloser: &mockReadCloser{
				CloseFn: func() error {
					return fmt.Errorf("foobar error")
				},
			},
			CloseTest: func(t *testing.T, closer *safeReadCloser) {
				t.Helper()
				if err := closer.Close(); err == nil {
					t.Error("expect close error, got nil")
				} else if e, a := "foobar error", err.Error(); e != a {
					t.Errorf("expect %v, got %v", e, a)
				}
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			sr := NewSafeReadCloser(tt.ReadCloser).(*safeReadCloser)
			if e, a := false, sr.closed; e != a {
				t.Errorf("expect %v, got %v", e, a)
			}
			if tt.ReadTest != nil {
				tt.ReadTest(t, sr)
			}
			if tt.CloseTest != nil {
				tt.CloseTest(t, sr)
			} else {
				sr.Close()
			}
			if e, a := true, sr.closed; e != a {
				t.Errorf("expect %v, got %v", e, a)
			}
			if sr.readCloser != nil {
				t.Errorf("expect reader to be nil after Close")
			}
			if err := sr.Close(); err != nil {
				t.Errorf("expect subsequent Close returns to be nil, got %v", err)
			}
		})
	}
}

func TestNewSafeReadCloser_WriteTo(t *testing.T) {
	{
		rc := &mockWriteTo{}
		writeToReadCloser := NewSafeReadCloser(rc).(*safeWriteToReadCloser)
		_, err := writeToReadCloser.WriteTo(nil)
		if err != nil {
			t.Errorf("expect no error, got %v", err)
		}

		err = writeToReadCloser.Close()
		if err != nil {
			t.Errorf("expect no error, got %v", err)
		}

		_, err = writeToReadCloser.WriteTo(nil)
		if err != io.EOF {
			t.Errorf("expect %T, got %T", io.EOF, err)
		}
	}
	{
		rc := &mockWriteTo{
			WriteToFn: func(writer io.Writer) (int64, error) {
				write, err := writer.Write([]byte("foo"))
				return int64(write), err
			},
		}
		writeToReadCloser := NewSafeReadCloser(rc).(*safeWriteToReadCloser)
		var buf bytes.Buffer
		_, err := writeToReadCloser.WriteTo(&buf)
		if err != nil {
			t.Errorf("expect no error, got %v", err)
		}
		if e, a := "foo", buf.String(); e != a {
			t.Errorf("expect %v, got %v", e, a)
		}
	}
}

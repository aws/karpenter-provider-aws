// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSemanticError(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{{
		err:  &SemanticError{},
		want: "json: cannot handle",
	}, {
		err:  &SemanticError{JSONKind: 'n'},
		want: "json: cannot handle JSON null",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: 't'},
		want: "json: cannot unmarshal JSON boolean",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: 'x'},
		want: "json: cannot unmarshal", // invalid token kinds are ignored
	}, {
		err:  &SemanticError{action: "marshal", JSONKind: '"'},
		want: "json: cannot marshal JSON string",
	}, {
		err:  &SemanticError{GoType: reflect.TypeOf(bool(false))},
		want: "json: cannot handle Go value of type bool",
	}, {
		err:  &SemanticError{action: "marshal", GoType: reflect.TypeOf(int(0))},
		want: "json: cannot marshal Go value of type int",
	}, {
		err:  &SemanticError{action: "unmarshal", GoType: reflect.TypeOf(uint(0))},
		want: "json: cannot unmarshal Go value of type uint",
	}, {
		err:  &SemanticError{JSONKind: '0', GoType: reflect.TypeOf(tar.Header{})},
		want: "json: cannot handle JSON number with Go value of type tar.Header",
	}, {
		err:  &SemanticError{action: "marshal", JSONKind: '{', GoType: reflect.TypeOf(bytes.Buffer{})},
		want: "json: cannot marshal JSON object from Go value of type bytes.Buffer",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: ']', GoType: reflect.TypeOf(strings.Reader{})},
		want: "json: cannot unmarshal JSON array into Go value of type strings.Reader",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: '{', GoType: reflect.TypeOf(float64(0)), ByteOffset: 123},
		want: "json: cannot unmarshal JSON object into Go value of type float64 after byte offset 123",
	}, {
		err:  &SemanticError{action: "marshal", JSONKind: 'f', GoType: reflect.TypeOf(complex128(0)), ByteOffset: 123, JSONPointer: "/foo/2/bar/3"},
		want: "json: cannot marshal JSON boolean from Go value of type complex128 within JSON value at \"/foo/2/bar/3\"",
	}, {
		err:  &SemanticError{action: "unmarshal", JSONKind: '}', GoType: reflect.TypeOf((*io.Reader)(nil)).Elem(), ByteOffset: 123, JSONPointer: "/foo/2/bar/3", Err: errors.New("some underlying error")},
		want: "json: cannot unmarshal JSON object into Go value of type io.Reader within JSON value at \"/foo/2/bar/3\": some underlying error",
	}, {
		err:  &SemanticError{Err: errors.New("some underlying error")},
		want: "json: cannot handle: some underlying error",
	}, {
		err:  &SemanticError{ByteOffset: 123},
		want: "json: cannot handle after byte offset 123",
	}, {
		err:  &SemanticError{JSONPointer: "/foo/2/bar/3"},
		want: "json: cannot handle within JSON value at \"/foo/2/bar/3\"",
	}}

	for _, tt := range tests {
		got := tt.err.Error()
		// Cleanup the error of non-deterministic rendering effects.
		if strings.HasPrefix(got, errorPrefix+"unable to ") {
			got = errorPrefix + "cannot " + strings.TrimPrefix(got, errorPrefix+"unable to ")
		}
		if got != tt.want {
			t.Errorf("%#v.Error mismatch:\ngot  %v\nwant %v", tt.err, got, tt.want)
		}
	}
}

func TestErrorsIs(t *testing.T) {
	const (
		someGlobalError  = jsonError("some global error")
		otherGlobalError = jsonError("other global error")
	)

	var (
		someIOError         = &ioError{action: "write", err: io.ErrShortWrite}
		otherIOError        = &ioError{action: "read", err: io.ErrUnexpectedEOF}
		someSyntacticError  = &SyntacticError{str: "some syntactic error"}
		otherSyntacticError = &SyntacticError{str: "other syntactic error"}
		someSemanticError   = &SemanticError{action: "unmarshal", JSONKind: '0', GoType: reflect.TypeOf(int(0)), Err: strconv.ErrRange}
		otherSemanticError  = &SemanticError{action: "marshal", GoType: reflect.TypeOf(complex128(0))}
	)

	tests := []struct {
		err    error
		target error
		want   bool
	}{
		// Top-level Error should match itself (identity).
		{Error, Error, true},

		// All sub-error values should match the top-level Error value.
		{someGlobalError, Error, true},
		{someIOError, Error, true},
		{someSyntacticError, Error, true},
		{someSemanticError, Error, true},

		// Top-level Error should not match any other sub-error value.
		{Error, someGlobalError, false},
		{Error, someIOError, false},
		{Error, someSyntacticError, false},
		{Error, someSemanticError, false},

		// Sub-error values should match itself (identity).
		{someGlobalError, someGlobalError, true},
		{someIOError, someIOError, true},
		{someSyntacticError, someSyntacticError, true},
		{someSemanticError, someSemanticError, true},

		// Sub-error values should not match each other.
		{someGlobalError, someIOError, false},
		{someIOError, someSyntacticError, false},
		{someSyntacticError, someSemanticError, false},
		{someSemanticError, someGlobalError, false},

		// Sub-error values should not match other error values of same type.
		{someGlobalError, otherGlobalError, false},
		{someIOError, otherIOError, false},
		{someSyntacticError, otherSyntacticError, false},
		{someSemanticError, otherSemanticError, false},

		// Error should not match any other random error.
		{Error, nil, false},
		{nil, Error, false},
		{io.ErrShortWrite, Error, false},
		{Error, io.ErrShortWrite, false},

		// Wrapped errors should be matched.
		{&ioError{err: fmt.Errorf("%w", io.ErrShortWrite)}, io.ErrShortWrite, true}, // doubly wrapped
		{&ioError{err: io.ErrShortWrite}, io.ErrShortWrite, true},                   // singly wrapped
		{&ioError{err: io.ErrShortWrite}, io.EOF, false},
		{&SemanticError{Err: fmt.Errorf("%w", strconv.ErrRange)}, strconv.ErrRange, true}, // doubly wrapped
		{&SemanticError{Err: strconv.ErrRange}, strconv.ErrRange, true},                   // singly wrapped
		{&SemanticError{Err: strconv.ErrRange}, io.EOF, false},
	}

	for _, tt := range tests {
		got := errors.Is(tt.err, tt.target)
		if got != tt.want {
			t.Errorf("errors.Is(%#v, %#v) = %v, want %v", tt.err, tt.target, got, tt.want)
		}
		// If the type supports the Is method,
		// it should behave the same way if called directly.
		if iserr, ok := tt.err.(interface{ Is(error) bool }); ok {
			got := iserr.Is(tt.target)
			if got != tt.want {
				t.Errorf("%#v.Is(%#v) = %v, want %v", tt.err, tt.target, got, tt.want)
			}
		}
	}
}

func TestQuoteRune(t *testing.T) {
	tests := []struct{ in, want string }{
		{"x", `'x'`},
		{"\n", `'\n'`},
		{"'", `'\''`},
		{"\xff", `'\xff'`},
		{"💩", `'💩'`},
		{"💩"[:1], `'\xf0'`},
		{"\uffff", `'\uffff'`},
		{"\U00101234", `'\U00101234'`},
	}
	for _, tt := range tests {
		got := quoteRune([]byte(tt.in))
		if got != tt.want {
			t.Errorf("quoteRune(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

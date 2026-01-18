// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"io"
	"math"
	"net/http"
	"path"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
)

// TestEncoder tests whether we can produce JSON with either tokens or raw values.
func TestEncoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, formatName := range []string{"Compact", "Escaped", "Indented"} {
			for _, typeName := range []string{"Token", "Value", "TokenDelims"} {
				t.Run(path.Join(td.name.name, typeName, formatName), func(t *testing.T) {
					testEncoder(t, td.name.where, formatName, typeName, td)
				})
			}
		}
	}
}
func testEncoder(t *testing.T, where pc, formatName, typeName string, td coderTestdataEntry) {
	var want string
	dst := new(bytes.Buffer)
	enc := NewEncoder(dst)
	enc.options.omitTopLevelNewline = true
	want = td.outCompacted
	switch formatName {
	case "Escaped":
		enc.options.EscapeRune = func(rune) bool { return true }
		if td.outEscaped != "" {
			want = td.outEscaped
		}
	case "Indented":
		enc.options.multiline = true
		enc.options.IndentPrefix = "\t"
		enc.options.Indent = "    "
		if td.outIndented != "" {
			want = td.outIndented
		}
	}

	switch typeName {
	case "Token":
		var pointers []string
		for _, tok := range td.tokens {
			if err := enc.WriteToken(tok); err != nil {
				t.Fatalf("%s: Encoder.WriteToken error: %v", where, err)
			}
			if td.pointers != nil {
				pointers = append(pointers, enc.StackPointer())
			}
		}
		if !reflect.DeepEqual(pointers, td.pointers) {
			t.Fatalf("%s: pointers mismatch:\ngot  %q\nwant %q", where, pointers, td.pointers)
		}
	case "Value":
		if err := enc.WriteValue(RawValue(td.in)); err != nil {
			t.Fatalf("%s: Encoder.WriteValue error: %v", where, err)
		}
	case "TokenDelims":
		// Use WriteToken for object/array delimiters, WriteValue otherwise.
		for _, tok := range td.tokens {
			switch tok.Kind() {
			case '{', '}', '[', ']':
				if err := enc.WriteToken(tok); err != nil {
					t.Fatalf("%s: Encoder.WriteToken error: %v", where, err)
				}
			default:
				val := RawValue(tok.String())
				if tok.Kind() == '"' {
					val, _ = appendString(nil, tok.String(), false, nil)
				}
				if err := enc.WriteValue(val); err != nil {
					t.Fatalf("%s: Encoder.WriteValue error: %v", where, err)
				}
			}
		}
	}

	got := dst.String()
	if got != want {
		t.Errorf("%s: output mismatch:\ngot  %q\nwant %q", where, got, want)
	}
}

// TestFaultyEncoder tests that temporary I/O errors are not fatal.
func TestFaultyEncoder(t *testing.T) {
	for _, td := range coderTestdata {
		for _, typeName := range []string{"Token", "Value"} {
			t.Run(path.Join(td.name.name, typeName), func(t *testing.T) {
				testFaultyEncoder(t, td.name.where, typeName, td)
			})
		}
	}
}
func testFaultyEncoder(t *testing.T, where pc, typeName string, td coderTestdataEntry) {
	b := &FaultyBuffer{
		MaxBytes: 1,
		MayError: io.ErrShortWrite,
	}

	// Write all the tokens.
	// Even if the underlying io.Writer may be faulty,
	// writing a valid token or value is guaranteed to at least
	// be appended to the internal buffer.
	// In other words, syntactic errors occur before I/O errors.
	enc := NewEncoder(b)
	switch typeName {
	case "Token":
		for i, tok := range td.tokens {
			err := enc.WriteToken(tok)
			if err != nil && !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("%s: %d: Encoder.WriteToken error: %v", where, i, err)
			}
		}
	case "Value":
		err := enc.WriteValue(RawValue(td.in))
		if err != nil && !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("%s: Encoder.WriteValue error: %v", where, err)
		}
	}
	gotOutput := string(append(b.B, enc.unflushedBuffer()...))
	wantOutput := td.outCompacted + "\n"
	if gotOutput != wantOutput {
		t.Fatalf("%s: output mismatch:\ngot  %s\nwant %s", where, gotOutput, wantOutput)
	}
}

type encoderMethodCall struct {
	in          tokOrVal
	wantErr     error
	wantPointer string
}

var encoderErrorTestdata = []struct {
	name    testName
	opts    EncodeOptions
	calls   []encoderMethodCall
	wantOut string
}{{
	name: name("InvalidToken"),
	calls: []encoderMethodCall{
		{zeroToken, &SyntacticError{str: "invalid json.Token"}, ""},
	},
}, {
	name: name("InvalidValue"),
	calls: []encoderMethodCall{
		{RawValue(`#`), newInvalidCharacterError([]byte("#"), "at start of value"), ""},
	},
}, {
	name: name("InvalidValue/DoubleZero"),
	calls: []encoderMethodCall{
		{RawValue(`00`), newInvalidCharacterError([]byte("0"), "after top-level value"), ""},
	},
}, {
	name: name("TruncatedValue"),
	calls: []encoderMethodCall{
		{zeroValue, io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("TruncatedNull"),
	calls: []encoderMethodCall{
		{RawValue(`nul`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidNull"),
	calls: []encoderMethodCall{
		{RawValue(`nulL`), newInvalidCharacterError([]byte("L"), "within literal null (expecting 'l')"), ""},
	},
}, {
	name: name("TruncatedFalse"),
	calls: []encoderMethodCall{
		{RawValue(`fals`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidFalse"),
	calls: []encoderMethodCall{
		{RawValue(`falsE`), newInvalidCharacterError([]byte("E"), "within literal false (expecting 'e')"), ""},
	},
}, {
	name: name("TruncatedTrue"),
	calls: []encoderMethodCall{
		{RawValue(`tru`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidTrue"),
	calls: []encoderMethodCall{
		{RawValue(`truE`), newInvalidCharacterError([]byte("E"), "within literal true (expecting 'e')"), ""},
	},
}, {
	name: name("TruncatedString"),
	calls: []encoderMethodCall{
		{RawValue(`"star`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidString"),
	calls: []encoderMethodCall{
		{RawValue(`"ok` + "\x00"), newInvalidCharacterError([]byte("\x00"), `within string (expecting non-control character)`), ""},
	},
}, {
	name: name("ValidString/AllowInvalidUTF8/Token"),
	opts: EncodeOptions{AllowInvalidUTF8: true},
	calls: []encoderMethodCall{
		{String("living\xde\xad\xbe\xef"), nil, ""},
	},
	wantOut: "\"living\xde\xad\ufffd\ufffd\"\n",
}, {
	name: name("ValidString/AllowInvalidUTF8/Value"),
	opts: EncodeOptions{AllowInvalidUTF8: true},
	calls: []encoderMethodCall{
		{RawValue("\"living\xde\xad\xbe\xef\""), nil, ""},
	},
	wantOut: "\"living\xde\xad\ufffd\ufffd\"\n",
}, {
	name: name("InvalidString/RejectInvalidUTF8"),
	opts: EncodeOptions{AllowInvalidUTF8: false},
	calls: []encoderMethodCall{
		{String("living\xde\xad\xbe\xef"), &SyntacticError{str: "invalid UTF-8 within string"}, ""},
		{RawValue("\"living\xde\xad\xbe\xef\""), &SyntacticError{str: "invalid UTF-8 within string"}, ""},
	},
}, {
	name: name("TruncatedNumber"),
	calls: []encoderMethodCall{
		{RawValue(`0.`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidNumber"),
	calls: []encoderMethodCall{
		{RawValue(`0.e`), newInvalidCharacterError([]byte("e"), "within number (expecting digit)"), ""},
	},
}, {
	name: name("TruncatedObject/AfterStart"),
	calls: []encoderMethodCall{
		{RawValue(`{`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("TruncatedObject/AfterName"),
	calls: []encoderMethodCall{
		{RawValue(`{"0"`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("TruncatedObject/AfterColon"),
	calls: []encoderMethodCall{
		{RawValue(`{"0":`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("TruncatedObject/AfterValue"),
	calls: []encoderMethodCall{
		{RawValue(`{"0":0`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("TruncatedObject/AfterComma"),
	calls: []encoderMethodCall{
		{RawValue(`{"0":0,`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("InvalidObject/MissingColon"),
	calls: []encoderMethodCall{
		{RawValue(` { "fizz" "buzz" } `), newInvalidCharacterError([]byte("\""), "after object name (expecting ':')"), ""},
		{RawValue(` { "fizz" , "buzz" } `), newInvalidCharacterError([]byte(","), "after object name (expecting ':')"), ""},
	},
}, {
	name: name("InvalidObject/MissingComma"),
	calls: []encoderMethodCall{
		{RawValue(` { "fizz" : "buzz" "gazz" } `), newInvalidCharacterError([]byte("\""), "after object value (expecting ',' or '}')"), ""},
		{RawValue(` { "fizz" : "buzz" : "gazz" } `), newInvalidCharacterError([]byte(":"), "after object value (expecting ',' or '}')"), ""},
	},
}, {
	name: name("InvalidObject/ExtraComma"),
	calls: []encoderMethodCall{
		{RawValue(` { , } `), newInvalidCharacterError([]byte(","), `at start of string (expecting '"')`), ""},
		{RawValue(` { "fizz" : "buzz" , } `), newInvalidCharacterError([]byte("}"), `at start of string (expecting '"')`), ""},
	},
}, {
	name: name("InvalidObject/InvalidName"),
	calls: []encoderMethodCall{
		{RawValue(`{ null }`), newInvalidCharacterError([]byte("n"), `at start of string (expecting '"')`), ""},
		{RawValue(`{ false }`), newInvalidCharacterError([]byte("f"), `at start of string (expecting '"')`), ""},
		{RawValue(`{ true }`), newInvalidCharacterError([]byte("t"), `at start of string (expecting '"')`), ""},
		{RawValue(`{ 0 }`), newInvalidCharacterError([]byte("0"), `at start of string (expecting '"')`), ""},
		{RawValue(`{ {} }`), newInvalidCharacterError([]byte("{"), `at start of string (expecting '"')`), ""},
		{RawValue(`{ [] }`), newInvalidCharacterError([]byte("["), `at start of string (expecting '"')`), ""},
		{ObjectStart, nil, ""},
		{Null, errMissingName, ""},
		{RawValue(`null`), errMissingName, ""},
		{False, errMissingName, ""},
		{RawValue(`false`), errMissingName, ""},
		{True, errMissingName, ""},
		{RawValue(`true`), errMissingName, ""},
		{Uint(0), errMissingName, ""},
		{RawValue(`0`), errMissingName, ""},
		{ObjectStart, errMissingName, ""},
		{RawValue(`{}`), errMissingName, ""},
		{ArrayStart, errMissingName, ""},
		{RawValue(`[]`), errMissingName, ""},
		{ObjectEnd, nil, ""},
	},
	wantOut: "{}\n",
}, {
	name: name("InvalidObject/InvalidValue"),
	calls: []encoderMethodCall{
		{RawValue(`{ "0": x }`), newInvalidCharacterError([]byte("x"), `at start of value`), ""},
	},
}, {
	name: name("InvalidObject/MismatchingDelim"),
	calls: []encoderMethodCall{
		{RawValue(` { ] `), newInvalidCharacterError([]byte("]"), `at start of string (expecting '"')`), ""},
		{RawValue(` { "0":0 ] `), newInvalidCharacterError([]byte("]"), `after object value (expecting ',' or '}')`), ""},
		{ObjectStart, nil, ""},
		{ArrayEnd, errMismatchDelim, ""},
		{RawValue(`]`), newInvalidCharacterError([]byte("]"), "at start of value"), ""},
		{ObjectEnd, nil, ""},
	},
	wantOut: "{}\n",
}, {
	name: name("ValidObject/UniqueNames"),
	calls: []encoderMethodCall{
		{ObjectStart, nil, ""},
		{String("0"), nil, ""},
		{Uint(0), nil, ""},
		{String("1"), nil, ""},
		{Uint(1), nil, ""},
		{ObjectEnd, nil, ""},
		{RawValue(` { "0" : 0 , "1" : 1 } `), nil, ""},
	},
	wantOut: `{"0":0,"1":1}` + "\n" + `{"0":0,"1":1}` + "\n",
}, {
	name: name("ValidObject/DuplicateNames"),
	opts: EncodeOptions{AllowDuplicateNames: true},
	calls: []encoderMethodCall{
		{ObjectStart, nil, ""},
		{String("0"), nil, ""},
		{Uint(0), nil, ""},
		{String("0"), nil, ""},
		{Uint(0), nil, ""},
		{ObjectEnd, nil, ""},
		{RawValue(` { "0" : 0 , "0" : 0 } `), nil, ""},
	},
	wantOut: `{"0":0,"0":0}` + "\n" + `{"0":0,"0":0}` + "\n",
}, {
	name: name("InvalidObject/DuplicateNames"),
	calls: []encoderMethodCall{
		{ObjectStart, nil, ""},
		{String("0"), nil, ""},
		{ObjectStart, nil, ""},
		{ObjectEnd, nil, ""},
		{String("0"), &SyntacticError{str: `duplicate name "0" in object`}, "/0"},
		{RawValue(`"0"`), &SyntacticError{str: `duplicate name "0" in object`}, "/0"},
		{String("1"), nil, ""},
		{ObjectStart, nil, ""},
		{ObjectEnd, nil, ""},
		{String("0"), &SyntacticError{str: `duplicate name "0" in object`}, "/1"},
		{RawValue(`"0"`), &SyntacticError{str: `duplicate name "0" in object`}, "/1"},
		{String("1"), &SyntacticError{str: `duplicate name "1" in object`}, "/1"},
		{RawValue(`"1"`), &SyntacticError{str: `duplicate name "1" in object`}, "/1"},
		{ObjectEnd, nil, ""},
		{RawValue(` { "0" : 0 , "1" : 1 , "0" : 0 } `), &SyntacticError{str: `duplicate name "0" in object`}, ""},
	},
	wantOut: `{"0":{},"1":{}}` + "\n",
}, {
	name: name("TruncatedArray/AfterStart"),
	calls: []encoderMethodCall{
		{RawValue(`[`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("TruncatedArray/AfterValue"),
	calls: []encoderMethodCall{
		{RawValue(`[0`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("TruncatedArray/AfterComma"),
	calls: []encoderMethodCall{
		{RawValue(`[0,`), io.ErrUnexpectedEOF, ""},
	},
}, {
	name: name("TruncatedArray/MissingComma"),
	calls: []encoderMethodCall{
		{RawValue(` [ "fizz" "buzz" ] `), newInvalidCharacterError([]byte("\""), "after array value (expecting ',' or ']')"), ""},
	},
}, {
	name: name("InvalidArray/MismatchingDelim"),
	calls: []encoderMethodCall{
		{RawValue(` [ } `), newInvalidCharacterError([]byte("}"), `at start of value`), ""},
		{ArrayStart, nil, ""},
		{ObjectEnd, errMismatchDelim, ""},
		{RawValue(`}`), newInvalidCharacterError([]byte("}"), "at start of value"), ""},
		{ArrayEnd, nil, ""},
	},
	wantOut: "[]\n",
}}

// TestEncoderErrors test that Encoder errors occur when we expect and
// leaves the Encoder in a consistent state.
func TestEncoderErrors(t *testing.T) {
	for _, td := range encoderErrorTestdata {
		t.Run(path.Join(td.name.name), func(t *testing.T) {
			testEncoderErrors(t, td.name.where, td.opts, td.calls, td.wantOut)
		})
	}
}
func testEncoderErrors(t *testing.T, where pc, opts EncodeOptions, calls []encoderMethodCall, wantOut string) {
	dst := new(bytes.Buffer)
	enc := opts.NewEncoder(dst)
	for i, call := range calls {
		var gotErr error
		switch tokVal := call.in.(type) {
		case Token:
			gotErr = enc.WriteToken(tokVal)
		case RawValue:
			gotErr = enc.WriteValue(tokVal)
		}
		if !reflect.DeepEqual(gotErr, call.wantErr) {
			t.Fatalf("%s: %d: error mismatch: got %#v, want %#v", where, i, gotErr, call.wantErr)
		}
		if call.wantPointer != "" {
			gotPointer := enc.StackPointer()
			if gotPointer != call.wantPointer {
				t.Fatalf("%s: %d: Encoder.StackPointer = %s, want %s", where, i, gotPointer, call.wantPointer)
			}
		}
	}
	gotOut := dst.String() + string(enc.unflushedBuffer())
	if gotOut != wantOut {
		t.Fatalf("%s: output mismatch:\ngot  %q\nwant %q", where, gotOut, wantOut)
	}
	gotOffset := int(enc.OutputOffset())
	wantOffset := len(wantOut)
	if gotOffset != wantOffset {
		t.Fatalf("%s: Encoder.OutputOffset = %v, want %v", where, gotOffset, wantOffset)
	}
}

func TestAppendString(t *testing.T) {
	var (
		escapeNothing    = func(r rune) bool { return false }
		escapeHTML       = func(r rune) bool { return r == '<' || r == '>' || r == '&' || r == '\u2028' || r == '\u2029' }
		escapeNonASCII   = func(r rune) bool { return r > unicode.MaxASCII }
		escapeEverything = func(r rune) bool { return true }
	)

	tests := []struct {
		in          string
		escapeRune  func(rune) bool
		want        string
		wantErr     error
		wantErrUTF8 error
	}{
		{"", nil, `""`, nil, nil},
		{"hello", nil, `"hello"`, nil, nil},
		{"\x00", nil, `"\u0000"`, nil, nil},
		{"\x1f", nil, `"\u001f"`, nil, nil},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", nil, `"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"`, nil, nil},
		{" !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f", nil, "\" !#$%&'()*+,-./0123456789:;<=>?@[]^_`{|}~\x7f\"", nil, nil},
		{"x\x80\ufffd", nil, "\"x\ufffd\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xff\ufffd", nil, "\"x\ufffd\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\x80\ufffd", escapeNonASCII, "\"x\\ufffd\\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xff\ufffd", escapeNonASCII, "\"x\\ufffd\\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xc0", nil, "\"x\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xc0\x80", nil, "\"x\ufffd\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xe0", nil, "\"x\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xe0\x80", nil, "\"x\ufffd\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xe0\x80\x80", nil, "\"x\ufffd\ufffd\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xf0", nil, "\"x\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xf0\x80", nil, "\"x\ufffd\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xf0\x80\x80", nil, "\"x\ufffd\ufffd\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xf0\x80\x80\x80", nil, "\"x\ufffd\ufffd\ufffd\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"x\xed\xba\xad", nil, "\"x\ufffd\ufffd\ufffd\"", nil, &SyntacticError{str: "invalid UTF-8 within string"}},
		{"\"\\/\b\f\n\r\t", nil, `"\"\\/\b\f\n\r\t"`, nil, nil},
		{"\"\\/\b\f\n\r\t", escapeEverything, `"\u0022\u005c\u002f\u0008\u000c\u000a\u000d\u0009"`, nil, nil},
		{"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃).", nil, `"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃)."`, nil, nil},
		{"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃).", escapeNonASCII, `"\u0669(-\u032e\u032e\u0303-\u0303)\u06f6 \u0669(\u25cf\u032e\u032e\u0303\u2022\u0303)\u06f6 \u0669(\u0361\u0e4f\u032f\u0361\u0e4f)\u06f6 \u0669(-\u032e\u032e\u0303\u2022\u0303)."`, nil, nil},
		{"٩(-̮̮̃-̃)۶ ٩(●̮̮̃•̃)۶ ٩(͡๏̯͡๏)۶ ٩(-̮̮̃•̃).", escapeEverything, `"\u0669\u0028\u002d\u032e\u032e\u0303\u002d\u0303\u0029\u06f6\u0020\u0669\u0028\u25cf\u032e\u032e\u0303\u2022\u0303\u0029\u06f6\u0020\u0669\u0028\u0361\u0e4f\u032f\u0361\u0e4f\u0029\u06f6\u0020\u0669\u0028\u002d\u032e\u032e\u0303\u2022\u0303\u0029\u002e"`, nil, nil},
		{"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602", nil, "\"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602\"", nil, nil},
		{"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\U0001f602", escapeEverything, `"\u0080\u00f6\u20ac\ud799\ue000\ufb33\ufffd\ud83d\ude02"`, nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", nil, "\"\\u0000\\u001f\u0020\\\"\u0026\u003c\u003e\\\\\u007f\u0080\u2028\u2029\ufffd\U0001f602\"", nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", escapeNothing, "\"\\u0000\\u001f\u0020\\\"\u0026\u003c\u003e\\\\\u007f\u0080\u2028\u2029\ufffd\U0001f602\"", nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", escapeHTML, "\"\\u0000\\u001f\u0020\\\"\\u0026\\u003c\\u003e\\\\\u007f\u0080\\u2028\\u2029\ufffd\U0001f602\"", nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", escapeNonASCII, "\"\\u0000\\u001f\u0020\\\"\u0026\u003c\u003e\\\\\u007f\\u0080\\u2028\\u2029\\ufffd\\ud83d\\ude02\"", nil, nil},
		{"\u0000\u001f\u0020\u0022\u0026\u003c\u003e\u005c\u007f\u0080\u2028\u2029\ufffd\U0001f602", escapeEverything, "\"\\u0000\\u001f\\u0020\\u0022\\u0026\\u003c\\u003e\\u005c\\u007f\\u0080\\u2028\\u2029\\ufffd\\ud83d\\ude02\"", nil, nil},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, gotErr := appendString(nil, tt.in, false, tt.escapeRune)
			if string(got) != tt.want || !reflect.DeepEqual(gotErr, tt.wantErr) {
				t.Errorf("appendString(nil, %q, false, ...) = (%s, %v), want (%s, %v)", tt.in, got, gotErr, tt.want, tt.wantErr)
			}
			switch got, gotErr := appendString(nil, tt.in, true, tt.escapeRune); {
			case tt.wantErrUTF8 == nil && (string(got) != tt.want || !reflect.DeepEqual(gotErr, tt.wantErr)):
				t.Errorf("appendString(nil, %q, true, ...) = (%s, %v), want (%s, %v)", tt.in, got, gotErr, tt.want, tt.wantErr)
			case tt.wantErrUTF8 != nil && (!strings.HasPrefix(tt.want, string(got)) || !reflect.DeepEqual(gotErr, tt.wantErrUTF8)):
				t.Errorf("appendString(nil, %q, true, ...) = (%s, %v), want (%s, %v)", tt.in, got, gotErr, tt.want, tt.wantErrUTF8)
			}
		})
	}
}

func TestAppendNumber(t *testing.T) {
	tests := []struct {
		in     float64
		want32 string
		want64 string
	}{
		{math.E, "2.7182817", "2.718281828459045"},
		{math.Pi, "3.1415927", "3.141592653589793"},
		{math.SmallestNonzeroFloat32, "1e-45", "1.401298464324817e-45"},
		{math.SmallestNonzeroFloat64, "0", "5e-324"},
		{math.MaxFloat32, "3.4028235e+38", "3.4028234663852886e+38"},
		{math.MaxFloat64, "", "1.7976931348623157e+308"},
		{0.1111111111111111, "0.11111111", "0.1111111111111111"},
		{0.2222222222222222, "0.22222222", "0.2222222222222222"},
		{0.3333333333333333, "0.33333334", "0.3333333333333333"},
		{0.4444444444444444, "0.44444445", "0.4444444444444444"},
		{0.5555555555555555, "0.5555556", "0.5555555555555555"},
		{0.6666666666666666, "0.6666667", "0.6666666666666666"},
		{0.7777777777777777, "0.7777778", "0.7777777777777777"},
		{0.8888888888888888, "0.8888889", "0.8888888888888888"},
		{0.9999999999999999, "1", "0.9999999999999999"},

		// The following entries are from RFC 8785, appendix B
		// which are designed to ensure repeatable formatting of 64-bit floats.
		{math.Float64frombits(0x0000000000000000), "0", "0"},
		{math.Float64frombits(0x8000000000000000), "-0", "-0"}, // differs from RFC 8785
		{math.Float64frombits(0x0000000000000001), "0", "5e-324"},
		{math.Float64frombits(0x8000000000000001), "-0", "-5e-324"},
		{math.Float64frombits(0x7fefffffffffffff), "", "1.7976931348623157e+308"},
		{math.Float64frombits(0xffefffffffffffff), "", "-1.7976931348623157e+308"},
		{math.Float64frombits(0x4340000000000000), "9007199000000000", "9007199254740992"},
		{math.Float64frombits(0xc340000000000000), "-9007199000000000", "-9007199254740992"},
		{math.Float64frombits(0x4430000000000000), "295147900000000000000", "295147905179352830000"},
		{math.Float64frombits(0x44b52d02c7e14af5), "1e+23", "9.999999999999997e+22"},
		{math.Float64frombits(0x44b52d02c7e14af6), "1e+23", "1e+23"},
		{math.Float64frombits(0x44b52d02c7e14af7), "1e+23", "1.0000000000000001e+23"},
		{math.Float64frombits(0x444b1ae4d6e2ef4e), "1e+21", "999999999999999700000"},
		{math.Float64frombits(0x444b1ae4d6e2ef4f), "1e+21", "999999999999999900000"},
		{math.Float64frombits(0x444b1ae4d6e2ef50), "1e+21", "1e+21"},
		{math.Float64frombits(0x3eb0c6f7a0b5ed8c), "0.000001", "9.999999999999997e-7"},
		{math.Float64frombits(0x3eb0c6f7a0b5ed8d), "0.000001", "0.000001"},
		{math.Float64frombits(0x41b3de4355555553), "333333340", "333333333.3333332"},
		{math.Float64frombits(0x41b3de4355555554), "333333340", "333333333.33333325"},
		{math.Float64frombits(0x41b3de4355555555), "333333340", "333333333.3333333"},
		{math.Float64frombits(0x41b3de4355555556), "333333340", "333333333.3333334"},
		{math.Float64frombits(0x41b3de4355555557), "333333340", "333333333.33333343"},
		{math.Float64frombits(0xbecbf647612f3696), "-0.0000033333333", "-0.0000033333333333333333"},
		{math.Float64frombits(0x43143ff3c1cb0959), "1424953900000000", "1424953923781206.2"},

		// The following are select entries from RFC 8785, appendix B,
		// but modified for equivalent 32-bit behavior.
		{float64(math.Float32frombits(0x65a96815)), "9.999999e+22", "9.999998877476383e+22"},
		{float64(math.Float32frombits(0x65a96816)), "1e+23", "9.999999778196308e+22"},
		{float64(math.Float32frombits(0x65a96817)), "1.0000001e+23", "1.0000000678916234e+23"},
		{float64(math.Float32frombits(0x6258d725)), "999999900000000000000", "999999879303389000000"},
		{float64(math.Float32frombits(0x6258d726)), "999999950000000000000", "999999949672133200000"},
		{float64(math.Float32frombits(0x6258d727)), "1e+21", "1.0000000200408773e+21"},
		{float64(math.Float32frombits(0x6258d728)), "1.0000001e+21", "1.0000000904096215e+21"},
		{float64(math.Float32frombits(0x358637bc)), "9.999999e-7", "9.99999883788405e-7"},
		{float64(math.Float32frombits(0x358637bd)), "0.000001", "9.999999974752427e-7"},
		{float64(math.Float32frombits(0x358637be)), "0.0000010000001", "0.0000010000001111620804"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got32 := string(appendNumber(nil, tt.in, 32)); got32 != tt.want32 && tt.want32 != "" {
				t.Errorf("appendNumber(nil, %v, 32) = %v, want %v", tt.in, got32, tt.want32)
			}
			if got64 := string(appendNumber(nil, tt.in, 64)); got64 != tt.want64 && tt.want64 != "" {
				t.Errorf("appendNumber(nil, %v, 64) = %v, want %v", tt.in, got64, tt.want64)
			}
		})
	}
}

// The default of 1e4 lines was chosen since it is sufficiently large to include
// test numbers from all three categories (i.e., static, series, and random).
// Yet, it is sufficiently low to execute quickly relative to other tests.
//
// Processing 1e8 lines takes a minute and processes about 4GiB worth of text.
var testCanonicalNumberLines = flag.Float64("canonical-number-lines", 1e4, "specify the number of lines to check from the canonical numbers testdata")

// TestCanonicalNumber verifies that appendNumber complies with RFC 8785
// according to the testdata provided by the reference implementation.
// See https://github.com/cyberphone/json-canonicalization/tree/master/testdata#es6-numbers.
func TestCanonicalNumber(t *testing.T) {
	const testfileURL = "https://github.com/cyberphone/json-canonicalization/releases/download/es6testfile/es6testfile100m.txt.gz"
	hashes := map[float64]string{
		1e3: "be18b62b6f69cdab33a7e0dae0d9cfa869fda80ddc712221570f9f40a5878687",
		1e4: "b9f7a8e75ef22a835685a52ccba7f7d6bdc99e34b010992cbc5864cd12be6892",
		1e5: "22776e6d4b49fa294a0d0f349268e5c28808fe7e0cb2bcbe28f63894e494d4c7",
		1e6: "49415fee2c56c77864931bd3624faad425c3c577d6d74e89a83bc725506dad16",
		1e7: "b9f8a44a91d46813b21b9602e72f112613c91408db0b8341fb94603d9db135e0",
		1e8: "0f7dda6b0837dde083c5d6b896f7d62340c8a2415b0c7121d83145e08a755272",
	}
	wantHash := hashes[*testCanonicalNumberLines]
	if wantHash == "" {
		t.Fatalf("canonical-number-lines must be one of the following values: 1e3, 1e4, 1e5, 1e6, 1e7, 1e8")
	}
	numLines := int(*testCanonicalNumberLines)

	// generator returns a function that generates the next float64 to format.
	// This implements the algorithm specified in the reference implementation.
	generator := func() func() float64 {
		static := [...]uint64{
			0x0000000000000000, 0x8000000000000000, 0x0000000000000001, 0x8000000000000001,
			0xc46696695dbd1cc3, 0xc43211ede4974a35, 0xc3fce97ca0f21056, 0xc3c7213080c1a6ac,
			0xc39280f39a348556, 0xc35d9b1f5d20d557, 0xc327af4c4a80aaac, 0xc2f2f2a36ecd5556,
			0xc2be51057e155558, 0xc28840d131aaaaac, 0xc253670dc1555557, 0xc21f0b4935555557,
			0xc1e8d5d42aaaaaac, 0xc1b3de4355555556, 0xc17fca0555555556, 0xc1496e6aaaaaaaab,
			0xc114585555555555, 0xc0e046aaaaaaaaab, 0xc0aa0aaaaaaaaaaa, 0xc074d55555555555,
			0xc040aaaaaaaaaaab, 0xc00aaaaaaaaaaaab, 0xbfd5555555555555, 0xbfa1111111111111,
			0xbf6b4e81b4e81b4f, 0xbf35d867c3ece2a5, 0xbf0179ec9cbd821e, 0xbecbf647612f3696,
			0xbe965e9f80f29212, 0xbe61e54c672874db, 0xbe2ca213d840baf8, 0xbdf6e80fe033c8c6,
			0xbdc2533fe68fd3d2, 0xbd8d51ffd74c861c, 0xbd5774ccac3d3817, 0xbd22c3d6f030f9ac,
			0xbcee0624b3818f79, 0xbcb804ea293472c7, 0xbc833721ba905bd3, 0xbc4ebe9c5db3c61e,
			0xbc18987d17c304e5, 0xbbe3ad30dfcf371d, 0xbbaf7b816618582f, 0xbb792f9ab81379bf,
			0xbb442615600f9499, 0xbb101e77800c76e1, 0xbad9ca58cce0be35, 0xbaa4a1e0a3e6fe90,
			0xba708180831f320d, 0xba3a68cd9e985016, 0x446696695dbd1cc3, 0x443211ede4974a35,
			0x43fce97ca0f21056, 0x43c7213080c1a6ac, 0x439280f39a348556, 0x435d9b1f5d20d557,
			0x4327af4c4a80aaac, 0x42f2f2a36ecd5556, 0x42be51057e155558, 0x428840d131aaaaac,
			0x4253670dc1555557, 0x421f0b4935555557, 0x41e8d5d42aaaaaac, 0x41b3de4355555556,
			0x417fca0555555556, 0x41496e6aaaaaaaab, 0x4114585555555555, 0x40e046aaaaaaaaab,
			0x40aa0aaaaaaaaaaa, 0x4074d55555555555, 0x4040aaaaaaaaaaab, 0x400aaaaaaaaaaaab,
			0x3fd5555555555555, 0x3fa1111111111111, 0x3f6b4e81b4e81b4f, 0x3f35d867c3ece2a5,
			0x3f0179ec9cbd821e, 0x3ecbf647612f3696, 0x3e965e9f80f29212, 0x3e61e54c672874db,
			0x3e2ca213d840baf8, 0x3df6e80fe033c8c6, 0x3dc2533fe68fd3d2, 0x3d8d51ffd74c861c,
			0x3d5774ccac3d3817, 0x3d22c3d6f030f9ac, 0x3cee0624b3818f79, 0x3cb804ea293472c7,
			0x3c833721ba905bd3, 0x3c4ebe9c5db3c61e, 0x3c18987d17c304e5, 0x3be3ad30dfcf371d,
			0x3baf7b816618582f, 0x3b792f9ab81379bf, 0x3b442615600f9499, 0x3b101e77800c76e1,
			0x3ad9ca58cce0be35, 0x3aa4a1e0a3e6fe90, 0x3a708180831f320d, 0x3a3a68cd9e985016,
			0x4024000000000000, 0x4014000000000000, 0x3fe0000000000000, 0x3fa999999999999a,
			0x3f747ae147ae147b, 0x3f40624dd2f1a9fc, 0x3f0a36e2eb1c432d, 0x3ed4f8b588e368f1,
			0x3ea0c6f7a0b5ed8d, 0x3e6ad7f29abcaf48, 0x3e35798ee2308c3a, 0x3ed539223589fa95,
			0x3ed4ff26cd5a7781, 0x3ed4f95a762283ff, 0x3ed4f8c60703520c, 0x3ed4f8b72f19cd0d,
			0x3ed4f8b5b31c0c8d, 0x3ed4f8b58d1c461a, 0x3ed4f8b5894f7f0e, 0x3ed4f8b588ee37f3,
			0x3ed4f8b588e47da4, 0x3ed4f8b588e3849c, 0x3ed4f8b588e36bb5, 0x3ed4f8b588e36937,
			0x3ed4f8b588e368f8, 0x3ed4f8b588e368f1, 0x3ff0000000000000, 0xbff0000000000000,
			0xbfeffffffffffffa, 0xbfeffffffffffffb, 0x3feffffffffffffa, 0x3feffffffffffffb,
			0x3feffffffffffffc, 0x3feffffffffffffe, 0xbfefffffffffffff, 0xbfefffffffffffff,
			0x3fefffffffffffff, 0x3fefffffffffffff, 0x3fd3333333333332, 0x3fd3333333333333,
			0x3fd3333333333334, 0x0010000000000000, 0x000ffffffffffffd, 0x000fffffffffffff,
			0x7fefffffffffffff, 0xffefffffffffffff, 0x4340000000000000, 0xc340000000000000,
			0x4430000000000000, 0x44b52d02c7e14af5, 0x44b52d02c7e14af6, 0x44b52d02c7e14af7,
			0x444b1ae4d6e2ef4e, 0x444b1ae4d6e2ef4f, 0x444b1ae4d6e2ef50, 0x3eb0c6f7a0b5ed8c,
			0x3eb0c6f7a0b5ed8d, 0x41b3de4355555553, 0x41b3de4355555554, 0x41b3de4355555555,
			0x41b3de4355555556, 0x41b3de4355555557, 0xbecbf647612f3696, 0x43143ff3c1cb0959,
		}
		var state struct {
			idx   int
			data  []byte
			block [sha256.Size]byte
		}
		return func() float64 {
			const numSerial = 2000
			var f float64
			switch {
			case state.idx < len(static):
				f = math.Float64frombits(static[state.idx])
			case state.idx < len(static)+numSerial:
				f = math.Float64frombits(0x0010000000000000 + uint64(state.idx-len(static)))
			default:
				for f == 0 || math.IsNaN(f) || math.IsInf(f, 0) {
					if len(state.data) == 0 {
						state.block = sha256.Sum256(state.block[:])
						state.data = state.block[:]
					}
					f = math.Float64frombits(binary.LittleEndian.Uint64(state.data))
					state.data = state.data[8:]
				}
			}
			state.idx++
			return f
		}
	}

	// Pass through the test twice. In the first pass we only hash the output,
	// while in the second pass we check every line against the golden testdata.
	// If the hashes match in the first pass, then we skip the second pass.
	for _, checkGolden := range []bool{false, true} {
		var br *bufio.Reader // for line-by-line reading of es6testfile100m.txt
		if checkGolden {
			resp, err := http.Get(testfileURL)
			if err != nil {
				t.Fatalf("http.Get error: %v", err)
			}
			defer resp.Body.Close()

			zr, err := gzip.NewReader(resp.Body)
			if err != nil {
				t.Fatalf("gzip.NewReader error: %v", err)
			}

			br = bufio.NewReader(zr)
		}

		// appendNumberJCS differs from appendNumber only for -0.
		appendNumberJCS := func(b []byte, f float64) []byte {
			if math.Signbit(f) && f == 0 {
				return append(b, '0')
			}
			return appendNumber(b, f, 64)
		}

		var gotLine []byte
		next := generator()
		hash := sha256.New()
		start := time.Now()
		lastPrint := start
		for n := 1; n <= numLines; n++ {
			// Generate the formatted line for this number.
			f := next()
			gotLine = gotLine[:0] // reset from previous usage
			gotLine = strconv.AppendUint(gotLine, math.Float64bits(f), 16)
			gotLine = append(gotLine, ',')
			gotLine = appendNumberJCS(gotLine, f)
			gotLine = append(gotLine, '\n')
			hash.Write(gotLine)

			// Check that the formatted line matches.
			if checkGolden {
				wantLine, err := br.ReadBytes('\n')
				if err != nil {
					t.Fatalf("bufio.Reader.ReadBytes error: %v", err)
				}
				if !bytes.Equal(gotLine, wantLine) {
					t.Errorf("mismatch on line %d:\n\tgot  %v\n\twant %v",
						n, strings.TrimSpace(string(gotLine)), strings.TrimSpace(string(wantLine)))
				}
			}

			// Print progress.
			if now := time.Now(); now.Sub(lastPrint) > time.Second || n == numLines {
				remaining := float64(now.Sub(start)) * float64(numLines-n) / float64(n)
				t.Logf("%0.3f%% (%v remaining)",
					100.0*float64(n)/float64(numLines),
					time.Duration(remaining).Round(time.Second))
				lastPrint = now
			}
		}

		gotHash := hex.EncodeToString(hash.Sum(nil))
		if gotHash == wantHash {
			return // hashes match, no need to check golden testdata
		}
	}
}

// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"reflect"
	"testing"
)

func FuzzCoder(f *testing.F) {
	// Add a number of inputs to the corpus including valid and invalid data.
	for _, td := range coderTestdata {
		f.Add(int64(0), []byte(td.in))
	}
	for _, td := range decoderErrorTestdata {
		f.Add(int64(0), []byte(td.in))
	}
	for _, td := range encoderErrorTestdata {
		f.Add(int64(0), []byte(td.wantOut))
	}
	for _, td := range jsonTestdata() {
		f.Add(int64(0), td.data)
	}

	f.Fuzz(func(t *testing.T, seed int64, b []byte) {
		var tokVals []tokOrVal
		rn := rand.NewSource(seed)

		// Read a sequence of tokens or values. Skip the test for any errors
		// since we expect this with randomly generated fuzz inputs.
		src := bytes.NewReader(b)
		dec := NewDecoder(src)
		for {
			if rn.Int63()%8 > 0 {
				tok, err := dec.ReadToken()
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Skipf("Decoder.ReadToken error: %v", err)
				}
				tokVals = append(tokVals, tok.Clone())
			} else {
				val, err := dec.ReadValue()
				if err != nil {
					expectError := dec.PeekKind() == '}' || dec.PeekKind() == ']'
					if expectError && errors.As(err, new(*SyntacticError)) {
						continue
					}
					if err == io.EOF {
						break
					}
					t.Skipf("Decoder.ReadValue error: %v", err)
				}
				tokVals = append(tokVals, append(zeroValue, val...))
			}
		}

		// Write a sequence of tokens or values. Fail the test for any errors
		// since the previous stage guarantees that the input is valid.
		dst := new(bytes.Buffer)
		enc := NewEncoder(dst)
		for _, tokVal := range tokVals {
			switch tokVal := tokVal.(type) {
			case Token:
				if err := enc.WriteToken(tokVal); err != nil {
					t.Fatalf("Encoder.WriteToken error: %v", err)
				}
			case RawValue:
				if err := enc.WriteValue(tokVal); err != nil {
					t.Fatalf("Encoder.WriteValue error: %v", err)
				}
			}
		}

		// Encoded output and original input must decode to the same thing.
		var got, want []Token
		for dec := NewDecoder(bytes.NewReader(b)); dec.PeekKind() > 0; {
			tok, err := dec.ReadToken()
			if err != nil {
				t.Fatalf("Decoder.ReadToken error: %v", err)
			}
			got = append(got, tok.Clone())
		}
		for dec := NewDecoder(dst); dec.PeekKind() > 0; {
			tok, err := dec.ReadToken()
			if err != nil {
				t.Fatalf("Decoder.ReadToken error: %v", err)
			}
			want = append(want, tok.Clone())
		}
		if !equalTokens(got, want) {
			t.Fatalf("mismatching output:\ngot  %v\nwant %v", got, want)
		}
	})
}

func FuzzResumableDecoder(f *testing.F) {
	for _, td := range resumableDecoderTestdata {
		f.Add(int64(0), []byte(td))
	}

	f.Fuzz(func(t *testing.T, seed int64, b []byte) {
		rn := rand.NewSource(seed)

		// Regardless of how many bytes the underlying io.Reader produces,
		// the provided tokens, values, and errors should always be identical.
		t.Run("ReadToken", func(t *testing.T) {
			decGot := NewDecoder(&FaultyBuffer{B: b, MaxBytes: 8, Rand: rn})
			decWant := NewDecoder(bytes.NewReader(b))
			gotTok, gotErr := decGot.ReadToken()
			wantTok, wantErr := decWant.ReadToken()
			if gotTok.String() != wantTok.String() || !reflect.DeepEqual(gotErr, wantErr) {
				t.Errorf("Decoder.ReadToken = (%v, %v), want (%v, %v)", gotTok, gotErr, wantTok, wantErr)
			}
		})
		t.Run("ReadValue", func(t *testing.T) {
			decGot := NewDecoder(&FaultyBuffer{B: b, MaxBytes: 8, Rand: rn})
			decWant := NewDecoder(bytes.NewReader(b))
			gotVal, gotErr := decGot.ReadValue()
			wantVal, wantErr := decWant.ReadValue()
			if !reflect.DeepEqual(gotVal, wantVal) || !reflect.DeepEqual(gotErr, wantErr) {
				t.Errorf("Decoder.ReadValue = (%s, %v), want (%s, %v)", gotVal, gotErr, wantVal, wantErr)
			}
		})
	})
}

func FuzzRawValueReformat(f *testing.F) {
	for _, td := range rawValueTestdata {
		f.Add([]byte(td.in))
	}

	// isValid reports whether b is valid according to the specified options.
	isValid := func(opts DecodeOptions, b []byte) bool {
		d := opts.NewDecoder(bytes.NewReader(b))
		_, errVal := d.ReadValue()
		_, errEOF := d.ReadToken()
		return errVal == nil && errEOF == io.EOF
	}

	// stripWhitespace removes all JSON whitespace characters from the input.
	stripWhitespace := func(in []byte) (out []byte) {
		out = make([]byte, 0, len(in))
		for _, c := range in {
			switch c {
			case ' ', '\n', '\r', '\t':
			default:
				out = append(out, c)
			}
		}
		return out
	}

	// unmarshal unmarshals the input into an any.
	unmarshal := func(in []byte) (out any) {
		if err := Unmarshal(in, &out); err != nil {
			return nil // ignore invalid input
		}
		return out
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		validRFC7159 := isValid(DecodeOptions{AllowInvalidUTF8: true, AllowDuplicateNames: true}, b)
		validRFC8259 := isValid(DecodeOptions{AllowInvalidUTF8: false, AllowDuplicateNames: true}, b)
		validRFC7493 := isValid(DecodeOptions{AllowInvalidUTF8: false, AllowDuplicateNames: false}, b)
		switch {
		case !validRFC7159 && validRFC8259:
			t.Errorf("invalid input per RFC 7159 implies invalid per RFC 8259")
		case !validRFC8259 && validRFC7493:
			t.Errorf("invalid input per RFC 8259 implies invalid per RFC 7493")
		}

		gotValid := RawValue(b).IsValid()
		wantValid := validRFC7493
		if gotValid != wantValid {
			t.Errorf("RawValue.IsValid = %v, want %v", gotValid, wantValid)
		}

		gotCompacted := RawValue(string(b))
		gotCompactOk := gotCompacted.Compact() == nil
		wantCompactOk := validRFC7159
		if !bytes.Equal(stripWhitespace(gotCompacted), stripWhitespace(b)) {
			t.Errorf("stripWhitespace(RawValue.Compact) = %s, want %s", stripWhitespace(gotCompacted), stripWhitespace(b))
		}
		if !reflect.DeepEqual(unmarshal(gotCompacted), unmarshal(b)) {
			t.Errorf("unmarshal(RawValue.Compact) = %s, want %s", unmarshal(gotCompacted), unmarshal(b))
		}
		if gotCompactOk != wantCompactOk {
			t.Errorf("RawValue.Compact success mismatch: got %v, want %v", gotCompactOk, wantCompactOk)
		}

		gotIndented := RawValue(string(b))
		gotIndentOk := gotIndented.Indent("", " ") == nil
		wantIndentOk := validRFC7159
		if !bytes.Equal(stripWhitespace(gotIndented), stripWhitespace(b)) {
			t.Errorf("stripWhitespace(RawValue.Indent) = %s, want %s", stripWhitespace(gotIndented), stripWhitespace(b))
		}
		if !reflect.DeepEqual(unmarshal(gotIndented), unmarshal(b)) {
			t.Errorf("unmarshal(RawValue.Indent) = %s, want %s", unmarshal(gotIndented), unmarshal(b))
		}
		if gotIndentOk != wantIndentOk {
			t.Errorf("RawValue.Indent success mismatch: got %v, want %v", gotIndentOk, wantIndentOk)
		}

		gotCanonicalized := RawValue(string(b))
		gotCanonicalizeOk := gotCanonicalized.Canonicalize() == nil
		wantCanonicalizeOk := validRFC7493
		if !reflect.DeepEqual(unmarshal(gotCanonicalized), unmarshal(b)) {
			t.Errorf("unmarshal(RawValue.Canonicalize) = %s, want %s", unmarshal(gotCanonicalized), unmarshal(b))
		}
		if gotCanonicalizeOk != wantCanonicalizeOk {
			t.Errorf("RawValue.Canonicalize success mismatch: got %v, want %v", gotCanonicalizeOk, wantCanonicalizeOk)
		}
	})
}

func FuzzEqualFold(f *testing.F) {
	for _, tt := range equalFoldTestdata {
		f.Add([]byte(tt.in1), []byte(tt.in2))
	}

	equalFoldSimple := func(x, y []byte) bool {
		strip := func(b []byte) []byte {
			return bytes.Map(func(r rune) rune {
				if r == '_' || r == '-' {
					return -1 // ignore underscores and dashes
				}
				return r
			}, b)
		}
		return bytes.EqualFold(strip(x), strip(y))
	}

	f.Fuzz(func(t *testing.T, s1, s2 []byte) {
		// Compare the optimized and simplified implementations.
		got := equalFold(s1, s2)
		want := equalFoldSimple(s1, s2)
		if got != want {
			t.Errorf("equalFold(%q, %q) = %v, want %v", s1, s2, got, want)
		}
	})
}

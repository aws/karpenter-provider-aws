// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

type rawValueTestdataEntry struct {
	name                testName
	in                  string
	wantValid           bool
	wantCompacted       string
	wantCompactErr      error  // implies wantCompacted is in
	wantIndented        string // wantCompacted if empty; uses "\t" for indent prefix and "    " for indent
	wantIndentErr       error  // implies wantCompacted is in
	wantCanonicalized   string // wantCompacted if empty
	wantCanonicalizeErr error  // implies wantCompacted is in
}

var rawValueTestdata = append(func() (out []rawValueTestdataEntry) {
	// Initialize rawValueTestdata from coderTestdata.
	for _, td := range coderTestdata {
		// NOTE: The Compact method preserves the raw formatting of strings,
		// while the Encoder (by default) does not.
		if td.name.name == "ComplicatedString" {
			td.outCompacted = strings.TrimSpace(td.in)
		}
		out = append(out, rawValueTestdataEntry{
			name:              td.name,
			in:                td.in,
			wantValid:         true,
			wantCompacted:     td.outCompacted,
			wantIndented:      td.outIndented,
			wantCanonicalized: td.outCanonicalized,
		})
	}
	return out
}(), []rawValueTestdataEntry{{
	name: name("RFC8785/Primitives"),
	in: `{
		"numbers": [333333333.33333329, 1E30, 4.50,
					2e-3, 0.000000000000000000000000001],
		"string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/",
		"literals": [null, true, false]
	}`,
	wantValid:     true,
	wantCompacted: `{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001],"string":"\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/","literals":[null,true,false]}`,
	wantIndented: `{
	    "numbers": [
	        333333333.33333329,
	        1E30,
	        4.50,
	        2e-3,
	        0.000000000000000000000000001
	    ],
	    "string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/",
	    "literals": [
	        null,
	        true,
	        false
	    ]
	}`,
	wantCanonicalized: `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`,
}, {
	name: name("RFC8785/ObjectOrdering"),
	in: `{
		"\u20ac": "Euro Sign",
		"\r": "Carriage Return",
		"\ufb33": "Hebrew Letter Dalet With Dagesh",
		"1": "One",
		"\ud83d\ude00": "Emoji: Grinning Face",
		"\u0080": "Control",
		"\u00f6": "Latin Small Letter O With Diaeresis"
	}`,
	wantValid:     true,
	wantCompacted: `{"\u20ac":"Euro Sign","\r":"Carriage Return","\ufb33":"Hebrew Letter Dalet With Dagesh","1":"One","\ud83d\ude00":"Emoji: Grinning Face","\u0080":"Control","\u00f6":"Latin Small Letter O With Diaeresis"}`,
	wantIndented: `{
	    "\u20ac": "Euro Sign",
	    "\r": "Carriage Return",
	    "\ufb33": "Hebrew Letter Dalet With Dagesh",
	    "1": "One",
	    "\ud83d\ude00": "Emoji: Grinning Face",
	    "\u0080": "Control",
	    "\u00f6": "Latin Small Letter O With Diaeresis"
	}`,
	wantCanonicalized: `{"\r":"Carriage Return","1":"One","":"Control","ö":"Latin Small Letter O With Diaeresis","€":"Euro Sign","😀":"Emoji: Grinning Face","דּ":"Hebrew Letter Dalet With Dagesh"}`,
}, {
	name:          name("LargeIntegers"),
	in:            ` [ -9223372036854775808 , 9223372036854775807 ] `,
	wantValid:     true,
	wantCompacted: `[-9223372036854775808,9223372036854775807]`,
	wantIndented: `[
	    -9223372036854775808,
	    9223372036854775807
	]`,
	wantCanonicalized: `[-9223372036854776000,9223372036854776000]`, // NOTE: Loss of precision due to numbers being treated as floats.
}, {
	name:                name("InvalidUTF8"),
	in:                  `  "living` + "\xde\xad\xbe\xef" + `\ufffd�"  `,
	wantValid:           false, // uses RFC 7493 as the definition; which validates UTF-8
	wantCompacted:       `"living` + "\xde\xad\xbe\xef" + `\ufffd�"`,
	wantCanonicalizeErr: &SyntacticError{str: "invalid UTF-8 within string"},
}, {
	name:                name("InvalidUTF8/SurrogateHalf"),
	in:                  `"\ud800"`,
	wantValid:           false, // uses RFC 7493 as the definition; which validates UTF-8
	wantCompacted:       `"\ud800"`,
	wantCanonicalizeErr: &SyntacticError{str: `invalid escape sequence "\"" within string`},
}, {
	name:              name("UppercaseEscaped"),
	in:                `"\u000B"`,
	wantValid:         true,
	wantCompacted:     `"\u000B"`,
	wantCanonicalized: `"\u000b"`,
}, {
	name:          name("DuplicateNames"),
	in:            ` { "0" : 0 , "1" : 1 , "0" : 0 }`,
	wantValid:     false, // uses RFC 7493 as the definition; which does check for object uniqueness
	wantCompacted: `{"0":0,"1":1,"0":0}`,
	wantIndented: `{
	    "0": 0,
	    "1": 1,
	    "0": 0
	}`,
	wantCanonicalizeErr: &SyntacticError{str: `duplicate name "0" in object`},
}}...)

func TestRawValueMethods(t *testing.T) {
	for _, td := range rawValueTestdata {
		t.Run(td.name.name, func(t *testing.T) {
			if td.wantIndented == "" {
				td.wantIndented = td.wantCompacted
			}
			if td.wantCanonicalized == "" {
				td.wantCanonicalized = td.wantCompacted
			}
			if td.wantCompactErr != nil {
				td.wantCompacted = td.in
			}
			if td.wantIndentErr != nil {
				td.wantIndented = td.in
			}
			if td.wantCanonicalizeErr != nil {
				td.wantCanonicalized = td.in
			}

			v := RawValue(td.in)
			gotValid := v.IsValid()
			if gotValid != td.wantValid {
				t.Errorf("%s: RawValue.IsValid = %v, want %v", td.name.where, gotValid, td.wantValid)
			}

			gotCompacted := RawValue(td.in)
			gotCompactErr := gotCompacted.Compact()
			if string(gotCompacted) != td.wantCompacted {
				t.Errorf("%s: RawValue.Compact = %s, want %s", td.name.where, gotCompacted, td.wantCompacted)
			}
			if !reflect.DeepEqual(gotCompactErr, td.wantCompactErr) {
				t.Errorf("%s: RawValue.Compact error mismatch: got %#v, want %#v", td.name.where, gotCompactErr, td.wantCompactErr)
			}

			gotIndented := RawValue(td.in)
			gotIndentErr := gotIndented.Indent("\t", "    ")
			if string(gotIndented) != td.wantIndented {
				t.Errorf("%s: RawValue.Indent = %s, want %s", td.name.where, gotIndented, td.wantIndented)
			}
			if !reflect.DeepEqual(gotIndentErr, td.wantIndentErr) {
				t.Errorf("%s: RawValue.Indent error mismatch: got %#v, want %#v", td.name.where, gotIndentErr, td.wantIndentErr)
			}

			gotCanonicalized := RawValue(td.in)
			gotCanonicalizeErr := gotCanonicalized.Canonicalize()
			if string(gotCanonicalized) != td.wantCanonicalized {
				t.Errorf("%s: RawValue.Canonicalize = %s, want %s", td.name.where, gotCanonicalized, td.wantCanonicalized)
			}
			if !reflect.DeepEqual(gotCanonicalizeErr, td.wantCanonicalizeErr) {
				t.Errorf("%s: RawValue.Canonicalize error mismatch: got %#v, want %#v", td.name.where, gotCanonicalizeErr, td.wantCanonicalizeErr)
			}
		})
	}
}

var lessUTF16Testdata = []string{"", "\r", "1", "\u0080", "\u00f6", "\u20ac", "\U0001f600", "\ufb33"}

func TestLessUTF16(t *testing.T) {
	for i, si := range lessUTF16Testdata {
		for j, sj := range lessUTF16Testdata {
			got := lessUTF16([]byte(si), []byte(sj))
			want := i < j
			if got != want {
				t.Errorf("lessUTF16(%q, %q) = %v, want %v", si, sj, got, want)
			}
		}
	}
}

func FuzzLessUTF16(f *testing.F) {
	for _, td1 := range lessUTF16Testdata {
		for _, td2 := range lessUTF16Testdata {
			f.Add([]byte(td1), []byte(td2))
		}
	}

	// lessUTF16Simple is identical to lessUTF16,
	// but relies on naively converting a string to a []uint16 codepoints.
	// It is easy to verify as correct, but is slow.
	lessUTF16Simple := func(x, y []byte) bool {
		ux := utf16.Encode([]rune(string(x)))
		uy := utf16.Encode([]rune(string(y)))
		// TODO(https://go.dev/issue/57433): Use slices.Compare.
		for {
			if len(ux) == 0 || len(uy) == 0 {
				if len(ux) == len(uy) {
					return string(x) < string(y)
				}
				return len(ux) < len(uy)
			}
			if ux[0] != uy[0] {
				return ux[0] < uy[0]
			}
			ux, uy = ux[1:], uy[1:]
		}
	}

	f.Fuzz(func(t *testing.T, s1, s2 []byte) {
		// Compare the optimized and simplified implementations.
		got := lessUTF16(s1, s2)
		want := lessUTF16Simple(s1, s2)
		if got != want {
			t.Errorf("lessUTF16(%q, %q) = %v, want %v", s1, s2, got, want)
		}
	})
}

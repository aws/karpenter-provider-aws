// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestDiagnosticNotationExamples(t *testing.T) {
	// https://www.rfc-editor.org/rfc/rfc8949.html#name-examples-of-encoded-cbor-da
	testCases := []struct {
		cbor []byte
		diag string
	}{
		{
			cbor: mustHexDecode("00"),
			diag: `0`,
		},
		{
			cbor: mustHexDecode("01"),
			diag: `1`,
		},
		{
			cbor: mustHexDecode("0a"),
			diag: `10`,
		},
		{
			cbor: mustHexDecode("17"),
			diag: `23`,
		},
		{
			cbor: mustHexDecode("1818"),
			diag: `24`,
		},
		{
			cbor: mustHexDecode("1819"),
			diag: `25`,
		},
		{
			cbor: mustHexDecode("1864"),
			diag: `100`,
		},
		{
			cbor: mustHexDecode("1903e8"),
			diag: `1000`,
		},
		{
			cbor: mustHexDecode("1a000f4240"),
			diag: `1000000`,
		},
		{
			cbor: mustHexDecode("1b000000e8d4a51000"),
			diag: `1000000000000`,
		},
		{
			cbor: mustHexDecode("1bffffffffffffffff"),
			diag: `18446744073709551615`,
		},
		{
			cbor: mustHexDecode("c249010000000000000000"),
			diag: `18446744073709551616`,
		},
		{
			cbor: mustHexDecode("3bffffffffffffffff"),
			diag: `-18446744073709551616`,
		},
		{
			cbor: mustHexDecode("c349010000000000000000"),
			diag: `-18446744073709551617`,
		},
		{
			cbor: mustHexDecode("20"),
			diag: `-1`,
		},
		{
			cbor: mustHexDecode("29"),
			diag: `-10`,
		},
		{
			cbor: mustHexDecode("3863"),
			diag: `-100`,
		},
		{
			cbor: mustHexDecode("3903e7"),
			diag: `-1000`,
		},
		{
			cbor: mustHexDecode("f90000"),
			diag: `0.0`,
		},
		{
			cbor: mustHexDecode("f98000"),
			diag: `-0.0`,
		},
		{
			cbor: mustHexDecode("f93c00"),
			diag: `1.0`,
		},
		{
			cbor: mustHexDecode("fb3ff199999999999a"),
			diag: `1.1`,
		},
		{
			cbor: mustHexDecode("f93e00"),
			diag: `1.5`,
		},
		{
			cbor: mustHexDecode("f97bff"),
			diag: `65504.0`,
		},
		{
			cbor: mustHexDecode("fa47c35000"),
			diag: `100000.0`,
		},
		{
			cbor: mustHexDecode("fa7f7fffff"),
			diag: `3.4028234663852886e+38`,
		},
		{
			cbor: mustHexDecode("fb7e37e43c8800759c"),
			diag: `1.0e+300`,
		},
		{
			cbor: mustHexDecode("f90001"),
			diag: `5.960464477539063e-8`,
		},
		{
			cbor: mustHexDecode("f90400"),
			diag: `0.00006103515625`,
		},
		{
			cbor: mustHexDecode("f9c400"),
			diag: `-4.0`,
		},
		{
			cbor: mustHexDecode("fbc010666666666666"),
			diag: `-4.1`,
		},
		{
			cbor: mustHexDecode("f97c00"),
			diag: `Infinity`,
		},
		{
			cbor: mustHexDecode("f97e00"),
			diag: `NaN`,
		},
		{
			cbor: mustHexDecode("f9fc00"),
			diag: `-Infinity`,
		},
		{
			cbor: mustHexDecode("fa7f800000"),
			diag: `Infinity`,
		},
		{
			cbor: mustHexDecode("fa7fc00000"),
			diag: `NaN`,
		},
		{
			cbor: mustHexDecode("faff800000"),
			diag: `-Infinity`,
		},
		{
			cbor: mustHexDecode("fb7ff0000000000000"),
			diag: `Infinity`,
		},
		{
			cbor: mustHexDecode("fb7ff8000000000000"),
			diag: `NaN`,
		},
		{
			cbor: mustHexDecode("fbfff0000000000000"),
			diag: `-Infinity`,
		},
		{
			cbor: mustHexDecode("f4"),
			diag: `false`,
		},
		{
			cbor: mustHexDecode("f5"),
			diag: `true`,
		},
		{
			cbor: mustHexDecode("f6"),
			diag: `null`,
		},
		{
			cbor: mustHexDecode("f7"),
			diag: `undefined`,
		},
		{
			cbor: mustHexDecode("f0"),
			diag: `simple(16)`,
		},
		{
			cbor: mustHexDecode("f8ff"),
			diag: `simple(255)`,
		},
		{
			cbor: mustHexDecode("c074323031332d30332d32315432303a30343a30305a"),
			diag: `0("2013-03-21T20:04:00Z")`,
		},
		{
			cbor: mustHexDecode("c11a514b67b0"),
			diag: `1(1363896240)`,
		},
		{
			cbor: mustHexDecode("c1fb41d452d9ec200000"),
			diag: `1(1363896240.5)`,
		},
		{
			cbor: mustHexDecode("d74401020304"),
			diag: `23(h'01020304')`,
		},
		{
			cbor: mustHexDecode("d818456449455446"),
			diag: `24(h'6449455446')`,
		},
		{
			cbor: mustHexDecode("d82076687474703a2f2f7777772e6578616d706c652e636f6d"),
			diag: `32("http://www.example.com")`,
		},
		{
			cbor: mustHexDecode("40"),
			diag: `h''`,
		},
		{
			cbor: mustHexDecode("4401020304"),
			diag: `h'01020304'`,
		},
		{
			cbor: mustHexDecode("60"),
			diag: `""`,
		},
		{
			cbor: mustHexDecode("6161"),
			diag: `"a"`,
		},
		{
			cbor: mustHexDecode("6449455446"),
			diag: `"IETF"`,
		},
		{
			cbor: mustHexDecode("62225c"),
			diag: `"\"\\"`,
		},
		{
			cbor: mustHexDecode("62c3bc"),
			diag: `"\u00fc"`,
		},
		{
			cbor: mustHexDecode("63e6b0b4"),
			diag: `"\u6c34"`,
		},
		{
			cbor: mustHexDecode("64f0908591"),
			diag: `"\ud800\udd51"`,
		},
		{
			cbor: mustHexDecode("80"),
			diag: `[]`,
		},
		{
			cbor: mustHexDecode("83010203"),
			diag: `[1, 2, 3]`,
		},
		{
			cbor: mustHexDecode("8301820203820405"),
			diag: `[1, [2, 3], [4, 5]]`,
		},
		{
			cbor: mustHexDecode("98190102030405060708090a0b0c0d0e0f101112131415161718181819"),
			diag: `[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]`,
		},
		{
			cbor: mustHexDecode("a0"),
			diag: `{}`,
		},
		{
			cbor: mustHexDecode("a201020304"),
			diag: `{1: 2, 3: 4}`,
		},
		{
			cbor: mustHexDecode("a26161016162820203"),
			diag: `{"a": 1, "b": [2, 3]}`,
		},
		{
			cbor: mustHexDecode("826161a161626163"),
			diag: `["a", {"b": "c"}]`,
		},
		{
			cbor: mustHexDecode("a56161614161626142616361436164614461656145"),
			diag: `{"a": "A", "b": "B", "c": "C", "d": "D", "e": "E"}`,
		},
		{
			cbor: mustHexDecode("5f42010243030405ff"),
			diag: `(_ h'0102', h'030405')`,
		},
		{
			cbor: mustHexDecode("7f657374726561646d696e67ff"),
			diag: `(_ "strea", "ming")`,
		},
		{
			cbor: mustHexDecode("9fff"),
			diag: `[_ ]`,
		},
		{
			cbor: mustHexDecode("9f018202039f0405ffff"),
			diag: `[_ 1, [2, 3], [_ 4, 5]]`,
		},
		{
			cbor: mustHexDecode("9f01820203820405ff"),
			diag: `[_ 1, [2, 3], [4, 5]]`,
		},
		{
			cbor: mustHexDecode("83018202039f0405ff"),
			diag: `[1, [2, 3], [_ 4, 5]]`,
		},
		{
			cbor: mustHexDecode("83019f0203ff820405"),
			diag: `[1, [_ 2, 3], [4, 5]]`,
		},
		{
			cbor: mustHexDecode("9f0102030405060708090a0b0c0d0e0f101112131415161718181819ff"),
			diag: `[_ 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]`,
		},
		{
			cbor: mustHexDecode("bf61610161629f0203ffff"),
			diag: `{_ "a": 1, "b": [_ 2, 3]}`,
		},
		{
			cbor: mustHexDecode("826161bf61626163ff"),
			diag: `["a", {_ "b": "c"}]`,
		},
		{
			cbor: mustHexDecode("bf6346756ef563416d7421ff"),
			diag: `{_ "Fun": true, "Amt": -2}`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Diagnostic %d", i), func(t *testing.T) {
			str, err := Diagnose(tc.cbor)
			if err != nil {
				t.Errorf("Diagnostic(0x%x) returned error %q", tc.cbor, err)
			} else if str != tc.diag {
				t.Errorf("Diagnostic(0x%x) returned `%s`, want `%s`", tc.cbor, str, tc.diag)
			}

			str, rest, err := DiagnoseFirst(tc.cbor)
			if err != nil {
				t.Errorf("Diagnostic(0x%x) returned error %q", tc.cbor, err)
			} else if str != tc.diag {
				t.Errorf("Diagnostic(0x%x) returned `%s`, want `%s`", tc.cbor, str, tc.diag)
			}

			if rest == nil {
				t.Errorf("Diagnostic(0x%x) returned nil rest", tc.cbor)
			} else if len(rest) != 0 {
				t.Errorf("Diagnostic(0x%x) returned non-empty rest '%x'", tc.cbor, rest)
			}
		})
	}
}

func TestDiagnoseByteString(t *testing.T) {
	testCases := []struct {
		title string
		cbor  []byte
		diag  string
		opts  *DiagOptions
	}{
		{
			title: "base16",
			cbor:  mustHexDecode("4412345678"),
			diag:  `h'12345678'`,
			opts: &DiagOptions{
				ByteStringEncoding: ByteStringBase16Encoding,
			},
		},
		{
			title: "base32",
			cbor:  mustHexDecode("4412345678"),
			diag:  `b32'CI2FM6A'`,
			opts: &DiagOptions{
				ByteStringEncoding: ByteStringBase32Encoding,
			},
		},
		{
			title: "base32hex",
			cbor:  mustHexDecode("4412345678"),
			diag:  `h32'28Q5CU0'`,
			opts: &DiagOptions{
				ByteStringEncoding: ByteStringBase32HexEncoding,
			},
		},
		{
			title: "base64",
			cbor:  mustHexDecode("4412345678"),
			diag:  `b64'EjRWeA'`,
			opts: &DiagOptions{
				ByteStringEncoding: ByteStringBase64Encoding,
			},
		},
		{
			title: "without ByteStringHexWhitespace option",
			cbor:  mustHexDecode("4b48656c6c6f20776f726c64"),
			diag:  `h'48656c6c6f20776f726c64'`,
			opts: &DiagOptions{
				ByteStringHexWhitespace: false,
			},
		},
		{
			title: "with ByteStringHexWhitespace option",
			cbor:  mustHexDecode("4b48656c6c6f20776f726c64"),
			diag:  `h'48 65 6c 6c 6f 20 77 6f 72 6c 64'`,
			opts: &DiagOptions{
				ByteStringHexWhitespace: true,
			},
		},
		{
			title: "without ByteStringText option",
			cbor:  mustHexDecode("4b68656c6c6f20776f726c64"),
			diag:  `h'68656c6c6f20776f726c64'`,
			opts: &DiagOptions{
				ByteStringText: false,
			},
		},
		{
			title: "with ByteStringText option",
			cbor:  mustHexDecode("4b68656c6c6f20776f726c64"),
			diag:  `'hello world'`,
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
		{
			title: "without ByteStringText option and with ByteStringHexWhitespace option",
			cbor:  mustHexDecode("4b68656c6c6f20776f726c64"),
			diag:  `h'68 65 6c 6c 6f 20 77 6f 72 6c 64'`,
			opts: &DiagOptions{
				ByteStringText:          false,
				ByteStringHexWhitespace: true,
			},
		},
		{
			title: "without ByteStringEmbeddedCBOR",
			cbor:  mustHexDecode("4101"),
			diag:  `h'01'`,
			opts: &DiagOptions{
				ByteStringEmbeddedCBOR: false,
			},
		},
		{
			title: "with ByteStringEmbeddedCBOR",
			cbor:  mustHexDecode("4101"),
			diag:  `<<1>>`,
			opts: &DiagOptions{
				ByteStringEmbeddedCBOR: true,
			},
		},
		{
			title: "multi CBOR items without ByteStringEmbeddedCBOR",
			cbor:  mustHexDecode("420102"),
			diag:  `h'0102'`,
			opts: &DiagOptions{
				ByteStringEmbeddedCBOR: false,
			},
		},
		{
			title: "multi CBOR items with ByteStringEmbeddedCBOR",
			cbor:  mustHexDecode("420102"),
			diag:  `<<1, 2>>`,
			opts: &DiagOptions{
				ByteStringEmbeddedCBOR: true,
			},
		},
		{
			title: "multi CBOR items with ByteStringEmbeddedCBOR",
			cbor:  mustHexDecode("4563666F6FF6"),
			diag:  `h'63666f6ff6'`,
			opts: &DiagOptions{
				ByteStringEmbeddedCBOR: false,
			},
		},
		{
			title: "multi CBOR items with ByteStringEmbeddedCBOR",
			cbor:  mustHexDecode("4563666F6FF6"),
			diag:  `<<"foo", null>>`,
			opts: &DiagOptions{
				ByteStringEmbeddedCBOR: true,
			},
		},
		{
			title: "indefinite length byte string with no chunks",
			cbor:  mustHexDecode("5fff"),
			diag:  `''_`,
			opts:  &DiagOptions{},
		},
		{
			title: "indefinite length byte string with a empty byte string",
			cbor:  mustHexDecode("5f40ff"),
			diag:  `(_ h'')`, // RFC 8949, Section 8.1 says `(_ '')` but it looks wrong and conflicts with Appendix A.
			opts:  &DiagOptions{},
		},
		{
			title: "indefinite length byte string with two empty byte string",
			cbor:  mustHexDecode("5f4040ff"),
			diag:  `(_ h'', h'')`,
			opts:  &DiagOptions{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			dm, err := tc.opts.DiagMode()
			if err != nil {
				t.Errorf("DiagMode() for 0x%x returned error %q", tc.cbor, err)
			}

			str, err := dm.Diagnose(tc.cbor)
			if err != nil {
				t.Errorf("Diagnose(0x%x) returned error %q", tc.cbor, err)
			} else if str != tc.diag {
				t.Errorf("Diagnose(0x%x) returned `%s`, want %s", tc.cbor, str, tc.diag)
			}
		})
	}
}

func TestDiagnoseTextString(t *testing.T) {
	testCases := []struct {
		title string
		cbor  []byte
		diag  string
		opts  *DiagOptions
	}{
		{
			title: "\t",
			cbor:  mustHexDecode("6109"),
			diag:  `"\t"`,
			opts:  &DiagOptions{},
		},
		{
			title: "\r",
			cbor:  mustHexDecode("610d"),
			diag:  `"\r"`,
			opts:  &DiagOptions{},
		},
		{
			title: "other ascii",
			cbor:  mustHexDecode("611b"),
			diag:  `"\u001b"`,
			opts:  &DiagOptions{},
		},
		{
			title: "valid UTF-8 text in byte string",
			cbor:  mustHexDecode("4d68656c6c6f2c20e4bda0e5a5bd"),
			diag:  `'hello, \u4f60\u597d'`,
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
		{
			title: "valid UTF-8 text in text string",
			cbor:  mustHexDecode("6d68656c6c6f2c20e4bda0e5a5bd"),
			diag:  `"hello, \u4f60\u597d"`, // "hello, 你好"
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
		{
			title: "invalid UTF-8 text in byte string",
			cbor:  mustHexDecode("4d68656c6c6fffeee4bda0e5a5bd"),
			diag:  `h'68656c6c6fffeee4bda0e5a5bd'`,
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
		{
			title: "valid grapheme cluster text in byte string",
			cbor:  mustHexDecode("583448656c6c6f2c2027e29da4efb88fe2808df09f94a5270ae4bda0e5a5bdefbc8c22f09fa791e2808df09fa49de2808df09fa79122"),
			diag:  `'Hello, \'\u2764\ufe0f\u200d\ud83d\udd25\'\n\u4f60\u597d\uff0c"\ud83e\uddd1\u200d\ud83e\udd1d\u200d\ud83e\uddd1"'`,
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
		{
			title: "valid grapheme cluster text in text string",
			cbor:  mustHexDecode("783448656c6c6f2c2027e29da4efb88fe2808df09f94a5270ae4bda0e5a5bdefbc8c22f09fa791e2808df09fa49de2808df09fa79122"),
			diag:  `"Hello, '\u2764\ufe0f\u200d\ud83d\udd25'\n\u4f60\u597d\uff0c\"\ud83e\uddd1\u200d\ud83e\udd1d\u200d\ud83e\uddd1\""`, // "Hello, '❤️‍🔥'\n你好，\"🧑‍🤝‍🧑\""
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
		{
			title: "invalid grapheme cluster text in byte string",
			cbor:  mustHexDecode("583448656c6c6feeff27e29da4efb88fe2808df09f94a5270de4bda0e5a5bdefbc8c22f09fa791e2808df09fa49de2808df09fa79122"),
			diag:  `h'48656c6c6feeff27e29da4efb88fe2808df09f94a5270de4bda0e5a5bdefbc8c22f09fa791e2808df09fa49de2808df09fa79122'`,
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
		{
			title: "indefinite length text string with no chunks",
			cbor:  mustHexDecode("7fff"),
			diag:  `""_`,
			opts:  &DiagOptions{},
		},
		{
			title: "indefinite length text string with a empty text string",
			cbor:  mustHexDecode("7f60ff"),
			diag:  `(_ "")`,
			opts:  &DiagOptions{},
		},
		{
			title: "indefinite length text string with two empty text string",
			cbor:  mustHexDecode("7f6060ff"),
			diag:  `(_ "", "")`,
			opts:  &DiagOptions{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			dm, err := tc.opts.DiagMode()
			if err != nil {
				t.Errorf("DiagMode() for 0x%x returned error %q", tc.cbor, err)
			}

			str, err := dm.Diagnose(tc.cbor)
			if err != nil {
				t.Errorf("Diagnose(0x%x) returned error %q", tc.cbor, err)
			} else if str != tc.diag {
				t.Errorf("Diagnose(0x%x) returned `%s`, want %s", tc.cbor, str, tc.diag)
			}
		})
	}
}

func TestDiagnoseInvalidTextString(t *testing.T) {
	testCases := []struct {
		title        string
		cbor         []byte
		wantErrorMsg string
		opts         *DiagOptions
	}{
		{
			title:        "invalid UTF-8 text in text string",
			cbor:         mustHexDecode("6d68656c6c6fffeee4bda0e5a5bd"),
			wantErrorMsg: "invalid UTF-8 string",
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
		{
			title:        "invalid grapheme cluster text in text string",
			cbor:         mustHexDecode("783448656c6c6feeff27e29da4efb88fe2808df09f94a5270de4bda0e5a5bdefbc8c22f09fa791e2808df09fa49de2808df09fa79122"),
			wantErrorMsg: "invalid UTF-8 string",
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
		{
			title:        "invalid indefinite length text string",
			cbor:         mustHexDecode("7f6040ff"),
			wantErrorMsg: `wrong element type`,
			opts: &DiagOptions{
				ByteStringText: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			dm, err := tc.opts.DiagMode()
			if err != nil {
				t.Errorf("DiagMode() for 0x%x returned error %q", tc.cbor, err)
			}

			_, err = dm.Diagnose(tc.cbor)
			if err == nil {
				t.Errorf("Diagnose(0x%x) didn't return error", tc.cbor)
			} else if !strings.Contains(err.Error(), tc.wantErrorMsg) {
				t.Errorf("Diagnose(0x%x) returned error %q", tc.cbor, err)
			}
		})
	}
}

func TestDiagnoseFloatingPointNumber(t *testing.T) {
	testCases := []struct {
		title string
		cbor  []byte
		diag  string
		opts  *DiagOptions
	}{
		{
			title: "float16 without FloatPrecisionIndicator option",
			cbor:  mustHexDecode("f93e00"),
			diag:  `1.5`,
			opts: &DiagOptions{
				FloatPrecisionIndicator: false,
			},
		},
		{
			title: "float16 with FloatPrecisionIndicator option",
			cbor:  mustHexDecode("f93e00"),
			diag:  `1.5_1`,
			opts: &DiagOptions{
				FloatPrecisionIndicator: true,
			},
		},
		{
			title: "float32 without FloatPrecisionIndicator option",
			cbor:  mustHexDecode("fa47c35000"),
			diag:  `100000.0`,
			opts: &DiagOptions{
				FloatPrecisionIndicator: false,
			},
		},
		{
			title: "float32 with FloatPrecisionIndicator option",
			cbor:  mustHexDecode("fa47c35000"),
			diag:  `100000.0_2`,
			opts: &DiagOptions{
				FloatPrecisionIndicator: true,
			},
		},
		{
			title: "float64 without FloatPrecisionIndicator option",
			cbor:  mustHexDecode("fbc010666666666666"),
			diag:  `-4.1`,
			opts: &DiagOptions{
				FloatPrecisionIndicator: false,
			},
		},
		{
			title: "float64 with FloatPrecisionIndicator option",
			cbor:  mustHexDecode("fbc010666666666666"),
			diag:  `-4.1_3`,
			opts: &DiagOptions{
				FloatPrecisionIndicator: true,
			},
		},
		{
			title: "with FloatPrecisionIndicator option",
			cbor:  mustHexDecode("c1fb41d452d9ec200000"),
			diag:  `1(1363896240.5_3)`,
			opts: &DiagOptions{
				FloatPrecisionIndicator: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			dm, err := tc.opts.DiagMode()
			if err != nil {
				t.Errorf("DiagMode() for 0x%x returned error %q", tc.cbor, err)
			}

			str, err := dm.Diagnose(tc.cbor)
			if err != nil {
				t.Errorf("Diagnose(0x%x) returned error %q", tc.cbor, err)
			} else if str != tc.diag {
				t.Errorf("Diagnose(0x%x) returned `%s`, want %s", tc.cbor, str, tc.diag)
			}
		})
	}
}

func TestDiagnoseFirst(t *testing.T) {
	testCases := []struct {
		title        string
		cbor         []byte
		diag         string
		wantRest     []byte
		wantErrorMsg string
	}{
		{
			title:        "with no trailing data",
			cbor:         mustHexDecode("f93e00"),
			diag:         `1.5`,
			wantRest:     []byte{},
			wantErrorMsg: "",
		},
		{
			title:        "with CBOR Sequences",
			cbor:         mustHexDecode("f93e0064494554464401020304"),
			diag:         `1.5`,
			wantRest:     mustHexDecode("64494554464401020304"),
			wantErrorMsg: "",
		},
		{
			title:        "with invalid CBOR trailing data",
			cbor:         mustHexDecode("f93e00ff494554464401020304"),
			diag:         `1.5`,
			wantRest:     mustHexDecode("ff494554464401020304"),
			wantErrorMsg: "",
		},
		{
			title:        "with invalid CBOR data",
			cbor:         mustHexDecode("f93e"),
			diag:         ``,
			wantRest:     nil,
			wantErrorMsg: "unexpected EOF",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			str, rest, err := DiagnoseFirst(tc.cbor)
			if str != tc.diag {
				t.Errorf("DiagnoseFirst(0x%x) returned `%s`, want %s", tc.cbor, str, tc.diag)
			}

			if bytes.Equal(rest, tc.wantRest) == false {
				if str != tc.diag {
					t.Errorf("DiagnoseFirst(0x%x) returned rest `%x`, want rest %x", tc.cbor, rest, tc.wantRest)
				}
			}

			switch {
			case tc.wantErrorMsg == "" && err != nil:
				t.Errorf("DiagnoseFirst(0x%x) returned error %q", tc.cbor, err)
			case tc.wantErrorMsg != "" && err == nil:
				t.Errorf("DiagnoseFirst(0x%x) returned nil error, want error %q", tc.cbor, err)
			case tc.wantErrorMsg != "" && !strings.Contains(err.Error(), tc.wantErrorMsg):
				t.Errorf("DiagnoseFirst(0x%x) returned error %q, want error %q", tc.cbor, err, tc.wantErrorMsg)
			}
		})
	}
}

func TestDiagnoseCBORSequences(t *testing.T) {
	testCases := []struct {
		title       string
		cbor        []byte
		diag        string
		opts        *DiagOptions
		returnError bool
	}{
		{
			title: "CBOR Sequences without CBORSequence option",
			cbor:  mustHexDecode("f93e0064494554464401020304"),
			diag:  ``,
			opts: &DiagOptions{
				CBORSequence: false,
			},
			returnError: true,
		},
		{
			title: "CBOR Sequences with CBORSequence option",
			cbor:  mustHexDecode("f93e0064494554464401020304"),
			diag:  `1.5, "IETF", h'01020304'`,
			opts: &DiagOptions{
				CBORSequence: true,
			},
			returnError: false,
		},
		{
			title: "CBOR Sequences with CBORSequence option",
			cbor:  mustHexDecode("0102"),
			diag:  `1, 2`,
			opts: &DiagOptions{
				CBORSequence: true,
			},
			returnError: false,
		},
		{
			title: "CBOR Sequences with CBORSequence option",
			cbor:  mustHexDecode("63666F6FF6"),
			diag:  `"foo", null`,
			opts: &DiagOptions{
				CBORSequence: true,
			},
			returnError: false,
		},
		{
			title: "partial/incomplete CBOR Sequences",
			cbor:  mustHexDecode("f93e00644945544644010203"),
			diag:  `1.5, "IETF"`,
			opts: &DiagOptions{
				CBORSequence: true,
			},
			returnError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			dm, err := tc.opts.DiagMode()
			if err != nil {
				t.Errorf("DiagMode() for 0x%x returned error %q", tc.cbor, err)
			}

			str, err := dm.Diagnose(tc.cbor)
			if tc.returnError && err == nil {
				t.Errorf("Diagnose(0x%x) returned error %q", tc.cbor, err)
			} else if !tc.returnError && err != nil {
				t.Errorf("Diagnose(0x%x) returned error %q", tc.cbor, err)
			}

			if str != tc.diag {
				t.Errorf("Diagnose(0x%x) returned `%s`, want %s", tc.cbor, str, tc.diag)
			}
		})
	}
}

func TestDiagnoseTag(t *testing.T) {
	testCases := []struct {
		title       string
		cbor        []byte
		diag        string
		opts        *DiagOptions
		returnError bool
	}{
		{
			title:       "CBOR tag number 2 with not well-formed encoded CBOR data item",
			cbor:        mustHexDecode("c201"),
			diag:        ``,
			opts:        &DiagOptions{},
			returnError: true,
		},
		{
			title:       "CBOR tag number 3 with not well-formed encoded CBOR data item",
			cbor:        mustHexDecode("c301"),
			diag:        ``,
			opts:        &DiagOptions{},
			returnError: true,
		},
		{
			title:       "CBOR tag number 2 with well-formed encoded CBOR data item",
			cbor:        mustHexDecode("c240"),
			diag:        `0`,
			opts:        &DiagOptions{},
			returnError: false,
		},
		{
			title:       "CBOR tag number 3 with well-formed encoded CBOR data item",
			cbor:        mustHexDecode("c340"),
			diag:        `-1`, // -1 - n
			opts:        &DiagOptions{},
			returnError: false,
		},
		{
			title:       "CBOR tag number 2 with well-formed encoded CBOR data item",
			cbor:        mustHexDecode("c249010000000000000000"),
			diag:        `18446744073709551616`,
			opts:        &DiagOptions{},
			returnError: false,
		},
		{
			title:       "CBOR tag number 3 with well-formed encoded CBOR data item",
			cbor:        mustHexDecode("c349010000000000000000"),
			diag:        `-18446744073709551617`, // -1 - n
			opts:        &DiagOptions{},
			returnError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			dm, err := tc.opts.DiagMode()
			if err != nil {
				t.Errorf("DiagMode() for 0x%x returned error %q", tc.cbor, err)
			}

			str, err := dm.Diagnose(tc.cbor)
			if tc.returnError && err == nil {
				t.Errorf("Diagnose(0x%x) returned error %q", tc.cbor, err)
			} else if !tc.returnError && err != nil {
				t.Errorf("Diagnose(0x%x) returned error %q", tc.cbor, err)
			}

			if str != tc.diag {
				t.Errorf("Diagnose(0x%x) returned `%s`, want %s", tc.cbor, str, tc.diag)
			}
		})
	}
}

func TestDiagnoseOptions(t *testing.T) {
	opts := DiagOptions{
		ByteStringEncoding:      ByteStringBase32Encoding,
		ByteStringHexWhitespace: true,
		ByteStringText:          false,
		ByteStringEmbeddedCBOR:  true,
		CBORSequence:            false,
		FloatPrecisionIndicator: true,
		MaxNestedLevels:         100,
		MaxArrayElements:        101,
		MaxMapPairs:             102,
	}
	dm, err := opts.DiagMode()
	if err != nil {
		t.Errorf("DiagMode() returned an error %v", err)
	}
	opts2 := dm.DiagOptions()
	if !reflect.DeepEqual(opts, opts2) {
		t.Errorf("DiagOptions() returned wrong options %v, want %v", opts2, opts)
	}

	opts = DiagOptions{
		ByteStringEncoding:      ByteStringBase64Encoding,
		ByteStringHexWhitespace: false,
		ByteStringText:          true,
		ByteStringEmbeddedCBOR:  false,
		CBORSequence:            true,
		FloatPrecisionIndicator: false,
		MaxNestedLevels:         100,
		MaxArrayElements:        101,
		MaxMapPairs:             102,
	}
	dm, err = opts.DiagMode()
	if err != nil {
		t.Errorf("DiagMode() returned an error %v", err)
	}
	opts2 = dm.DiagOptions()
	if !reflect.DeepEqual(opts, opts2) {
		t.Errorf("DiagOptions() returned wrong options %v, want %v", opts2, opts)
	}
}

func TestInvalidDiagnoseOptions(t *testing.T) {
	opts := &DiagOptions{
		ByteStringEncoding: ByteStringBase64Encoding + 1,
	}
	_, err := opts.DiagMode()
	if err == nil {
		t.Errorf("DiagMode() with invalid ByteStringEncoding option didn't return error")
	}
}

func TestDiagnoseExtraneousData(t *testing.T) {
	data := mustHexDecode("63666F6FF6")
	_, err := Diagnose(data)
	if err == nil {
		t.Errorf("Diagnose(0x%x) didn't return error", data)
	} else if !strings.Contains(err.Error(), `extraneous data`) {
		t.Errorf("Diagnose(0x%x) returned error %q", data, err)
	}

	_, _, err = DiagnoseFirst(data)
	if err != nil {
		t.Errorf("DiagnoseFirst(0x%x) returned error %v", data, err)
	}
}

func TestDiagnoseNotwellformedData(t *testing.T) {
	data := mustHexDecode("5f4060ff")
	_, err := Diagnose(data)
	if err == nil {
		t.Errorf("Diagnose(0x%x) didn't return error", data)
	} else if !strings.Contains(err.Error(), `wrong element type`) {
		t.Errorf("Diagnose(0x%x) returned error %q", data, err)
	}
}

func TestDiagnoseEmptyData(t *testing.T) {
	var emptyData []byte

	defaultMode, _ := DiagOptions{}.DiagMode()
	sequenceMode, _ := DiagOptions{CBORSequence: true}.DiagMode()

	testCases := []struct {
		name string
		dm   DiagMode
	}{
		{name: "default", dm: defaultMode},
		{name: "sequence", dm: sequenceMode},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := tc.dm.Diagnose(emptyData)
			if s != "" {
				t.Errorf("Diagnose() didn't return empty notation for empty data")
			}
			if err != io.EOF {
				t.Errorf("Diagnose() didn't return io.EOF for empty data")
			}

			s, rest, err := tc.dm.DiagnoseFirst(emptyData)
			if s != "" {
				t.Errorf("DiagnoseFirst() didn't return empty notation for empty data")
			}
			if len(rest) != 0 {
				t.Errorf("DiagnoseFirst() didn't return empty rest for empty data")
			}
			if err != io.EOF {
				t.Errorf("DiagnoseFirst() didn't return io.EOF for empty data")
			}
		})
	}
}

func BenchmarkDiagnose(b *testing.B) {
	for _, tc := range []struct {
		name  string
		opts  DiagOptions
		input []byte
	}{
		{
			name:  "escaped character in text string",
			opts:  DiagOptions{},
			input: mustHexDecode("62c3bc"), // "\u00fc"
		},
		{
			name:  "byte string base16 encoding",
			opts:  DiagOptions{ByteStringEncoding: ByteStringBase16Encoding},
			input: []byte("\x45hello"),
		},
		{
			name:  "byte string base32 encoding",
			opts:  DiagOptions{ByteStringEncoding: ByteStringBase32Encoding},
			input: []byte("\x45hello"),
		},
		{
			name:  "byte string base32hex encoding",
			opts:  DiagOptions{ByteStringEncoding: ByteStringBase32HexEncoding},
			input: []byte("\x45hello"),
		},
		{
			name:  "byte string base64url encoding",
			opts:  DiagOptions{ByteStringEncoding: ByteStringBase64Encoding},
			input: []byte("\x45hello"),
		},
	} {
		b.Run(tc.name, func(b *testing.B) {
			dm, err := tc.opts.DiagMode()
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = dm.Diagnose(tc.input)
			}
		})
	}
}

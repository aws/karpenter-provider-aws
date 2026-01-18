// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

// TestStdlibJSONCompatibility tests compatibility as a drop-in replacement for the standard library
// encoding/json package on a round trip encoding from Go object to interface{}.
func TestStdlibJSONCompatibility(t *testing.T) {
	// TODO: With better coverage and compatibility, it could be useful to expose these option
	// configurations to users.

	enc, err := cbor.EncOptions{
		ByteSliceLaterFormat: cbor.ByteSliceLaterFormatBase64,
		String:               cbor.StringToByteString,
		ByteArray:            cbor.ByteArrayToArray,
	}.EncMode()
	if err != nil {
		t.Fatal(err)
	}

	dec, err := cbor.DecOptions{
		DefaultByteStringType:    reflect.TypeOf(""),
		ByteStringToString:       cbor.ByteStringToStringAllowedWithExpectedLaterEncoding,
		ByteStringExpectedFormat: cbor.ByteStringExpectedBase64,
	}.DecMode()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		original   any
		ifaceEqual bool // require equal intermediate interface{} values from both protocols
	}{
		{
			name:       "byte slice to base64-encoded string",
			original:   []byte("hello world"),
			ifaceEqual: true,
		},
		{
			name:       "byte array to array of integers",
			original:   [11]byte{'h', 'e', 'l', 'l', 'o', ' ', 'w', 'o', 'r', 'l', 'd'},
			ifaceEqual: false, // encoding/json decodes the array elements to float64
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("original: %#v", tc.original)

			j1, err := json.Marshal(tc.original)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("original to json: %s", string(j1))

			c1, err := enc.Marshal(tc.original)
			if err != nil {
				t.Fatal(err)
			}
			diag1, err := cbor.Diagnose(c1)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("original to cbor: %s", diag1)

			var jintf any
			err = json.Unmarshal(j1, &jintf)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("json to interface{} (%T): %#v", jintf, jintf)

			var cintf any
			err = dec.Unmarshal(c1, &cintf)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("cbor to interface{} (%T): %#v", cintf, cintf)

			j2, err := json.Marshal(jintf)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("interface{} to json: %s", string(j2))

			c2, err := enc.Marshal(cintf)
			if err != nil {
				t.Fatal(err)
			}
			diag2, err := cbor.Diagnose(c2)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("interface{} to cbor: %s", diag2)

			if !reflect.DeepEqual(jintf, cintf) {
				if tc.ifaceEqual {
					t.Errorf("native-to-interface{} via cbor differed from native-to-interface{} via json")
				} else {
					t.Logf("native-to-interface{} via cbor differed from native-to-interface{} via json")
				}
			}

			jfinalValue := reflect.New(reflect.TypeOf(tc.original))
			err = json.Unmarshal(j2, jfinalValue.Interface())
			if err != nil {
				t.Fatal(err)
			}
			jfinal := jfinalValue.Elem().Interface()
			t.Logf("json to native: %#v", jfinal)
			if !reflect.DeepEqual(tc.original, jfinal) {
				t.Error("diff in json roundtrip")
			}

			cfinalValue := reflect.New(reflect.TypeOf(tc.original))
			err = dec.Unmarshal(c2, cfinalValue.Interface())
			if err != nil {
				t.Fatal(err)
			}
			cfinal := cfinalValue.Elem().Interface()
			t.Logf("cbor to native: %#v", cfinal)
			if !reflect.DeepEqual(tc.original, cfinal) {
				t.Error("diff in cbor roundtrip")
			}

		})
	}
}

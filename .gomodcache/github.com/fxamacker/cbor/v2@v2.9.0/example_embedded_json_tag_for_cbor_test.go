// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor_test

// fxamacker/cbor allows user apps to use almost any current or future
// CBOR tag number by implementing cbor.Marshaler and cbor.Unmarshaler
// interfaces.  Essentially, MarshalCBOR and UnmarshalCBOR functions that
// are implemented by user apps will automatically be called by this
// CBOR codec's Marshal, Unmarshal, etc.
//
// This example shows how to encode and decode a tagged CBOR data item with
// tag number 262 and the tag content is a JSON object "embedded" as a
// CBOR byte string (major type 2).
//
// NOTE: RFC 8949 does not mention tag number 262. IANA assigned
// CBOR tag number 262 as "Embedded JSON Object" specified by the
// document Embedded JSON Tag for CBOR:
//
//	"Tag 262 can be applied to a byte string (major type 2) to indicate
//	that the byte string is a JSON Object. The length of the byte string
//	indicates the content."
//
// For more info, see Embedded JSON Tag for CBOR at:
// https://github.com/toravir/CBOR-Tag-Specs/blob/master/embeddedJSON.md

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// cborTagNumForEmbeddedJSON is the CBOR tag number 262.
const cborTagNumForEmbeddedJSON = 262

// EmbeddedJSON represents a Go value to be encoded as a tagged CBOR data item
// with tag number 262 and the tag content is a JSON object "embedded" as a
// CBOR byte string (major type 2).
type EmbeddedJSON struct {
	any
}

func NewEmbeddedJSON(val any) EmbeddedJSON {
	return EmbeddedJSON{val}
}

// MarshalCBOR encodes EmbeddedJSON to a tagged CBOR data item with the
// tag number 262 and the tag content is a JSON object that is
// "embedded" as a CBOR byte string.
func (v EmbeddedJSON) MarshalCBOR() ([]byte, error) {
	// Encode v to JSON object.
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Create cbor.Tag representing a tagged CBOR data item.
	tag := cbor.Tag{
		Number:  cborTagNumForEmbeddedJSON,
		Content: data,
	}

	// Marshal to a tagged CBOR data item.
	return cbor.Marshal(tag)
}

// UnmarshalCBOR decodes a tagged CBOR data item to EmbeddedJSON.
// The byte slice provided to this function must contain a single
// tagged CBOR data item with the tag number 262 and tag content
// must be a JSON object "embedded" as a CBOR byte string.
func (v *EmbeddedJSON) UnmarshalCBOR(b []byte) error {
	// Unmarshal tagged CBOR data item.
	var tag cbor.Tag
	if err := cbor.Unmarshal(b, &tag); err != nil {
		return err
	}

	// Check tag number.
	if tag.Number != cborTagNumForEmbeddedJSON {
		return fmt.Errorf("got tag number %d, expect tag number %d", tag.Number, cborTagNumForEmbeddedJSON)
	}

	// Check tag content.
	jsonData, isByteString := tag.Content.([]byte)
	if !isByteString {
		return fmt.Errorf("got tag content type %T, expect tag content []byte", tag.Content)
	}

	// Unmarshal JSON object.
	return json.Unmarshal(jsonData, v)
}

// MarshalJSON encodes EmbeddedJSON to a JSON object.
func (v EmbeddedJSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.any)
}

// UnmarshalJSON decodes a JSON object.
func (v *EmbeddedJSON) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return dec.Decode(&v.any)
}

func Example_embeddedJSONTagForCBOR() {
	value := NewEmbeddedJSON(map[string]any{
		"name": "gopher",
		"id":   json.Number("42"),
	})

	data, err := cbor.Marshal(value)
	if err != nil {
		panic(err)
	}

	fmt.Printf("cbor: %x\n", data)

	var v EmbeddedJSON
	err = cbor.Unmarshal(data, &v)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%+v\n", v.any)
	for k, v := range v.any.(map[string]any) {
		fmt.Printf("  %s: %v (%T)\n", k, v, v)
	}
}

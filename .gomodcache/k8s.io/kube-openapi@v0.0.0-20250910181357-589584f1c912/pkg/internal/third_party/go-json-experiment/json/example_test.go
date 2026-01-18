// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/netip"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"k8s.io/kube-openapi/pkg/internal/third_party/go-json-experiment/json"
)

// If a type implements encoding.TextMarshaler and/or encoding.TextUnmarshaler,
// then the MarshalText and UnmarshalText methods are used to encode/decode
// the value to/from a JSON string.
func Example_textMarshal() {
	// Round-trip marshal and unmarshal a hostname map where the netip.Addr type
	// implements both encoding.TextMarshaler and encoding.TextUnmarshaler.
	want := map[netip.Addr]string{
		netip.MustParseAddr("192.168.0.100"): "carbonite",
		netip.MustParseAddr("192.168.0.101"): "obsidian",
		netip.MustParseAddr("192.168.0.102"): "diamond",
	}
	b, err := json.Marshal(&want)
	if err != nil {
		log.Fatal(err)
	}
	var got map[netip.Addr]string
	err = json.Unmarshal(b, &got)
	if err != nil {
		log.Fatal(err)
	}

	// Sanity check.
	if !reflect.DeepEqual(got, want) {
		log.Fatalf("roundtrip mismatch: got %v, want %v", got, want)
	}

	// Print the serialized JSON object. Canonicalize the JSON first since
	// Go map entries are not serialized in a deterministic order.
	(*json.RawValue)(&b).Canonicalize()
	(*json.RawValue)(&b).Indent("", "\t") // indent for readability
	fmt.Println(string(b))

	// Output:
	// {
	// 	"192.168.0.100": "carbonite",
	// 	"192.168.0.101": "obsidian",
	// 	"192.168.0.102": "diamond"
	// }
}

// By default, JSON object names for Go struct fields are derived from
// the Go field name, but may be specified in the `json` tag.
// Due to JSON's heritage in JavaScript, the most common naming convention
// used for JSON object names is camelCase.
func Example_fieldNames() {
	var value struct {
		// This field is explicitly ignored with the special "-" name.
		Ignored any `json:"-"`
		// No JSON name is not provided, so the Go field name is used.
		GoName any
		// A JSON name is provided without any special characters.
		JSONName any `json:"jsonName"`
		// No JSON name is not provided, so the Go field name is used.
		Option any `json:",nocase"`
		// An empty JSON name specified using an single-quoted string literal.
		Empty any `json:"''"`
		// A dash JSON name specified using an single-quoted string literal.
		Dash any `json:"'-'"`
		// A comma JSON name specified using an single-quoted string literal.
		Comma any `json:"','"`
		// JSON name with quotes specified using a single-quoted string literal.
		Quote any `json:"'\"\\''"`
		// An unexported field is always ignored.
		unexported any
	}

	b, err := json.Marshal(value)
	if err != nil {
		log.Fatal(err)
	}
	(*json.RawValue)(&b).Indent("", "\t") // indent for readability
	fmt.Println(string(b))

	// Output:
	// {
	// 	"GoName": null,
	// 	"jsonName": null,
	// 	"Option": null,
	// 	"": null,
	// 	"-": null,
	// 	",": null,
	// 	"\"'": null
	// }
}

// Unmarshal matches JSON object names with Go struct fields using
// a case-sensitive match, but can be configured to use a case-insensitive
// match with the "nocase" option. This permits unmarshaling from inputs that
// use naming conventions such as camelCase, snake_case, or kebab-case.
func Example_caseSensitivity() {
	// JSON input using various naming conventions.
	const input = `[
		{"firstname": true},
		{"firstName": true},
		{"FirstName": true},
		{"FIRSTNAME": true},
		{"first_name": true},
		{"FIRST_NAME": true},
		{"first-name": true},
		{"FIRST-NAME": true},
		{"unknown": true}
	]`

	// Without "nocase", Unmarshal looks for an exact match.
	var withcase []struct {
		X bool `json:"firstName"`
	}
	if err := json.Unmarshal([]byte(input), &withcase); err != nil {
		log.Fatal(err)
	}
	fmt.Println(withcase) // exactly 1 match found

	// With "nocase", Unmarshal looks first for an exact match,
	// then for a case-insensitive match if none found.
	var nocase []struct {
		X bool `json:"firstName,nocase"`
	}
	if err := json.Unmarshal([]byte(input), &nocase); err != nil {
		log.Fatal(err)
	}
	fmt.Println(nocase) // 8 matches found

	// Output:
	// [{false} {true} {false} {false} {false} {false} {false} {false} {false}]
	// [{true} {true} {true} {true} {true} {true} {true} {true} {false}]
}

// Go struct fields can be omitted from the output depending on either
// the input Go value or the output JSON encoding of the value.
// The "omitzero" option omits a field if it is the zero Go value or
// implements a "IsZero() bool" method that reports true.
// The "omitempty" option omits a field if it encodes as an empty JSON value,
// which we define as a JSON null or empty JSON string, object, or array.
// In many cases, the behavior of "omitzero" and "omitempty" are equivalent.
// If both provide the desired effect, then using "omitzero" is preferred.
func Example_omitFields() {
	type MyStruct struct {
		Foo string `json:",omitzero"`
		Bar []int  `json:",omitempty"`
		// Both "omitzero" and "omitempty" can be specified together,
		// in which case the field is omitted if either would take effect.
		// This omits the Baz field either if it is a nil pointer or
		// if it would have encoded as an empty JSON object.
		Baz *MyStruct `json:",omitzero,omitempty"`
	}

	// Demonstrate behavior of "omitzero".
	b, err := json.Marshal(struct {
		Bool         bool        `json:",omitzero"`
		Int          int         `json:",omitzero"`
		String       string      `json:",omitzero"`
		Time         time.Time   `json:",omitzero"`
		Addr         netip.Addr  `json:",omitzero"`
		Struct       MyStruct    `json:",omitzero"`
		SliceNil     []int       `json:",omitzero"`
		Slice        []int       `json:",omitzero"`
		MapNil       map[int]int `json:",omitzero"`
		Map          map[int]int `json:",omitzero"`
		PointerNil   *string     `json:",omitzero"`
		Pointer      *string     `json:",omitzero"`
		InterfaceNil any         `json:",omitzero"`
		Interface    any         `json:",omitzero"`
	}{
		// Bool is omitted since false is the zero value for a Go bool.
		Bool: false,
		// Int is omitted since 0 is the zero value for a Go int.
		Int: 0,
		// String is omitted since "" is the zero value for a Go string.
		String: "",
		// Time is omitted since time.Time.IsZero reports true.
		Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		// Addr is omitted since netip.Addr{} is the zero value for a Go struct.
		Addr: netip.Addr{},
		// Struct is NOT omitted since it is not the zero value for a Go struct.
		Struct: MyStruct{Bar: []int{}, Baz: new(MyStruct)},
		// SliceNil is omitted since nil is the zero value for a Go slice.
		SliceNil: nil,
		// Slice is NOT omitted since []int{} is not the zero value for a Go slice.
		Slice: []int{},
		// MapNil is omitted since nil is the zero value for a Go map.
		MapNil: nil,
		// Map is NOT omitted since map[int]int{} is not the zero value for a Go map.
		Map: map[int]int{},
		// PointerNil is omitted since nil is the zero value for a Go pointer.
		PointerNil: nil,
		// Pointer is NOT omitted since new(string) is not the zero value for a Go pointer.
		Pointer: new(string),
		// InterfaceNil is omitted since nil is the zero value for a Go interface.
		InterfaceNil: nil,
		// Interface is NOT omitted since (*string)(nil) is not the zero value for a Go interface.
		Interface: (*string)(nil),
	})
	if err != nil {
		log.Fatal(err)
	}
	(*json.RawValue)(&b).Indent("", "\t") // indent for readability
	fmt.Println("OmitZero:", string(b))   // outputs "Struct", "Slice", "Map", "Pointer", and "Interface"

	// Demonstrate behavior of "omitempty".
	b, err = json.Marshal(struct {
		Bool         bool        `json:",omitempty"`
		Int          int         `json:",omitempty"`
		String       string      `json:",omitempty"`
		Time         time.Time   `json:",omitempty"`
		Addr         netip.Addr  `json:",omitempty"`
		Struct       MyStruct    `json:",omitempty"`
		Slice        []int       `json:",omitempty"`
		Map          map[int]int `json:",omitempty"`
		PointerNil   *string     `json:",omitempty"`
		Pointer      *string     `json:",omitempty"`
		InterfaceNil any         `json:",omitempty"`
		Interface    any         `json:",omitempty"`
	}{
		// Bool is NOT omitted since false is not an empty JSON value.
		Bool: false,
		// Int is NOT omitted since 0 is not a empty JSON value.
		Int: 0,
		// String is omitted since "" is an empty JSON string.
		String: "",
		// Time is NOT omitted since this encodes as a non-empty JSON string.
		Time: time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		// Addr is omitted since this encodes as an empty JSON string.
		Addr: netip.Addr{},
		// Struct is omitted since {} is an empty JSON object.
		Struct: MyStruct{Bar: []int{}, Baz: new(MyStruct)},
		// Slice is omitted since [] is an empty JSON array.
		Slice: []int{},
		// Map is omitted since {} is an empty JSON object.
		Map: map[int]int{},
		// PointerNil is ommited since null is an empty JSON value.
		PointerNil: nil,
		// Pointer is omitted since "" is an empty JSON string.
		Pointer: new(string),
		// InterfaceNil is omitted since null is an empty JSON value.
		InterfaceNil: nil,
		// Interface is omitted since null is an empty JSON value.
		Interface: (*string)(nil),
	})
	if err != nil {
		log.Fatal(err)
	}
	(*json.RawValue)(&b).Indent("", "\t") // indent for readability
	fmt.Println("OmitEmpty:", string(b))  // outputs "Bool", "Int", and "Time"

	// Output:
	// OmitZero: {
	// 	"Struct": {},
	// 	"Slice": [],
	// 	"Map": {},
	// 	"Pointer": "",
	// 	"Interface": null
	// }
	// OmitEmpty: {
	// 	"Bool": false,
	// 	"Int": 0,
	// 	"Time": "0001-01-01T00:00:00Z"
	// }
}

// JSON objects can be inlined within a parent object similar to
// how Go structs can be embedded within a parent struct.
// The inlining rules are similar to those of Go embedding,
// but operates upon the JSON namespace.
func Example_inlinedFields() {
	// Base is embedded within Container.
	type Base struct {
		// ID is promoted into the JSON object for Container.
		ID string
		// Type is ignored due to presence of Container.Type.
		Type string
		// Time cancels out with Container.Inlined.Time.
		Time time.Time
	}
	// Other is embedded within Container.
	type Other struct{ Cost float64 }
	// Container embeds Base and Other.
	type Container struct {
		// Base is an embedded struct and is implicitly JSON inlined.
		Base
		// Type takes precedence over Base.Type.
		Type int
		// Inlined is a named Go field, but is explicitly JSON inlined.
		Inlined struct {
			// User is promoted into the JSON object for Container.
			User string
			// Time cancels out with Base.Time.
			Time string
		} `json:",inline"`
		// ID does not conflict with Base.ID since the JSON name is different.
		ID string `json:"uuid"`
		// Other is not JSON inlined since it has an explicit JSON name.
		Other `json:"other"`
	}

	// Format an empty Container to show what fields are JSON serializable.
	var input Container
	b, err := json.Marshal(&input)
	if err != nil {
		log.Fatal(err)
	}
	(*json.RawValue)(&b).Indent("", "\t") // indent for readability
	fmt.Println(string(b))

	// Output:
	// {
	// 	"ID": "",
	// 	"Type": 0,
	// 	"User": "",
	// 	"uuid": "",
	// 	"other": {
	// 		"Cost": 0
	// 	}
	// }
}

// Due to version skew, the set of JSON object members known at compile-time
// may differ from the set of members encountered at execution-time.
// As such, it may be useful to have finer grain handling of unknown members.
// This package supports preserving, rejecting, or discarding such members.
func Example_unknownMembers() {
	const input = `{
		"Name": "Teal",
		"Value": "#008080",
		"WebSafe": false
	}`
	type Color struct {
		Name  string
		Value string

		// Unknown is a Go struct field that holds unknown JSON object members.
		// It is marked as having this behavior with the "unknown" tag option.
		//
		// The type may be a RawValue or map[string]T.
		Unknown json.RawValue `json:",unknown"`
	}

	// By default, unknown members are stored in a Go field marked as "unknown"
	// or ignored if no such field exists.
	var color Color
	err := json.Unmarshal([]byte(input), &color)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Unknown members:", string(color.Unknown))

	// Specifying UnmarshalOptions.RejectUnknownMembers causes
	// Unmarshal to reject the presence of any unknown members.
	err = json.UnmarshalOptions{
		RejectUnknownMembers: true,
	}.Unmarshal(json.DecodeOptions{}, []byte(input), new(Color))
	if err != nil {
		fmt.Println("Unmarshal error:", errors.Unwrap(err))
	}

	// By default, Marshal preserves unknown members stored in
	// a Go struct field marked as "unknown".
	b, err := json.Marshal(color)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Output with unknown members:   ", string(b))

	// Specifying MarshalOptions.DiscardUnknownMembers causes
	// Marshal to discard any unknown members.
	b, err = json.MarshalOptions{
		DiscardUnknownMembers: true,
	}.Marshal(json.EncodeOptions{}, color)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Output without unknown members:", string(b))

	// Output:
	// Unknown members: {"WebSafe":false}
	// Unmarshal error: unknown name "WebSafe"
	// Output with unknown members:    {"Name":"Teal","Value":"#008080","WebSafe":false}
	// Output without unknown members: {"Name":"Teal","Value":"#008080"}
}

// The "format" tag option can be used to alter the formatting of certain types.
func Example_formatFlags() {
	value := struct {
		BytesBase64    []byte         `json:",format:base64"`
		BytesHex       [8]byte        `json:",format:hex"`
		BytesArray     []byte         `json:",format:array"`
		FloatNonFinite float64        `json:",format:nonfinite"`
		MapEmitNull    map[string]any `json:",format:emitnull"`
		SliceEmitNull  []any          `json:",format:emitnull"`
		TimeDateOnly   time.Time      `json:",format:'2006-01-02'"`
		DurationNanos  time.Duration  `json:",format:nanos"`
	}{
		BytesBase64:    []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
		BytesHex:       [8]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
		BytesArray:     []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
		FloatNonFinite: math.NaN(),
		MapEmitNull:    nil,
		SliceEmitNull:  nil,
		TimeDateOnly:   time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationNanos:  time.Second + time.Millisecond + time.Microsecond + time.Nanosecond,
	}

	b, err := json.Marshal(&value)
	if err != nil {
		log.Fatal(err)
	}
	(*json.RawValue)(&b).Indent("", "\t") // indent for readability
	fmt.Println(string(b))

	// Output:
	// {
	// 	"BytesBase64": "ASNFZ4mrze8=",
	// 	"BytesHex": "0123456789abcdef",
	// 	"BytesArray": [
	// 		1,
	// 		35,
	// 		69,
	// 		103,
	// 		137,
	// 		171,
	// 		205,
	// 		239
	// 	],
	// 	"FloatNonFinite": "NaN",
	// 	"MapEmitNull": null,
	// 	"SliceEmitNull": null,
	// 	"TimeDateOnly": "2000-01-01",
	// 	"DurationNanos": 1001001001
	// }
}

// When implementing HTTP endpoints, it is common to be operating with an
// io.Reader and an io.Writer. The UnmarshalFull and MarshalFull functions
// assist in operating on such input/output types.
// UnmarshalFull reads the entirety of the io.Reader to ensure that io.EOF
// is encountered without any unexpected bytes after the top-level JSON value.
func Example_serveHTTP() {
	// Some global state maintained by the server.
	var n int64

	// The "add" endpoint accepts a POST request with a JSON object
	// containing a number to atomically add to the server's global counter.
	// It returns the updated value of the counter.
	http.HandleFunc("/api/add", func(w http.ResponseWriter, r *http.Request) {
		// Unmarshal the request from the client.
		var val struct{ N int64 }
		if err := json.UnmarshalFull(r.Body, &val); err != nil {
			// Inability to unmarshal the input suggests a client-side problem.
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Marshal a response from the server.
		val.N = atomic.AddInt64(&n, val.N)
		if err := json.MarshalFull(w, &val); err != nil {
			// Inability to marshal the output suggests a server-side problem.
			// This error is not always observable by the client since
			// json.MarshalFull may have already written to the output.
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Some Go types have a custom JSON represention where the implementation
// is delegated to some external package. Consequentely, the "json" package
// will not know how to use that external implementation.
// For example, the "google.golang.org/protobuf/encoding/protojson" package
// implements JSON for all "google.golang.org/protobuf/proto".Message types.
// MarshalOptions.Marshalers and UnmarshalOptions.Unmarshalers can be used
// to configure "json" and "protojson" to cooperate together.
func Example_protoJSON() {
	// Let protoMessage be "google.golang.org/protobuf/proto".Message.
	type protoMessage interface{ ProtoReflect() }
	// Let foopbMyMessage be a concrete implementation of proto.Message.
	type foopbMyMessage struct{ protoMessage }
	// Let protojson be an import of "google.golang.org/protobuf/encoding/protojson".
	var protojson struct {
		Marshal   func(protoMessage) ([]byte, error)
		Unmarshal func([]byte, protoMessage) error
	}

	// This value mixes both non-proto.Message types and proto.Message types.
	// It should use the "json" package to handle non-proto.Message types and
	// should use the "protojson" package to handle proto.Message types.
	var value struct {
		// GoStruct does not implement proto.Message and
		// should use the default behavior of the "json" package.
		GoStruct struct {
			Name string
			Age  int
		}

		// ProtoMessage implements proto.Message and
		// should be handled using protojson.Marshal.
		ProtoMessage *foopbMyMessage
	}

	// Marshal using protojson.Marshal for proto.Message types.
	b, err := json.MarshalOptions{
		// Use protojson.Marshal as a type-specific marshaler.
		Marshalers: json.MarshalFuncV1(protojson.Marshal),
	}.Marshal(json.EncodeOptions{}, &value)
	if err != nil {
		log.Fatal(err)
	}

	// Unmarshal using protojson.Unmarshal for proto.Message types.
	err = json.UnmarshalOptions{
		// Use protojson.Unmarshal as a type-specific unmarshaler.
		Unmarshalers: json.UnmarshalFuncV1(protojson.Unmarshal),
	}.Unmarshal(json.DecodeOptions{}, b, &value)
	if err != nil {
		log.Fatal(err)
	}
}

// This example demonstrates the use of the Encoder and Decoder to
// parse and modify JSON without unmarshaling it into a concrete Go type.
func Example_stringReplace() {
	// Example input with non-idiomatic use of "Golang" instead of "Go".
	const input = `{
		"title": "Golang version 1 is released",
		"author": "Andrew Gerrand",
		"date": "2012-03-28",
		"text": "Today marks a major milestone in the development of the Golang programming language.",
		"otherArticles": [
			"Twelve Years of Golang",
			"The Laws of Reflection",
			"Learn Golang from your browser"
		]
	}`

	// Using a Decoder and Encoder, we can parse through every token,
	// check and modify the token if necessary, and
	// write the token to the output.
	var replacements []string
	in := strings.NewReader(input)
	dec := json.NewDecoder(in)
	out := new(bytes.Buffer)
	enc := json.EncodeOptions{Indent: "\t"}.NewEncoder(out) // indent for readability
	for {
		// Read a token from the input.
		tok, err := dec.ReadToken()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}

		// Check whether the token contains the string "Golang" and
		// replace each occurence with "Go" instead.
		if tok.Kind() == '"' && strings.Contains(tok.String(), "Golang") {
			replacements = append(replacements, dec.StackPointer())
			tok = json.String(strings.ReplaceAll(tok.String(), "Golang", "Go"))
		}

		// Write the (possibly modified) token to the output.
		if err := enc.WriteToken(tok); err != nil {
			log.Fatal(err)
		}
	}

	// Print the list of replacements and the adjusted JSON output.
	if len(replacements) > 0 {
		fmt.Println(`Replaced "Golang" with "Go" in:`)
		for _, where := range replacements {
			fmt.Println("\t" + where)
		}
		fmt.Println()
	}
	fmt.Println("Result:", out.String())

	// Output:
	// Replaced "Golang" with "Go" in:
	// 	/title
	// 	/text
	// 	/otherArticles/0
	// 	/otherArticles/2
	//
	// Result: {
	// 	"title": "Go version 1 is released",
	// 	"author": "Andrew Gerrand",
	// 	"date": "2012-03-28",
	// 	"text": "Today marks a major milestone in the development of the Go programming language.",
	// 	"otherArticles": [
	// 		"Twelve Years of Go",
	// 		"The Laws of Reflection",
	// 		"Learn Go from your browser"
	// 	]
	// }
}

// Directly embedding JSON within HTML requires special handling for safety.
// Escape certain runes to prevent JSON directly treated as HTML
// from being able to perform <script> injection.
//
// This example shows how to obtain equivalent behavior provided by the
// "encoding/json" package that is no longer directly supported by this package.
// Newly written code that intermix JSON and HTML should instead be using the
// "github.com/google/safehtml" module for safety purposes.
func ExampleEncodeOptions_escapeHTML() {
	page := struct {
		Title string
		Body  string
	}{
		Title: "Example Embedded Javascript",
		Body:  `<script> console.log("Hello, world!"); </script>`,
	}

	b, err := json.MarshalOptions{}.Marshal(json.EncodeOptions{
		// Escape certain runes within a JSON string so that
		// JSON will be safe to directly embed inside HTML.
		EscapeRune: func(r rune) bool {
			switch r {
			case '&', '<', '>', '\u2028', '\u2029':
				return true
			default:
				return false
			}
		},
		// Indent the output for readability.
		Indent: "\t",
	}, &page)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))

	// Output:
	// {
	// 	"Title": "Example Embedded Javascript",
	// 	"Body": "\u003cscript\u003e console.log(\"Hello, world!\"); \u003c/script\u003e"
	// }
}

// Many error types are not serializable since they tend to be Go structs
// without any exported fields (e.g., errors constructed with errors.New).
// Some applications, may desire to marshal an error as a JSON string
// even if these errors cannot be unmarshaled.
func ExampleMarshalOptions_errors() {
	// Response to serialize with some Go errors encountered.
	response := []struct {
		Result string `json:",omitzero"`
		Error  error  `json:",omitzero"`
	}{
		{Result: "Oranges are a good source of Vitamin C."},
		{Error: &strconv.NumError{Func: "ParseUint", Num: "-1234", Err: strconv.ErrSyntax}},
		{Error: &os.PathError{Op: "ReadFile", Path: "/path/to/secret/file", Err: os.ErrPermission}},
	}

	b, err := json.MarshalOptions{
		// Intercept every attempt to marshal an error type.
		Marshalers: json.NewMarshalers(
			// Suppose we consider strconv.NumError to be a safe to serialize:
			// this type-specific marshal function intercepts this type
			// and encodes the error message as a JSON string.
			json.MarshalFuncV2(func(opts json.MarshalOptions, enc *json.Encoder, err *strconv.NumError) error {
				return enc.WriteToken(json.String(err.Error()))
			}),
			// Error messages may contain sensitive information that may not
			// be appropriate to serialize. For all errors not handled above,
			// report some generic error message.
			json.MarshalFuncV1(func(error) ([]byte, error) {
				return []byte(`"internal server error"`), nil
			}),
		),
	}.Marshal(json.EncodeOptions{
		Indent: "\t", // indent for readability
	}, &response)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))

	// Output:
	// [
	// 	{
	// 		"Result": "Oranges are a good source of Vitamin C."
	// 	},
	// 	{
	// 		"Error": "strconv.ParseUint: parsing \"-1234\": invalid syntax"
	// 	},
	// 	{
	// 		"Error": "internal server error"
	// 	}
	// ]
}

// In some applications, the exact precision of JSON numbers needs to be
// preserved when unmarshaling. This can be accomplished using a type-specific
// unmarshal function that intercepts all any types and pre-populates the
// interface value with a RawValue, which can represent a JSON number exactly.
func ExampleUnmarshalOptions_rawNumber() {
	// Input with JSON numbers beyond the representation of a float64.
	const input = `[false, 1e-1000, 3.141592653589793238462643383279, 1e+1000, true]`

	var value any
	err := json.UnmarshalOptions{
		// Intercept every attempt to unmarshal into the any type.
		Unmarshalers: json.UnmarshalFuncV2(func(opts json.UnmarshalOptions, dec *json.Decoder, val *any) error {
			// If the next value to be decoded is a JSON number,
			// then provide a concrete Go type to unmarshal into.
			if dec.PeekKind() == '0' {
				*val = json.RawValue(nil)
			}
			// Return SkipFunc to fallback on default unmarshal behavior.
			return json.SkipFunc
		}),
	}.Unmarshal(json.DecodeOptions{}, []byte(input), &value)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(value)

	// Sanity check.
	want := []any{false, json.RawValue("1e-1000"), json.RawValue("3.141592653589793238462643383279"), json.RawValue("1e+1000"), true}
	if !reflect.DeepEqual(value, want) {
		log.Fatalf("value mismatch:\ngot  %v\nwant %v", value, want)
	}

	// Output:
	// [false 1e-1000 3.141592653589793238462643383279 1e+1000 true]
}

// When using JSON for parsing configuration files,
// the parsing logic often needs to report an error with a line and column
// indicating where in the input an error occurred.
func ExampleUnmarshalOptions_recordOffsets() {
	// Hypothetical configuration file.
	const input = `[
		{"Source": "192.168.0.100:1234", "Destination": "192.168.0.1:80"},
		{"Source": "192.168.0.251:4004"},
		{"Source": "192.168.0.165:8080", "Destination": "0.0.0.0:80"}
	]`
	type Tunnel struct {
		Source      netip.AddrPort
		Destination netip.AddrPort

		// ByteOffset is populated during unmarshal with the byte offset
		// within the JSON input of the JSON object for this Go struct.
		ByteOffset int64 `json:"-"` // metadata to be ignored for JSON serialization
	}

	var tunnels []Tunnel
	err := json.UnmarshalOptions{
		// Intercept every attempt to unmarshal into the Tunnel type.
		Unmarshalers: json.UnmarshalFuncV2(func(opts json.UnmarshalOptions, dec *json.Decoder, tunnel *Tunnel) error {
			// Decoder.InputOffset reports the offset after the last token,
			// but we want to record the offset before the next token.
			//
			// Call Decoder.PeekKind to buffer enough to reach the next token.
			// Add the number of leading whitespace, commas, and colons
			// to locate the start of the next token.
			dec.PeekKind()
			unread := dec.UnreadBuffer()
			n := len(unread) - len(bytes.TrimLeft(unread, " \n\r\t,:"))
			tunnel.ByteOffset = dec.InputOffset() + int64(n)

			// Return SkipFunc to fallback on default unmarshal behavior.
			return json.SkipFunc
		}),
	}.Unmarshal(json.DecodeOptions{}, []byte(input), &tunnels)
	if err != nil {
		log.Fatal(err)
	}

	// lineColumn converts a byte offset into a one-indexed line and column.
	// The offset must be within the bounds of the input.
	lineColumn := func(input string, offset int) (line, column int) {
		line = 1 + strings.Count(input[:offset], "\n")
		column = 1 + offset - (strings.LastIndex(input[:offset], "\n") + len("\n"))
		return line, column
	}

	// Verify that the configuration file is valid.
	for _, tunnel := range tunnels {
		if !tunnel.Source.IsValid() || !tunnel.Destination.IsValid() {
			line, column := lineColumn(input, int(tunnel.ByteOffset))
			fmt.Printf("%d:%d: source and destination must both be specified", line, column)
		}
	}

	// Output:
	// 3:3: source and destination must both be specified
}

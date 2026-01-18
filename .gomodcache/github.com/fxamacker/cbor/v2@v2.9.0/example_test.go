// Copyright (c) Faye Amacker. All rights reserved.
// Licensed under the MIT License. See LICENSE in the project root for license information.

package cbor_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/fxamacker/cbor/v2" // remove "/v2" suffix if you're not using Go modules (see README.md)
)

func ExampleMarshal() {
	type Animal struct {
		Age    int
		Name   string
		Owners []string
		Male   bool
	}
	animal := Animal{Age: 4, Name: "Candy", Owners: []string{"Mary", "Joe"}}
	b, err := cbor.Marshal(animal)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", b)
	// Output:
	// a46341676504644e616d656543616e6479664f776e65727382644d617279634a6f65644d616c65f4
}

func ExampleMarshal_time() {
	tm, _ := time.Parse(time.RFC3339, "2013-03-21T20:04:00Z")

	// Encode time as string in RFC3339 format with second precision.
	em, err := cbor.EncOptions{Time: cbor.TimeRFC3339}.EncMode()
	if err != nil {
		fmt.Println("error:", err)
	}
	b, err := em.Marshal(tm)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", b)

	// Encode time as numerical representation of seconds since January 1, 1970 UTC.
	em, err = cbor.EncOptions{Time: cbor.TimeUnix}.EncMode()
	if err != nil {
		fmt.Println("error:", err)
	}
	b, err = em.Marshal(tm)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", b)
	// Output:
	// 74323031332d30332d32315432303a30343a30305a
	// 1a514b67b0
}

// This example uses Marshal to encode struct and map in canonical form.
func ExampleMarshal_canonical() {
	type Animal struct {
		Age      int
		Name     string
		Contacts map[string]string
		Male     bool
	}
	animal := Animal{Age: 4, Name: "Candy", Contacts: map[string]string{"Mary": "111-111-1111", "Joe": "222-222-2222"}}
	em, err := cbor.CanonicalEncOptions().EncMode()
	if err != nil {
		fmt.Println("error:", err)
	}
	b, err := em.Marshal(animal)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", b)
	// Output:
	// a46341676504644d616c65f4644e616d656543616e647968436f6e7461637473a2634a6f656c3232322d3232322d32323232644d6172796c3131312d3131312d31313131
}

// This example uses "toarray" struct tag option to encode struct as CBOR array.
func ExampleMarshal_toarray() {
	type Record struct {
		_           struct{} `cbor:",toarray"`
		Name        string
		Unit        string
		Measurement int
	}
	rec := Record{Name: "current", Unit: "V", Measurement: 1}
	b, err := cbor.Marshal(rec)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", b)
	// Output:
	// 836763757272656e74615601
}

// This example uses "keyasint" struct tag option to encode struct's field names as integer.
// This feature is very useful in handling COSE, CWT, SenML data.
func ExampleMarshal_keyasint() {
	type Record struct {
		Name        string `cbor:"1,keyasint"`
		Unit        string `cbor:"2,keyasint"`
		Measurement int    `cbor:"3,keyasint"`
	}
	rec := Record{Name: "current", Unit: "V", Measurement: 1}
	b, err := cbor.Marshal(rec)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", b)
	// Output:
	// a3016763757272656e740261560301
}

func ExampleUnmarshal() {
	type Animal struct {
		Age    int
		Name   string
		Owners []string
		Male   bool
	}
	data, _ := hex.DecodeString("a46341676504644e616d656543616e6479664f776e65727382644d617279634a6f65644d616c65f4")
	var animal Animal
	err := cbor.Unmarshal(data, &animal)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v", animal)
	// Output:
	// {Age:4 Name:Candy Owners:[Mary Joe] Male:false}
}

func ExampleUnmarshal_time() {
	cborRFC3339Time, _ := hex.DecodeString("74323031332d30332d32315432303a30343a30305a")
	tm := time.Time{}
	if err := cbor.Unmarshal(cborRFC3339Time, &tm); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v\n", tm.UTC().Format(time.RFC3339Nano))
	cborUnixTime, _ := hex.DecodeString("1a514b67b0")
	tm = time.Time{}
	if err := cbor.Unmarshal(cborUnixTime, &tm); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v\n", tm.UTC().Format(time.RFC3339Nano))
	// Output:
	// 2013-03-21T20:04:00Z
	// 2013-03-21T20:04:00Z
}

func ExampleEncoder() {
	type Animal struct {
		Age    int
		Name   string
		Owners []string
		Male   bool
	}
	animals := []Animal{
		{Age: 4, Name: "Candy", Owners: []string{"Mary", "Joe"}, Male: false},
		{Age: 6, Name: "Rudy", Owners: []string{"Cindy"}, Male: true},
		{Age: 2, Name: "Duke", Owners: []string{"Norton"}, Male: true},
	}
	var buf bytes.Buffer
	em, err := cbor.CanonicalEncOptions().EncMode()
	if err != nil {
		fmt.Println("error:", err)
	}
	enc := em.NewEncoder(&buf)
	for _, animal := range animals {
		err := enc.Encode(animal)
		if err != nil {
			fmt.Println("error:", err)
		}
	}
	fmt.Printf("%x\n", buf.Bytes())
	// Output:
	// a46341676504644d616c65f4644e616d656543616e6479664f776e65727382644d617279634a6f65a46341676506644d616c65f5644e616d656452756479664f776e657273816543696e6479a46341676502644d616c65f5644e616d656444756b65664f776e65727381664e6f72746f6e
}

// ExampleEncoder_indefiniteLengthByteString encodes a stream of definite
// length byte string ("chunks") as an indefinite length byte string.
func ExampleEncoder_indefiniteLengthByteString() {
	var buf bytes.Buffer
	encoder := cbor.NewEncoder(&buf)
	// Start indefinite length byte string encoding.
	if err := encoder.StartIndefiniteByteString(); err != nil {
		fmt.Println("error:", err)
	}
	// Encode definite length byte string.
	if err := encoder.Encode([]byte{1, 2}); err != nil {
		fmt.Println("error:", err)
	}
	// Encode definite length byte string.
	if err := encoder.Encode([3]byte{3, 4, 5}); err != nil {
		fmt.Println("error:", err)
	}
	// Close indefinite length byte string.
	if err := encoder.EndIndefinite(); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", buf.Bytes())
	// Output:
	// 5f42010243030405ff
}

// ExampleEncoder_indefiniteLengthTextString encodes a stream of definite
// length text string ("chunks") as an indefinite length text string.
func ExampleEncoder_indefiniteLengthTextString() {
	var buf bytes.Buffer
	encoder := cbor.NewEncoder(&buf)
	// Start indefinite length text string encoding.
	if err := encoder.StartIndefiniteTextString(); err != nil {
		fmt.Println("error:", err)
	}
	// Encode definite length text string.
	if err := encoder.Encode("strea"); err != nil {
		fmt.Println("error:", err)
	}
	// Encode definite length text string.
	if err := encoder.Encode("ming"); err != nil {
		fmt.Println("error:", err)
	}
	// Close indefinite length text string.
	if err := encoder.EndIndefinite(); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", buf.Bytes())
	// Output:
	// 7f657374726561646d696e67ff
}

// ExampleEncoder_indefiniteLengthArray encodes a stream of elements as an
// indefinite length array.  Encoder supports nested indefinite length values.
func ExampleEncoder_indefiniteLengthArray() {
	var buf bytes.Buffer
	enc := cbor.NewEncoder(&buf)
	// Start indefinite length array encoding.
	if err := enc.StartIndefiniteArray(); err != nil {
		fmt.Println("error:", err)
	}
	// Encode array element.
	if err := enc.Encode(1); err != nil {
		fmt.Println("error:", err)
	}
	// Encode array element.
	if err := enc.Encode([]int{2, 3}); err != nil {
		fmt.Println("error:", err)
	}
	// Start a nested indefinite length array as array element.
	if err := enc.StartIndefiniteArray(); err != nil {
		fmt.Println("error:", err)
	}
	// Encode nested array element.
	if err := enc.Encode(4); err != nil {
		fmt.Println("error:", err)
	}
	// Encode nested array element.
	if err := enc.Encode(5); err != nil {
		fmt.Println("error:", err)
	}
	// Close nested indefinite length array.
	if err := enc.EndIndefinite(); err != nil {
		fmt.Println("error:", err)
	}
	// Close outer indefinite length array.
	if err := enc.EndIndefinite(); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", buf.Bytes())
	// Output:
	// 9f018202039f0405ffff
}

// ExampleEncoder_indefiniteLengthMap encodes a stream of elements as an
// indefinite length map.  Encoder supports nested indefinite length values.
func ExampleEncoder_indefiniteLengthMap() {
	var buf bytes.Buffer
	em, err := cbor.EncOptions{Sort: cbor.SortCanonical}.EncMode()
	if err != nil {
		fmt.Println("error:", err)
	}
	enc := em.NewEncoder(&buf)
	// Start indefinite length map encoding.
	if err := enc.StartIndefiniteMap(); err != nil {
		fmt.Println("error:", err)
	}
	// Encode map key.
	if err := enc.Encode("a"); err != nil {
		fmt.Println("error:", err)
	}
	// Encode map value.
	if err := enc.Encode(1); err != nil {
		fmt.Println("error:", err)
	}
	// Encode map key.
	if err := enc.Encode("b"); err != nil {
		fmt.Println("error:", err)
	}
	// Start an indefinite length array as map value.
	if err := enc.StartIndefiniteArray(); err != nil {
		fmt.Println("error:", err)
	}
	// Encoded array element.
	if err := enc.Encode(2); err != nil {
		fmt.Println("error:", err)
	}
	// Encoded array element.
	if err := enc.Encode(3); err != nil {
		fmt.Println("error:", err)
	}
	// Close indefinite length array.
	if err := enc.EndIndefinite(); err != nil {
		fmt.Println("error:", err)
	}
	// Close indefinite length map.
	if err := enc.EndIndefinite(); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%x\n", buf.Bytes())
	// Output:
	// bf61610161629f0203ffff
}

func ExampleDecoder() {
	type Animal struct {
		Age    int
		Name   string
		Owners []string
		Male   bool
	}
	data, _ := hex.DecodeString("a46341676504644d616c65f4644e616d656543616e6479664f776e65727382644d617279634a6f65a46341676506644d616c65f5644e616d656452756479664f776e657273816543696e6479a46341676502644d616c65f5644e616d656444756b65664f776e65727381664e6f72746f6e")
	dec := cbor.NewDecoder(bytes.NewReader(data))
	for {
		var animal Animal
		if err := dec.Decode(&animal); err != nil {
			if err != io.EOF {
				fmt.Println("error:", err)
			}
			break
		}
		fmt.Printf("%+v\n", animal)
	}
	// Output:
	// {Age:4 Name:Candy Owners:[Mary Joe] Male:false}
	// {Age:6 Name:Rudy Owners:[Cindy] Male:true}
	// {Age:2 Name:Duke Owners:[Norton] Male:true}
}

func Example_cWT() {
	// Use "keyasint" struct tag option to encode/decode struct to/from CBOR map.
	type claims struct {
		Iss string `cbor:"1,keyasint"`
		Sub string `cbor:"2,keyasint"`
		Aud string `cbor:"3,keyasint"`
		Exp int    `cbor:"4,keyasint"`
		Nbf int    `cbor:"5,keyasint"`
		Iat int    `cbor:"6,keyasint"`
		Cti []byte `cbor:"7,keyasint"`
	}
	// Data from https://tools.ietf.org/html/rfc8392#appendix-A section A.1
	data, _ := hex.DecodeString("a70175636f61703a2f2f61732e6578616d706c652e636f6d02656572696b77037818636f61703a2f2f6c696768742e6578616d706c652e636f6d041a5612aeb0051a5610d9f0061a5610d9f007420b71")
	var v claims
	if err := cbor.Unmarshal(data, &v); err != nil {
		fmt.Println("error:", err)
	}
	if _, err := cbor.Marshal(v); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v", v)
	// Output:
	// {Iss:coap://as.example.com Sub:erikw Aud:coap://light.example.com Exp:1444064944 Nbf:1443944944 Iat:1443944944 Cti:[11 113]}
}

func Example_cWTWithDupMapKeyOption() {
	type claims struct {
		Iss string `cbor:"1,keyasint"`
		Sub string `cbor:"2,keyasint"`
		Aud string `cbor:"3,keyasint"`
		Exp int    `cbor:"4,keyasint"`
		Nbf int    `cbor:"5,keyasint"`
		Iat int    `cbor:"6,keyasint"`
		Cti []byte `cbor:"7,keyasint"`
	}

	// Data from https://tools.ietf.org/html/rfc8392#appendix-A section A.1
	data, _ := hex.DecodeString("a70175636f61703a2f2f61732e6578616d706c652e636f6d02656572696b77037818636f61703a2f2f6c696768742e6578616d706c652e636f6d041a5612aeb0051a5610d9f0061a5610d9f007420b71")

	dm, _ := cbor.DecOptions{DupMapKey: cbor.DupMapKeyEnforcedAPF}.DecMode()

	var v claims
	if err := dm.Unmarshal(data, &v); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v", v)
	// Output:
	// {Iss:coap://as.example.com Sub:erikw Aud:coap://light.example.com Exp:1444064944 Nbf:1443944944 Iat:1443944944 Cti:[11 113]}
}

func Example_signedCWT() {
	// Use "keyasint" struct tag option to encode/decode struct to/from CBOR map.
	// Partial COSE header definition
	type coseHeader struct {
		Alg int    `cbor:"1,keyasint,omitempty"`
		Kid []byte `cbor:"4,keyasint,omitempty"`
		IV  []byte `cbor:"5,keyasint,omitempty"`
	}
	// Use "toarray" struct tag option to encode/decode struct to/from CBOR array.
	type signedCWT struct {
		_           struct{} `cbor:",toarray"`
		Protected   []byte
		Unprotected coseHeader
		Payload     []byte
		Signature   []byte
	}
	// Data from https://tools.ietf.org/html/rfc8392#appendix-A section A.3
	data, _ := hex.DecodeString("d28443a10126a104524173796d6d657472696345434453413235365850a70175636f61703a2f2f61732e6578616d706c652e636f6d02656572696b77037818636f61703a2f2f6c696768742e6578616d706c652e636f6d041a5612aeb0051a5610d9f0061a5610d9f007420b7158405427c1ff28d23fbad1f29c4c7c6a555e601d6fa29f9179bc3d7438bacaca5acd08c8d4d4f96131680c429a01f85951ecee743a52b9b63632c57209120e1c9e30")
	var v signedCWT
	if err := cbor.Unmarshal(data, &v); err != nil {
		fmt.Println("error:", err)
	}
	if _, err := cbor.Marshal(v); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v", v)
	// Output:
	// {_:{} Protected:[161 1 38] Unprotected:{Alg:0 Kid:[65 115 121 109 109 101 116 114 105 99 69 67 68 83 65 50 53 54] IV:[]} Payload:[167 1 117 99 111 97 112 58 47 47 97 115 46 101 120 97 109 112 108 101 46 99 111 109 2 101 101 114 105 107 119 3 120 24 99 111 97 112 58 47 47 108 105 103 104 116 46 101 120 97 109 112 108 101 46 99 111 109 4 26 86 18 174 176 5 26 86 16 217 240 6 26 86 16 217 240 7 66 11 113] Signature:[84 39 193 255 40 210 63 186 209 242 156 76 124 106 85 94 96 29 111 162 159 145 121 188 61 116 56 186 202 202 90 205 8 200 212 212 249 97 49 104 12 66 154 1 248 89 81 236 238 116 58 82 185 182 54 50 197 114 9 18 14 28 158 48]}
}

func Example_signedCWTWithTag() {
	// Use "keyasint" struct tag option to encode/decode struct to/from CBOR map.
	// Partial COSE header definition
	type coseHeader struct {
		Alg int    `cbor:"1,keyasint,omitempty"`
		Kid []byte `cbor:"4,keyasint,omitempty"`
		IV  []byte `cbor:"5,keyasint,omitempty"`
	}
	// Use "toarray" struct tag option to encode/decode struct to/from CBOR array.
	type signedCWT struct {
		_           struct{} `cbor:",toarray"`
		Protected   []byte
		Unprotected coseHeader
		Payload     []byte
		Signature   []byte
	}

	// Data from https://tools.ietf.org/html/rfc8392#appendix-A section A.3
	data, _ := hex.DecodeString("d28443a10126a104524173796d6d657472696345434453413235365850a70175636f61703a2f2f61732e6578616d706c652e636f6d02656572696b77037818636f61703a2f2f6c696768742e6578616d706c652e636f6d041a5612aeb0051a5610d9f0061a5610d9f007420b7158405427c1ff28d23fbad1f29c4c7c6a555e601d6fa29f9179bc3d7438bacaca5acd08c8d4d4f96131680c429a01f85951ecee743a52b9b63632c57209120e1c9e30")

	// Register tag COSE_Sign1 18 with signedCWT type.
	tags := cbor.NewTagSet()
	if err := tags.Add(
		cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
		reflect.TypeOf(signedCWT{}),
		18); err != nil {
		fmt.Println("error:", err)
	}

	dm, _ := cbor.DecOptions{}.DecModeWithTags(tags)
	em, _ := cbor.EncOptions{}.EncModeWithTags(tags)

	var v signedCWT
	if err := dm.Unmarshal(data, &v); err != nil {
		fmt.Println("error:", err)
	}

	if _, err := em.Marshal(v); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v", v)
	// Output:
	// {_:{} Protected:[161 1 38] Unprotected:{Alg:0 Kid:[65 115 121 109 109 101 116 114 105 99 69 67 68 83 65 50 53 54] IV:[]} Payload:[167 1 117 99 111 97 112 58 47 47 97 115 46 101 120 97 109 112 108 101 46 99 111 109 2 101 101 114 105 107 119 3 120 24 99 111 97 112 58 47 47 108 105 103 104 116 46 101 120 97 109 112 108 101 46 99 111 109 4 26 86 18 174 176 5 26 86 16 217 240 6 26 86 16 217 240 7 66 11 113] Signature:[84 39 193 255 40 210 63 186 209 242 156 76 124 106 85 94 96 29 111 162 159 145 121 188 61 116 56 186 202 202 90 205 8 200 212 212 249 97 49 104 12 66 154 1 248 89 81 236 238 116 58 82 185 182 54 50 197 114 9 18 14 28 158 48]}
}

func Example_cOSE() {
	// Use "keyasint" struct tag option to encode/decode struct to/from CBOR map.
	// Use cbor.RawMessage to delay unmarshaling (CrvOrNOrK's data type depends on Kty's value).
	type coseKey struct {
		Kty       int             `cbor:"1,keyasint,omitempty"`
		Kid       []byte          `cbor:"2,keyasint,omitempty"`
		Alg       int             `cbor:"3,keyasint,omitempty"`
		KeyOpts   int             `cbor:"4,keyasint,omitempty"`
		IV        []byte          `cbor:"5,keyasint,omitempty"`
		CrvOrNOrK cbor.RawMessage `cbor:"-1,keyasint,omitempty"` // K for symmetric keys, Crv for elliptic curve keys, N for RSA modulus
		XOrE      cbor.RawMessage `cbor:"-2,keyasint,omitempty"` // X for curve x-coordinate, E for RSA public exponent
		Y         cbor.RawMessage `cbor:"-3,keyasint,omitempty"` // Y for curve y-coordinate
		D         []byte          `cbor:"-4,keyasint,omitempty"`
	}
	// Data from https://tools.ietf.org/html/rfc8392#appendix-A section A.2
	// 128-Bit Symmetric Key
	data, _ := hex.DecodeString("a42050231f4c4d4d3051fdc2ec0a3851d5b3830104024c53796d6d6574726963313238030a")
	var v coseKey
	if err := cbor.Unmarshal(data, &v); err != nil {
		fmt.Println("error:", err)
	}
	if _, err := cbor.Marshal(v); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v", v)
	// Output:
	// {Kty:4 Kid:[83 121 109 109 101 116 114 105 99 49 50 56] Alg:10 KeyOpts:0 IV:[] CrvOrNOrK:[80 35 31 76 77 77 48 81 253 194 236 10 56 81 213 179 131] XOrE:[] Y:[] D:[]}
}

func Example_senML() {
	// Use "keyasint" struct tag option to encode/decode struct to/from CBOR map.
	type SenMLRecord struct {
		BaseName    string  `cbor:"-2,keyasint,omitempty"`
		BaseTime    float64 `cbor:"-3,keyasint,omitempty"`
		BaseUnit    string  `cbor:"-4,keyasint,omitempty"`
		BaseValue   float64 `cbor:"-5,keyasint,omitempty"`
		BaseSum     float64 `cbor:"-6,keyasint,omitempty"`
		BaseVersion int     `cbor:"-1,keyasint,omitempty"`
		Name        string  `cbor:"0,keyasint,omitempty"`
		Unit        string  `cbor:"1,keyasint,omitempty"`
		Time        float64 `cbor:"6,keyasint,omitempty"`
		UpdateTime  float64 `cbor:"7,keyasint,omitempty"`
		Value       float64 `cbor:"2,keyasint,omitempty"`
		ValueS      string  `cbor:"3,keyasint,omitempty"`
		ValueB      bool    `cbor:"4,keyasint,omitempty"`
		ValueD      string  `cbor:"8,keyasint,omitempty"`
		Sum         float64 `cbor:"5,keyasint,omitempty"`
	}
	// Data from https://tools.ietf.org/html/rfc8428#section-6
	data, _ := hex.DecodeString("87a721781b75726e3a6465763a6f773a3130653230373361303130383030363a22fb41d303a15b00106223614120050067766f6c7461676501615602fb405e066666666666a3006763757272656e74062402fb3ff3333333333333a3006763757272656e74062302fb3ff4cccccccccccda3006763757272656e74062202fb3ff6666666666666a3006763757272656e74062102f93e00a3006763757272656e74062002fb3ff999999999999aa3006763757272656e74060002fb3ffb333333333333")
	var v []*SenMLRecord
	if err := cbor.Unmarshal(data, &v); err != nil {
		fmt.Println("error:", err)
	}
	// Encoder uses ShortestFloat16 option to use float16 as the shortest form that preserves floating-point value.
	em, err := cbor.EncOptions{ShortestFloat: cbor.ShortestFloat16}.EncMode()
	if err != nil {
		fmt.Println("error:", err)
	}
	if _, err := em.Marshal(v); err != nil {
		fmt.Println("error:", err)
	}
	for _, rec := range v {
		fmt.Printf("%+v\n", *rec)
	}
	// Output:
	// {BaseName:urn:dev:ow:10e2073a0108006: BaseTime:1.276020076001e+09 BaseUnit:A BaseValue:0 BaseSum:0 BaseVersion:5 Name:voltage Unit:V Time:0 UpdateTime:0 Value:120.1 ValueS: ValueB:false ValueD: Sum:0}
	// {BaseName: BaseTime:0 BaseUnit: BaseValue:0 BaseSum:0 BaseVersion:0 Name:current Unit: Time:-5 UpdateTime:0 Value:1.2 ValueS: ValueB:false ValueD: Sum:0}
	// {BaseName: BaseTime:0 BaseUnit: BaseValue:0 BaseSum:0 BaseVersion:0 Name:current Unit: Time:-4 UpdateTime:0 Value:1.3 ValueS: ValueB:false ValueD: Sum:0}
	// {BaseName: BaseTime:0 BaseUnit: BaseValue:0 BaseSum:0 BaseVersion:0 Name:current Unit: Time:-3 UpdateTime:0 Value:1.4 ValueS: ValueB:false ValueD: Sum:0}
	// {BaseName: BaseTime:0 BaseUnit: BaseValue:0 BaseSum:0 BaseVersion:0 Name:current Unit: Time:-2 UpdateTime:0 Value:1.5 ValueS: ValueB:false ValueD: Sum:0}
	// {BaseName: BaseTime:0 BaseUnit: BaseValue:0 BaseSum:0 BaseVersion:0 Name:current Unit: Time:-1 UpdateTime:0 Value:1.6 ValueS: ValueB:false ValueD: Sum:0}
	// {BaseName: BaseTime:0 BaseUnit: BaseValue:0 BaseSum:0 BaseVersion:0 Name:current Unit: Time:0 UpdateTime:0 Value:1.7 ValueS: ValueB:false ValueD: Sum:0}
}

func Example_webAuthn() {
	// Use cbor.RawMessage to delay unmarshaling (AttStmt's data type depends on Fmt's value).
	type attestationObject struct {
		AuthnData []byte          `cbor:"authData"`
		Fmt       string          `cbor:"fmt"`
		AttStmt   cbor.RawMessage `cbor:"attStmt"`
	}
	data, _ := hex.DecodeString("a363666d74686669646f2d7532666761747453746d74a26373696758483046022100e7ab373cfbd99fcd55fd59b0f6f17fef5b77a20ddec3db7f7e4d55174e366236022100828336b4822125fb56541fb14a8a273876acd339395ec2dad95cf41c1dd2a9ae637835638159024e3082024a30820132a0030201020204124a72fe300d06092a864886f70d01010b0500302e312c302a0603550403132359756269636f2055324620526f6f742043412053657269616c203435373230303633313020170d3134303830313030303030305a180f32303530303930343030303030305a302c312a302806035504030c2159756269636f205532462045452053657269616c203234393431343937323135383059301306072a8648ce3d020106082a8648ce3d030107034200043d8b1bbd2fcbf6086e107471601468484153c1c6d3b4b68a5e855e6e40757ee22bcd8988bf3befd7cdf21cb0bf5d7a150d844afe98103c6c6607d9faae287c02a33b3039302206092b0601040182c40a020415312e332e362e312e342e312e34313438322e312e313013060b2b0601040182e51c020101040403020520300d06092a864886f70d01010b05000382010100a14f1eea0076f6b8476a10a2be72e60d0271bb465b2dfbfc7c1bd12d351989917032631d795d097fa30a26a325634e85721bc2d01a86303f6bc075e5997319e122148b0496eec8d1f4f94cf4110de626c289443d1f0f5bbb239ca13e81d1d5aa9df5af8e36126475bfc23af06283157252762ff68879bcf0ef578d55d67f951b4f32b63c8aea5b0f99c67d7d814a7ff5a6f52df83e894a3a5d9c8b82e7f8bc8daf4c80175ff8972fda79333ec465d806eacc948f1bab22045a95558a48c20226dac003d41fbc9e05ea28a6bb5e10a49de060a0a4f6a2676a34d68c4abe8c61874355b9027e828ca9e064b002d62e8d8cf0744921753d35e3c87c5d5779453e7768617574684461746158c449960de5880e8c687434170f6476605b8fe4aeb9a28632c7995cf3ba831d976341000000000000000000000000000000000000000000408903fd7dfd2c9770e98cae0123b13a2c27828a106349bc6277140e7290b7e9eb7976aa3c04ed347027caf7da3a2fa76304751c02208acfc4e7fc6c7ebbc375c8a5010203262001215820ad7f7992c335b90d882b2802061b97a4fabca7e2ee3e7a51e728b8055e4eb9c7225820e0966ba7005987fece6f0e0e13447aa98cec248e4000a594b01b74c1cb1d40b3")
	var v attestationObject
	if err := cbor.Unmarshal(data, &v); err != nil {
		fmt.Println("error:", err)
	}
	if _, err := cbor.Marshal(v); err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v", v)
}

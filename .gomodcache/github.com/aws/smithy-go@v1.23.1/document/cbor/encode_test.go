package cbor

import (
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/aws/smithy-go/encoding/cbor"
	"github.com/aws/smithy-go/ptr"
)

func TestEncode_KitchenSink(t *testing.T) {
	type subtarget struct {
		Int8  int8
		Int16 int16
	}
	type target struct {
		Int8          int8
		Int16         int16
		Int32         int32
		Int64         int64
		Uint8         uint8
		Uint16        uint16
		Uint32        uint32
		Uint64        uint64
		String        string
		List          []subtarget
		Map           map[string]subtarget
		UintptrNil    *uint
		UintptrNonnil *uint
		Bool          bool
		Float         float64
		BigInt        *big.Int
		BigNegInt     *big.Int
	}

	in := target{
		Int8:   -8,
		Int16:  -16,
		Int32:  -32,
		Int64:  -64,
		Uint8:  8,
		Uint16: 16,
		Uint32: 32,
		Uint64: 64,
		String: "foo",
		List: []subtarget{
			{Int8: -8},
		},
		Map: map[string]subtarget{
			"k0": {Int8: -8},
		},
		UintptrNil:    nil,
		UintptrNonnil: ptr.Uint(4),
		Bool:          true,
		Float:         math.Inf(1),
		BigInt:        new(big.Int).SetBytes([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0}),
		BigNegInt: new(big.Int).Sub(
			big.NewInt(-1),
			new(big.Int).SetBytes([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0}),
		),
	}

	expect := cbor.Map{
		"Int8":   cbor.NegInt(8),
		"Int16":  cbor.NegInt(16),
		"Int32":  cbor.NegInt(32),
		"Int64":  cbor.NegInt(64),
		"Uint8":  cbor.Uint(8),
		"Uint16": cbor.Uint(16),
		"Uint32": cbor.Uint(32),
		"Uint64": cbor.Uint(64),
		"String": cbor.String("foo"),
		"List": cbor.List{
			cbor.Map{
				"Int8":  cbor.NegInt(8),
				"Int16": cbor.Uint(0), // implicit
			},
		},
		"Map": cbor.Map{
			"k0": cbor.Map{
				"Int8":  cbor.NegInt(8),
				"Int16": cbor.Uint(0),
			},
		},
		"UintptrNil":    &cbor.Nil{},
		"UintptrNonnil": cbor.Uint(4),
		"Bool":          cbor.Bool(true),
		"Float":         cbor.Float64(math.Inf(1)),
		"BigInt": &cbor.Tag{
			ID:    2,
			Value: cbor.Slice{1, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		"BigNegInt": &cbor.Tag{
			ID:    3,
			Value: cbor.Slice{1, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}

	enc := &encoder{}
	encoded, err := enc.Encode(in)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := cbor.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(expect, actual) {
		t.Errorf("%v != %v", expect, actual)
	}
}

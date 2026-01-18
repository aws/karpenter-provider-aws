package cbor

import (
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/aws/smithy-go/encoding/cbor"
	"github.com/aws/smithy-go/ptr"
)

func TestDecode_KitchenSink(t *testing.T) {
	type target struct {
		Int8   int8
		Int16  int16
		Int32  int32
		Int64  int64
		Uint8  uint8
		Uint16 uint16
		Uint32 uint32
		Uint64 uint64

		Slice  []byte
		String string

		List []target
		Map  map[string]target

		UintptrNil    *uint
		UintptrNonnil *uint
		Bool          bool
		Float         float64

		BigInt    *big.Int
		BigNegInt *big.Int
		BigFloat  *big.Float
	}

	in := cbor.Map{
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
				"Int8": cbor.NegInt(8),
			},
		},
		"Map": cbor.Map{
			"k0": cbor.Map{
				"Int8": cbor.NegInt(8),
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
		"BigFloat": &cbor.Tag{
			ID: 4,
			Value: cbor.List{
				cbor.NegInt(200), // exp
				cbor.Uint(200),   // mant
			},
		},

		"UnknownField": &cbor.Nil{},
	}

	expect := target{
		Int8:   -8,
		Int16:  -16,
		Int32:  -32,
		Int64:  -64,
		Uint8:  8,
		Uint16: 16,
		Uint32: 32,
		Uint64: 64,

		String: "foo",

		List: []target{
			{Int8: -8},
		},
		Map: map[string]target{
			"k0": {Int8: -8},
		},

		UintptrNil:    nil,
		UintptrNonnil: ptr.Uint(4),
		Bool:          true,
		Float:         math.Inf(1),

		BigInt: new(big.Int).SetBytes([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0}),
		BigNegInt: new(big.Int).Sub(
			big.NewInt(-1),
			new(big.Int).SetBytes([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0}),
		),
		BigFloat: func() *big.Float {
			x, _ := new(big.Float).SetString("200e-200")
			return x
		}(),
	}

	var actual target
	dec := &decoder{}
	if err := dec.Decode(in, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expect, actual) {
		t.Errorf("%v != %v", expect, actual)
	}
}

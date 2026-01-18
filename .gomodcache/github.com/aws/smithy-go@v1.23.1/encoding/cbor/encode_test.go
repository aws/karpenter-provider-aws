package cbor

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"
)

func TestEncode_Atomic(t *testing.T) {
	for name, c := range map[string]struct {
		Expect []byte
		In     Value
	}{
		"uint/0/min": {
			[]byte{0<<5 | 0},
			Uint(0),
		},
		"uint/0/max": {
			[]byte{0<<5 | 23},
			Uint(23),
		},
		"uint/1/min": {
			[]byte{0<<5 | 24, 24},
			Uint(24),
		},
		"uint/1/max": {
			[]byte{0<<5 | 24, 0xff},
			Uint(0xff),
		},
		"uint/2/min": {
			[]byte{0<<5 | 25, 1, 0},
			Uint(0x100),
		},
		"uint/2/max": {
			[]byte{0<<5 | 25, 0xff, 0xff},
			Uint(0xffff),
		},
		"uint/4/min": {
			[]byte{0<<5 | 26, 1, 0, 0, 0},
			Uint(0x1000000),
		},
		"uint/4/max": {
			[]byte{0<<5 | 26, 0xff, 0xff, 0xff, 0xff},
			Uint(0xffffffff),
		},
		"uint/8/min": {
			[]byte{0<<5 | 27, 1, 0, 0, 0, 0, 0, 0, 0},
			Uint(0x1000000_00000000),
		},
		"uint/8/max": {
			[]byte{0<<5 | 27, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			Uint(0xffffffff_ffffffff),
		},
		"negint/0/min": {
			[]byte{1<<5 | 0},
			NegInt(1),
		},
		"negint/0/max": {
			[]byte{1<<5 | 23},
			NegInt(24),
		},
		"negint/1/min": {
			[]byte{1<<5 | 24, 24},
			NegInt(25),
		},
		"negint/1/max": {
			[]byte{1<<5 | 24, 0xff},
			NegInt(0x100),
		},
		"negint/2/min": {
			[]byte{1<<5 | 25, 1, 0},
			NegInt(0x101),
		},
		"negint/2/max": {
			[]byte{1<<5 | 25, 0xff, 0xff},
			NegInt(0x10000),
		},
		"negint/4/min": {
			[]byte{1<<5 | 26, 1, 0, 0, 0},
			NegInt(0x1000001),
		},
		"negint/4/max": {
			[]byte{1<<5 | 26, 0xff, 0xff, 0xff, 0xff},
			NegInt(0x100000000),
		},
		"negint/8/min": {
			[]byte{1<<5 | 27, 1, 0, 0, 0, 0, 0, 0, 0},
			NegInt(0x1000000_00000001),
		},
		"negint/8/max": {
			[]byte{1<<5 | 27, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe},
			NegInt(0xffffffff_ffffffff),
		},
		"true": {
			[]byte{7<<5 | major7True},
			Bool(true),
		},
		"false": {
			[]byte{7<<5 | major7False},
			Bool(false),
		},
		"null": {
			[]byte{7<<5 | major7Nil},
			&Nil{},
		},
		"undefined": {
			[]byte{7<<5 | major7Undefined},
			&Undefined{},
		},
		"float32": {
			[]byte{7<<5 | major7Float32, 0x7f, 0x80, 0, 0},
			Float32(math.Float32frombits(0x7f800000)),
		},
		"float64": {
			[]byte{7<<5 | major7Float64, 0x7f, 0xf0, 0, 0, 0, 0, 0, 0},
			Float64(math.Float64frombits(0x7ff00000_00000000)),
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := Encode(c.In)
			if !bytes.Equal(c.Expect, actual) {
				t.Errorf("bytes not equal (%s != %s)", hex.EncodeToString(c.Expect), hex.EncodeToString(actual))
			}
		})
	}
}

func TestEncode_Slice(t *testing.T) {
	for name, c := range map[string]struct {
		Expect []byte
		In     Value
	}{
		"len = 0": {
			[]byte{2<<5 | 0},
			Slice{},
		},
		"len > 0": {
			[]byte{2<<5 | 3, 0x66, 0x6f, 0x6f},
			Slice{0x66, 0x6f, 0x6f},
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := Encode(c.In)
			if !bytes.Equal(c.Expect, actual) {
				t.Errorf("bytes not equal (%s != %s)", hex.EncodeToString(c.Expect), hex.EncodeToString(actual))
			}
		})
	}
}

func TestEncode_String(t *testing.T) {
	for name, c := range map[string]struct {
		Expect []byte
		In     Value
	}{
		"len = 0": {
			[]byte{3<<5 | 0},
			String(""),
		},
		"len > 0": {
			[]byte{3<<5 | 3, 0x66, 0x6f, 0x6f},
			String("foo"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := Encode(c.In)
			if !bytes.Equal(c.Expect, actual) {
				t.Errorf("bytes not equal (%s != %s)", hex.EncodeToString(c.Expect), hex.EncodeToString(actual))
			}
		})
	}
}

func TestEncode_List(t *testing.T) {
	for name, c := range map[string]struct {
		Expect []byte
		In     Value
	}{
		"[uint/0/min]": {
			withDefiniteList([]byte{0<<5 | 0}),
			List{Uint(0)},
		},
		"[uint/0/max]": {
			withDefiniteList([]byte{0<<5 | 23}),
			List{Uint(23)},
		},
		"[uint/1/min]": {
			withDefiniteList([]byte{0<<5 | 24, 24}),
			List{Uint(24)},
		},
		"[uint/1/max]": {
			withDefiniteList([]byte{0<<5 | 24, 0xff}),
			List{Uint(0xff)},
		},
		"[uint/2/min]": {
			withDefiniteList([]byte{0<<5 | 25, 1, 0}),
			List{Uint(0x100)},
		},
		"[uint/2/max]": {
			withDefiniteList([]byte{0<<5 | 25, 0xff, 0xff}),
			List{Uint(0xffff)},
		},
		"[uint/4/min]": {
			withDefiniteList([]byte{0<<5 | 26, 1, 0, 0, 0}),
			List{Uint(0x1000000)},
		},
		"[uint/4/max]": {
			withDefiniteList([]byte{0<<5 | 26, 0xff, 0xff, 0xff, 0xff}),
			List{Uint(0xffffffff)},
		},
		"[uint/8/min]": {
			withDefiniteList([]byte{0<<5 | 27, 1, 0, 0, 0, 0, 0, 0, 0}),
			List{Uint(0x1000000_00000000)},
		},
		"[uint/8/max]": {
			withDefiniteList([]byte{0<<5 | 27, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}),
			List{Uint(0xffffffff_ffffffff)},
		},
		"[negint/0/min]": {
			withDefiniteList([]byte{1<<5 | 0}),
			List{NegInt(1)},
		},
		"[negint/0/max]": {
			withDefiniteList([]byte{1<<5 | 23}),
			List{NegInt(24)},
		},
		"[negint/1/min]": {
			withDefiniteList([]byte{1<<5 | 24, 24}),
			List{NegInt(25)},
		},
		"[negint/1/max]": {
			withDefiniteList([]byte{1<<5 | 24, 0xff}),
			List{NegInt(0x100)},
		},
		"[negint/2/min]": {
			withDefiniteList([]byte{1<<5 | 25, 1, 0}),
			List{NegInt(0x101)},
		},
		"[negint/2/max]": {
			withDefiniteList([]byte{1<<5 | 25, 0xff, 0xff}),
			List{NegInt(0x10000)},
		},
		"[negint/4/min]": {
			withDefiniteList([]byte{1<<5 | 26, 1, 0, 0, 0}),
			List{NegInt(0x1000001)},
		},
		"[negint/4/max]": {
			withDefiniteList([]byte{1<<5 | 26, 0xff, 0xff, 0xff, 0xff}),
			List{NegInt(0x100000000)},
		},
		"[negint/8/min]": {
			withDefiniteList([]byte{1<<5 | 27, 1, 0, 0, 0, 0, 0, 0, 0}),
			List{NegInt(0x1000000_00000001)},
		},
		"[negint/8/max]": {
			withDefiniteList([]byte{1<<5 | 27, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe}),
			List{NegInt(0xffffffff_ffffffff)},
		},
		"[true]": {
			withDefiniteList([]byte{7<<5 | major7True}),
			List{Bool(true)},
		},
		"[false]": {
			withDefiniteList([]byte{7<<5 | major7False}),
			List{Bool(false)},
		},
		"[null]": {
			withDefiniteList([]byte{7<<5 | major7Nil}),
			List{&Nil{}},
		},
		"[undefined]": {
			withDefiniteList([]byte{7<<5 | major7Undefined}),
			List{&Undefined{}},
		},
		"[float32]": {
			withDefiniteList([]byte{7<<5 | major7Float32, 0x7f, 0x80, 0, 0}),
			List{Float32(math.Float32frombits(0x7f800000))},
		},
		"[float64]": {
			withDefiniteList([]byte{7<<5 | major7Float64, 0x7f, 0xf0, 0, 0, 0, 0, 0, 0}),
			List{Float64(math.Float64frombits(0x7ff00000_00000000))},
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := Encode(c.In)
			if !bytes.Equal(c.Expect, actual) {
				t.Errorf("bytes not equal (%s != %s)", hex.EncodeToString(c.Expect), hex.EncodeToString(actual))
			}
		})
	}
}

func TestEncode_Map(t *testing.T) {
	for name, c := range map[string]struct {
		Expect []byte
		In     Value
	}{
		"{uint/0/min}": {
			withDefiniteMap([]byte{0<<5 | 0}),
			Map{"foo": Uint(0)},
		},
		"{uint/0/max}": {
			withDefiniteMap([]byte{0<<5 | 23}),
			Map{"foo": Uint(23)},
		},
		"{uint/1/min}": {
			withDefiniteMap([]byte{0<<5 | 24, 24}),
			Map{"foo": Uint(24)},
		},
		"{uint/1/max}": {
			withDefiniteMap([]byte{0<<5 | 24, 0xff}),
			Map{"foo": Uint(0xff)},
		},
		"{uint/2/min}": {
			withDefiniteMap([]byte{0<<5 | 25, 1, 0}),
			Map{"foo": Uint(0x100)},
		},
		"{uint/2/max}": {
			withDefiniteMap([]byte{0<<5 | 25, 0xff, 0xff}),
			Map{"foo": Uint(0xffff)},
		},
		"{uint/4/min}": {
			withDefiniteMap([]byte{0<<5 | 26, 1, 0, 0, 0}),
			Map{"foo": Uint(0x1000000)},
		},
		"{uint/4/max}": {
			withDefiniteMap([]byte{0<<5 | 26, 0xff, 0xff, 0xff, 0xff}),
			Map{"foo": Uint(0xffffffff)},
		},
		"{uint/8/min}": {
			withDefiniteMap([]byte{0<<5 | 27, 1, 0, 0, 0, 0, 0, 0, 0}),
			Map{"foo": Uint(0x1000000_00000000)},
		},
		"{uint/8/max}": {
			withDefiniteMap([]byte{0<<5 | 27, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}),
			Map{"foo": Uint(0xffffffff_ffffffff)},
		},
		"{negint/0/min}": {
			withDefiniteMap([]byte{1<<5 | 0}),
			Map{"foo": NegInt(1)},
		},
		"{negint/0/max}": {
			withDefiniteMap([]byte{1<<5 | 23}),
			Map{"foo": NegInt(24)},
		},
		"{negint/1/min}": {
			withDefiniteMap([]byte{1<<5 | 24, 24}),
			Map{"foo": NegInt(25)},
		},
		"{negint/1/max}": {
			withDefiniteMap([]byte{1<<5 | 24, 0xff}),
			Map{"foo": NegInt(0x100)},
		},
		"{negint/2/min}": {
			withDefiniteMap([]byte{1<<5 | 25, 1, 0}),
			Map{"foo": NegInt(0x101)},
		},
		"{negint/2/max}": {
			withDefiniteMap([]byte{1<<5 | 25, 0xff, 0xff}),
			Map{"foo": NegInt(0x10000)},
		},
		"{negint/4/min}": {
			withDefiniteMap([]byte{1<<5 | 26, 1, 0, 0, 0}),
			Map{"foo": NegInt(0x1000001)},
		},
		"{negint/4/max}": {
			withDefiniteMap([]byte{1<<5 | 26, 0xff, 0xff, 0xff, 0xff}),
			Map{"foo": NegInt(0x100000000)},
		},
		"{negint/8/min}": {
			withDefiniteMap([]byte{1<<5 | 27, 1, 0, 0, 0, 0, 0, 0, 0}),
			Map{"foo": NegInt(0x1000000_00000001)},
		},
		"{negint/8/max}": {
			withDefiniteMap([]byte{1<<5 | 27, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xfe}),
			Map{"foo": NegInt(0xffffffff_ffffffff)},
		},
		"{true}": {
			withDefiniteMap([]byte{7<<5 | major7True}),
			Map{"foo": Bool(true)},
		},
		"{false}": {
			withDefiniteMap([]byte{7<<5 | major7False}),
			Map{"foo": Bool(false)},
		},
		"{null}": {
			withDefiniteMap([]byte{7<<5 | major7Nil}),
			Map{"foo": &Nil{}},
		},
		"{undefined}": {
			withDefiniteMap([]byte{7<<5 | major7Undefined}),
			Map{"foo": &Undefined{}},
		},
		"{float32}": {
			withDefiniteMap([]byte{7<<5 | major7Float32, 0x7f, 0x80, 0, 0}),
			Map{"foo": Float32(math.Float32frombits(0x7f800000))},
		},
		"{float64}": {
			withDefiniteMap([]byte{7<<5 | major7Float64, 0x7f, 0xf0, 0, 0, 0, 0, 0, 0}),
			Map{"foo": Float64(math.Float64frombits(0x7ff00000_00000000))},
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := Encode(c.In)
			if !bytes.Equal(c.Expect, actual) {
				t.Errorf("bytes not equal (%s != %s)", hex.EncodeToString(c.Expect), hex.EncodeToString(actual))
			}
		})
	}
}

func TestEncode_Tag(t *testing.T) {
	for name, c := range map[string]struct {
		Expect []byte
		In     Value
	}{
		"0/min": {
			[]byte{6<<5 | 0, 1},
			&Tag{0, Uint(1)},
		},
		"0/max": {
			[]byte{6<<5 | 23, 1},
			&Tag{23, Uint(1)},
		},
		"1/min": {
			[]byte{6<<5 | 24, 24, 1},
			&Tag{24, Uint(1)},
		},
		"1/max": {
			[]byte{6<<5 | 24, 0xff, 1},
			&Tag{0xff, Uint(1)},
		},
		"2/min": {
			[]byte{6<<5 | 25, 1, 0, 1},
			&Tag{0x100, Uint(1)},
		},
		"2/max": {
			[]byte{6<<5 | 25, 0xff, 0xff, 1},
			&Tag{0xffff, Uint(1)},
		},
		"4/min": {
			[]byte{6<<5 | 26, 1, 0, 0, 0, 1},
			&Tag{0x1000000, Uint(1)},
		},
		"4/max": {
			[]byte{6<<5 | 26, 0xff, 0xff, 0xff, 0xff, 1},
			&Tag{0xffffffff, Uint(1)},
		},
		"8/min": {
			[]byte{6<<5 | 27, 1, 0, 0, 0, 0, 0, 0, 0, 1},
			&Tag{0x1000000_00000000, Uint(1)},
		},
		"8/max": {
			[]byte{6<<5 | 27, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 1},
			&Tag{0xffffffff_ffffffff, Uint(1)},
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := Encode(c.In)
			if !bytes.Equal(c.Expect, actual) {
				t.Errorf("bytes not equal (%s != %s)", hex.EncodeToString(c.Expect), hex.EncodeToString(actual))
			}
		})
	}
}

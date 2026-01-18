package cbor

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestAsInt8(t *testing.T) {
	const maxv = 0x7f
	for name, c := range map[string]struct {
		In     Value
		Expect int8
		Err    string
	}{
		"wrong type": {
			In:  String(""),
			Err: "unexpected value type cbor.String",
		},
		"uint oob": {
			In:  Uint(maxv + 1),
			Err: fmt.Sprintf("cbor uint %d exceeds", maxv+1),
		},
		"negint oob": {
			In:  NegInt(maxv + 2),
			Err: fmt.Sprintf("cbor negint %s exceeds", fmtNegint(NegInt(maxv+2))),
		},
		"negint wrap oob": {
			In:  NegInt(0),
			Err: "cbor negint -2^64 exceeds",
		},
		"uint ok min": {
			In:     Uint(0),
			Expect: 0,
		},
		"uint ok max": {
			In:     Uint(maxv),
			Expect: maxv,
		},
		"negint ok min": {
			In:     NegInt(1),
			Expect: -1,
		},
		"negint ok max": {
			In:     NegInt(maxv + 1),
			Expect: -maxv - 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := AsInt8(c.In)
			if c.Err == "" {
				if err != nil {
					t.Fatalf("expect no err, got %v", err)
				}
				if actual != c.Expect {
					t.Fatalf("%v != %v", c.Expect, actual)
				}
			} else {
				if err == nil {
					t.Fatalf("expect err %v", err)
				}
				if !strings.Contains(err.Error(), c.Err) {
					t.Fatalf("'%v' does not contain '%s'", err, c.Err)
				}
			}
		})
	}
}

func TestAsInt16(t *testing.T) {
	const maxv = 0x7fff
	for name, c := range map[string]struct {
		In     Value
		Expect int16
		Err    string
	}{
		"wrong type": {
			In:  String(""),
			Err: "unexpected value type cbor.String",
		},
		"uint oob": {
			In:  Uint(maxv + 1),
			Err: fmt.Sprintf("cbor uint %d exceeds", maxv+1),
		},
		"negint oob": {
			In:  NegInt(maxv + 2),
			Err: fmt.Sprintf("cbor negint %s exceeds", fmtNegint(NegInt(maxv+2))),
		},
		"negint wrap oob": {
			In:  NegInt(0),
			Err: "cbor negint -2^64 exceeds",
		},
		"uint ok min": {
			In:     Uint(0),
			Expect: 0,
		},
		"uint ok max": {
			In:     Uint(maxv),
			Expect: maxv,
		},
		"negint ok min": {
			In:     NegInt(1),
			Expect: -1,
		},
		"negint ok max": {
			In:     NegInt(maxv + 1),
			Expect: -maxv - 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := AsInt16(c.In)
			if c.Err == "" {
				if err != nil {
					t.Fatalf("expect no err, got %v", err)
				}
				if actual != c.Expect {
					t.Fatalf("%v != %v", c.Expect, actual)
				}
			} else {
				if err == nil {
					t.Fatalf("expect err %v", err)
				}
				if !strings.Contains(err.Error(), c.Err) {
					t.Fatalf("'%v' does not contain '%s'", err, c.Err)
				}
			}
		})
	}
}

func TestAsInt32(t *testing.T) {
	const maxv = 0x7fffffff
	for name, c := range map[string]struct {
		In     Value
		Expect int32
		Err    string
	}{
		"wrong type": {
			In:  String(""),
			Err: "unexpected value type cbor.String",
		},
		"uint oob": {
			In:  Uint(maxv + 1),
			Err: fmt.Sprintf("cbor uint %d exceeds", Uint(maxv+1)),
		},
		"negint oob": {
			In:  NegInt(maxv + 2),
			Err: fmt.Sprintf("cbor negint %s exceeds", fmtNegint(NegInt(maxv+2))),
		},
		"negint wrap oob": {
			In:  NegInt(0),
			Err: "cbor negint -2^64 exceeds",
		},
		"uint ok min": {
			In:     Uint(0),
			Expect: 0,
		},
		"uint ok max": {
			In:     Uint(maxv),
			Expect: maxv,
		},
		"negint ok min": {
			In:     NegInt(1),
			Expect: -1,
		},
		"negint ok max": {
			In:     NegInt(maxv + 1),
			Expect: -maxv - 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := AsInt32(c.In)
			if c.Err == "" {
				if err != nil {
					t.Fatalf("expect no err, got %v", err)
				}
				if actual != c.Expect {
					t.Fatalf("%v != %v", c.Expect, actual)
				}
			} else {
				if err == nil {
					t.Fatalf("expect err %v", err)
				}
				if !strings.Contains(err.Error(), c.Err) {
					t.Fatalf("'%v' does not contain '%s'", err, c.Err)
				}
			}
		})
	}
}

func TestAsInt64(t *testing.T) {
	const maxv = 0x7fffffff_ffffffff
	for name, c := range map[string]struct {
		In     Value
		Expect int64
		Err    string
	}{
		"wrong type": {
			In:  String(""),
			Err: "unexpected value type cbor.String",
		},
		"uint oob": {
			In:  Uint(uint64(maxv) + 1),
			Err: fmt.Sprintf("cbor uint %d exceeds", uint64(maxv)+1),
		},
		"negint oob": {
			In:  NegInt(uint64(maxv) + 2),
			Err: fmt.Sprintf("cbor negint %s exceeds", fmtNegint(NegInt(uint64(maxv)+2))),
		},
		"negint wrap oob": {
			In:  NegInt(0),
			Err: "cbor negint -2^64 exceeds",
		},
		"uint ok min": {
			In:     Uint(0),
			Expect: 0,
		},
		"uint ok max": {
			In:     Uint(maxv),
			Expect: maxv,
		},
		"negint ok min": {
			In:     NegInt(1),
			Expect: -1,
		},
		"negint ok max": {
			In:     NegInt(maxv + 1),
			Expect: -maxv - 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := AsInt64(c.In)
			if c.Err == "" {
				if err != nil {
					t.Fatalf("expect no err, got %v", err)
				}
				if actual != c.Expect {
					t.Fatalf("%v != %v", c.Expect, actual)
				}
			} else {
				if err == nil {
					t.Fatalf("expect err %v", err)
				}
				if !strings.Contains(err.Error(), c.Err) {
					t.Fatalf("'%v' does not contain '%s'", err, c.Err)
				}
			}
		})
	}
}

func TestAsFloat32(t *testing.T) {
	const maxv = 1 << 24
	for name, c := range map[string]struct {
		In     Value
		Expect float32
		Err    string
	}{
		"wrong type": {
			In:  String(""),
			Err: "unexpected value type cbor.String",
		},
		"uint oob": {
			In:  Uint(maxv + 1),
			Err: fmt.Sprintf("cbor uint %d exceeds", maxv+1),
		},
		"negint oob": {
			In:  NegInt(maxv + 2),
			Err: fmt.Sprintf("cbor negint %s exceeds", fmtNegint(NegInt(maxv+2))),
		},
		"negint wrap oob": {
			In:  NegInt(0),
			Err: "cbor negint -2^64 exceeds",
		},
		"uint ok min": {
			In:     Uint(0),
			Expect: 0,
		},
		"uint ok max": {
			In:     Uint(maxv),
			Expect: maxv,
		},
		"negint ok min": {
			In:     NegInt(1),
			Expect: -1,
		},
		"negint ok max": {
			In:     NegInt(maxv),
			Expect: -maxv,
		},
		"direct": {
			In:     Float32(0.5),
			Expect: 0.5,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := AsFloat32(c.In)
			if c.Err == "" {
				if err != nil {
					t.Fatalf("expect no err, got %v", err)
				}
				if actual != c.Expect {
					t.Fatalf("%v != %v", c.Expect, actual)
				}
			} else {
				if err == nil {
					t.Fatalf("expect err %v", err)
				}
				if !strings.Contains(err.Error(), c.Err) {
					t.Fatalf("'%v' does not contain '%s'", err, c.Err)
				}
			}
		})
	}
}

func TestAsFloat64(t *testing.T) {
	const maxv = 1 << 54
	for name, c := range map[string]struct {
		In     Value
		Expect float64
		Err    string
	}{
		"wrong type": {
			In:  String(""),
			Err: "unexpected value type cbor.String",
		},
		"uint oob": {
			In:  Uint(maxv + 1),
			Err: fmt.Sprintf("cbor uint %d exceeds", Uint(maxv+1)),
		},
		"negint oob": {
			In:  NegInt(maxv + 2),
			Err: fmt.Sprintf("cbor negint %s exceeds", fmtNegint(NegInt(maxv+2))),
		},
		"negint wrap oob": {
			In:  NegInt(0),
			Err: "cbor negint -2^64 exceeds",
		},
		"uint ok min": {
			In:     Uint(0),
			Expect: 0,
		},
		"uint ok max": {
			In:     Uint(maxv),
			Expect: maxv,
		},
		"negint ok min": {
			In:     NegInt(1),
			Expect: -1,
		},
		"negint ok max": {
			In:     NegInt(maxv),
			Expect: -maxv,
		},
		"float32": {
			In:     Float32(0.5),
			Expect: 0.5,
		},
		"direct": {
			In:     Float64(0.5),
			Expect: 0.5,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := AsFloat64(c.In)
			if c.Err == "" {
				if err != nil {
					t.Fatalf("expect no err, got %v", err)
				}
				if actual != c.Expect {
					t.Fatalf("%v != %v", c.Expect, actual)
				}
			} else {
				if err == nil {
					t.Fatalf("expect err %v", err)
				}
				if !strings.Contains(err.Error(), c.Err) {
					t.Fatalf("'%v' does not contain '%s'", err, c.Err)
				}
			}
		})
	}
}

func TestAsTime(t *testing.T) {
	for name, c := range map[string]struct {
		In     Value
		Expect time.Time
		Err    string
	}{
		"wrong type": {
			In:  String(""),
			Err: "unexpected value type cbor.String",
		},
		"wrong tag": {
			In:  &Tag{ID: 2},
			Err: "unexpected tag ID 2",
		},
		"wrong tag value": {
			In:  &Tag{ID: 1, Value: String("")},
			Err: "coerce tag value: unexpected value type cbor.String",
		},
		"no tag value": {
			In:  &Tag{ID: 1},
			Err: "coerce tag value: unexpected value type <nil>",
		},
		"negint": {
			In:     &Tag{ID: 1, Value: Uint(4)},
			Expect: time.UnixMilli(4000),
		},
		"float32": {
			In:     &Tag{ID: 1, Value: Float32(3.997)},
			Expect: time.UnixMilli(3997),
		},
		"float64": {
			In:     &Tag{ID: 1, Value: Float64(3.997)},
			Expect: time.UnixMilli(3997),
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := AsTime(c.In)
			if c.Err == "" {
				if err != nil {
					t.Fatalf("expect no err, got %v", err)
				}
				if actual != c.Expect {
					t.Fatalf("%v != %v", c.Expect, actual)
				}
			} else {
				if err == nil {
					t.Fatalf("expect err %v", err)
				}
				if !strings.Contains(err.Error(), c.Err) {
					t.Fatalf("'%v' does not contain '%s'", err, c.Err)
				}
			}
		})
	}
}

func TestAsBigInt(t *testing.T) {
	for name, c := range map[string]struct {
		In     Value
		Expect *big.Int
		Err    string
	}{
		"wrong type": {
			In:  String(""),
			Err: "unexpected value type cbor.String",
		},
		"wrong tag": {
			In:  &Tag{ID: 1},
			Err: "unexpected tag ID 1",
		},
		"wrong tag value": {
			In:  &Tag{ID: 2, Value: String("")},
			Err: "unexpected tag value type cbor.String",
		},
		"uint min": {
			In:     Uint(0),
			Expect: big.NewInt(0),
		},
		"uint max": {
			In: Uint(0xffffffff_ffffffff),
			Expect: new(big.Int).SetBytes(
				[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			),
		},
		"negint min": {
			In:     NegInt(1),
			Expect: big.NewInt(-1),
		},
		"negint max": {
			In: NegInt(0),
			Expect: func() *big.Int {
				i := new(big.Int).SetBytes(
					[]byte{1, 0, 0, 0, 0, 0, 0, 0, 0},
				)
				return i.Neg(i)
			}(),
		},
		"tag 2": {
			In: &Tag{
				ID:    2,
				Value: Slice{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			},
			Expect: new(big.Int).SetBytes(
				[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			),
		},
		"tag 3": {
			In: &Tag{
				ID:    3,
				Value: Slice{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			},
			Expect: func() *big.Int {
				i := new(big.Int).SetBytes(
					[]byte{1, 0, 0, 0, 0, 0, 0, 0, 0},
				)
				return i.Neg(i)
			}(),
		},
		"nil": {
			In:     &Nil{},
			Expect: nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := AsBigInt(c.In)
			if c.Err == "" {
				if err != nil {
					t.Fatalf("expect no err, got %v", err)
				}
				if c.Expect.Cmp(actual) != 0 {
					t.Fatalf("%v != %v", c.Expect, actual)
				}
			} else {
				if err == nil {
					t.Fatalf("expect err %v", err)
				}
				if !strings.Contains(err.Error(), c.Err) {
					t.Fatalf("'%v' does not contain '%s'", err, c.Err)
				}
			}
		})
	}
}

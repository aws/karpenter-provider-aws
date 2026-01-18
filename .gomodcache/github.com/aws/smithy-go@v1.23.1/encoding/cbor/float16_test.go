package cbor

import (
	"testing"
)

func TestFloat16To32(t *testing.T) {
	for name, c := range map[string]struct {
		In     uint16
		Expect uint32
	}{
		"+infinity": {
			0b0_11111_0000000000,
			0b0_11111111_00000000000000000000000,
		},
		"-infinity": {
			0b1_11111_0000000000,
			0b1_11111111_00000000000000000000000,
		},
		"NaN": {
			0b0_11111_0101010101,
			0b0_11111111_01010101010000000000000,
		},
		"absolute zero": {0, 0},
		"subnormal": {
			0b0_00000_0001010000,
			0b0_01101101_01000000000000000000000,
		},
		"normal": {
			0b0_00001_0001010000,
			0b0_0001110001_00010100000000000000000,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if actual := float16to32(c.In); c.Expect != actual {
				t.Errorf("%x != %x", c.Expect, actual)
			}
		})
	}

}

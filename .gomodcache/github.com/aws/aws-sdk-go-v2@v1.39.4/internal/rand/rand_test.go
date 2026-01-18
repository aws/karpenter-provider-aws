package rand

import (
	"bytes"
	"io"
	"testing"
)

func TestGitterDelay(t *testing.T) {
	maxFloat1 := 1 - 1/float64(1<<53)

	cases := map[string]struct {
		Reader io.Reader
		Expect float64
	}{
		"floor": {
			Reader: bytes.NewReader([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}),
			Expect: 0,
		},
		"ceiling": {
			Reader: bytes.NewReader([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}),
			Expect: maxFloat1,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			d, err := Float64(c.Reader)
			if err != nil {
				t.Fatalf("expect no error, %v", err)
			}

			if e, a := c.Expect, d; e != a {
				t.Errorf("expect %v delay, got %v", e, a)
			}
		})
	}
}

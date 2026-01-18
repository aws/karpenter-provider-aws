//go:build fuzz
// +build fuzz

package cbor

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
)

// caught by fuzz:
// - broken typecast from uint64 to int when checking encoded string(mt2,3) length vs buflen
// - huge encoded list/map sizes would cause panics on make()
// - map declaration at end of buffer would attempt to peek p[0] when len(p) == 0

func TestDecode_Fuzz(t *testing.T) {
	const runs = 1_000_000
	const buflen = 512

	p := make([]byte, buflen)

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(hex.EncodeToString(p))
			dump(p)

			t.Fatalf("decode panic: %v\n", err)
		}
	}()

	for i := 0; i < runs; i++ {
		if _, err := rand.Read(p); err != nil {
			t.Fatalf("create randbuf: %v", err)
		}

		decode(p)
	}
}

func dump(p []byte) {
	for len(p) > 0 {
		var off int

		major, minor := peekMajor(p), peekMinor(p)
		switch major {
		case majorTypeUint, majorTypeNegInt, majorType7:
			if minor > 27 {
				fmt.Printf("%d, %d (invalid)\n", major, minor)
				return
			}

			arg, n, err := decodeArgument(p)
			if err != nil {
				panic(err)
			}

			fmt.Printf("%d, %d\n", major, arg)
			off = n
		case majorTypeSlice, majorTypeString:
			if minor == 31 {
				panic("todo")
			} else if minor > 27 {
				fmt.Printf("%d, %d (invalid)\n", major, minor)
				return
			}

			arg, n, err := decodeArgument(p)
			if err != nil {
				panic(err)
			}

			fmt.Printf("str(%d), len %d\n", major, arg)
			off = n + int(arg)
		case majorTypeList, majorTypeMap:
			if minor == 31 {
				panic("todo")
			} else if minor > 27 {
				fmt.Printf("%d, %d (invalid)\n", major, minor)
				return
			}

			arg, n, err := decodeArgument(p)
			if err != nil {
				panic(err)
			}

			fmt.Printf("container(%d), len %d\n", major, arg)
			off = n
		case majorTypeTag:
			if minor > 27 {
				fmt.Printf("tag, %d (invalid)\n", minor)
				return
			}

			arg, n, err := decodeArgument(p)
			if err != nil {
				panic(err)
			}

			fmt.Printf("tag, %d\n", arg)
			off = n
		}

		if off > len(p) {
			fmt.Println("overflow, stop")
			return
		}
		p = p[off:]
	}

	fmt.Println("EOF")
}

package rulesfn

import (
	"testing"
	"github.com/aws/smithy-go/ptr"
)

func TestSubString(t *testing.T) {
	cases := map[string]struct {
		input       string
		start, stop int
		reverse     bool
		expect      *string
	}{
		"prefix": {
			input: "abcde", start: 0, stop: 3, reverse: false,
			expect: ptr.String("abc"),
		},
		"prefix max-ascii": {
			input: "abcde\u007F", start: 0, stop: 3, reverse: false,
			expect: ptr.String("abc"),
		},
		"suffix reverse": {
			input: "abcde", start: 0, stop: 3, reverse: true,
			expect: ptr.String("cde"),
		},
		"too long": {
			input: "ab", start: 0, stop: 3, reverse: false,
			expect: nil,
		},
		"invalid start index": {
			input: "ab", start: -1, stop: 3, reverse: false,
			expect: nil,
		},
		"invalid stop index": {
			input: "ab", start: 0, stop: 0, reverse: false,
			expect: nil,
		},
		"non-ascii": {
			input: "ab🐱", start: 0, stop: 1, reverse: false,
			expect: nil,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			actual := SubString(c.input, c.start, c.stop, c.reverse)
			if c.expect == nil {
				if actual != nil {
					t.Fatalf("expect no result, got %v", *actual)
				}
				return
			}

			if actual == nil {
				t.Fatalf("expect result, got none")
			}

			if e, a := *c.expect, *actual; e != a {
				t.Errorf("expect %q, got %q", e, a)
			}
		})
	}
}

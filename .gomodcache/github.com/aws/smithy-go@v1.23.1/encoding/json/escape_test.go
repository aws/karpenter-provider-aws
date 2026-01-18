package json

import (
	"bytes"
	"testing"
)

func TestEscapeStringBytes(t *testing.T) {
	cases := map[string]struct {
		expected string
		input    []byte
	}{
		"safeSet only": {
			expected: `"mountainPotato"`,
			input:    []byte("mountainPotato"),
		},
		"parenthesis": {
			expected: `"foo\""`,
			input:    []byte(`foo"`),
		},
		"double escape": {
			expected: `"hello\\\\world"`,
			input:    []byte(`hello\\world`),
		},
		"new line": {
			expected: `"foo\nbar"`,
			input:    []byte("foo\nbar"),
		},
		"carriage return": {
			expected: `"foo\rbar"`,
			input:    []byte("foo\rbar"),
		},
		"tab": {
			expected: `"foo\tbar"`,
			input:    []byte("foo\tbar"),
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var buffer bytes.Buffer
			escapeStringBytes(&buffer, c.input)
			expected := c.expected
			actual := buffer.String()
			if expected != actual {
				t.Errorf("\nexpected %v \nactual %v", expected, actual)
			}
		})
	}
}

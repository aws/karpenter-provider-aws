package testing

import (
	"bytes"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/aws/smithy-go/middleware"
	"github.com/aws/smithy-go/ptr"
)

func TestCompareValues(t *testing.T) {
	const float64NaN = 0x7fffffff_ffffffff // mantissa flipped all the way on

	cases := map[string]struct {
		A, B      interface{}
		ExpectErr string
	}{
		"totally different types": {
			A: 1,
			B: struct {
				Foo string
				Bar int
			}{
				Foo: "abc",
				Bar: 123,
			},
			ExpectErr: "<root>: kind int != struct",
		},
		"simple match": {
			A: struct {
				Foo      string
				Bar      int
				Metadata middleware.Metadata
			}{
				Foo: "abc",
				Bar: 123,
				Metadata: func() middleware.Metadata {
					var md middleware.Metadata
					md.Set(1, 1)
					return md
				}(),
			},
			B: struct {
				Foo      string
				Bar      int
				Metadata middleware.Metadata
			}{
				Foo:      "abc",
				Bar:      123,
				Metadata: middleware.Metadata{}, // different, shouldn't matter
			},
		},
		"simple diff": {
			A: struct {
				Foo string
				Bar int
			}{
				Foo: "abc",
				Bar: 123,
			},
			B: struct {
				Foo string
				Bar int
			}{
				Foo: "abc",
				Bar: 456,
			},
			ExpectErr: "<root>.Bar: 123 != 456",
		},
		"reader match": {
			A: struct {
				Foo io.Reader
				Bar int
			}{
				Foo: bytes.NewBuffer([]byte("abc123")),
				Bar: 123,
			},
			B: struct {
				Foo io.Reader
				Bar int
			}{
				Foo: io.NopCloser(strings.NewReader("abc123")),
				Bar: 123,
			},
		},
		"reader diff": {
			A: struct {
				Foo io.Reader
				Bar int
			}{
				Foo: bytes.NewBuffer([]byte("abc123")),
				Bar: 123,
			},
			B: struct {
				Foo io.Reader
				Bar int
			}{
				Foo: io.NopCloser(strings.NewReader("123abc")),
				Bar: 123,
			},
			ExpectErr: "<root>.Foo: bytes do not match",
		},
		"float match": {
			A: struct {
				Foo float64
				Bar int
			}{
				Foo: math.Float64frombits(float64NaN),
				Bar: 123,
			},
			B: struct {
				Foo float64
				Bar int
			}{
				Foo: math.Float64frombits(float64NaN),
				Bar: 123,
			},
		},
		"float diff NaN": {
			A: struct {
				Foo float64
				Bar int
			}{
				Foo: math.Float64frombits(float64NaN),
				Bar: 123,
			},
			B: struct {
				Foo float64
				Bar int
			}{
				Foo: math.Float64frombits(float64NaN - 1),
				Bar: 123,
			},
		},
		"float diff": {
			A: struct {
				Foo float64
				Bar int
			}{
				Foo: math.Float64frombits(0x100),
				Bar: 123,
			},
			B: struct {
				Foo float64
				Bar int
			}{
				Foo: math.Float64frombits(0x101),
				Bar: 123,
			},
			ExpectErr: "<root>.Foo: float64(0x100) != float64(0x101)",
		},

		"document equal": {
			A: &mockDocumentMarshaler{[]byte("123"), nil},
			B: &mockDocumentMarshaler{[]byte("123"), nil},
		},
		"document unequal": {
			A:         &mockDocumentMarshaler{[]byte("123"), nil},
			B:         &mockDocumentMarshaler{[]byte("124"), nil},
			ExpectErr: "<root>: document values unequal",
		},
		"slice equal": {
			A: []struct {
				Bar int
			}{{0}, {1}},
			B: []struct {
				Bar int
			}{{0}, {1}},
		},
		"slice length unequal": {
			A: []struct {
				Bar int
			}{{0}},
			B: []struct {
				Bar int
			}{{0}, {1}},
			ExpectErr: "slice length unequal",
		},
		"slice value unequal": {
			A: []struct {
				Bar int
			}{{2}, {1}, {0}},
			B: []struct {
				Bar int
			}{{2}, {0}, {1}},
			ExpectErr: "<root>[1].Bar: 1 != 0",
		},
		"map equal": {
			A: map[string]struct {
				Bar int
			}{
				"foo": {0},
				"bar": {1},
			},
			B: map[string]struct {
				Bar int
			}{
				"bar": {1},
				"foo": {0},
			},
		},
		"map length unequal": {
			A: map[string]struct {
				Bar int
			}{
				"foo": {0},
				"bar": {1},
			},
			B: map[string]struct {
				Bar int
			}{
				"foo": {0},
			},
			ExpectErr: "map length unequal",
		},
		"map value unequal": {
			A: map[string]struct {
				IntField int
			}{
				"foo": {0},
				"bar": {1},
			},
			B: map[string]struct {
				IntField int
			}{
				"bar": {1},
				"foo": {1},
			},
			ExpectErr: `<root>["foo"].IntField: 0 != 1`,
		},
		"handles deref, nil equal": {
			A: struct {
				Int *int
			}{nil},
			B: struct {
				Int *int
			}{nil},
		},
		"handles deref, value equal": {
			A: struct {
				Int *int
			}{ptr.Int(12)},
			B: struct {
				Int *int
			}{ptr.Int(12)},
		},
		"handles deref, different types are unequal": {
			A: struct {
				Int *int
			}{nil},
			B: struct {
				Int *string
			}{nil},
			ExpectErr: "<root>.Int: type mismatch",
		},
		"handles deref, unequal": {
			A: struct {
				Int *int
			}{ptr.Int(12)},
			B: struct {
				Int *int
			}{nil},
			ExpectErr: "<root>.Int: non-nil != nil",
		},
		"handles deref, unequal switched": {
			A: struct {
				Int *int
			}{nil},
			B: struct {
				Int *int
			}{ptr.Int(12)},
			ExpectErr: "<root>.Int: nil != non-nil",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := CompareValues(c.A, c.B)

			if len(c.ExpectErr) != 0 {
				if err == nil {
					t.Fatalf("expect error, got none")
				}
				if e, a := c.ExpectErr, err.Error(); !strings.Contains(a, e) {
					t.Fatalf("expect error to contain %v, got %v", e, a)
				}
				return
			}
			if err != nil {
				t.Fatalf("expect no error, got %v", err)
			}
		})
	}
}

func TestCompareValues_Document(t *testing.T) {
	cases := map[string]struct {
		A, B      interface{}
		ExpectErr string
	}{
		"equal": {
			A: &mockDocumentMarshaler{[]byte("123"), nil},
			B: &mockDocumentMarshaler{[]byte("123"), nil},
		},
		"unequal": {
			A:         &mockDocumentMarshaler{[]byte("123"), nil},
			B:         &mockDocumentMarshaler{[]byte("124"), nil},
			ExpectErr: "<root>: document values unequal",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := CompareValues(c.A, c.B)

			if len(c.ExpectErr) != 0 {
				if err == nil {
					t.Errorf("expect error, got none")
				}
				if e, a := c.ExpectErr, err.Error(); !strings.Contains(a, e) {
					t.Errorf("expect error to contain %v, got %v", e, a)
				}
				return
			}
			if err != nil {
				t.Errorf("expect no error, got %v", err)
			}
		})
	}
}

type mockDocumentMarshaler struct {
	p   []byte
	err error
}

var _ documentInterface = (*mockDocumentMarshaler)(nil)

func (m *mockDocumentMarshaler) MarshalSmithyDocument() ([]byte, error)      { return m.p, m.err }
func (m *mockDocumentMarshaler) UnmarshalSmithyDocument(v interface{}) error { return nil }

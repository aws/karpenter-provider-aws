package xml

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"testing"
)

var (
	oneInt   = new(big.Int).SetInt64(1)
	oneFloat = new(big.Float).SetFloat64(1.0)
)

func TestValue(t *testing.T) {
	nested := StartElement{Name: Name{Local: "nested"}}

	cases := map[string]struct {
		setter   func(Value)
		expected string
	}{
		"string value": {
			setter: func(value Value) {
				value.String("foo")
			},
			expected: `foo`,
		},
		"string escaped": {
			setter: func(value Value) {
				value.String("{\"foo\":\"bar\"}")
			},
			expected: fmt.Sprintf("{%sfoo%s:%sbar%s}", escQuot, escQuot, escQuot, escQuot),
		},
		"integer": {
			setter: func(value Value) {
				value.Long(1024)
			},
			expected: `1024`,
		},
		"float": {
			setter: func(value Value) {
				value.Double(1e20)
			},
			expected: `100000000000000000000`,
		},
		"float exponent component": {
			setter: func(value Value) {
				value.Double(3e22)
			},
			expected: `3e+22`,
		},
		"boolean true": {
			setter: func(value Value) {
				value.Boolean(true)
			},
			expected: `true`,
		},
		"boolean false": {
			setter: func(value Value) {
				value.Boolean(false)
			},
			expected: `false`,
		},
		"encode bytes": {
			setter: func(value Value) {
				value.Base64EncodeBytes([]byte("foo bar"))
			},
			expected: `Zm9vIGJhcg==`,
		},
		"encode bytes nil": {
			setter: func(value Value) {
				value.Base64EncodeBytes(nil)
			},
			expected: ``,
		},
		"object": {
			setter: func(value Value) {
				defer value.Close()
				value.MemberElement(nested).String("value")
			},
			expected: `<nested>value</nested>`,
		},
		"null": {
			setter: func(value Value) {
				value.Close()
			},
			expected: ``,
		},
		"nullWithRoot": {
			setter: func(value Value) {
				defer value.Close()
				o := value.MemberElement(nested)
				defer o.Close()
			},
			expected: `<nested></nested>`,
		},
		"write text": {
			setter: func(value Value) {
				defer value.Close()
				o := value.MemberElement(nested)
				o.Write([]byte(`{"nested":"value"}`), false)
			},
			expected: `<nested>{"nested":"value"}</nested>`,
		},
		"write escaped text": {
			setter: func(value Value) {
				defer value.Close()
				o := value.MemberElement(nested)
				o.Write([]byte(`{"nested":"value"}`), true)
			},
			expected: fmt.Sprintf("<nested>{%snested%s:%svalue%s}</nested>", escQuot, escQuot, escQuot, escQuot),
		},
		"bigInteger": {
			setter: func(value Value) {
				v := new(big.Int).SetInt64(math.MaxInt64)
				value.BigInteger(v.Sub(v, oneInt))
			},
			expected: strconv.FormatInt(math.MaxInt64-1, 10),
		},
		"bigInteger > int64": {
			setter: func(value Value) {
				v := new(big.Int).SetInt64(math.MaxInt64)
				value.BigInteger(v.Add(v, oneInt))
			},
			expected: "9223372036854775808",
		},
		"bigInteger < int64": {
			setter: func(value Value) {
				v := new(big.Int).SetInt64(math.MinInt64)
				value.BigInteger(v.Sub(v, oneInt))
			},
			expected: "-9223372036854775809",
		},
		"bigFloat": {
			setter: func(value Value) {
				v := new(big.Float).SetFloat64(math.MaxFloat64)
				value.BigDecimal(v.Sub(v, oneFloat))
			},
			expected: strconv.FormatFloat(math.MaxFloat64-1, 'e', -1, 64),
		},
		"bigFloat fits in int64": {
			setter: func(value Value) {
				v := new(big.Float).SetInt64(math.MaxInt64)
				value.BigDecimal(v)
			},
			expected: "9223372036854775807",
		},
	}
	scratch := make([]byte, 64)

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			b := bytes.NewBuffer(nil)
			root := StartElement{Name: Name{Local: "root"}}
			value := newValue(b, &scratch, root)
			tt.setter(value)

			if e, a := []byte("<root>"+tt.expected+"</root>"), b.Bytes(); bytes.Compare(e, a) != 0 {
				t.Errorf("expected %+q, but got %+q", e, a)
			}
		})
	}
}

func TestWrappedValue(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	scratch := make([]byte, 64)

	func() {
		root := StartElement{Name: Name{Local: "root"}}
		object := newValue(buffer, &scratch, root)
		defer object.Close()

		foo := StartElement{Name: Name{Local: "foo"}}
		faz := StartElement{Name: Name{Local: "faz"}}

		object.MemberElement(foo).String("bar")
		object.MemberElement(faz).String("baz")
	}()

	e := []byte(`<root><foo>bar</foo><faz>baz</faz></root>`)
	if a := buffer.Bytes(); bytes.Compare(e, a) != 0 {
		t.Errorf("expected %+q, but got %+q", e, a)
	}
}

func TestWrappedValueWithNameSpaceAndAttributes(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	scratch := make([]byte, 64)

	func() {
		root := StartElement{Name: Name{Local: "root"}}
		object := newValue(buffer, &scratch, root)
		defer object.Close()

		foo := StartElement{Name: Name{Local: "foo"}, Attr: []Attr{
			NewNamespaceAttribute("newspace", "https://endpoint.com"),
			NewAttribute("attrName", "attrValue"),
		}}
		faz := StartElement{Name: Name{Local: "faz"}}

		object.MemberElement(foo).String("bar")
		object.MemberElement(faz).String("baz")
	}()

	e := []byte(`<root><foo xmlns:newspace="https://endpoint.com" attrName="attrValue">bar</foo><faz>baz</faz></root>`)
	if a := buffer.Bytes(); bytes.Compare(e, a) != 0 {
		t.Errorf("expected %+q, but got %+q", e, a)
	}
}

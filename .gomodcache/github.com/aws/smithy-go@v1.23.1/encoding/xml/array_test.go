package xml

import (
	"bytes"
	"testing"
)

func TestWrappedArray(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	scratch := make([]byte, 64)

	root := StartElement{Name: Name{Local: "array"}}
	a := newArray(buffer, &scratch, arrayMemberWrapper, root, false)
	a.Member().String("bar")
	a.Member().String("baz")

	e := []byte(`<member>bar</member><member>baz</member>`)
	if a := buffer.Bytes(); bytes.Compare(e, a) != 0 {
		t.Errorf("expected %+q, but got %+q", e, a)
	}
}

func TestWrappedArrayWithCustomName(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	scratch := make([]byte, 64)

	root := StartElement{Name: Name{Local: "array"}}
	item := StartElement{Name: Name{Local: "item"}}
	a := newArray(buffer, &scratch, item, root, false)
	a.Member().String("bar")
	a.Member().String("baz")

	e := []byte(`<item>bar</item><item>baz</item>`)
	if a := buffer.Bytes(); bytes.Compare(e, a) != 0 {
		t.Errorf("expected %+q, but got %+q", e, a)
	}
}

func TestFlattenedArray(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	scratch := make([]byte, 64)

	root := StartElement{Name: Name{Local: "array"}}
	a := newArray(buffer, &scratch, arrayMemberWrapper, root, true)
	a.Member().String("bar")
	a.Member().String("bix")

	e := []byte(`<array>bar</array><array>bix</array>`)
	if a := buffer.Bytes(); bytes.Compare(e, a) != 0 {
		t.Errorf("expected %+q, but got %+q", e, a)
	}
}

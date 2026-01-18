package xml

import (
	"bytes"
	"testing"
)

func TestWrappedMap(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	scratch := make([]byte, 64)

	func() {
		m := newMap(buffer, &scratch)

		key := StartElement{Name: Name{Local: "key"}}
		value := StartElement{Name: Name{Local: "value"}}

		// map entry
		e := m.Entry()
		e.MemberElement(key).String("example-key1")
		e.MemberElement(value).String("example1")
		e.Close()

		// map entry
		e = m.Entry()
		e.MemberElement(key).String("example-key2")
		e.MemberElement(value).String("example2")
		e.Close()

		// map entry
		e = m.Entry()
		e.MemberElement(key).String("example-key3")
		e.MemberElement(value).String("example3")
		e.Close()
	}()

	ex := []byte(`<entry><key>example-key1</key><value>example1</value></entry><entry><key>example-key2</key><value>example2</value></entry><entry><key>example-key3</key><value>example3</value></entry>`)
	if a := buffer.Bytes(); bytes.Compare(ex, a) != 0 {
		t.Errorf("expected %+q, but got %+q", ex, a)
	}
}

func TestFlattenedMapWithCustomName(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	scratch := make([]byte, 64)

	func() {
		root := StartElement{Name: Name{Local: "flatMap"}}
		m := newFlattenedMap(buffer, &scratch, root)

		key := StartElement{Name: Name{Local: "key"}}
		value := StartElement{Name: Name{Local: "value"}}

		// map entry
		e := m.Entry()
		e.MemberElement(key).String("example-key1")
		e.MemberElement(value).String("example1")
		e.Close()

		// map entry
		e = m.Entry()
		e.MemberElement(key).String("example-key2")
		e.MemberElement(value).String("example2")
		e.Close()

		// map entry
		e = m.Entry()
		e.MemberElement(key).String("example-key3")
		e.MemberElement(value).String("example3")
		e.Close()
	}()

	ex := []byte(`<flatMap><key>example-key1</key><value>example1</value></flatMap><flatMap><key>example-key2</key><value>example2</value></flatMap><flatMap><key>example-key3</key><value>example3</value></flatMap>`)
	if a := buffer.Bytes(); bytes.Compare(ex, a) != 0 {
		t.Errorf("expected %+q, but got %+q", ex, a)
	}
}

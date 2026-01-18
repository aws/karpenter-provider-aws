package xml_test

import (
	"bytes"
	"log"
	"sort"
	"testing"

	"github.com/aws/smithy-go/encoding/xml"
)

var root = xml.StartElement{Name: xml.Name{Local: "root"}}

func TestEncoder(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		root := encoder.RootElement(root)
		defer root.Close()

		stringKey := xml.StartElement{Name: xml.Name{Local: "stringKey"}}
		integerKey := xml.StartElement{Name: xml.Name{Local: "integerKey"}}
		floatKey := xml.StartElement{Name: xml.Name{Local: "floatKey"}}
		foo := xml.StartElement{Name: xml.Name{Local: "foo"}}
		byteSlice := xml.StartElement{Name: xml.Name{Local: "byteSlice"}}

		root.MemberElement(stringKey).String("stringValue")
		root.MemberElement(integerKey).Integer(1024)
		root.MemberElement(floatKey).Float(3.14)

		ns := root.MemberElement(foo)
		defer ns.Close()
		ns.MemberElement(byteSlice).String("Zm9vIGJhcg==")
	}()

	e := []byte(`<root><stringKey>stringValue</stringKey><integerKey>1024</integerKey><floatKey>3.14</floatKey><foo><byteSlice>Zm9vIGJhcg==</byteSlice></foo></root>`)
	verify(t, encoder, e)
}

func TestEncodeAttribute(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := xml.StartElement{
			Name: xml.Name{Local: "payload", Space: "baz"},
			Attr: []xml.Attr{
				xml.NewAttribute("attrkey", "value"),
			},
		}

		obj := encoder.RootElement(r)
		obj.String("")
	}()

	expect := `<baz:payload attrkey="value"></baz:payload>`

	verify(t, encoder, []byte(expect))
}

func TestEncodeNamespace(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		root := encoder.RootElement(root)
		defer root.Close()

		key := xml.StartElement{
			Name: xml.Name{Local: "namespace"},
			Attr: []xml.Attr{
				xml.NewNamespaceAttribute("prefix", "https://example.com"),
			},
		}

		n := root.MemberElement(key)
		defer n.Close()

		prefix := xml.StartElement{Name: xml.Name{Local: "user"}}
		n.MemberElement(prefix).String("abc")
	}()

	e := []byte(`<root><namespace xmlns:prefix="https://example.com"><user>abc</user></namespace></root>`)
	verify(t, encoder, e)
}

func TestEncodeEmptyNamespacePrefix(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)
	func() {
		root := encoder.RootElement(root)
		defer root.Close()

		key := xml.StartElement{
			Name: xml.Name{Local: "namespace"},
			Attr: []xml.Attr{
				xml.NewNamespaceAttribute("", "https://example.com"),
			},
		}

		n := root.MemberElement(key)
		defer n.Close()

		prefix := xml.StartElement{Name: xml.Name{Local: "user"}}
		n.MemberElement(prefix).String("abc")
	}()

	e := []byte(`<root><namespace xmlns="https://example.com"><user>abc</user></namespace></root>`)
	verify(t, encoder, e)
}

func verify(t *testing.T, encoder *xml.Encoder, e []byte) {
	if a := encoder.Bytes(); bytes.Compare(e, a) != 0 {
		t.Errorf("expected %+q, but got %+q", e, a)
	}

	if a := encoder.String(); string(encoder.Bytes()) != a {
		t.Errorf("expected %s, but got %s", e, a)
	}
}

func TestEncodeNestedShape(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// nested `nested` shape
		nested := xml.StartElement{Name: xml.Name{Local: "nested"}}
		n1 := r.MemberElement(nested)
		defer n1.Close()

		// nested `value` shape
		value := xml.StartElement{Name: xml.Name{Local: "value"}}
		n1.MemberElement(value).String("expected value")
	}()

	e := []byte(`<root><nested><value>expected value</value></nested></root>`)
	defer verify(t, encoder, e)
}

func TestEncodeMapString(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)
	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// nested `mapStr` shape
		mapstr := xml.StartElement{Name: xml.Name{Local: "mapstr"}}
		mapElement := r.MemberElement(mapstr)
		defer mapElement.Close()

		m := mapElement.Map()

		key := xml.StartElement{Name: xml.Name{Local: "key"}}
		value := xml.StartElement{Name: xml.Name{Local: "value"}}

		e := m.Entry()
		defer e.Close()
		e.MemberElement(key).String("abc")
		e.MemberElement(value).Integer(123)
	}()

	ex := []byte(`<root><mapstr><entry><key>abc</key><value>123</value></entry></mapstr></root>`)
	verify(t, encoder, ex)
}

func TestEncodeMapFlatten(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()
		// nested `mapStr` shape
		mapstr := xml.StartElement{Name: xml.Name{Local: "mapstr"}}
		flatElement := r.FlattenedElement(mapstr)

		m := flatElement.Map()
		e := m.Entry()
		defer e.Close()

		key := xml.StartElement{Name: xml.Name{Local: "key"}}
		e.MemberElement(key).String("abc")

		value := xml.StartElement{Name: xml.Name{Local: "value"}}
		e.MemberElement(value).Integer(123)
	}()

	ex := []byte(`<root><mapstr><key>abc</key><value>123</value></mapstr></root>`)
	verify(t, encoder, ex)
}

func TestEncodeMapNamed(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()
		// nested `mapStr` shape
		mapstr := xml.StartElement{Name: xml.Name{Local: "mapNamed"}}
		mapElement := r.MemberElement(mapstr)
		defer mapElement.Close()

		m := mapElement.Map()
		e := m.Entry()
		defer e.Close()

		key := xml.StartElement{Name: xml.Name{Local: "namedKey"}}
		e.MemberElement(key).String("abc")

		value := xml.StartElement{Name: xml.Name{Local: "namedValue"}}
		e.MemberElement(value).Integer(123)
	}()

	ex := []byte(`<root><mapNamed><entry><namedKey>abc</namedKey><namedValue>123</namedValue></entry></mapNamed></root>`)
	verify(t, encoder, ex)
}

func TestEncodeMapShape(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// nested `mapStr` shape
		mapstr := xml.StartElement{Name: xml.Name{Local: "mapShape"}}
		mapElement := r.MemberElement(mapstr)
		defer mapElement.Close()

		m := mapElement.Map()

		e := m.Entry()
		defer e.Close()

		key := xml.StartElement{Name: xml.Name{Local: "key"}}
		e.MemberElement(key).String("abc")

		value := xml.StartElement{Name: xml.Name{Local: "value"}}
		n1 := e.MemberElement(value)
		defer n1.Close()

		shapeVal := xml.StartElement{Name: xml.Name{Local: "shapeVal"}}
		n1.MemberElement(shapeVal).Integer(1)
	}()

	ex := []byte(`<root><mapShape><entry><key>abc</key><value><shapeVal>1</shapeVal></value></entry></mapShape></root>`)
	verify(t, encoder, ex)
}

func TestEncodeMapFlattenShape(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()
		// nested `mapStr` shape
		mapstr := xml.StartElement{Name: xml.Name{Local: "mapShape"}}
		flatElement := r.FlattenedElement(mapstr)
		m := flatElement.Map()

		e := m.Entry()
		defer e.Close()

		key := xml.StartElement{Name: xml.Name{Local: "key"}}
		e.MemberElement(key).String("abc")

		value := xml.StartElement{Name: xml.Name{Local: "value"}}
		n1 := e.MemberElement(value)
		defer n1.Close()

		shapeVal := xml.StartElement{Name: xml.Name{Local: "shapeVal"}}
		n1.MemberElement(shapeVal).Integer(1)
	}()
	ex := []byte(`<root><mapShape><key>abc</key><value><shapeVal>1</shapeVal></value></mapShape></root>`)
	verify(t, encoder, ex)
}

func TestEncodeMapNamedShape(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// nested `mapStr` shape
		mapstr := xml.StartElement{Name: xml.Name{Local: "mapNamedShape"}}
		mapElement := r.MemberElement(mapstr)
		defer mapElement.Close()

		m := mapElement.Map()
		e := m.Entry()
		defer e.Close()

		key := xml.StartElement{Name: xml.Name{Local: "namedKey"}}
		e.MemberElement(key).String("abc")

		value := xml.StartElement{Name: xml.Name{Local: "namedValue"}}
		n1 := e.MemberElement(value)
		defer n1.Close()

		shapeVal := xml.StartElement{Name: xml.Name{Local: "shapeVal"}}
		n1.MemberElement(shapeVal).Integer(1)
	}()

	ex := []byte(`<root><mapNamedShape><entry><namedKey>abc</namedKey><namedValue><shapeVal>1</shapeVal></namedValue></entry></mapNamedShape></root>`)
	verify(t, encoder, ex)
}

func TestEncodeListString(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// Object key `liststr`
		liststr := xml.StartElement{Name: xml.Name{Local: "liststr"}}
		m := r.MemberElement(liststr)
		defer m.Close()

		a := m.Array()
		a.Member().String("abc")
		a.Member().Integer(123)
	}()

	ex := []byte(`<root><liststr><member>abc</member><member>123</member></liststr></root>`)
	verify(t, encoder, ex)
}

func TestEncodeListFlatten(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// Object key `liststr`
		liststr := xml.StartElement{Name: xml.Name{Local: "liststr"}}
		m := r.FlattenedElement(liststr)

		a := m.Array()
		a.Member().String("abc")
		a.Member().Integer(123)
	}()

	ex := []byte(`<root><liststr>abc</liststr><liststr>123</liststr></root>`)
	verify(t, encoder, ex)
}

func TestEncodeListNamed(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// Object key `liststr`
		liststr := xml.StartElement{Name: xml.Name{Local: "liststr"}}

		namedMember := xml.StartElement{Name: xml.Name{Local: "namedMember"}}
		m := r.MemberElement(liststr)
		defer m.Close()

		a := m.ArrayWithCustomName(namedMember)
		a.Member().String("abc")
		a.Member().Integer(123)
	}()

	ex := []byte(`<root><liststr><namedMember>abc</namedMember><namedMember>123</namedMember></liststr></root>`)
	verify(t, encoder, ex)
}

//
func TestEncodeListShape(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// Object key `liststr`
		liststr := xml.StartElement{Name: xml.Name{Local: "liststr"}}

		m := r.MemberElement(liststr)
		defer m.Close()

		a := m.Array()

		value := xml.StartElement{Name: xml.Name{Local: "value"}}

		m1 := a.Member()
		m1.MemberElement(value).String("abc")
		m1.Close()

		m2 := a.Member()
		m2.MemberElement(value).Integer(123)
		m2.Close()
	}()

	ex := []byte(`<root><liststr><member><value>abc</value></member><member><value>123</value></member></liststr></root>`)
	verify(t, encoder, ex)
}

//
func TestEncodeListFlattenShape(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// Object key `liststr`
		liststr := xml.StartElement{Name: xml.Name{Local: "liststr"}}

		m := r.FlattenedElement(liststr)

		a := m.Array()
		value := xml.StartElement{Name: xml.Name{Local: "value"}}

		m1 := a.Member()
		m1.MemberElement(value).String("abc")
		m1.Close()

		m2 := a.Member()
		m2.MemberElement(value).Integer(123)
		m2.Close()
	}()

	ex := []byte(`<root><liststr><value>abc</value></liststr><liststr><value>123</value></liststr></root>`)
	verify(t, encoder, ex)
}

//
func TestEncodeListNamedShape(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		// Object key `liststr`
		liststr := xml.StartElement{Name: xml.Name{Local: "liststr"}}
		namedMember := xml.StartElement{Name: xml.Name{Local: "namedMember"}}

		// member element
		m := r.MemberElement(liststr)
		defer m.Close()

		// Build array
		a := m.ArrayWithCustomName(namedMember)

		value := xml.StartElement{Name: xml.Name{Local: "value"}}
		m1 := a.Member()
		m1.MemberElement(value).String("abc")
		m1.Close()

		m2 := a.Member()
		m2.MemberElement(value).Integer(123)
		m2.Close()
	}()

	ex := []byte(`<root><liststr><namedMember><value>abc</value></namedMember><namedMember><value>123</value></namedMember></liststr></root>`)
	verify(t, encoder, ex)
}

func TestEncodeEscaping(t *testing.T) {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	func() {
		r := encoder.RootElement(root)
		defer r.Close()

		cases := map[string]rune{
			"quote":          '"',
			"apos":           '\'',
			"amp":            '&',
			"lt":             '<',
			"gt":             '>',
			"tab":            '\t',
			"newLine":        '\n',
			"carriageReturn": '\r',
			"nextLine":       '\u0085',
			"lineSeparator":  '\u2028',
		}

		var sortedKeys []string
		for name := range cases {
			sortedKeys = append(sortedKeys, name)
		}

		sort.Strings(sortedKeys)

		for _, name := range sortedKeys {
			rr := cases[name]

			st := xml.StartElement{Name: xml.Name{Local: name}}
			st.Attr = append(st.Attr, xml.Attr{
				Name: xml.Name{
					Local: "key",
				},
				Value: name + string(rr) + name,
			})
			value := r.MemberElement(st)
			value.String(name + string(rr) + name)
		}
	}()

	ex := []byte(`<root><amp key="amp&amp;amp">amp&amp;amp</amp><apos key="apos&#39;apos">apos&#39;apos</apos><carriageReturn key="carriageReturn&#xD;carriageReturn">carriageReturn&#xD;carriageReturn</carriageReturn><gt key="gt&gt;gt">gt&gt;gt</gt><lineSeparator key="lineSeparator&#x2028;lineSeparator">lineSeparator&#x2028;lineSeparator</lineSeparator><lt key="lt&lt;lt">lt&lt;lt</lt><newLine key="newLine&#xA;newLine">newLine&#xA;newLine</newLine><nextLine key="nextLine&#x85;nextLine">nextLine&#x85;nextLine</nextLine><quote key="quote&#34;quote">quote&#34;quote</quote><tab key="tab&#x9;tab">tab&#x9;tab</tab></root>`)
	verify(t, encoder, ex)
}

// ExampleEncoder is the example function on how to use an encoder
func ExampleEncoder() {
	b := bytes.NewBuffer(nil)
	encoder := xml.NewEncoder(b)

	// expected encoded xml document is :
	// `<root><liststr><namedMember><value>abc</value></namedMember><namedMember><value>123</value></namedMember></liststr></root>`
	defer log.Printf("Encoded xml document: %v", encoder.String())

	r := encoder.RootElement(root)
	defer r.Close()

	// Object key `liststr`
	liststr := xml.StartElement{Name: xml.Name{Local: "liststr"}}
	namedMember := xml.StartElement{Name: xml.Name{Local: "namedMember"}}

	// member element
	m := r.MemberElement(liststr)
	defer m.Close()

	// Build array
	a := m.ArrayWithCustomName(namedMember)

	value := xml.StartElement{Name: xml.Name{Local: "value"}}
	m1 := a.Member()
	m1.MemberElement(value).String("abc")
	m1.Close()

	m2 := a.Member()
	m2.MemberElement(value).Integer(123)
	m2.Close()
}

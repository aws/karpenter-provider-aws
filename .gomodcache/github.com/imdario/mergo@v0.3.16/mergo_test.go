// Copyright 2013 Dario Castañé. All rights reserved.
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mergo_test

import (
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/imdario/mergo"
	"gopkg.in/yaml.v3"
)

type simpleTest struct {
	Value int
}

type complexTest struct {
	ID string
	St simpleTest
	sz int
}

type mapTest struct {
	M map[int]int
}

type ifcTest struct {
	I interface{}
}

type moreComplextText struct {
	Ct complexTest
	St simpleTest
	Nt simpleTest
}

type pointerTest struct {
	C *simpleTest
}

type sliceTest struct {
	S []int
}

func TestKb(t *testing.T) {
	type testStruct struct {
		KeyValue map[string]interface{}
		Name     string
	}

	akv := make(map[string]interface{})
	akv["Key1"] = "not value 1"
	akv["Key2"] = "value2"
	a := testStruct{}
	a.Name = "A"
	a.KeyValue = akv

	bkv := make(map[string]interface{})
	bkv["Key1"] = "value1"
	bkv["Key3"] = "value3"
	b := testStruct{}
	b.Name = "B"
	b.KeyValue = bkv

	ekv := make(map[string]interface{})
	ekv["Key1"] = "value1"
	ekv["Key2"] = "value2"
	ekv["Key3"] = "value3"
	expected := testStruct{}
	expected.Name = "B"
	expected.KeyValue = ekv

	if err := mergo.Merge(&b, a); err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(b, expected) {
		t.Errorf("Actual: %#v did not match \nExpected: %#v", b, expected)
	}
}

func TestNil(t *testing.T) {
	if err := mergo.Merge(nil, nil); err != mergo.ErrNilArguments {
		t.Fail()
	}
}

func TestDifferentTypes(t *testing.T) {
	a := simpleTest{42}
	b := 42
	if err := mergo.Merge(&a, b); err != mergo.ErrDifferentArgumentsTypes {
		t.Fail()
	}
}

func TestSimpleStruct(t *testing.T) {
	a := simpleTest{}
	b := simpleTest{42}
	if err := mergo.Merge(&a, b); err != nil {
		t.FailNow()
	}
	if a.Value != 42 {
		t.Errorf("b not merged in properly: a.Value(%d) != b.Value(%d)", a.Value, b.Value)
	}
	if !reflect.DeepEqual(a, b) {
		t.FailNow()
	}
}

func TestComplexStruct(t *testing.T) {
	a := complexTest{}
	a.ID = "athing"
	b := complexTest{"bthing", simpleTest{42}, 1}
	if err := mergo.Merge(&a, b); err != nil {
		t.FailNow()
	}
	if a.St.Value != 42 {
		t.Errorf("b not merged in properly: a.St.Value(%d) != b.St.Value(%d)", a.St.Value, b.St.Value)
	}
	if a.sz == 1 {
		t.Errorf("a's private field sz not preserved from merge: a.sz(%d) == b.sz(%d)", a.sz, b.sz)
	}
	if a.ID == b.ID {
		t.Errorf("a's field ID merged unexpectedly: a.ID(%s) == b.ID(%s)", a.ID, b.ID)
	}
}

func TestComplexStructWithOverwrite(t *testing.T) {
	a := complexTest{"do-not-overwrite-with-empty-value", simpleTest{1}, 1}
	b := complexTest{"", simpleTest{42}, 2}

	expect := complexTest{"do-not-overwrite-with-empty-value", simpleTest{42}, 1}
	if err := mergo.MergeWithOverwrite(&a, b); err != nil {
		t.FailNow()
	}

	if !reflect.DeepEqual(a, expect) {
		t.Errorf("Test failed:\ngot  :\n%#v\n\nwant :\n%#v\n\n", a, expect)
	}
}

func TestPointerStruct(t *testing.T) {
	s1 := simpleTest{}
	s2 := simpleTest{19}
	a := pointerTest{&s1}
	b := pointerTest{&s2}
	if err := mergo.Merge(&a, b); err != nil {
		t.FailNow()
	}
	if a.C.Value != b.C.Value {
		t.Errorf("b not merged in properly: a.C.Value(%d) != b.C.Value(%d)", a.C.Value, b.C.Value)
	}
}

type embeddingStruct struct {
	embeddedStruct
}

type embeddedStruct struct {
	A string
}

func TestEmbeddedStruct(t *testing.T) {
	tests := []struct {
		src      embeddingStruct
		dst      embeddingStruct
		expected embeddingStruct
	}{
		{
			src: embeddingStruct{
				embeddedStruct{"foo"},
			},
			dst: embeddingStruct{
				embeddedStruct{""},
			},
			expected: embeddingStruct{
				embeddedStruct{"foo"},
			},
		},
		{
			src: embeddingStruct{
				embeddedStruct{""},
			},
			dst: embeddingStruct{
				embeddedStruct{"bar"},
			},
			expected: embeddingStruct{
				embeddedStruct{"bar"},
			},
		},
		{
			src: embeddingStruct{
				embeddedStruct{"foo"},
			},
			dst: embeddingStruct{
				embeddedStruct{"bar"},
			},
			expected: embeddingStruct{
				embeddedStruct{"bar"},
			},
		},
	}

	for _, test := range tests {
		err := mergo.Merge(&test.dst, test.src)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		if !reflect.DeepEqual(test.dst, test.expected) {
			t.Errorf("unexpected output\nexpected:\n%+v\nsaw:\n%+v\n", test.expected, test.dst)
		}
	}
}

func TestPointerStructNil(t *testing.T) {
	a := pointerTest{nil}
	b := pointerTest{&simpleTest{19}}
	if err := mergo.Merge(&a, b); err != nil {
		t.FailNow()
	}
	if a.C.Value != b.C.Value {
		t.Errorf("b not merged in a properly: a.C.Value(%d) != b.C.Value(%d)", a.C.Value, b.C.Value)
	}
}

func testSlice(t *testing.T, a []int, b []int, e []int, opts ...func(*mergo.Config)) {
	t.Helper()
	bc := b

	sa := sliceTest{a}
	sb := sliceTest{b}
	if err := mergo.Merge(&sa, sb, opts...); err != nil {
		t.FailNow()
	}
	if !reflect.DeepEqual(sb.S, bc) {
		t.Errorf("Source slice was modified %d != %d", sb.S, bc)
	}
	if !reflect.DeepEqual(sa.S, e) {
		t.Errorf("b not merged in a proper way %d != %d", sa.S, e)
	}

	ma := map[string][]int{"S": a}
	mb := map[string][]int{"S": b}
	if err := mergo.Merge(&ma, mb, opts...); err != nil {
		t.FailNow()
	}
	if !reflect.DeepEqual(mb["S"], bc) {
		t.Errorf("map value: Source slice was modified %d != %d", mb["S"], bc)
	}
	if !reflect.DeepEqual(ma["S"], e) {
		t.Errorf("map value: b not merged in a proper way %d != %d", ma["S"], e)
	}

	if a == nil {
		// test case with missing dst key
		ma := map[string][]int{}
		mb := map[string][]int{"S": b}
		if err := mergo.Merge(&ma, mb); err != nil {
			t.FailNow()
		}
		if !reflect.DeepEqual(mb["S"], bc) {
			t.Errorf("missing dst key: Source slice was modified %d != %d", mb["S"], bc)
		}
		if !reflect.DeepEqual(ma["S"], e) {
			t.Errorf("missing dst key: b not merged in a proper way %d != %d", ma["S"], e)
		}
	}

	if b == nil {
		// test case with missing src key
		ma := map[string][]int{"S": a}
		mb := map[string][]int{}
		if err := mergo.Merge(&ma, mb); err != nil {
			t.FailNow()
		}
		if !reflect.DeepEqual(mb["S"], bc) {
			t.Errorf("missing src key: Source slice was modified %d != %d", mb["S"], bc)
		}
		if !reflect.DeepEqual(ma["S"], e) {
			t.Errorf("missing src key: b not merged in a proper way %d != %d", ma["S"], e)
		}
	}
}

func TestSlice(t *testing.T) {
	testSlice(t, nil, []int{1, 2, 3}, []int{1, 2, 3})
	testSlice(t, []int{}, []int{1, 2, 3}, []int{1, 2, 3})
	testSlice(t, []int{1}, []int{2, 3}, []int{1})
	testSlice(t, []int{1}, []int{}, []int{1})
	testSlice(t, []int{1}, nil, []int{1})
	testSlice(t, nil, []int{1, 2, 3}, []int{1, 2, 3}, mergo.WithAppendSlice)
	testSlice(t, []int{}, []int{1, 2, 3}, []int{1, 2, 3}, mergo.WithAppendSlice)
	testSlice(t, []int{1}, []int{2, 3}, []int{1, 2, 3}, mergo.WithAppendSlice)
	testSlice(t, []int{1}, []int{2, 3}, []int{1, 2, 3}, mergo.WithAppendSlice, mergo.WithOverride)
	testSlice(t, []int{1}, []int{}, []int{1}, mergo.WithAppendSlice)
	testSlice(t, []int{1}, nil, []int{1}, mergo.WithAppendSlice)
}

func TestEmptyMaps(t *testing.T) {
	a := mapTest{}
	b := mapTest{
		map[int]int{},
	}
	if err := mergo.Merge(&a, b); err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(a, b) {
		t.FailNow()
	}
}

func TestEmptyToEmptyMaps(t *testing.T) {
	a := mapTest{}
	b := mapTest{}
	if err := mergo.Merge(&a, b); err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(a, b) {
		t.FailNow()
	}
}

func TestEmptyToNotEmptyMaps(t *testing.T) {
	a := mapTest{map[int]int{
		1: 2,
		3: 4,
	}}
	aa := mapTest{map[int]int{
		1: 2,
		3: 4,
	}}
	b := mapTest{
		map[int]int{},
	}
	if err := mergo.Merge(&a, b); err != nil {
		t.Fail()
	}
	if !reflect.DeepEqual(a, aa) {
		t.FailNow()
	}
}

func TestMapsWithOverwrite(t *testing.T) {
	m := map[string]simpleTest{
		"a": {},   // overwritten by 16
		"b": {42}, // overwritten by 0, as map Value is not addressable and it doesn't check for b is set or not set in `n`
		"c": {13}, // overwritten by 12
		"d": {61},
	}
	n := map[string]simpleTest{
		"a": {16},
		"b": {},
		"c": {12},
		"e": {14},
	}
	expect := map[string]simpleTest{
		"a": {16},
		"b": {},
		"c": {12},
		"d": {61},
		"e": {14},
	}

	if err := mergo.MergeWithOverwrite(&m, n); err != nil {
		t.Errorf(err.Error())
	}

	if !reflect.DeepEqual(m, expect) {
		t.Errorf("Test failed:\ngot  :\n%#v\n\nwant :\n%#v\n\n", m, expect)
	}
}

func TestMapWithEmbeddedStructPointer(t *testing.T) {
	m := map[string]*simpleTest{
		"a": {},   // overwritten by 16
		"b": {42}, // not overwritten by empty value
		"c": {13}, // overwritten by 12
		"d": {61},
	}
	n := map[string]*simpleTest{
		"a": {16},
		"b": {},
		"c": {12},
		"e": {14},
	}
	expect := map[string]*simpleTest{
		"a": {16},
		"b": {42},
		"c": {12},
		"d": {61},
		"e": {14},
	}

	if err := mergo.Merge(&m, n, mergo.WithOverride); err != nil {
		t.Errorf(err.Error())
	}

	if !reflect.DeepEqual(m, expect) {
		t.Errorf("Test failed:\ngot  :\n%#v\n\nwant :\n%#v\n\n", m, expect)
	}
}

func TestMergeUsingStructAndMap(t *testing.T) {
	type multiPtr struct {
		Text   string
		Number int
	}
	type final struct {
		Msg1 string
		Msg2 string
	}
	type params struct {
		Multi *multiPtr
		Final *final
		Name  string
	}
	type config struct {
		Params *params
		Foo    string
		Bar    string
	}

	cases := []struct {
		changes   *config
		target    *config
		output    *config
		name      string
		overwrite bool
	}{
		{
			name:      "Should overwrite values in target for non-nil values in source",
			overwrite: true,
			changes: &config{
				Bar: "from changes",
				Params: &params{
					Final: &final{
						Msg1: "from changes",
						Msg2: "from changes",
					},
				},
			},
			target: &config{
				Foo: "from target",
				Params: &params{
					Name: "from target",
					Multi: &multiPtr{
						Text:   "from target",
						Number: 5,
					},
					Final: &final{
						Msg1: "from target",
						Msg2: "",
					},
				},
			},
			output: &config{
				Foo: "from target",
				Bar: "from changes",
				Params: &params{
					Name: "from target",
					Multi: &multiPtr{
						Text:   "from target",
						Number: 5,
					},
					Final: &final{
						Msg1: "from changes",
						Msg2: "from changes",
					},
				},
			},
		},
		{
			name:      "Should not overwrite values in target for non-nil values in source",
			overwrite: false,
			changes: &config{
				Bar: "from changes",
				Params: &params{
					Final: &final{
						Msg1: "from changes",
						Msg2: "from changes",
					},
				},
			},
			target: &config{
				Foo: "from target",
				Params: &params{
					Name: "from target",
					Multi: &multiPtr{
						Text:   "from target",
						Number: 5,
					},
					Final: &final{
						Msg1: "from target",
						Msg2: "",
					},
				},
			},
			output: &config{
				Foo: "from target",
				Bar: "from changes",
				Params: &params{
					Name: "from target",
					Multi: &multiPtr{
						Text:   "from target",
						Number: 5,
					},
					Final: &final{
						Msg1: "from target",
						Msg2: "from changes",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.overwrite {
				err = mergo.Merge(tc.target, *tc.changes, mergo.WithOverride)
			} else {
				err = mergo.Merge(tc.target, *tc.changes)
			}
			if err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(tc.target, tc.output) {
				t.Errorf("Test failed:\ngot  :\n%+v\n\nwant :\n%+v\n\n", tc.target.Params, tc.output.Params)
			}
		})
	}
}
func TestMaps(t *testing.T) {
	m := map[string]simpleTest{
		"a": {},
		"b": {42},
		"c": {13},
		"d": {61},
	}
	n := map[string]simpleTest{
		"a": {16},
		"b": {},
		"c": {12},
		"e": {14},
	}
	expect := map[string]simpleTest{
		"a": {0},
		"b": {42},
		"c": {13},
		"d": {61},
		"e": {14},
	}

	if err := mergo.Merge(&m, n); err != nil {
		t.Errorf(err.Error())
	}

	if !reflect.DeepEqual(m, expect) {
		t.Errorf("Test failed:\ngot  :\n%#v\n\nwant :\n%#v\n\n", m, expect)
	}
	if m["a"].Value != 0 {
		t.Errorf(`n merged in m because I solved non-addressable map values TODO: m["a"].Value(%d) != n["a"].Value(%d)`, m["a"].Value, n["a"].Value)
	}
	if m["b"].Value != 42 {
		t.Errorf(`n wrongly merged in m: m["b"].Value(%d) != n["b"].Value(%d)`, m["b"].Value, n["b"].Value)
	}
	if m["c"].Value != 13 {
		t.Errorf(`n overwritten in m: m["c"].Value(%d) != n["c"].Value(%d)`, m["c"].Value, n["c"].Value)
	}
}

func TestMapsWithNilPointer(t *testing.T) {
	m := map[string]*simpleTest{
		"a": nil,
		"b": nil,
	}
	n := map[string]*simpleTest{
		"b": nil,
		"c": nil,
	}
	expect := map[string]*simpleTest{
		"a": nil,
		"b": nil,
		"c": nil,
	}

	if err := mergo.Merge(&m, n, mergo.WithOverride); err != nil {
		t.Errorf(err.Error())
	}

	if !reflect.DeepEqual(m, expect) {
		t.Errorf("Test failed:\ngot   :\n%#v\n\nwant :\n%#v\n\n", m, expect)
	}
}

func TestYAMLMaps(t *testing.T) {
	thing := loadYAML("testdata/thing.yml")
	license := loadYAML("testdata/license.yml")
	ft := thing["fields"].(map[string]interface{})
	fl := license["fields"].(map[string]interface{})
	// license has one extra field (site) and another already existing in thing (author) that Mergo won't override.
	expectedLength := len(ft) + len(fl) - 1
	if err := mergo.Merge(&license, thing); err != nil {
		t.Error(err.Error())
	}
	currentLength := len(license["fields"].(map[string]interface{}))
	if currentLength != expectedLength {
		t.Errorf(`thing not merged in license properly, license must have %d elements instead of %d`, expectedLength, currentLength)
	}
	fields := license["fields"].(map[string]interface{})
	if _, ok := fields["id"]; !ok {
		t.Errorf(`thing not merged in license properly, license must have a new id field from thing`)
	}
}

func TestTwoPointerValues(t *testing.T) {
	a := &simpleTest{}
	b := &simpleTest{42}
	if err := mergo.Merge(a, b); err != nil {
		t.Errorf(`Boom. You crossed the streams: %s`, err)
	}
}

func TestMap(t *testing.T) {
	a := complexTest{}
	a.ID = "athing"
	c := moreComplextText{a, simpleTest{}, simpleTest{}}
	b := map[string]interface{}{
		"ct": map[string]interface{}{
			"st": map[string]interface{}{
				"value": 42,
			},
			"sz": 1,
			"id": "bthing",
		},
		"st": &simpleTest{144}, // Mapping a reference
		"zt": simpleTest{299},  // Mapping a missing field (zt doesn't exist)
		"nt": simpleTest{3},
	}
	if err := mergo.Map(&c, b); err != nil {
		t.FailNow()
	}
	m := b["ct"].(map[string]interface{})
	n := m["st"].(map[string]interface{})
	o := b["st"].(*simpleTest)
	p := b["nt"].(simpleTest)
	if c.Ct.St.Value != 42 {
		t.Errorf("b not merged in properly: c.Ct.St.Value(%d) != b.Ct.St.Value(%d)", c.Ct.St.Value, n["value"])
	}
	if c.St.Value != 144 {
		t.Errorf("b not merged in properly: c.St.Value(%d) != b.St.Value(%d)", c.St.Value, o.Value)
	}
	if c.Nt.Value != 3 {
		t.Errorf("b not merged in properly: c.Nt.Value(%d) != b.Nt.Value(%d)", c.St.Value, p.Value)
	}
	if c.Ct.sz == 1 {
		t.Errorf("a's private field sz not preserved from merge: c.Ct.sz(%d) == b.Ct.sz(%d)", c.Ct.sz, m["sz"])
	}
	if c.Ct.ID == m["id"] {
		t.Errorf("a's field ID merged unexpectedly: c.Ct.ID(%s) == b.Ct.ID(%s)", c.Ct.ID, m["id"])
	}
}

func TestSimpleMap(t *testing.T) {
	a := simpleTest{}
	b := map[string]interface{}{
		"value": 42,
	}
	if err := mergo.Map(&a, b); err != nil {
		t.FailNow()
	}
	if a.Value != 42 {
		t.Errorf("b not merged in properly: a.Value(%d) != b.Value(%v)", a.Value, b["value"])
	}
}

func TestIfcMap(t *testing.T) {
	a := ifcTest{}
	b := ifcTest{42}
	if err := mergo.Map(&a, b); err != nil {
		t.FailNow()
	}
	if a.I != 42 {
		t.Errorf("b not merged in properly: a.I(%d) != b.I(%d)", a.I, b.I)
	}
	if !reflect.DeepEqual(a, b) {
		t.FailNow()
	}
}

func TestIfcMapNoOverwrite(t *testing.T) {
	a := ifcTest{13}
	b := ifcTest{42}
	if err := mergo.Map(&a, b); err != nil {
		t.FailNow()
	}
	if a.I != 13 {
		t.Errorf("a not left alone: a.I(%d) == b.I(%d)", a.I, b.I)
	}
}

func TestIfcMapWithOverwrite(t *testing.T) {
	a := ifcTest{13}
	b := ifcTest{42}
	if err := mergo.MapWithOverwrite(&a, b); err != nil {
		t.FailNow()
	}
	if a.I != 42 {
		t.Errorf("b not merged in properly: a.I(%d) != b.I(%d)", a.I, b.I)
	}
	if !reflect.DeepEqual(a, b) {
		t.FailNow()
	}
}

type pointerMapTest struct {
	B      *simpleTest
	A      int
	hidden int
}

func TestBackAndForth(t *testing.T) {
	pt := pointerMapTest{&simpleTest{66}, 42, 1}
	m := make(map[string]interface{})
	if err := mergo.Map(&m, pt); err != nil {
		t.FailNow()
	}
	var (
		v  interface{}
		ok bool
	)
	if v, ok = m["a"]; v.(int) != pt.A || !ok {
		t.Errorf("pt not merged in properly: m[`a`](%d) != pt.A(%d)", v, pt.A)
	}
	if v, ok = m["b"]; !ok {
		t.Errorf("pt not merged in properly: B is missing in m")
	}
	var st *simpleTest
	if st = v.(*simpleTest); st.Value != 66 {
		t.Errorf("something went wrong while mapping pt on m, B wasn't copied")
	}
	bpt := pointerMapTest{}
	if err := mergo.Map(&bpt, m); err != nil {
		t.Error(err)
	}
	if bpt.A != pt.A {
		t.Errorf("pt not merged in properly: bpt.A(%d) != pt.A(%d)", bpt.A, pt.A)
	}
	if bpt.hidden == pt.hidden {
		t.Errorf("pt unexpectedly merged: bpt.hidden(%d) == pt.hidden(%d)", bpt.hidden, pt.hidden)
	}
	if bpt.B.Value != pt.B.Value {
		t.Errorf("pt not merged in properly: bpt.B.Value(%d) != pt.B.Value(%d)", bpt.B.Value, pt.B.Value)
	}
}

func TestEmbeddedPointerUnpacking(t *testing.T) {
	tests := []struct{ input pointerMapTest }{
		{pointerMapTest{nil, 42, 1}},
		{pointerMapTest{&simpleTest{66}, 42, 1}},
	}
	newValue := 77
	m := map[string]interface{}{
		"b": map[string]interface{}{
			"value": newValue,
		},
	}
	for _, test := range tests {
		pt := test.input
		if err := mergo.MapWithOverwrite(&pt, m); err != nil {
			t.FailNow()
		}
		if pt.B.Value != newValue {
			t.Errorf("pt not mapped properly: pt.A.Value(%d) != m[`b`][`value`](%d)", pt.B.Value, newValue)
		}

	}
}

type structWithTimePointer struct {
	Birth *time.Time
}

func TestTime(t *testing.T) {
	now := time.Now()
	dataStruct := structWithTimePointer{
		Birth: &now,
	}
	dataMap := map[string]interface{}{
		"Birth": &now,
	}
	b := structWithTimePointer{}
	if err := mergo.Merge(&b, dataStruct); err != nil {
		t.FailNow()
	}
	if b.Birth.IsZero() {
		t.Errorf("time.Time not merged in properly: b.Birth(%v) != dataStruct['Birth'](%v)", b.Birth, dataStruct.Birth)
	}
	if b.Birth != dataStruct.Birth {
		t.Errorf("time.Time not merged in properly: b.Birth(%v) != dataStruct['Birth'](%v)", b.Birth, dataStruct.Birth)
	}
	b = structWithTimePointer{}
	if err := mergo.Map(&b, dataMap); err != nil {
		t.FailNow()
	}
	if b.Birth.IsZero() {
		t.Errorf("time.Time not merged in properly: b.Birth(%v) != dataMap['Birth'](%v)", b.Birth, dataMap["Birth"])
	}
}

type simpleNested struct {
	A int
}

type structWithNestedPtrValueMap struct {
	NestedPtrValue map[string]*simpleNested
}

func TestNestedPtrValueInMap(t *testing.T) {
	src := &structWithNestedPtrValueMap{
		NestedPtrValue: map[string]*simpleNested{
			"x": {
				A: 1,
			},
		},
	}
	dst := &structWithNestedPtrValueMap{
		NestedPtrValue: map[string]*simpleNested{
			"x": {},
		},
	}
	if err := mergo.Map(dst, src); err != nil {
		t.FailNow()
	}
	if dst.NestedPtrValue["x"].A == 0 {
		t.Errorf("Nested Ptr value not merged in properly: dst.NestedPtrValue[\"x\"].A(%v) != src.NestedPtrValue[\"x\"].A(%v)", dst.NestedPtrValue["x"].A, src.NestedPtrValue["x"].A)
	}
}

func loadYAML(path string) (m map[string]interface{}) {
	m = make(map[string]interface{})
	raw, _ := ioutil.ReadFile(path)
	_ = yaml.Unmarshal(raw, &m)
	return
}

type structWithMap struct {
	m map[string]structWithUnexportedProperty
}

type structWithUnexportedProperty struct {
	s string
}

func TestUnexportedProperty(t *testing.T) {
	a := structWithMap{map[string]structWithUnexportedProperty{
		"key": {"hello"},
	}}
	b := structWithMap{map[string]structWithUnexportedProperty{
		"key": {"hi"},
	}}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Should not have panicked")
		}
	}()
	mergo.Merge(&a, b)
}

type structWithBoolPointer struct {
	C *bool
}

func TestBooleanPointer(t *testing.T) {
	bt, bf := true, false
	src := structWithBoolPointer{
		&bt,
	}
	dst := structWithBoolPointer{
		&bf,
	}
	if err := mergo.Merge(&dst, src); err != nil {
		t.FailNow()
	}
	if dst.C == src.C {
		t.Errorf("dst.C should be a different pointer than src.C")
	}
	if *dst.C != *src.C {
		t.Errorf("dst.C should be true")
	}
}

func TestMergeMapWithInnerSliceOfDifferentType(t *testing.T) {
	testCases := []struct {
		name    string
		err     string
		options []func(*mergo.Config)
	}{
		{
			"With override and append slice",
			"cannot append two slices with different type",
			[]func(*mergo.Config){mergo.WithOverride, mergo.WithAppendSlice},
		},
		{
			"With override and type check",
			"cannot override two slices with different type",
			[]func(*mergo.Config){mergo.WithOverride, mergo.WithTypeCheck},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := map[string]interface{}{
				"foo": []string{"a", "b"},
			}
			dst := map[string]interface{}{
				"foo": []int{1, 2},
			}

			if err := mergo.Merge(&src, &dst, tc.options...); err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("expected %q, got %q", tc.err, err)
			}
		})
	}
}

func TestMergeDifferentSlicesIsNotSupported(t *testing.T) {
	src := []string{"a", "b"}
	dst := []int{1, 2}

	if err := mergo.Merge(&src, &dst, mergo.WithOverride, mergo.WithAppendSlice); err != mergo.ErrDifferentArgumentsTypes {
		t.Errorf("expected %q, got %q", mergo.ErrNotSupported, err)
	}
}

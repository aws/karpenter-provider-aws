/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package serialize_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/internal/serialize"
	"k8s.io/klog/v2/internal/test"
)

// point conforms to fmt.Stringer interface as it implements the String() method
type point struct {
	x int
	y int
}

// we now have a value receiver
func (p point) String() string {
	return fmt.Sprintf("x=%d, y=%d", p.x, p.y)
}

type dummyStruct struct {
	key   string
	value string
}

func (d *dummyStruct) MarshalLog() interface{} {
	return map[string]string{
		"key-data":   d.key,
		"value-data": d.value,
	}
}

type dummyStructWithStringMarshal struct {
	key   string
	value string
}

func (d *dummyStructWithStringMarshal) MarshalLog() interface{} {
	return fmt.Sprintf("%s=%s", d.key, d.value)
}

// Test that kvListFormat works as advertised.
func TestKvListFormat(t *testing.T) {
	var emptyPoint *point
	var testKVList = []struct {
		keysValues []interface{}
		want       string
	}{
		{
			keysValues: []interface{}{"data", &dummyStruct{key: "test", value: "info"}},
			want:       ` data={"key-data":"test","value-data":"info"}`,
		},
		{
			keysValues: []interface{}{"data", &dummyStructWithStringMarshal{key: "test", value: "info"}},
			want:       ` data="test=info"`,
		},
		{
			keysValues: []interface{}{"pod", "kubedns"},
			want:       " pod=\"kubedns\"",
		},
		{
			keysValues: []interface{}{"pod", "kubedns", "update", true},
			want:       " pod=\"kubedns\" update=true",
		},
		{
			keysValues: []interface{}{"pod", "kubedns", "spec", struct {
				X int
				Y string
				N time.Time
			}{X: 76, Y: "strval", N: time.Date(2006, 1, 2, 15, 4, 5, .067890e9, time.UTC)}},
			want: ` pod="kubedns" spec={"X":76,"Y":"strval","N":"2006-01-02T15:04:05.06789Z"}`,
		},
		{
			keysValues: []interface{}{"pod", "kubedns", "values", []int{8, 6, 7, 5, 3, 0, 9}},
			want:       " pod=\"kubedns\" values=[8,6,7,5,3,0,9]",
		},
		{
			keysValues: []interface{}{"pod", "kubedns", "values", []string{"deployment", "svc", "configmap"}},
			want:       ` pod="kubedns" values=["deployment","svc","configmap"]`,
		},
		{
			keysValues: []interface{}{"pod", "kubedns", "bytes", []byte("test case for byte array")},
			want:       " pod=\"kubedns\" bytes=\"test case for byte array\"",
		},
		{
			keysValues: []interface{}{"pod", "kubedns", "bytes", []byte("��=� ⌘")},
			want:       " pod=\"kubedns\" bytes=\"\\ufffd\\ufffd=\\ufffd \\u2318\"",
		},
		{
			keysValues: []interface{}{"multiLineString", `Hello world!
	Starts with tab.
  Starts with spaces.
No whitespace.`,
				"pod", "kubedns",
			},
			want: ` multiLineString=<
	Hello world!
		Starts with tab.
	  Starts with spaces.
	No whitespace.
 > pod="kubedns"`,
		},
		{
			keysValues: []interface{}{"pod", "kubedns", "maps", map[string]int{"three": 4}},
			want:       ` pod="kubedns" maps={"three":4}`,
		},
		{
			keysValues: []interface{}{"pod", klog.KRef("kube-system", "kubedns"), "status", "ready"},
			want:       " pod=\"kube-system/kubedns\" status=\"ready\"",
		},
		{
			keysValues: []interface{}{"pod", klog.KRef("", "kubedns"), "status", "ready"},
			want:       " pod=\"kubedns\" status=\"ready\"",
		},
		{
			keysValues: []interface{}{"pod", klog.KObj(test.KMetadataMock{Name: "test-name", NS: "test-ns"}), "status", "ready"},
			want:       " pod=\"test-ns/test-name\" status=\"ready\"",
		},
		{
			keysValues: []interface{}{"pod", klog.KObj(test.KMetadataMock{Name: "test-name", NS: ""}), "status", "ready"},
			want:       " pod=\"test-name\" status=\"ready\"",
		},
		{
			keysValues: []interface{}{"pod", klog.KObj(nil), "status", "ready"},
			want:       " pod=\"\" status=\"ready\"",
		},
		{
			keysValues: []interface{}{"pod", klog.KObj((*test.PtrKMetadataMock)(nil)), "status", "ready"},
			want:       " pod=\"\" status=\"ready\"",
		},
		{
			keysValues: []interface{}{"pod", klog.KObj((*test.KMetadataMock)(nil)), "status", "ready"},
			want:       " pod=\"\" status=\"ready\"",
		},
		{
			keysValues: []interface{}{"pods", klog.KObjs([]test.KMetadataMock{
				{
					Name: "kube-dns",
					NS:   "kube-system",
				},
				{
					Name: "mi-conf",
				},
			})},
			want: ` pods=[{"name":"kube-dns","namespace":"kube-system"},{"name":"mi-conf"}]`,
		},
		{
			keysValues: []interface{}{"point-1", point{100, 200}, "point-2", emptyPoint},
			want:       " point-1=\"x=100, y=200\" point-2=\"<panic: value method k8s.io/klog/v2/internal/serialize_test.point.String called using nil *point pointer>\"",
		},
		{
			keysValues: []interface{}{struct{ key string }{key: "k1"}, "value"},
			want:       " {k1}=\"value\"",
		},
		{
			keysValues: []interface{}{1, "test"},
			want:       " %!s(int=1)=\"test\"",
		},
		{
			keysValues: []interface{}{map[string]string{"k": "key"}, "value"},
			want:       " map[k:key]=\"value\"",
		},
	}

	for _, d := range testKVList {
		b := &bytes.Buffer{}
		serialize.KVListFormat(b, d.keysValues...)
		if b.String() != d.want {
			t.Errorf("KVListFormat error:\n got:\n\t%s\nwant:\t%s", b.String(), d.want)
		}
	}
}

func TestDuplicates(t *testing.T) {
	for name, test := range map[string]struct {
		first, second []interface{}
		expected      []interface{}
	}{
		"empty": {},
		"no-duplicates": {
			first:    makeKV("a", 3),
			second:   makeKV("b", 3),
			expected: append(makeKV("a", 3), makeKV("b", 3)...),
		},
		"all-duplicates": {
			first:    makeKV("a", 3),
			second:   makeKV("a", 3),
			expected: makeKV("a", 3),
		},
		"start-duplicate": {
			first:    append([]interface{}{"x", 1}, makeKV("a", 3)...),
			second:   append([]interface{}{"x", 2}, makeKV("b", 3)...),
			expected: append(append(makeKV("a", 3), "x", 2), makeKV("b", 3)...),
		},
		"subset-first": {
			first:    append([]interface{}{"x", 1}, makeKV("a", 3)...),
			second:   append([]interface{}{"x", 2}, makeKV("a", 3)...),
			expected: append([]interface{}{"x", 2}, makeKV("a", 3)...),
		},
		"subset-second": {
			first:    append([]interface{}{"x", 1}, makeKV("a", 1)...),
			second:   append([]interface{}{"x", 2}, makeKV("b", 2)...),
			expected: append(append(makeKV("a", 1), "x", 2), makeKV("b", 2)...),
		},
		"end-duplicate": {
			first:    append(makeKV("a", 3), "x", 1),
			second:   append(makeKV("b", 3), "x", 2),
			expected: append(makeKV("a", 3), append(makeKV("b", 3), "x", 2)...),
		},
		"middle-duplicate": {
			first:    []interface{}{"a-0", 0, "x", 1, "a-1", 2},
			second:   []interface{}{"b-0", 0, "x", 2, "b-1", 2},
			expected: []interface{}{"a-0", 0, "a-1", 2, "b-0", 0, "x", 2, "b-1", 2},
		},
		"internal-duplicates": {
			first:  []interface{}{"a", 0, "x", 1, "a", 2},
			second: []interface{}{"b", 0, "x", 2, "b", 2},
			// This is the case where Merged keeps key/value pairs
			// that were already duplicated inside the slices, for
			// performance.
			expected: []interface{}{"a", 0, "a", 2, "b", 0, "x", 2, "b", 2},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("Merged", func(t *testing.T) {
				actual := serialize.MergeKVs(test.first, test.second)
				expectEqual(t, "merged key/value pairs", test.expected, actual)
			})
		})
	}
}

// BenchmarkMergeKVs checks performance when MergeKVs is called with two slices.
// In practice that is how the function is used.
func BenchmarkMergeKVs(b *testing.B) {
	for firstLength := 0; firstLength < 10; firstLength++ {
		firstA := makeKV("a", firstLength)
		for secondLength := 0; secondLength < 10; secondLength++ {
			secondA := makeKV("a", secondLength)
			secondB := makeKV("b", secondLength)
			b.Run(fmt.Sprintf("%dx%d", firstLength, secondLength), func(b *testing.B) {
				// This is the most common case: all key/value pairs are kept.
				b.Run("no-duplicates", func(b *testing.B) {
					expected := append(firstA, secondB...)
					benchMergeKVs(b, expected, firstA, secondB)
				})

				// Fairly unlikely...
				b.Run("all-duplicates", func(b *testing.B) {
					var expected []interface{}
					if firstLength > secondLength {
						expected = firstA[secondLength*2:]
					}
					expected = append(expected, secondA...)
					benchMergeKVs(b, expected, firstA, secondA)
				})

				// First entry is the same.
				b.Run("start-duplicate", func(b *testing.B) {
					first := []interface{}{"x", 1}
					first = append(first, firstA...)
					second := []interface{}{"x", 1}
					second = append(second, secondB...)
					expected := append(firstA, second...)
					benchMergeKVs(b, expected, first, second)
				})

				// Last entry is the same.
				b.Run("end-duplicate", func(b *testing.B) {
					first := firstA[:]
					first = append(first, "x", 1)
					second := secondB[:]
					second = append(second, "x", 1)
					expected := append(firstA, second...)
					benchMergeKVs(b, expected, first, second)
				})
			})
		}
	}
}

func makeKV(prefix string, length int) []interface{} {
	if length == 0 {
		return []interface{}{}
	}
	kv := make([]interface{}, 0, length*2)
	for i := 0; i < length; i++ {
		kv = append(kv, fmt.Sprintf("%s-%d", prefix, i), i)
	}
	return kv
}

func benchMergeKVs(b *testing.B, expected []interface{}, first, second []interface{}) {
	if len(expected) == 0 {
		expected = nil
	}
	actual := serialize.MergeKVs(first, second)
	expectEqual(b, "trimmed key/value pairs", expected, actual)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serialize.MergeKVs(first, second)
	}
}

func expectEqual(tb testing.TB, what string, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		tb.Fatalf("Did not get correct %s. Expected:\n    %v\nActual:\n    %v", what, expected, actual)
	}
}

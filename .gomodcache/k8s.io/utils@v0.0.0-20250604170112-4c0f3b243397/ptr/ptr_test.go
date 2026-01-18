/*
Copyright 2023 The Kubernetes Authors.

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

package ptr_test

import (
	"fmt"
	"testing"

	"k8s.io/utils/ptr"
)

func TestAllPtrFieldsNil(t *testing.T) {
	testCases := []struct {
		obj      interface{}
		expected bool
	}{
		{struct{}{}, true},
		{struct{ Foo int }{12345}, true},
		{&struct{ Foo int }{12345}, true},
		{struct{ Foo *int }{nil}, true},
		{&struct{ Foo *int }{nil}, true},
		{struct {
			Foo int
			Bar *int
		}{12345, nil}, true},
		{&struct {
			Foo int
			Bar *int
		}{12345, nil}, true},
		{struct {
			Foo *int
			Bar *int
		}{nil, nil}, true},
		{&struct {
			Foo *int
			Bar *int
		}{nil, nil}, true},
		{struct{ Foo *int }{new(int)}, false},
		{&struct{ Foo *int }{new(int)}, false},
		{struct {
			Foo *int
			Bar *int
		}{nil, new(int)}, false},
		{&struct {
			Foo *int
			Bar *int
		}{nil, new(int)}, false},
		{(*struct{})(nil), true},
	}
	for i, tc := range testCases {
		name := fmt.Sprintf("case[%d]", i)
		t.Run(name, func(t *testing.T) {
			if actual := ptr.AllPtrFieldsNil(tc.obj); actual != tc.expected {
				t.Errorf("%s: expected %t, got %t", name, tc.expected, actual)
			}
		})
	}
}

func TestRef(t *testing.T) {
	type T int

	val := T(0)
	pointer := ptr.To(val)
	if *pointer != val {
		t.Errorf("expected %d, got %d", val, *pointer)
	}

	val = T(1)
	pointer = ptr.To(val)
	if *pointer != val {
		t.Errorf("expected %d, got %d", val, *pointer)
	}
}

func TestDeref(t *testing.T) {
	type T int

	var val, def T = 1, 0

	out := ptr.Deref(&val, def)
	if out != val {
		t.Errorf("expected %d, got %d", val, out)
	}

	out = ptr.Deref(nil, def)
	if out != def {
		t.Errorf("expected %d, got %d", def, out)
	}
}

func TestEqual(t *testing.T) {
	type T int

	if !ptr.Equal[T](nil, nil) {
		t.Errorf("expected true (nil == nil)")
	}
	if !ptr.Equal(ptr.To(T(123)), ptr.To(T(123))) {
		t.Errorf("expected true (val == val)")
	}
	if ptr.Equal(nil, ptr.To(T(123))) {
		t.Errorf("expected false (nil != val)")
	}
	if ptr.Equal(ptr.To(T(123)), nil) {
		t.Errorf("expected false (val != nil)")
	}
	if ptr.Equal(ptr.To(T(123)), ptr.To(T(456))) {
		t.Errorf("expected false (val != val)")
	}
}

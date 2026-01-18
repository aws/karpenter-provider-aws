package mergo_test

import (
	"testing"

	"github.com/imdario/mergo"
)

type foz struct {
	A *bool
	B string
	C *bool
	D *bool
	E *bool
}

func TestIssue131MergeWithOverwriteWithEmptyValue(t *testing.T) {
	src := foz{
		A: func(v bool) *bool { return &v }(false),
		B: "src",
	}
	dest := foz{
		A: func(v bool) *bool { return &v }(true),
		B: "dest",
	}
	if err := mergo.Merge(&dest, src, mergo.WithOverwriteWithEmptyValue); err != nil {
		t.Error(err)
	}
	if *src.A != *dest.A {
		t.Errorf("dest.A not merged in properly: %v != %v", *src.A, *dest.A)
	}
	if src.B != dest.B {
		t.Errorf("dest.B not merged in properly: %v != %v", src.B, dest.B)
	}
}

func TestIssue131MergeWithoutDereferenceWithOverride(t *testing.T) {
	src := foz{
		A: func(v bool) *bool { return &v }(false),
		B: "src",
		C: nil,
		D: func(v bool) *bool { return &v }(false),
		E: func(v bool) *bool { return &v }(true),
	}
	dest := foz{
		A: func(v bool) *bool { return &v }(true),
		B: "dest",
		C: func(v bool) *bool { return &v }(false),
		D: nil,
		E: func(v bool) *bool { return &v }(false),
	}
	if err := mergo.Merge(&dest, src, mergo.WithoutDereference, mergo.WithOverride); err != nil {
		t.Error(err)
	}
	if *src.A != *dest.A {
		t.Errorf("dest.A not merged in properly: %v != %v", *src.A, *dest.A)
	}
	if src.B != dest.B {
		t.Errorf("dest.B not merged in properly: %v != %v", src.B, dest.B)
	}
	if *dest.C != false {
		t.Errorf("dest.C not merged in properly: %v != %v", *src.C, *dest.C)
	}
	if *dest.D != false {
		t.Errorf("dest.D not merged in properly: %v != %v", src.D, *dest.D)
	}
	if *dest.E != true {
		t.Errorf("dest.E not merged in properly: %v != %v", *src.E, *dest.E)
	}
}

func TestIssue131MergeWithoutDereference(t *testing.T) {
	src := foz{
		A: func(v bool) *bool { return &v }(false),
		B: "src",
		C: nil,
		D: func(v bool) *bool { return &v }(false),
		E: func(v bool) *bool { return &v }(true),
	}
	dest := foz{
		A: func(v bool) *bool { return &v }(true),
		B: "dest",
		C: func(v bool) *bool { return &v }(false),
		D: nil,
		E: func(v bool) *bool { return &v }(false),
	}
	if err := mergo.Merge(&dest, src, mergo.WithoutDereference); err != nil {
		t.Error(err)
	}
	if *src.A == *dest.A {
		t.Errorf("dest.A should not have been merged: %v == %v", *src.A, *dest.A)
	}
	if src.B == dest.B {
		t.Errorf("dest.B should not have been merged: %v == %v", src.B, dest.B)
	}
	if *dest.C != false {
		t.Errorf("dest.C not merged in properly: %v != %v", *src.C, *dest.C)
	}
	if *dest.D != false {
		t.Errorf("dest.D not merged in properly: %v != %v", src.D, *dest.D)
	}
	if *dest.E == true {
		t.Errorf("dest.Eshould not have been merged: %v == %v", *src.E, *dest.E)
	}
}

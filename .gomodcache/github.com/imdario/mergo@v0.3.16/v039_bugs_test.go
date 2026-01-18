package mergo_test

import (
	"testing"

	"github.com/imdario/mergo"
)

type inner struct {
	A int
}

type outer struct {
	inner
	B int
}

func TestV039Issue139(t *testing.T) {
	dst := outer{
		inner: inner{A: 1},
		B:     2,
	}
	src := outer{
		inner: inner{A: 10},
		B:     20,
	}
	err := mergo.MergeWithOverwrite(&dst, src)
	if err != nil {
		panic(err.Error())
	}
	if dst.inner.A == 1 {
		t.Errorf("expected %d, got %d", src.inner.A, dst.inner.A)
	}
}

func TestV039Issue152(t *testing.T) {
	dst := map[string]interface{}{
		"properties": map[string]interface{}{
			"field1": map[string]interface{}{
				"type": "text",
			},
			"field2": "ohai",
		},
	}
	src := map[string]interface{}{
		"properties": map[string]interface{}{
			"field1": "wrong",
		},
	}
	if err := mergo.Map(&dst, src, mergo.WithOverride); err != nil {
		t.Error(err)
	}
}

type issue146Foo struct {
	B map[string]issue146Bar
	A string
}

type issue146Bar struct {
	C *string
	D *string
}

func TestV039Issue146(t *testing.T) {
	var (
		s1 = "asd"
		s2 = "sdf"
	)
	dst := issue146Foo{
		A: "two",
		B: map[string]issue146Bar{
			"foo": {
				C: &s1,
			},
		},
	}
	src := issue146Foo{
		A: "one",
		B: map[string]issue146Bar{
			"foo": {
				D: &s2,
			},
		},
	}
	if err := mergo.Merge(&dst, src, mergo.WithOverride); err != nil {
		t.Error(err)
	}
	if dst.B["foo"].D == nil {
		t.Errorf("expected %v, got nil", &s2)
	}
}

package mergo_test

import (
	"reflect"
	"testing"

	"github.com/imdario/mergo"
)

func TestMergeWithTransformerZeroValue(t *testing.T) {
	// This test specifically tests that a transformer can be used to
	// prevent overwriting a zero value (in this case a bool). This would fail prior to #211
	type fooWithBoolPtr struct {
		b *bool
	}
	var Bool = func(b bool) *bool { return &b }
	a := fooWithBoolPtr{b: Bool(false)}
	b := fooWithBoolPtr{b: Bool(true)}

	if err := mergo.Merge(&a, &b, mergo.WithTransformers(&transformer{
		m: map[reflect.Type]func(dst, src reflect.Value) error{
			reflect.TypeOf(Bool(false)): func(dst, src reflect.Value) error {
				if dst.CanSet() && dst.IsNil() {
					dst.Set(src)
				}
				return nil
			},
		},
	})); err != nil {
		t.Error(err)
	}

	if *a.b != false {
		t.Errorf("b not merged in properly: a.b(%v) != expected(%v)", a.b, false)
	}
}

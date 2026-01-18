package mergo_test

import (
	"github.com/imdario/mergo"
	"reflect"
	"testing"
)

type structWithStringMap struct {
	Data map[string]string
}

func TestIssue90(t *testing.T) {
	dst := map[string]structWithStringMap{
		"struct": {
			Data: nil,
		},
	}
	src := map[string]structWithStringMap{
		"struct": {
			Data: map[string]string{
				"foo": "bar",
			},
		},
	}
	expected := map[string]structWithStringMap{
		"struct": {
			Data: map[string]string{
				"foo": "bar",
			},
		},
	}

	err := mergo.Merge(&dst, src, mergo.WithOverride)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if !reflect.DeepEqual(dst, expected) {
		t.Errorf("expected: %#v\ngot: %#v", expected, dst)
	}
}

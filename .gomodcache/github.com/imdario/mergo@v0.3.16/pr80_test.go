package mergo_test

import (
	"testing"

	"github.com/imdario/mergo"
)

type mapInterface map[string]interface{}

func TestMergeMapsEmptyString(t *testing.T) {
	a := mapInterface{"s": ""}
	b := mapInterface{"s": "foo"}
	if err := mergo.Merge(&a, b); err != nil {
		t.Error(err)
	}
	if a["s"] != "foo" {
		t.Errorf("b not merged in properly: a.s.Value(%s) != expected(%s)", a["s"], "foo")
	}
}

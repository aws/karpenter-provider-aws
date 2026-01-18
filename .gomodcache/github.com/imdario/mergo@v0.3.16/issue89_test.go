package mergo_test

import (
	"testing"

	"github.com/imdario/mergo"
)

func TestIssue89Boolean(t *testing.T) {
	type Foo struct {
		Bar bool `json:"bar"`
	}

	src := Foo{Bar: true}
	dst := Foo{Bar: false}

	if err := mergo.Merge(&dst, src); err != nil {
		t.Error(err)
	}
	if dst.Bar == false {
		t.Errorf("expected true, got false")
	}
}

func TestIssue89MergeWithEmptyValue(t *testing.T) {
	p1 := map[string]interface{}{
		"A": 3, "B": "note", "C": true,
	}
	p2 := map[string]interface{}{
		"B": "", "C": false,
	}
	if err := mergo.Merge(&p1, p2, mergo.WithOverwriteWithEmptyValue); err != nil {
		t.Error(err)
	}
	testCases := []struct {
		expected interface{}
		key      string
	}{
		{
			"",
			"B",
		},
		{
			false,
			"C",
		},
	}
	for _, tC := range testCases {
		if p1[tC.key] != tC.expected {
			t.Errorf("expected %v in p1[%q], got %v", tC.expected, tC.key, p1[tC.key])
		}
	}
}

package tests

import (
	"testing"

	"github.com/mailru/easyjson"
)

func TestStringIntern(t *testing.T) {
	data := []byte(`{"field": "string interning test"}`)

	var i Intern
	allocsPerRun := testing.AllocsPerRun(1000, func() {
		i = Intern{}
		err := easyjson.Unmarshal(data, &i)
		if err != nil {
			t.Error(err)
		}
		if i.Field != "string interning test" {
			t.Fatalf("wrong value: %q", i.Field)
		}
	})
	if allocsPerRun != 1 {
		t.Fatalf("expected 1 allocs, got %f", allocsPerRun)
	}

	var n NoIntern
	allocsPerRun = testing.AllocsPerRun(1000, func() {
		n = NoIntern{}
		err := easyjson.Unmarshal(data, &n)
		if err != nil {
			t.Error(err)
		}
		if n.Field != "string interning test" {
			t.Fatalf("wrong value: %q", n.Field)
		}
	})
	if allocsPerRun != 2 {
		t.Fatalf("expected 2 allocs, got %f", allocsPerRun)
	}
}

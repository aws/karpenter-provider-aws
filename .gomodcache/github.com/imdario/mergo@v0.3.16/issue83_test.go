package mergo_test

import (
	"testing"

	"github.com/imdario/mergo"
)

type issue83My struct {
	Data []int
}

func TestIssue83(t *testing.T) {
	dst := issue83My{Data: []int{1, 2, 3}}
	new := issue83My{}
	if err := mergo.Merge(&dst, new, mergo.WithOverwriteWithEmptyValue); err != nil {
		t.Error(err)
	}
	if len(dst.Data) > 0 {
		t.Errorf("expected empty slice, got %v", dst.Data)
	}
}

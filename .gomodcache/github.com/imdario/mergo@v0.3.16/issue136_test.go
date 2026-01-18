package mergo_test

import (
	"testing"

	"github.com/imdario/mergo"
)

type embeddedTestA struct {
	Name string
	Age  uint8
}

type embeddedTestB struct {
	Address string
	embeddedTestA
}

func TestMergeEmbedded(t *testing.T) {
	var (
		err error
		a   = &embeddedTestA{
			"Suwon", 16,
		}
		b = &embeddedTestB{}
	)

	if err := mergo.Merge(&b.embeddedTestA, *a); err != nil {
		t.Error(err)
	}

	if b.Name != "Suwon" {
		t.Errorf("%v %v", b.Name, err)
	}
}

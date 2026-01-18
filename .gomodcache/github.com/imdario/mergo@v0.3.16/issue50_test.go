package mergo_test

import (
	"testing"
	"time"

	"github.com/imdario/mergo"
)

type testStruct struct {
	time.Duration
}

func TestIssue50Merge(t *testing.T) {
	to := testStruct{}
	from := testStruct{}

	if err := mergo.Merge(&to, from); err != nil {
		t.Fail()
	}
}

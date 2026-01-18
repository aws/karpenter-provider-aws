package mergo_test

import (
	"reflect"
	"testing"

	"github.com/imdario/mergo"
)

func TestIssue61MergeNilMap(t *testing.T) {
	type T struct {
		I map[string][]string
	}
	t1 := T{}
	t2 := T{I: map[string][]string{"hi": {"there"}}}

	if err := mergo.Merge(&t1, t2); err != nil {
		t.Fail()
	}

	if !reflect.DeepEqual(t2, T{I: map[string][]string{"hi": {"there"}}}) {
		t.FailNow()
	}
}

package mergo_test

import (
	"testing"

	"github.com/imdario/mergo"
)

var testDataS = []struct {
	S1            Student
	S2            Student
	ExpectedSlice []string
}{
	{Student{"Jack", []string{"a", "B"}}, Student{"Tom", []string{"1"}}, []string{"1", "a", "B"}},
	{Student{"Jack", []string{"a", "B"}}, Student{"Tom", []string{}}, []string{"a", "B"}},
	{Student{"Jack", []string{}}, Student{"Tom", []string{"1"}}, []string{"1"}},
	{Student{"Jack", []string{}}, Student{"Tom", []string{}}, []string{}},
}

func TestMergeSliceWithOverrideWithAppendSlice(t *testing.T) {
	for _, data := range testDataS {
		err := mergo.Merge(&data.S2, data.S1, mergo.WithOverride, mergo.WithAppendSlice)
		if err != nil {
			t.Errorf("Error while merging %s", err)
		}

		if len(data.S2.Books) != len(data.ExpectedSlice) {
			t.Errorf("Got %d elements in slice, but expected %d", len(data.S2.Books), len(data.ExpectedSlice))
		}

		for i, val := range data.S2.Books {
			if val != data.ExpectedSlice[i] {
				t.Errorf("Expected %s, but got %s while merging slice with override", data.ExpectedSlice[i], val)
			}
		}
	}
}

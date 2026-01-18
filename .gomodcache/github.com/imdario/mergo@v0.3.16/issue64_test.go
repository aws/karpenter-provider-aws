package mergo_test

import (
	"testing"

	"github.com/imdario/mergo"
)

type Student struct {
	Name  string
	Books []string
}

type issue64TestData struct {
	S1            Student
	S2            Student
	ExpectedSlice []string
}

func issue64Data() []issue64TestData {
	return []issue64TestData{
		{Student{"Jack", []string{"a", "B"}}, Student{"Tom", []string{"1"}}, []string{"a", "B"}},
		{Student{"Jack", []string{"a", "B"}}, Student{"Tom", []string{}}, []string{"a", "B"}},
		{Student{"Jack", []string{}}, Student{"Tom", []string{"1"}}, []string{"1"}},
		{Student{"Jack", []string{}}, Student{"Tom", []string{}}, []string{}},
	}
}

func TestIssue64MergeSliceWithOverride(t *testing.T) {
	for _, data := range issue64Data() {
		err := mergo.Merge(&data.S2, data.S1, mergo.WithOverride)
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

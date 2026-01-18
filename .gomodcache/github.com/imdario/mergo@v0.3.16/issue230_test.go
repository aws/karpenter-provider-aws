package mergo_test

import (
	"testing"

	"github.com/imdario/mergo"
)

var testDataM = []struct {
	M1                     mapTest
	M2                     mapTest
	WithOverrideEmptyValue bool
	ExpectedMap            map[int]int
}{
	{
		M1: mapTest{
			M: map[int]int{1: 1, 3: 3},
		},
		M2: mapTest{
			M: map[int]int{1: 2, 2: 2},
		},
		WithOverrideEmptyValue: true,
		ExpectedMap:            map[int]int{1: 1, 3: 3},
	},
	{
		M1: mapTest{
			M: map[int]int{1: 1, 3: 3},
		},
		M2: mapTest{
			M: map[int]int{1: 2, 2: 2},
		},
		WithOverrideEmptyValue: false,
		ExpectedMap:            map[int]int{1: 1, 2: 2, 3: 3},
	},
	{
		M1: mapTest{
			M: map[int]int{},
		},
		M2: mapTest{
			M: map[int]int{1: 2, 2: 2},
		},
		WithOverrideEmptyValue: true,
		ExpectedMap:            map[int]int{},
	},
	{
		M1: mapTest{
			M: map[int]int{},
		},
		M2: mapTest{
			M: map[int]int{1: 2, 2: 2},
		},
		WithOverrideEmptyValue: false,
		ExpectedMap:            map[int]int{1: 2, 2: 2},
	},
}

func withOverrideEmptyValue(enable bool) func(*mergo.Config) {
	if enable {
		return mergo.WithOverwriteWithEmptyValue
	}

	return mergo.WithOverride
}

func TestMergeMapWithOverride(t *testing.T) {
	t.Parallel()

	for _, data := range testDataM {
		err := mergo.Merge(&data.M2, data.M1, withOverrideEmptyValue(data.WithOverrideEmptyValue))
		if err != nil {
			t.Errorf("Error while merging %s", err)
		}

		if len(data.M2.M) != len(data.ExpectedMap) {
			t.Errorf("Got %d elements in map, but expected %d", len(data.M2.M), len(data.ExpectedMap))
			return
		}

		for i, val := range data.M2.M {
			if val != data.ExpectedMap[i] {
				t.Errorf("Expected value: %d, but got %d while merging map", data.ExpectedMap[i], val)
			}
		}
	}
}

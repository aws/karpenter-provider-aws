package query

import (
	"fmt"
	"strconv"
	"testing"
)

var output string

func Benchmark_sprintf_strings(b *testing.B) {
	for i := 0; i < b.N; i++ {
		output = fmt.Sprintf("%s.%s", "foo", "bar")
	}
}

func Benchmark_concat_strings(b *testing.B) {
	for i := 0; i < b.N; i++ {
		output = "foo" + keySeparator + "bar"
	}
}

func Benchmark_int_formatting(b *testing.B) {
	benchmarkFuncs := []struct {
		name      string
		formatter func(val int32)
	}{
		{
			name: "array - sprintf", formatter: func(val int32) {
				output = fmt.Sprintf("%s.%d", "foo", val)
			},
		},
		{
			name: "array - concat strconv", formatter: func(val int32) {
				output = "foo" + keySeparator + strconv.FormatInt(int64(val), 10)
			},
		},
		{
			name: "map - sprintf", formatter: func(val int32) {
				output = fmt.Sprintf("%s.%d.%s", "foo", val, "bar")
				output = fmt.Sprintf("%s.%d.%s", "foo", val, "bar")
			},
		},
		{
			name: "map - concat strconv", formatter: func(val int32) {
				valString := strconv.FormatInt(int64(val), 10)
				output = "foo" + keySeparator + valString + keySeparator + "bar"
				output = "foo" + keySeparator + valString + keySeparator + "bar"
			},
		},
	}

	sizesToTest := []int32{1, 10, 100, 250, 500, 1000}

	for _, bm := range benchmarkFuncs {
		for _, size := range sizesToTest {
			b.Run(fmt.Sprintf("%s with %d size", bm.name, size), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					bm.formatter(size)
				}
			})
		}
	}
}

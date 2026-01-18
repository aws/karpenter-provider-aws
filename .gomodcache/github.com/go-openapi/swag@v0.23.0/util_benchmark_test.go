package swag

import (
	"fmt"
	"io"
	"testing"
)

func BenchmarkToXXXName(b *testing.B) {
	samples := []string{
		"sample text",
		"sample-text",
		"sample_text",
		"sampleText",
		"sample 2 Text",
		"findThingById",
		"日本語sample 2 Text",
		"日本語findThingById",
		"findTHINGSbyID",
	}

	b.Run("ToGoName", benchmarkFunc(ToGoName, samples))
	b.Run("ToVarName", benchmarkFunc(ToVarName, samples))
	b.Run("ToFileName", benchmarkFunc(ToFileName, samples))
	b.Run("ToCommandName", benchmarkFunc(ToCommandName, samples))
	b.Run("ToHumanNameLower", benchmarkFunc(ToHumanNameLower, samples))
	b.Run("ToHumanNameTitle", benchmarkFunc(ToHumanNameTitle, samples))
}

func benchmarkFunc(fn func(string) string, samples []string) func(*testing.B) {
	return func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		var res string
		for i := 0; i < b.N; i++ {
			res = fn(samples[i%len(samples)])
		}

		fmt.Fprintln(io.Discard, res)
	}
}

package easyjson

import "testing"

func BenchmarkNilCheck(b *testing.B) {
	var a *int
	for i := 0; i < b.N; i++ {
		if !isNilInterface(a) {
			b.Fatal("expected it to be nil")
		}
	}
}

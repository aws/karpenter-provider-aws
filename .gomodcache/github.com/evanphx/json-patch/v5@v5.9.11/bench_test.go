package jsonpatch

import "testing"

func BenchmarkMergePatch(b *testing.B) {
	original := []byte(`{"name": "John", "age": 24, "height": 3.21}`)
	target := []byte(`{"name": "Jane", "age": 24}`)
	alternative := []byte(`{"name": "Tina", "age": 28, "height": 3.75}`)

	patch, err := CreateMergePatch(original, target)
	if err != nil {
		panic(err)
	}

	for n := 0; n < b.N; n++ {
		MergePatch(alternative, patch)
	}
}

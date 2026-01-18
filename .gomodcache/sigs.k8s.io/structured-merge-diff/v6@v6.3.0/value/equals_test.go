/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package value_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	yaml "go.yaml.in/yaml/v2"
	"sigs.k8s.io/structured-merge-diff/v6/value"
)

func testdata(file string) string {
	return filepath.Join("..", "internal", "testdata", file)
}

func read(file string) []byte {
	s, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	return s
}

func BenchmarkEquals(b *testing.B) {
	benches := []struct {
		filename string
	}{
		{
			filename: "pod.yaml",
		},
		{
			filename: "endpoints.yaml",
		},
		{
			filename: "list.yaml",
		},
		{
			filename: "node.yaml",
		},
		{
			filename: "prometheus-crd.yaml",
		},
	}

	for _, bench := range benches {
		b.Run(bench.filename, func(b *testing.B) {
			var obj interface{}
			err := yaml.Unmarshal(read(testdata(bench.filename)), &obj)
			if err != nil {
				b.Fatalf("Failed to unmarshal object: %v", err)
			}
			v := value.NewValueInterface(obj)
			b.Run("Equals", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if !value.Equals(v, v) {
						b.Fatalf("Object should be equal")
					}
				}
			})
			b.Run("EqualsUsingFreelist", func(b *testing.B) {
				b.ReportAllocs()
				a := value.NewFreelistAllocator()
				for i := 0; i < b.N; i++ {
					if !value.EqualsUsing(a, v, v) {
						b.Fatalf("Object should be equal")
					}
				}
			})
		})
	}
}

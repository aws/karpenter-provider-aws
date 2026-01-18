/*
Copyright 2019 The Kubernetes Authors.

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

package merge_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	. "sigs.k8s.io/structured-merge-diff/v6/internal/fixture"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
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

func loadParser(name string) Parser {
	s := read(testdata(name))
	parser, err := typed.NewParser(typed.YAMLObject(s))
	if err != nil {
		panic(err)
	}
	return parser
}

var k8s = loadParser("k8s-schema.yaml")
var apiresourceimport = loadParser("apiresourceimport.yaml")
var k8s100pctOverrides = loadParser("k8s-schema-100pct-fieldoverride.yaml")
var k8s10pctOverrides = loadParser("k8s-schema-10pct-fieldoverride.yaml")

func BenchmarkOperations(b *testing.B) {
	benches := []struct {
		name      string
		parseType typed.ParseableType
		filename  string
	}{
		{
			name:      "Pod",
			parseType: k8s.Type("io.k8s.api.core.v1.Pod"),
			filename:  "pod.yaml",
		},
		{
			name:      "Node",
			parseType: k8s.Type("io.k8s.api.core.v1.Node"),
			filename:  "node.yaml",
		},
		{
			name:      "Endpoints",
			parseType: k8s.Type("io.k8s.api.core.v1.Endpoints"),
			filename:  "endpoints.yaml",
		},
		{
			name:      "Node100%override",
			parseType: k8s100pctOverrides.Type("io.k8s.api.core.v1.Node"),
			filename:  "node.yaml",
		},
		{
			name:      "Node10%override",
			parseType: k8s10pctOverrides.Type("io.k8s.api.core.v1.Node"),
			filename:  "node.yaml",
		},
		{
			name:      "Endpoints100%override",
			parseType: k8s100pctOverrides.Type("io.k8s.api.core.v1.Endpoints"),
			filename:  "endpoints.yaml",
		},
		{
			name:      "Endpoints10%override",
			parseType: k8s10pctOverrides.Type("io.k8s.api.core.v1.Endpoints"),
			filename:  "endpoints.yaml",
		},
		{
			name:      "PrometheusCRD",
			parseType: k8s.Type("io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1beta1.CustomResourceDefinition"),
			filename:  "prometheus-crd.yaml",
		},
		{
			name:      "apiresourceimport",
			parseType: apiresourceimport.Type("apiresourceimport"),
			filename:  "apiresourceimport-cr.yaml",
		},
	}

	for _, bench := range benches {
		b.Run(bench.name, func(b *testing.B) {
			obj := typed.YAMLObject(read(testdata(bench.filename)))
			tests := []struct {
				name              string
				returnInputonNoop bool
				ops               []Operation
			}{
				{
					name: "Create",
					ops: []Operation{
						Update{
							Manager:    "controller",
							APIVersion: "v1",
							Object:     obj,
						},
					},
				},
				{
					name: "Apply",
					ops: []Operation{
						Apply{
							Manager:    "controller",
							APIVersion: "v1",
							Object:     obj,
						},
					},
				},
				{
					name: "ApplyTwice",
					ops: []Operation{
						Apply{
							Manager:    "controller",
							APIVersion: "v1",
							Object:     obj,
						},
						Apply{
							Manager:    "other-controller",
							APIVersion: "v1",
							Object:     obj,
						},
					},
				},
				{
					name:              "ApplyTwiceNoCompare",
					returnInputonNoop: true,
					ops: []Operation{
						Apply{
							Manager:    "controller",
							APIVersion: "v1",
							Object:     obj,
						},
						Apply{
							Manager:    "other-controller",
							APIVersion: "v1",
							Object:     obj,
						},
					},
				},
				{
					name: "Update",
					ops: []Operation{
						Update{
							Manager:    "controller",
							APIVersion: "v1",
							Object:     obj,
						},
						Update{
							Manager:    "other-controller",
							APIVersion: "v1",
							Object:     obj,
						},
					},
				},
				{
					name: "UpdateVersion",
					ops: []Operation{
						Update{
							Manager:    "controller",
							APIVersion: "v1",
							Object:     obj,
						},
						Update{
							Manager:    "other-controller",
							APIVersion: "v2",
							Object:     obj,
						},
					},
				},
			}
			for _, test := range tests {
				b.Run(test.name, func(b *testing.B) {
					tc := TestCase{
						Ops:               test.ops,
						ReturnInputOnNoop: test.returnInputonNoop,
					}
					p := SameVersionParser{T: bench.parseType}
					tc.PreprocessOperations(p)

					b.ReportAllocs()
					b.ResetTimer()
					for n := 0; n < b.N; n++ {
						if err := tc.Bench(p); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}

// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestIntern(t *testing.T) {
	var sc stringCache
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	for i := 0; i <= len(alphabet); i++ {
		want := alphabet[i:]
		if got := sc.make([]byte(want)); got != want {
			t.Fatalf("make = %v, want %v", got, want)
		}
	}
	for i := 0; i < 1000; i++ {
		want := fmt.Sprintf("test%b", i)
		if got := sc.make([]byte(want)); got != want {
			t.Fatalf("make = %v, want %v", got, want)
		}
	}
}

var sink string

func BenchmarkIntern(b *testing.B) {
	datasetStrings := func(name string) (out [][]byte) {
		var data []byte
		for _, ts := range jsonTestdata() {
			if ts.name == name {
				data = ts.data
			}
		}
		dec := NewDecoder(bytes.NewReader(data))
		for {
			k, n := dec.StackIndex(dec.StackDepth())
			isObjectName := k == '{' && n%2 == 0
			tok, err := dec.ReadToken()
			if err != nil {
				if err == io.EOF {
					break
				}
				b.Fatalf("ReadToken error: %v", err)
			}
			if tok.Kind() == '"' && !isObjectName {
				out = append(out, []byte(tok.String()))
			}
		}
		return out
	}

	tests := []struct {
		label string
		data  [][]byte
	}{
		// Best is the best case scenario where every string is the same.
		{"Best", func() (out [][]byte) {
			for i := 0; i < 1000; i++ {
				out = append(out, []byte("hello, world!"))
			}
			return out
		}()},

		// Repeat is a sequence of the same set of names repeated.
		// This commonly occurs when unmarshaling a JSON array of JSON objects,
		// where the set of all names is usually small.
		{"Repeat", func() (out [][]byte) {
			for i := 0; i < 100; i++ {
				for _, s := range []string{"first_name", "last_name", "age", "address", "street_address", "city", "state", "postal_code", "phone_numbers", "gender"} {
					out = append(out, []byte(s))
				}
			}
			return out
		}()},

		// Synthea is all string values encountered in the Synthea FHIR dataset.
		{"Synthea", datasetStrings("SyntheaFhir")},

		// Twitter is all string values encountered in the Twitter dataset.
		{"Twitter", datasetStrings("TwitterStatus")},

		// Worst is the worst case scenario where every string is different
		// resulting in wasted time looking up a string that will never match.
		{"Worst", func() (out [][]byte) {
			for i := 0; i < 1000; i++ {
				out = append(out, []byte(fmt.Sprintf("%016x", i)))
			}
			return out
		}()},
	}

	for _, tt := range tests {
		b.Run(tt.label, func(b *testing.B) {
			// Alloc simply heap allocates each string.
			// This provides an upper bound on the number of allocations.
			b.Run("Alloc", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					for _, b := range tt.data {
						sink = string(b)
					}
				}
			})
			// Cache interns strings using stringCache.
			// We want to optimize for having a faster runtime than Alloc,
			// and also keeping the number of allocations closer to GoMap.
			b.Run("Cache", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					var sc stringCache
					for _, b := range tt.data {
						sink = sc.make(b)
					}
				}
			})
			// GoMap interns all strings in a simple Go map.
			// This provides a lower bound on the number of allocations.
			b.Run("GoMap", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					m := make(map[string]string)
					for _, b := range tt.data {
						s, ok := m[string(b)]
						if !ok {
							s = string(b)
							m[s] = s
						}
						sink = s
					}
				}
			})
		})
	}
}

//go:build go1.24
// +build go1.24

/*
Copyright 2025 The Kubernetes Authors.

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

package kyaml

import (
	"strings"
	"testing"
)

func TestKYAMLOutputSyntax_omitzero(t *testing.T) {
	type testCase struct {
		name     string
		input    any
		expected string
	}

	tests := []testCase{
		{"omitzero struct", struct {
			I int    `json:",omitzero"`
			S string `json:",omitzero"`
			B bool   `json:",omitzero"`
			P *int   `json:",omitzero"`
		}{}, "{}"},
		{"omitzero struct nil slice", struct {
			S []int `json:",omitzero"`
		}{}, "{}"},
		{"omitzero struct zero slice", struct {
			S []int `json:",omitzero"`
		}{S: []int{}}, "{\n  S: [],\n}"},
		{"omitzero struct nil map", struct {
			M map[int]int `json:",omitzero"`
		}{}, "{}"},
		{"omitzero struct zero map", struct {
			M map[int]int `json:",omitzero"`
		}{M: map[int]int{}}, "{\n  M: {},\n}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ky := &Encoder{}
			yb, err := ky.Marshal(tt.input)
			if err != nil {
				t.Fatalf("failed to render KYAML: %v", err)
			}
			// always has a newline at the end
			if result := strings.TrimRight(string(yb), "\n"); result != tt.expected {
				t.Errorf("Marshal(%v):\nwanted: %q\n   got: %q", tt.input, tt.expected, result)
			}
		})
	}
}

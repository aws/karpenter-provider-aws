/*
Copyright 2013 Google Inc. All Rights Reserved.
Copyright 2022 The Kubernetes Authors.

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

package verbosity

import (
	"testing"

	"k8s.io/klog/v2/internal/test/require"
)

func TestV(t *testing.T) {
	vs := New()
	require.NoError(t, vs.verbosity.Set("2"))
	depth := 0
	if !vs.Enabled(1, depth) {
		t.Error("not enabled for 1")
	}
	if !vs.Enabled(2, depth) {
		t.Error("not enabled for 2")
	}
	if vs.Enabled(3, depth) {
		t.Error("enabled for 3")
	}
}

func TestVmoduleOn(t *testing.T) {
	vs := New()
	require.NoError(t, vs.vmodule.Set("verbosity_test=2"))
	depth := 0
	if !vs.Enabled(1, depth) {
		t.Error("not enabled for 1")
	}
	if !vs.Enabled(2, depth) {
		t.Error("not enabled for 2")
	}
	if vs.Enabled(3, depth) {
		t.Error("enabled for 3")
	}
	if enabledInHelper(vs, 1) {
		t.Error("enabled for helper at 1")
	}
	if enabledInHelper(vs, 2) {
		t.Error("enabled for helper at 2")
	}
	if enabledInHelper(vs, 3) {
		t.Error("enabled for helper at 3")
	}
}

// vGlobs are patterns that match/don't match this file at V=2.
var vGlobs = map[string]bool{
	// Easy to test the numeric match here.
	"verbosity_test=1": false, // If -vmodule sets V to 1, V(2) will fail.
	"verbosity_test=2": true,
	"verbosity_test=3": true, // If -vmodule sets V to 1, V(3) will succeed.
	// These all use 2 and check the patterns. All are true.
	"*=2":                true,
	"?e*=2":              true,
	"?????????_*=2":      true,
	"??[arx]??????_*t=2": true,
	// These all use 2 and check the patterns. All are false.
	"*x=2":         false,
	"m*=2":         false,
	"??_*=2":       false,
	"?[abc]?_*t=2": false,
}

// Test that vmodule globbing works as advertised.
func testVmoduleGlob(pat string, match bool, t *testing.T) {
	vs := New()
	require.NoError(t, vs.vmodule.Set(pat))
	depth := 0
	actual := vs.Enabled(2, depth)
	if actual != match {
		t.Errorf("incorrect match for %q: got %#v expected %#v", pat, actual, match)
	}
}

// Test that a vmodule globbing works as advertised.
func TestVmoduleGlob(t *testing.T) {
	for glob, match := range vGlobs {
		testVmoduleGlob(glob, match, t)
	}
}

// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.24

package http3

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestFiles checks that every file in this package has a build constraint on Go 1.24.
//
// Package tests rely on testing/synctest, added as an experiment in Go 1.24.
// When moving internal/http3 to an importable location, we can decide whether
// to relax the constraint for non-test files.
//
// Drop this test when the x/net go.mod depends on 1.24 or newer.
func TestFiles(t *testing.T) {
	f, err := os.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	names, err := f.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		// Check for copyright header while we're in here.
		if !bytes.Contains(b, []byte("The Go Authors.")) {
			t.Errorf("%v: missing copyright", name)
		}
		// doc.go doesn't need a build constraint.
		if name == "doc.go" {
			continue
		}
		if !bytes.Contains(b, []byte("//go:build go1.24")) {
			t.Errorf("%v: missing constraint on go1.24", name)
		}
		if bytes.Contains(b, []byte(`"testing/synctest"`)) &&
			!bytes.Contains(b, []byte("//go:build go1.24 && goexperiment.synctest")) {
			t.Errorf("%v: missing constraint on go1.24 && goexperiment.synctest", name)
		}
	}
}

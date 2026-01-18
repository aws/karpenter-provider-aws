// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !go1.25 && goexperiment.synctest

package http2

import (
	"slices"
	"testing"
	"testing/synctest"
)

// synctestTest emulates the Go 1.25 synctest.Test function on Go 1.24.
func synctestTest(t *testing.T, f func(t testing.TB)) {
	t.Helper()
	synctest.Run(func() {
		t.Helper()
		ct := &cleanupT{T: t}
		defer ct.done()
		f(ct)
	})
}

// cleanupT wraps a testing.T and adds its own Cleanup method.
// Used to execute cleanup functions within a synctest bubble.
type cleanupT struct {
	*testing.T
	cleanups []func()
}

// Cleanup replaces T.Cleanup.
func (t *cleanupT) Cleanup(f func()) {
	t.cleanups = append(t.cleanups, f)
}

func (t *cleanupT) done() {
	for _, f := range slices.Backward(t.cleanups) {
		f()
	}
}

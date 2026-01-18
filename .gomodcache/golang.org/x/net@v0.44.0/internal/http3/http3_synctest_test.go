// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.24 && goexperiment.synctest

package http3

import (
	"context"
	"slices"
	"testing"
	"testing/synctest"
)

// runSynctest runs f in a synctest.Run bubble.
// It arranges for t.Cleanup functions to run within the bubble.
// TODO: Replace with synctest.Test, which handles all this properly.
func runSynctest(t *testing.T, f func(t testing.TB)) {
	synctest.Run(func() {
		// Create a context within the bubble, rather than using t.Context.
		ctx, cancel := context.WithCancel(context.Background())
		ct := &cleanupT{
			T:      t,
			ctx:    ctx,
			cancel: cancel,
		}
		defer ct.done()
		f(ct)
	})
}

// runSynctestSubtest runs f in a subtest in a synctest.Run bubble.
func runSynctestSubtest(t *testing.T, name string, f func(t testing.TB)) {
	t.Run(name, func(t *testing.T) {
		runSynctest(t, f)
	})
}

// cleanupT wraps a testing.T and adds its own Cleanup method.
// Used to execute cleanup functions within a synctest bubble.
type cleanupT struct {
	*testing.T
	ctx      context.Context
	cancel   context.CancelFunc
	cleanups []func()
}

// Cleanup replaces T.Cleanup.
func (t *cleanupT) Cleanup(f func()) {
	t.cleanups = append(t.cleanups, f)
}

// Context replaces T.Context.
func (t *cleanupT) Context() context.Context {
	return t.ctx
}

func (t *cleanupT) done() {
	t.cancel()
	for _, f := range slices.Backward(t.cleanups) {
		f()
	}
}

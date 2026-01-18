// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.25

package http2

import (
	"testing"
	"testing/synctest"
)

func synctestTest(t *testing.T, f func(t testing.TB)) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		t.Helper()
		f(t)
	})
}

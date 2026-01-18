// Copyright 2025 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package zstd activates support for zstd compression.
// To enable zstd compression support, import like this:
//
//	import (
//	        _ "github.com/prometheus/client_golang/prometheus/promhttp/zstd"
//	)
//
// This support is currently implemented via the github.com/klauspost/compress library,
// so importing this package requires linking and building that library.
// Once stdlib support is added to the Go standard library (https://github.com/golang/go/issues/62513),
// this package is expected to become a no-op, and zstd support will be enabled by default.
package zstd

import (
	"io"

	"github.com/klauspost/compress/zstd"

	"github.com/prometheus/client_golang/prometheus/promhttp/internal"
)

func init() {
	// Enable zstd support
	internal.NewZstdWriter = func(rw io.Writer) (_ io.Writer, closeWriter func(), _ error) {
		// TODO(mrueg): Replace klauspost/compress with stdlib implementation once https://github.com/golang/go/issues/62513 is implemented, and move this package to be a no-op / backfill for older go versions.
		z, err := zstd.NewWriter(rw, zstd.WithEncoderLevel(zstd.SpeedFastest))
		if err != nil {
			return nil, func() {}, err
		}

		z.Reset(rw)
		return z, func() { _ = z.Close() }, nil
	}
}

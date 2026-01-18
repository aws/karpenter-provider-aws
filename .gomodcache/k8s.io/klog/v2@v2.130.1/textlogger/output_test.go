/*
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

package textlogger_test

import (
	"io"
	"testing"

	"github.com/go-logr/logr"

	"k8s.io/klog/v2/test"
	"k8s.io/klog/v2/textlogger"
)

// These test cover the textlogger, directly and as backend.
func newLogger(out io.Writer, v int, vmodule string) logr.Logger {
	config := textlogger.NewConfig(
		textlogger.Verbosity(v),
		textlogger.Output(out),
	)
	if err := config.VModule().Set(vmodule); err != nil {
		panic(err)
	}
	return textlogger.NewLogger(config)
}

var (
	directConfig   = test.OutputConfig{NewLogger: newLogger, SupportsVModule: true}
	indirectConfig = test.OutputConfig{NewLogger: newLogger, AsBackend: true}
)

func TestTextloggerOutput(t *testing.T) {
	test.InitKlog(t)
	t.Run("direct", func(t *testing.T) {
		test.Output(t, directConfig)
	})
	t.Run("klog-backend", func(t *testing.T) {
		test.Output(t, indirectConfig)
	})
}

func BenchmarkTextloggerOutput(b *testing.B) {
	test.InitKlog(b)
	test.Benchmark(b, directConfig)
}

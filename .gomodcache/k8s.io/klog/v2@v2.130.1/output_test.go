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

package klog_test

import (
	"io"
	"testing"

	"github.com/go-logr/logr"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/test"
)

// klogConfig tests klog output without a logger.
var klogConfig = test.OutputConfig{}

func TestKlogOutput(t *testing.T) {
	test.InitKlog(t)
	test.Output(t, klogConfig)
}

func BenchmarkKlogOutput(b *testing.B) {
	test.InitKlog(b)
	test.Benchmark(b, klogConfig)
}

// klogKlogrConfig tests klogr output via klog, using the klog/v2 klogr.
var klogKLogrConfig = test.OutputConfig{
	NewLogger: func(_ io.Writer, _ int, _ string) logr.Logger {
		return klog.NewKlogr()
	},
}

func TestKlogrOutput(t *testing.T) {
	test.InitKlog(t)
	test.Output(t, klogKLogrConfig)
}

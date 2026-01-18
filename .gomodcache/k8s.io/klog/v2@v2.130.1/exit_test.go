// Copyright 2022 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package klog_test

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/klog/v2"
)

func ExampleFlushAndExit() {
	// Set up klog so that we can test it below.

	var fs flag.FlagSet
	klog.InitFlags(&fs)
	state := klog.CaptureState()
	defer state.Restore()
	if err := fs.Set("skip_headers", "true"); err != nil {
		panic(err)
	}
	if err := fs.Set("logtostderr", "false"); err != nil {
		panic(err)
	}
	klog.SetOutput(os.Stdout)
	klog.OsExit = func(exitCode int) {
		fmt.Printf("os.Exit(%d)\n", exitCode)
	}

	// If we were to return or exit without flushing, this message would
	// get lost because it is buffered in memory by klog when writing to
	// files. Output to stderr is not buffered.
	klog.InfoS("exiting...")
	exitCode := 10
	klog.FlushAndExit(klog.ExitFlushTimeout, exitCode)

	// Output:
	// "exiting..."
	// os.Exit(10)
}

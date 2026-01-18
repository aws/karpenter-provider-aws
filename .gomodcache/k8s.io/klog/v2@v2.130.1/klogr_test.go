/*
Copyright 2023 The Kubernetes Authors.

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
	"bytes"
	"flag"
	"regexp"
	"testing"

	"k8s.io/klog/v2"
)

func TestVerbosity(t *testing.T) {
	state := klog.CaptureState()
	defer state.Restore()

	var fs flag.FlagSet
	klog.InitFlags(&fs)
	if err := fs.Set("v", "5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := fs.Set("vmodule", "klogr_helper_test=10"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := fs.Set("logtostderr", "false"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var buffer bytes.Buffer
	klog.SetOutput(&buffer)
	logger := klog.Background()

	// -v=5 is in effect here.
	logger.V(6).Info("v6 not visible from klogr_test.go")
	if logger.V(6).Enabled() {
		t.Error("V(6).Enabled() in klogr_test.go should have returned false.")
	}
	logger.V(5).Info("v5 visible from klogr_test.go")
	if !logger.V(5).Enabled() {
		t.Error("V(5).Enabled() in klogr_test.go should have returned true.")
	}

	// Now test with -v=5 -vmodule=klogr_helper_test=10.
	testVerbosity(t, logger)

	klog.Flush()
	expected := `^.*v5 visible from klogr_test.go.*
.*v10 visible from klogr_helper_test.go.*
`
	if !regexp.MustCompile(expected).Match(buffer.Bytes()) {
		t.Errorf("Output did not match regular expression.\nOutput:\n%s\n\nRegular expression:\n%s\n", buffer.String(), expected)
	}
}

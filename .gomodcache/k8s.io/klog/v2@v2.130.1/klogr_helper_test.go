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
	"testing"

	"k8s.io/klog/v2"
)

func testVerbosity(t *testing.T, logger klog.Logger) {
	// This runs with -v=5 -vmodule=klog_helper_test=10.
	logger.V(11).Info("v11 not visible from klogr_helper_test.go")
	if logger.V(11).Enabled() {
		t.Error("V(11).Enabled() in klogr_helper_test.go should have returned false.")
	}
	logger.V(10).Info("v10 visible from klogr_helper_test.go")
	if !logger.V(10).Enabled() {
		t.Error("V(10).Enabled() in klogr_helper_test.go should have returned true.")
	}
}

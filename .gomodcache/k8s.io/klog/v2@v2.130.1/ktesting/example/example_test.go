/*
Copyright 2021 The Kubernetes Authors.

SPDX-License-Identifier: Apache-2.0
*/

package example

import (
	"fmt"
	"testing"

	"k8s.io/klog/v2"
	"k8s.io/klog/v2/internal/test"
	"k8s.io/klog/v2/ktesting"
	_ "k8s.io/klog/v2/ktesting/init" // add command line flags
)

func TestKlogr(t *testing.T) {
	logger, _ := ktesting.NewTestContext(t)
	exampleOutput(logger)
}

type pair struct {
	a, b int
}

func (p pair) String() string {
	return fmt.Sprintf("(%d, %d)", p.a, p.b)
}

var _ fmt.Stringer = pair{}

type err struct {
	msg string
}

func (e err) Error() string {
	return "failed: " + e.msg
}

var _ error = err{}

func exampleOutput(logger klog.Logger) {
	logger.Info("hello world")
	logger.Error(err{msg: "some error"}, "failed")
	logger.V(1).Info("verbosity 1")
	logger.WithName("main").WithName("helper").Info("with prefix")
	obj := test.KMetadataMock{Name: "joe", NS: "kube-system"}
	logger.Info("key/value pairs",
		"int", 1,
		"float", 2.0,
		"pair", pair{a: 1, b: 2},
		"raw", obj,
		"kobj", klog.KObj(obj),
	)
	logger.V(4).Info("info message level 4")
	logger.V(5).Info("info message level 5")
	logger.V(6).Info("info message level 6")
}

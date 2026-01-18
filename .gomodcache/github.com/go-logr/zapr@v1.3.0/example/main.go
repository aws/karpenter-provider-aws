/*
Copyright 2019 The logr Authors.

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

// This example shows how to instantiate a zap logger and what output
// looks like.
package main

import (
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/go-logr/zapr/internal/types"
	"go.uber.org/zap"
)

type e struct {
	str string
}

func (e e) Error() string {
	return e.str
}

func helper(log logr.Logger, msg string) {
	helper2(log, msg)
}

func helper2(log logr.Logger, msg string) {
	log.WithCallDepth(2).Info(msg)
}

func main() {
	zc := zap.NewProductionConfig()
	zc.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	zc.DisableStacktrace = true
	z, _ := zc.Build()
	log := zapr.NewLogger(z)
	log = log.WithName("MyName")
	example(log.WithValues("module", "example"))
}

// example only depends on logr except when explicitly breaking the
// abstraction. Even that part is written so that it works with non-zap
// loggers.
func example(log logr.Logger) {
	v := types.ObjectRef{Name: "myname", Namespace: "myns"}
	log.Info("marshal", "stringer", v.String(), "raw", v)
	log.Info("hello", "val1", 1, "val2", map[string]int{"k": 1})
	log.V(1).Info("you should see this")
	log.V(1).V(1).Info("you should NOT see this")
	log.Error(nil, "uh oh", "trouble", true, "reasons", []float64{0.1, 0.11, 3.14})
	log.Error(e{"an error occurred"}, "goodbye", "code", -1)
	helper(log, "thru a helper")

	if zapLogger, ok := log.GetSink().(zapr.Underlier); ok {
		_ = zapLogger.GetUnderlying().Core().Sync()
	}
}

/*
Copyright 2024 The Kubernetes Authors.

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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	kubeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	log.SetLogger(zap.New(func(o *zap.Options) {
		o.Level = zapcore.Level(-5)
	}))

	// Setup a Manager
	mgr, err := manager.New(kubeconfig.GetConfigOrDie(), manager.Options{
		Controller: config.Controller{UsePriorityQueue: ptr.To(true)},
	})
	if err != nil {
		return fmt.Errorf("failed to set up controller-manager: %w", err)
	}

	if err := builder.ControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(reconcile.Func(func(ctx context.Context, r reconcile.Request) (reconcile.Result, error) {
			log.FromContext(ctx).Info("Reconciling")
			time.Sleep(10 * time.Second)

			return reconcile.Result{}, nil
		})); err != nil {
		return fmt.Errorf("failed to set up controller: %w", err)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		return fmt.Errorf("failed to start manager: %w", err)
	}

	return nil
}

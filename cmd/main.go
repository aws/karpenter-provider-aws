/*


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
	"flag"

	"github.com/ellistarn/karpenter/pkg/apis"
	karpenterv1alpha1 "github.com/ellistarn/karpenter/pkg/apis/horizontalautoscaler/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/controllers/horizontalautoscaler"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	controllerruntime "sigs.k8s.io/controller-runtime"
	controllerruntimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = apis.AddToScheme(scheme)
}

// Options for running this binary
type Options struct {
	EnableLeaderElection bool
	EnableWebhook        bool
	EnableReconciler     bool
	EnableVerboseLogging bool
	MetricsAddr          string
}

func main() {
	options := Options{}
	flag.BoolVar(&options.EnableLeaderElection, "enable-leader-election", true, "Enable leader election for this controller.")
	flag.BoolVar(&options.EnableWebhook, "enable-webhook", true, "Enable webhook for this controller.")
	flag.BoolVar(&options.EnableReconciler, "enable-reconciler", true, "Enable reconciler for this controller.")
	flag.BoolVar(&options.EnableVerboseLogging, "verbose", true, "Enable verbose logging.")
	flag.StringVar(&options.MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.Parse()

	logger := controllerruntimezap.NewRaw(controllerruntimezap.UseDevMode(options.EnableVerboseLogging))
	controllerruntime.SetLogger(zapr.NewLogger(logger))
	zap.ReplaceGlobals(logger)

	manager, err := controllerruntime.NewManager(controllerruntime.GetConfigOrDie(), controllerruntime.Options{
		Scheme:             scheme,
		MetricsBindAddress: options.MetricsAddr,
		Port:               9443,
		LeaderElection:     options.EnableLeaderElection,
		LeaderElectionID:   "karpenter-leader-election",
	})
	if err != nil {
		zap.S().Fatalf("Unable to start controller manager, %v", err)
	}

	resource := &karpenterv1alpha1.HorizontalAutoscaler{}
	reconciler := &horizontalautoscaler.Reconciler{
		Client: manager.GetClient(),
	}

	go reconciler.Start()

	if options.EnableReconciler {
		if err := controllerruntime.NewControllerManagedBy(manager).For(resource).Complete(reconciler); err != nil {
			zap.S().Fatalf("Unable to create controller for manager, %v", err)
		}
	}

	if options.EnableWebhook {
		if err = controllerruntime.NewWebhookManagedBy(manager).For(resource).Complete(); err != nil {
			zap.S().Fatalf("Unable to create webhook, %v", err)
		}
	}

	if err := manager.Start(controllerruntime.SetupSignalHandler()); err != nil {
		zap.S().Fatalf("Unable to start manager, %v", err)
	}
}

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
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	karpenterv1alpha1 "github.com/ellistarn/karpenter/pkg/api/v1alpha1"
	"github.com/ellistarn/karpenter/pkg/controllers"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
	log    = controllerruntime.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = karpenterv1alpha1.AddToScheme(scheme)
}

// Options for running this binary
type Options struct {
	EnableLeaderElection bool
	EnableWebhook        bool
	EnableReconciler     bool
	MetricsAddr          string
}

func main() {
	options := Options{}
	flag.StringVar(&options.MetricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&options.EnableLeaderElection, "enable-leader-election", true, "Enable leader election for this controller.")
	flag.BoolVar(&options.EnableWebhook, "enable-webhook", true, "Enable webhook for this controller.")
	flag.BoolVar(&options.EnableReconciler, "enable-reconciler", true, "Enable reconciler for this controller.")
	flag.Parse()

	controllerruntime.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := controllerruntime.NewManager(controllerruntime.GetConfigOrDie(), controllerruntime.Options{
		Scheme:             scheme,
		MetricsBindAddress: options.MetricsAddr,
		Port:               9443,
		LeaderElection:     options.EnableLeaderElection,
		LeaderElectionID:   "karpenter-leader-election",
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if options.EnableReconciler {
		if err = (&controllers.HorizontalAutoscalerReconciler{
			Client: mgr.GetClient(),
			Log:    controllerruntime.Log.WithName("controllers").WithName("HorizontalAutoscaler"),
			Scheme: mgr.GetScheme(),
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "HorizontalAutoscaler")
			os.Exit(1)
		}
	}
	if options.EnableWebhook {
		if err = (&karpenterv1alpha1.HorizontalAutoscaler{}).SetupWebhookWithManager(mgr); err != nil {
			log.Error(err, "unable to create webhook", "webhook", "HorizontalAutoscaler")
			os.Exit(1)
		}
	}

	log.Info("starting manager")
	if err := mgr.Start(controllerruntime.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

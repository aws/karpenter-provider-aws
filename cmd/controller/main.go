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
	"context"
	"fmt"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/cloudprovider"
	cpmetrics "github.com/aws/karpenter/pkg/cloudprovider/metrics"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/controllers/counter"
	"github.com/aws/karpenter/pkg/controllers/metrics"
	"github.com/aws/karpenter/pkg/controllers/node"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/selection"
	"github.com/aws/karpenter/pkg/controllers/termination"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/options"
	"github.com/go-logr/zapr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"knative.dev/pkg/configmap/informer"
	knativeinjection "knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var (
	scheme    = runtime.NewScheme()
	opts      = options.MustParse()
	component = "controller"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apis.AddToScheme(scheme))
}

func main() {
	if err := opts.Validate(); err != nil {
		panic(fmt.Sprintf("Input parameter validation failed, %s", err.Error()))
	}

	config := controllerruntime.GetConfigOrDie()
	config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(float32(opts.KubeClientQPS), opts.KubeClientBurst)
	config.UserAgent = "karpenter"
	clientSet := kubernetes.NewForConfigOrDie(config)

	// Set up logger and watch for changes to log level
	ctx := LoggingContextOrDie(config, clientSet)
	ctx = injection.WithConfig(ctx, config)
	ctx = injection.WithOptions(ctx, opts)

	// Set up controller runtime controller
	cloudProvider := registry.NewCloudProvider(ctx, cloudprovider.Options{ClientSet: clientSet})
	cloudProvider = cpmetrics.PublishLatency(cloudProvider)
	manager := controllers.NewManagerOrDie(ctx, config, controllerruntime.Options{
		Logger:                 zapr.NewLogger(logging.FromContext(ctx).Desugar()),
		LeaderElection:         true,
		LeaderElectionID:       "karpenter-leader-election",
		Scheme:                 scheme,
		MetricsBindAddress:     fmt.Sprintf(":%d", opts.MetricsPort),
		HealthProbeBindAddress: fmt.Sprintf(":%d", opts.HealthProbePort),
	})

	provisioningController := provisioning.NewController(ctx, manager.GetClient(), clientSet.CoreV1(), cloudProvider)

	if err := manager.RegisterControllers(ctx,
		provisioningController,
		selection.NewController(manager.GetClient(), provisioningController),
		termination.NewController(ctx, manager.GetClient(), clientSet.CoreV1(), cloudProvider),
		node.NewController(manager.GetClient()),
		metrics.NewController(manager.GetClient(), cloudProvider),
		counter.NewController(manager.GetClient()),
	).Start(ctx); err != nil {
		panic(fmt.Sprintf("Unable to start manager, %s", err.Error()))
	}
}

// LoggingContextOrDie injects a logger into the returned context. The logger is
// configured by the ConfigMap `config-logging` and live updates the level.
func LoggingContextOrDie(config *rest.Config, clientSet *kubernetes.Clientset) context.Context {
	ctx, startinformers := knativeinjection.EnableInjectionOrDie(signals.NewContext(), config)
	logger, atomicLevel := sharedmain.SetupLoggerOrDie(ctx, component)
	ctx = logging.WithLogger(ctx, logger)
	rest.SetDefaultWarningHandler(&logging.WarningHandler{Logger: logger})
	cmw := informer.NewInformedWatcher(clientSet, system.Namespace())
	sharedmain.WatchLoggingConfigOrDie(ctx, cmw, logger, atomicLevel, component)
	if err := cmw.Start(ctx.Done()); err != nil {
		logger.Fatalf("Failed to watch logging configuration, %s", err.Error())
	}
	startinformers()
	return ctx
}

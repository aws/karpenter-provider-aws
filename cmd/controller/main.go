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

	"knative.dev/pkg/system"

	"github.com/aws/karpenter/pkg/config"

	"github.com/aws/karpenter/pkg/events"

	"github.com/aws/karpenter/pkg/controllers/state"
	"github.com/aws/karpenter/pkg/utils/project"

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
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/cloudprovider"
	cloudprovidermetrics "github.com/aws/karpenter/pkg/cloudprovider/metrics"
	"github.com/aws/karpenter/pkg/cloudprovider/registry"
	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/controllers/counter"
	metricsnode "github.com/aws/karpenter/pkg/controllers/metrics/node"
	metricspod "github.com/aws/karpenter/pkg/controllers/metrics/pod"
	"github.com/aws/karpenter/pkg/controllers/node"
	"github.com/aws/karpenter/pkg/controllers/persistentvolumeclaim"
	"github.com/aws/karpenter/pkg/controllers/provisioning"
	"github.com/aws/karpenter/pkg/controllers/termination"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/options"
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
	controllerRuntimeConfig := controllerruntime.GetConfigOrDie()
	controllerRuntimeConfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(float32(opts.KubeClientQPS), opts.KubeClientBurst)
	controllerRuntimeConfig.UserAgent = "karpenter"
	clientSet := kubernetes.NewForConfigOrDie(controllerRuntimeConfig)

	cmw := informer.NewInformedWatcher(clientSet, system.Namespace())
	// Set up logger and watch for changes to log level
	ctx := LoggingContextOrDie(controllerRuntimeConfig, cmw)
	ctx = injection.WithConfig(ctx, controllerRuntimeConfig)
	ctx = injection.WithOptions(ctx, opts)

	logging.FromContext(ctx).Infof("Initializing with version %s", project.Version)
	// Set up controller runtime controller
	var recorder events.Recorder = &events.NoOpRecorder{}

	cloudProvider := registry.NewCloudProvider(ctx, cloudprovider.Options{ClientSet: clientSet})
	cloudProvider = cloudprovidermetrics.Decorate(cloudProvider)
	manager := controllers.NewManagerOrDie(ctx, controllerRuntimeConfig, controllerruntime.Options{
		Logger:                 zapr.NewLogger(logging.FromContext(ctx).Desugar()),
		LeaderElection:         true,
		LeaderElectionID:       "karpenter-leader-election",
		Scheme:                 scheme,
		MetricsBindAddress:     fmt.Sprintf(":%d", opts.MetricsPort),
		HealthProbeBindAddress: fmt.Sprintf(":%d", opts.HealthProbePort),
	})

	cfg, err := config.New(ctx, clientSet, cmw)
	if err != nil {
		// this does not happen if the config map is missing or invalid, only if some other error occurs
		logging.FromContext(ctx).Fatalf("unable to load config, %s", err)
	}

	if err := cmw.Start(ctx.Done()); err != nil {
		logging.FromContext(ctx).Fatalf("Failed to watch configmaps, %s", err)
	}

	cluster := state.NewCluster(ctx, manager.GetClient())

	if err := manager.RegisterControllers(ctx,
		provisioning.NewController(ctx, cfg, manager.GetClient(), clientSet.CoreV1(), recorder, cloudProvider, cluster),
		state.NewNodeController(manager.GetClient(), cluster),
		state.NewPodController(manager.GetClient(), cluster),
		persistentvolumeclaim.NewController(manager.GetClient()),
		termination.NewController(ctx, manager.GetClient(), clientSet.CoreV1(), cloudProvider),
		node.NewController(manager.GetClient()),
		metricspod.NewController(manager.GetClient()),
		metricsnode.NewController(manager.GetClient()),
		counter.NewController(manager.GetClient()),
	).Start(ctx); err != nil {
		panic(fmt.Sprintf("Unable to start manager, %s", err))
	}
}

// LoggingContextOrDie injects a logger into the returned context. The logger is
// configured by the ConfigMap `config-logging` and live updates the level.
func LoggingContextOrDie(config *rest.Config, cmw *informer.InformedWatcher) context.Context {
	ctx, startinformers := knativeinjection.EnableInjectionOrDie(signals.NewContext(), config)
	logger, atomicLevel := sharedmain.SetupLoggerOrDie(ctx, component)
	ctx = logging.WithLogger(ctx, logger)
	rest.SetDefaultWarningHandler(&logging.WarningHandler{Logger: logger})
	sharedmain.WatchLoggingConfigOrDie(ctx, cmw, logger, atomicLevel, component)
	startinformers()
	return ctx
}

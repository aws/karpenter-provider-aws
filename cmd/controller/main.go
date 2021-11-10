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
	"flag"
	"fmt"

	"github.com/awslabs/karpenter/pkg/apis"
	"github.com/awslabs/karpenter/pkg/cloudprovider"
	"github.com/awslabs/karpenter/pkg/cloudprovider/registry"
	"github.com/awslabs/karpenter/pkg/controllers"
	"github.com/awslabs/karpenter/pkg/controllers/allocation"
	nodemetrics "github.com/awslabs/karpenter/pkg/controllers/metrics/node"
	"github.com/awslabs/karpenter/pkg/controllers/node"
	"github.com/awslabs/karpenter/pkg/controllers/termination"
	"github.com/awslabs/karpenter/pkg/utils/env"
	"github.com/awslabs/karpenter/pkg/utils/options"
	"github.com/awslabs/karpenter/pkg/utils/restconfig"
	"github.com/go-logr/zapr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var (
	scheme    = runtime.NewScheme()
	opts      = options.Options{}
	component = "controller"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apis.AddToScheme(scheme))
}

func main() {
	flag.StringVar(&opts.ClusterName, "cluster-name", env.WithDefaultString("CLUSTER_NAME", ""), "The kubernetes cluster name for resource discovery")
	flag.StringVar(&opts.ClusterEndpoint, "cluster-endpoint", env.WithDefaultString("CLUSTER_ENDPOINT", ""), "The external kubernetes cluster endpoint for new nodes to connect with")
	flag.IntVar(&opts.MetricsPort, "metrics-port", env.WithDefaultInt("METRICS_PORT", 8080), "The port the metric endpoint binds to for operating metrics about the controller itself")
	flag.IntVar(&opts.HealthProbePort, "health-probe-port", env.WithDefaultInt("HEALTH_PROBE_PORT", 8081), "The port the health probe endpoint binds to for reporting controller health")
	flag.IntVar(&opts.KubeClientQPS, "kube-client-qps", env.WithDefaultInt("KUBE_CLIENT_QPS", 200), "The smoothed rate of qps to kube-apiserver")
	flag.IntVar(&opts.KubeClientBurst, "kube-client-burst", env.WithDefaultInt("KUBE_CLIENT_BURST", 300), "The maximum allowed burst of queries to the kube-apiserver")
	flag.Parse()

	if err := opts.Validate(); err != nil {
		panic(fmt.Sprintf("Input parameter validation failed, %s", err.Error()))
	}

	config := controllerruntime.GetConfigOrDie()
	config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(float32(opts.KubeClientQPS), opts.KubeClientBurst)
	clientSet := kubernetes.NewForConfigOrDie(config)

	// Set up logger and watch for changes to log level
	ctx := LoggingContextOrDie(config, clientSet)

	// Put REST config in context, as it can be used by arbitrary
	// parts of the code base
	ctx = restconfig.Inject(ctx, config)

	// Put CLI args into context for access across code base
	ctx = opts.Inject(ctx)

	// Set up controller runtime controller
	cloudProvider := registry.NewCloudProvider(ctx, cloudprovider.Options{ClientSet: clientSet})
	manager := controllers.NewManagerOrDie(config, controllerruntime.Options{
		Logger:                 zapr.NewLogger(logging.FromContext(ctx).Desugar()),
		LeaderElection:         true,
		LeaderElectionID:       "karpenter-leader-election",
		Scheme:                 scheme,
		MetricsBindAddress:     fmt.Sprintf(":%d", opts.MetricsPort),
		HealthProbeBindAddress: fmt.Sprintf(":%d", opts.HealthProbePort),
	})
	if err := manager.RegisterControllers(ctx,
		allocation.NewController(manager.GetClient(), clientSet.CoreV1(), cloudProvider),
		termination.NewController(ctx, manager.GetClient(), clientSet.CoreV1(), cloudProvider),
		node.NewController(manager.GetClient()),
		nodemetrics.NewController(manager.GetClient()),
	).Start(ctx); err != nil {
		panic(fmt.Sprintf("Unable to start manager, %s", err.Error()))
	}
}

// LoggingContextOrDie injects a logger into the returned context. The logger is
// configured by the ConfigMap `config-logging` and live updates the level.
func LoggingContextOrDie(config *rest.Config, clientSet *kubernetes.Clientset) context.Context {
	ctx, startinformers := injection.EnableInjectionOrDie(signals.NewContext(), config)
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

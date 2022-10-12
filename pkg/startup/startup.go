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

package startup

import (
	"context"
	"runtime/debug"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/utils/clock"
	"knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/aws/karpenter/pkg/apis"
	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/utils/injection"
	"github.com/aws/karpenter/pkg/utils/options"
	"github.com/aws/karpenter/pkg/utils/project"
)

const (
	component = "controller"
	appName   = "karpenter"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apis.AddToScheme(scheme))
}

func Initialize() Options {
	opts := options.New().MustParse()
	// Setup Client
	controllerRuntimeConfig := controllerruntime.GetConfigOrDie()
	controllerRuntimeConfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(float32(opts.KubeClientQPS), opts.KubeClientBurst)
	controllerRuntimeConfig.UserAgent = appName
	clientSet := kubernetes.NewForConfigOrDie(controllerRuntimeConfig)

	// Set up logger and watch for changes to log level
	cmw := informer.NewInformedWatcher(clientSet, system.Namespace())
	ctx := injection.LoggingContextOrDie(component, controllerRuntimeConfig, cmw)
	ctx = injection.WithConfig(ctx, controllerRuntimeConfig)
	ctx = injection.WithOptions(ctx, *opts)

	logging.FromContext(ctx).Infof("Initializing with version %s", project.Version)
	if opts.MemoryLimit > 0 {
		newLimit := int64(float64(opts.MemoryLimit) * 0.9)
		logging.FromContext(ctx).Infof("Setting GC memory limit to %d, container limit = %d", newLimit, opts.MemoryLimit)
		debug.SetMemoryLimit(newLimit)
	}

	manager := NewManagerOrDie(ctx, controllerRuntimeConfig, opts)
	realClock := clock.RealClock{}
	recorder := events.NewRecorder(manager.GetEventRecorderFor(appName))
	recorder = events.NewLoadSheddingRecorder(recorder)
	recorder = events.NewDedupeRecorder(recorder)

	if err := cmw.Start(ctx.Done()); err != nil {
		logging.FromContext(ctx).Errorf("watching configmaps, config changes won't be applied immediately, %s", err)
	}

	return Options{
		Ctx:       ctx,
		Cmw:       cmw,
		Recorder:  recorder,
		Clientset: clientSet,
		Clock:     realClock,
		Options:   opts,
		Manager:   manager,
	}
}

func NewRunnableContext(config *rest.Config, options *options.Options, logger *zap.SugaredLogger) func() context.Context {
	return func() context.Context {
		ctx := context.Background()
		ctx = logging.WithLogger(ctx, logger)
		ctx = injection.WithConfig(ctx, config)
		ctx = injection.WithOptions(ctx, *options)
		return ctx
	}
}

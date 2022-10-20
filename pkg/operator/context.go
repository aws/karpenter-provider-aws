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

package operator

import (
	"context"
	"runtime/debug"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/utils/clock"
	"knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/aws/karpenter-core/pkg/utils/project"
	"github.com/aws/karpenter/pkg/config"
	"github.com/aws/karpenter/pkg/events"
	"github.com/aws/karpenter/pkg/operator/injection"
	"github.com/aws/karpenter/pkg/operator/options"
)

const (
	appName   = "karpenter"
	component = "controller"
)

// Context exposes a global context of components that can be used across the binary
// for initialization.
type Context struct {
	context.Context

	EventRecorder     events.Recorder      // Decorated recorder for Karpenter core events
	BaseEventRecorder record.EventRecorder // Recorder from controller manager for use by other components
	Config            config.Config
	KubeClient        client.Client
	Clientset         *kubernetes.Clientset
	Clock             clock.Clock
	Options           *options.Options
	StartAsync        <-chan struct{}
}

func NewOrDie() (Context, manager.Manager) {
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

	cfg, err := config.New(ctx, clientSet, cmw)
	if err != nil {
		// this does not happen if the config map is missing or invalid, only if some other error occurs
		logging.FromContext(ctx).Fatalf("unable to load config, %s", err)
	}
	if err := cmw.Start(ctx.Done()); err != nil {
		logging.FromContext(ctx).Errorf("watching configmaps, config changes won't be applied immediately, %s", err)
	}

	manager := NewManagerOrDie(ctx, controllerRuntimeConfig, opts)

	baseRecorder := manager.GetEventRecorderFor(appName)
	recorder := events.NewRecorder(baseRecorder)
	recorder = events.NewLoadSheddingRecorder(recorder)
	recorder = events.NewDedupeRecorder(recorder)

	return Context{
		Context:           ctx,
		EventRecorder:     recorder,
		BaseEventRecorder: baseRecorder,
		Config:            cfg,
		Clientset:         clientSet,
		KubeClient:        manager.GetClient(),
		Clock:             clock.RealClock{},
		Options:           opts,
		StartAsync:        manager.Elected(),
	}, manager
}

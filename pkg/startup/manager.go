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
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/go-logr/zapr"
	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/aws/karpenter/pkg/utils/options"
)

// NewManagerOrDie instantiates a controller manager or panics
func NewManagerOrDie(ctx context.Context, config *rest.Config, opts *options.Options) manager.Manager {
	newManager, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger:                     ignoreDebugEvents(zapr.NewLogger(logging.FromContext(ctx).Desugar())),
		LeaderElection:             opts.EnableLeaderElection,
		LeaderElectionID:           "karpenter-leader-election",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		Scheme:                     scheme,
		MetricsBindAddress:         fmt.Sprintf(":%d", opts.MetricsPort),
		HealthProbeBindAddress:     fmt.Sprintf(":%d", opts.HealthProbePort),
		BaseContext:                NewRunnableContext(config, opts, logging.FromContext(ctx)),
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create controller newManager, %s", err))
	}
	if err := newManager.GetFieldIndexer().IndexField(ctx, &v1.Pod{}, "spec.nodeName", func(o client.Object) []string {
		return []string{o.(*v1.Pod).Spec.NodeName}
	}); err != nil {
		panic(fmt.Sprintf("Failed to setup pod indexer, %s", err))
	}
	if opts.EnableProfiling {
		utilruntime.Must(registerPprof(newManager))
	}

	return newManager
}

func registerPprof(manager manager.Manager) error {
	for path, handler := range map[string]http.Handler{
		"/debug/pprof/":             http.HandlerFunc(pprof.Index),
		"/debug/pprof/cmdline":      http.HandlerFunc(pprof.Cmdline),
		"/debug/pprof/profile":      http.HandlerFunc(pprof.Profile),
		"/debug/pprof/symbol":       http.HandlerFunc(pprof.Symbol),
		"/debug/pprof/trace":        http.HandlerFunc(pprof.Trace),
		"/debug/pprof/allocs":       pprof.Handler("allocs"),
		"/debug/pprof/heap":         pprof.Handler("heap"),
		"/debug/pprof/block":        pprof.Handler("block"),
		"/debug/pprof/goroutine":    pprof.Handler("goroutine"),
		"/debug/pprof/threadcreate": pprof.Handler("threadcreate"),
	} {
		err := manager.AddMetricsExtraHandler(path, handler)
		if err != nil {
			return err
		}
	}
	return nil
}

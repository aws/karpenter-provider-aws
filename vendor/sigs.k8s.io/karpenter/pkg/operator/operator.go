/*
Copyright The Kubernetes Authors.

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
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/awslabs/operatorpkg/controller"
	opmetrics "github.com/awslabs/operatorpkg/metrics"
	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/events"
	"sigs.k8s.io/karpenter/pkg/metrics"
	"sigs.k8s.io/karpenter/pkg/operator/injection"
	"sigs.k8s.io/karpenter/pkg/operator/logging"
	"sigs.k8s.io/karpenter/pkg/operator/options"
	"sigs.k8s.io/karpenter/pkg/utils/env"
)

const (
	appName   = "karpenter"
	component = "controller"
)

var (
	BuildInfo = opmetrics.NewPrometheusGauge(
		crmetrics.Registry,
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Name:      "build_info",
			Help:      "A metric with a constant '1' value labeled by version from which karpenter was built.",
		},
		[]string{"version", "goversion", "goarch", "commit"},
	)
)

// Version is the karpenter app version injected during compilation
// when using the Makefile
var Version = "unspecified"

func init() {
	opmetrics.RegisterClientMetrics(crmetrics.Registry)

	BuildInfo.Set(1, map[string]string{
		"version":   Version,
		"goversion": runtime.Version(),
		"goarch":    runtime.GOARCH,
		"commit":    env.GetRevision(),
	})
}

type Operator struct {
	manager.Manager

	KubernetesInterface kubernetes.Interface
	EventRecorder       events.Recorder
	Clock               clock.Clock
}

// NewOperator instantiates a controller manager or panics
func NewOperator() (context.Context, *Operator) {
	// Root Context
	ctx := context.Background()

	// Options
	ctx = injection.WithOptionsOrDie(ctx, options.Injectables...)

	// Make the Karpenter binary aware of the container memory limit
	// https://pkg.go.dev/runtime/debug#SetMemoryLimit
	if options.FromContext(ctx).MemoryLimit > 0 {
		newLimit := int64(float64(options.FromContext(ctx).MemoryLimit) * 0.9)
		debug.SetMemoryLimit(newLimit)
	}

	// Logging
	logger := zapr.NewLogger(logging.NewLogger(ctx, component))
	log.SetLogger(logger)
	klog.SetLogger(logger)

	// Client Config
	config := ctrl.GetConfigOrDie()
	config.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(float32(options.FromContext(ctx).KubeClientQPS), options.FromContext(ctx).KubeClientBurst)
	config.UserAgent = fmt.Sprintf("%s/%s", appName, Version)

	// Client
	kubernetesInterface := kubernetes.NewForConfigOrDie(config)

	log.FromContext(ctx).WithValues("version", Version).V(1).Info("discovered karpenter version")

	// Manager
	mgrOpts := ctrl.Options{
		Logger:                        logging.IgnoreDebugEvents(logger),
		LeaderElection:                !options.FromContext(ctx).DisableLeaderElection,
		LeaderElectionID:              options.FromContext(ctx).LeaderElectionName,
		LeaderElectionNamespace:       options.FromContext(ctx).LeaderElectionNamespace,
		LeaderElectionResourceLock:    resourcelock.LeasesResourceLock,
		LeaderElectionReleaseOnCancel: true,
		Metrics: server.Options{
			BindAddress: fmt.Sprintf(":%d", options.FromContext(ctx).MetricsPort),
		},
		HealthProbeBindAddress: fmt.Sprintf(":%d", options.FromContext(ctx).HealthProbePort),
		BaseContext: func() context.Context {
			ctx := log.IntoContext(context.Background(), logger)
			ctx = injection.WithOptionsOrDie(ctx, options.Injectables...)
			return ctx
		},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&coordinationv1.Lease{}: {
					Field: fields.SelectorFromSet(fields.Set{"metadata.namespace": "kube-node-lease"}),
				},
			},
		},
	}
	if options.FromContext(ctx).EnableProfiling {
		// TODO @joinnis: Investigate the mgrOpts.PprofBindAddress that would allow native support for pprof
		// On initial look, it seems like this native pprof doesn't support some of the routes that we have here
		// like "/debug/pprof/heap" or "/debug/pprof/block"
		mgrOpts.Metrics.ExtraHandlers = lo.Assign(mgrOpts.Metrics.ExtraHandlers, map[string]http.Handler{
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
		})
	}
	mgr, err := ctrl.NewManager(config, mgrOpts)
	mgr = lo.Must(mgr, err, "failed to setup manager")

	setupIndexers(ctx, mgr)

	lo.Must0(mgr.AddReadyzCheck("manager", func(req *http.Request) error {
		return lo.Ternary(mgr.GetCache().WaitForCacheSync(req.Context()), nil, fmt.Errorf("failed to sync caches"))
	}))
	lo.Must0(mgr.AddReadyzCheck("crd", func(_ *http.Request) error {
		objects := []client.Object{&v1.NodePool{}, &v1.NodeClaim{}}
		for _, obj := range objects {
			gvk, err := apiutil.GVKForObject(obj, scheme.Scheme)
			if err != nil {
				return err
			}
			if _, err := mgr.GetRESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version); err != nil {
				return err
			}
		}
		return nil
	}))
	lo.Must0(mgr.AddHealthzCheck("healthz", healthz.Ping))
	lo.Must0(mgr.AddReadyzCheck("readyz", healthz.Ping))

	return ctx, &Operator{
		Manager:             mgr,
		KubernetesInterface: kubernetesInterface,
		EventRecorder:       events.NewRecorder(mgr.GetEventRecorderFor(appName)),
		Clock:               clock.RealClock{},
	}
}

func (o *Operator) WithControllers(ctx context.Context, controllers ...controller.Controller) *Operator {
	for _, c := range controllers {
		lo.Must0(c.Register(ctx, o.Manager))
	}
	return o
}

func (o *Operator) Start(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		lo.Must0(o.Manager.Start(ctx))
	}()
	wg.Wait()
}

func setupIndexers(ctx context.Context, mgr manager.Manager) {
	lo.Must0(mgr.GetFieldIndexer().IndexField(ctx, &corev1.Pod{}, "spec.nodeName", func(o client.Object) []string {
		return []string{o.(*corev1.Pod).Spec.NodeName}
	}), "failed to setup pod indexer")
	lo.Must0(mgr.GetFieldIndexer().IndexField(ctx, &corev1.Node{}, "spec.providerID", func(o client.Object) []string {
		return []string{o.(*corev1.Node).Spec.ProviderID}
	}), "failed to setup node provider id indexer")
	lo.Must0(mgr.GetFieldIndexer().IndexField(ctx, &storagev1.VolumeAttachment{}, "spec.nodeName", func(o client.Object) []string {
		return []string{o.(*storagev1.VolumeAttachment).Spec.NodeName}
	}), "failed to setup volumeattachment indexer")

	// If the CRD does not exist, we should fail open when setting up indexers. This ensures controllers that aren't reliant on those CRDs may continue to function
	handleCRDIndexerError := func(err error, msg string) {
		noKindMatchError := &meta.NoKindMatchError{}
		if errors.As(err, &noKindMatchError) {
			log.FromContext(ctx).Error(err, msg)
		} else if err != nil {
			// lo.Must0 also does a panic
			panic(fmt.Sprintf("%s, %s", err, msg))
		}
	}
	handleCRDIndexerError(mgr.GetFieldIndexer().IndexField(ctx, &v1.NodeClaim{}, "status.providerID", func(o client.Object) []string {
		return []string{o.(*v1.NodeClaim).Status.ProviderID}
	}), "failed to setup nodeclaim provider id indexer")
	handleCRDIndexerError(mgr.GetFieldIndexer().IndexField(ctx, &v1.NodeClaim{}, "spec.nodeClassRef.group", func(o client.Object) []string {
		return []string{o.(*v1.NodeClaim).Spec.NodeClassRef.Group}
	}), "failed to setup nodeclaim nodeclassref apiversion indexer")
	handleCRDIndexerError(mgr.GetFieldIndexer().IndexField(ctx, &v1.NodeClaim{}, "spec.nodeClassRef.kind", func(o client.Object) []string {
		return []string{o.(*v1.NodeClaim).Spec.NodeClassRef.Kind}
	}), "failed to setup nodeclaim nodeclassref kind indexer")
	handleCRDIndexerError(mgr.GetFieldIndexer().IndexField(ctx, &v1.NodeClaim{}, "spec.nodeClassRef.name", func(o client.Object) []string {
		return []string{o.(*v1.NodeClaim).Spec.NodeClassRef.Name}
	}), "failed to setup nodeclaim nodeclassref name indexer")

	handleCRDIndexerError(mgr.GetFieldIndexer().IndexField(ctx, &v1.NodePool{}, "spec.template.spec.nodeClassRef.group", func(o client.Object) []string {
		return []string{o.(*v1.NodePool).Spec.Template.Spec.NodeClassRef.Group}
	}), "failed to setup nodepool nodeclassref apiversion indexer")
	handleCRDIndexerError(mgr.GetFieldIndexer().IndexField(ctx, &v1.NodePool{}, "spec.template.spec.nodeClassRef.kind", func(o client.Object) []string {
		return []string{o.(*v1.NodePool).Spec.Template.Spec.NodeClassRef.Kind}
	}), "failed to setup nodepool nodeclassref kind indexer")
	handleCRDIndexerError(mgr.GetFieldIndexer().IndexField(ctx, &v1.NodePool{}, "spec.template.spec.nodeClassRef.name", func(o client.Object) []string {
		return []string{o.(*v1.NodePool).Spec.Template.Spec.NodeClassRef.Name}
	}), "failed to setup nodepool nodeclassref name indexer")
}

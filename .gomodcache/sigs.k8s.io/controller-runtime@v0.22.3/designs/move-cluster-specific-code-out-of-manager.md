Move cluster-specific code out of the manager
===================

## Motivation

Today, it is already possible to use controller-runtime to build controllers that act on
more than one cluster. However, this is undocumented and not straight-forward, requiring
users to look into the implementation details to figure out how to make this work.

## Goals

* Provide an easy-to-discover way to build controllers that act on multiple clusters
* Decouple the management of `Runnables` from the construction of "things that require a kubeconfig"
* Do not introduce changes for users that build controllers that act on one cluster only

## Non-Goals

## Proposal

Currently, the `./pkg/manager.Manager` has two purposes:

* Handle running controllers/other runnables and managing their lifecycle
* Setting up various things to interact with the Kubernetes cluster,
  for example a `Client` and a `Cache`

This works very well when building controllers that talk to a single cluster,
however some use-cases require controllers that interact with more than
one cluster. This multi-cluster usecase is very awkward today, because it
requires to construct one manager per cluster and adding all subsequent
managers to the first one.

This document proposes to move all cluster-specific code out of the manager
and into a new package and interface, that then gets embedded into the manager.
This allows to keep the usage for single-cluster cases the same and introduce
this change in a backwards-compatible manner.

Furthermore, the manager gets extended to start all caches before any other
`runnables` are started.


The new `Cluster` interface will look like this:

```go
type Cluster interface {
	// SetFields will set cluster-specific dependencies on an object for which the object has implemented the inject
	// interface, specifically inject.Client, inject.Cache, inject.Scheme, inject.Config and inject.APIReader
	SetFields(interface{}) error

	// GetConfig returns an initialized Config
	GetConfig() *rest.Config

	// GetClient returns a client configured with the Config. This client may
	// not be a fully "direct" client -- it may read from a cache, for
	// instance.  See Options.NewClient for more information on how the default
	// implementation works.
	GetClient() client.Client

	// GetFieldIndexer returns a client.FieldIndexer configured with the client
	GetFieldIndexer() client.FieldIndexer

	// GetCache returns a cache.Cache
	GetCache() cache.Cache

	// GetEventRecorderFor returns a new EventRecorder for the provided name
	GetEventRecorderFor(name string) record.EventRecorder

	// GetRESTMapper returns a RESTMapper
	GetRESTMapper() meta.RESTMapper

	// GetAPIReader returns a reader that will be configured to use the API server.
	// This should be used sparingly and only when the client does not fit your
	// use case.
	GetAPIReader() client.Reader

	// GetScheme returns an initialized Scheme
	GetScheme() *runtime.Scheme

	// Start starts the connection tothe Cluster
	Start(<-chan struct{}) error
}
```

And the current `Manager` interface will change to look like this:

```go
type Manager interface {
	// Cluster holds objects to connect to a cluster
	cluser.Cluster

	// Add will set requested dependencies on the component, and cause the component to be
	// started when Start is called.  Add will inject any dependencies for which the argument
	// implements the inject interface - e.g. inject.Client.
	// Depending on if a Runnable implements LeaderElectionRunnable interface, a Runnable can be run in either
	// non-leaderelection mode (always running) or leader election mode (managed by leader election if enabled).
	Add(Runnable) error

	// Elected is closed when this manager is elected leader of a group of
	// managers, either because it won a leader election or because no leader
	// election was configured.
	Elected() <-chan struct{}

	// SetFields will set any dependencies on an object for which the object has implemented the inject
	// interface - e.g. inject.Client.
	SetFields(interface{}) error

	// AddMetricsExtraHandler adds an extra handler served on path to the http server that serves metrics.
	// Might be useful to register some diagnostic endpoints e.g. pprof. Note that these endpoints meant to be
	// sensitive and shouldn't be exposed publicly.
	// If the simple path -> handler mapping offered here is not enough, a new http server/listener should be added as
	// Runnable to the manager via Add method.
	AddMetricsExtraHandler(path string, handler http.Handler) error

	// AddHealthzCheck allows you to add Healthz checker
	AddHealthzCheck(name string, check healthz.Checker) error

	// AddReadyzCheck allows you to add Readyz checker
	AddReadyzCheck(name string, check healthz.Checker) error

	// Start starts all registered Controllers and blocks until the Stop channel is closed.
	// Returns an error if there is an error starting any controller.
	// If LeaderElection is used, the binary must be exited immediately after this returns,
	// otherwise components that need leader election might continue to run after the leader
	// lock was lost.
	Start(<-chan struct{}) error

	// GetWebhookServer returns a webhook.Server
	GetWebhookServer() *webhook.Server
}
```

Furthermore, during startup, the `Manager` will use type assertion to find `Cluster`s
to be able to start their caches before anything else:

```go
type HasCaches interface {
  GetCache()
}
if getter, hasCaches := runnable.(HasCaches); hasCaches {
	m.caches = append(m.caches, getter())
}
```

```go
for idx := range cm.caches {
	go func(idx int) {cm.caches[idx].Start(cm.internalStop)}
}

for _, cache := range cm.caches {
	cache.WaitForCacheSync(cm.internalStop)
}

// Start all other runnables
```

## Example

Below is a sample `reconciler` that will create a secret in a `mirrorCluster` for each
secret found in `referenceCluster` if none of that name already exists. To keep the sample
short, it won't compare the contents of the secrets.

```go
type secretMirrorReconciler struct {
	referenceClusterClient, mirrorClusterClient client.Client
}

func (r *secretMirrorReconciler) Reconcile(r reconcile.Request)(reconcile.Result, error){
	s := &corev1.Secret{}
	if err := r.referenceClusterClient.Get(context.TODO(), r.NamespacedName, s); err != nil {
		if kerrors.IsNotFound{ return reconcile.Result{}, nil }
		return reconcile.Result, err
	}

	if err := r.mirrorClusterClient.Get(context.TODO(), r.NamespacedName, &corev1.Secret); err != nil {
		if !kerrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}

		mirrorSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: s.Namespace, Name: s.Name},
			Data: s.Data,
		}
		return reconcile.Result{}, r.mirrorClusterClient.Create(context.TODO(), mirrorSecret)
	}

	return nil
}

func NewSecretMirrorReconciler(mgr manager.Manager, mirrorCluster cluster.Cluster) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Watch Secrets in the reference cluster
		For(&corev1.Secret{}).
		// Watch Secrets in the mirror cluster
		Watches(
			source.NewKindWithCache(&corev1.Secret{}, mirrorCluster.GetCache()),
			&handler.EnqueueRequestForObject{},
		).
		Complete(&secretMirrorReconciler{
			referenceClusterClient: mgr.GetClient(),
			mirrorClusterClient:    mirrorCluster.GetClient(),
		})
	}
}

func main(){

	mgr, err := manager.New( cfg1, manager.Options{})
	if err != nil {
		panic(err)
	}

	mirrorCluster, err := cluster.New(cfg2)
	if err != nil {
		panic(err)
	}

	if err := mgr.Add(mirrorCluster); err != nil {
		panic(err)
	}

	if err := NewSecretMirrorReconciler(mgr, mirrorCluster); err != nil {
		panic(err)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		panic(err)
	}
}
```

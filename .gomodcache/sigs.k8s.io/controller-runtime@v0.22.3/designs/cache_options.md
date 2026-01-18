Cache Options
===================

This document describes how we imagine the cache options to look in
the future.

## Goals

* Align everyone on what settings on the cache we want to support and
  their configuration surface
* Ensure that we support both complicated cache setups and provide an
  intuitive configuration UX

## Non-Goals

* Describe the design and implementation of the cache itself.
  The assumption is that the most granular level we will end up with is
  "per-object multiple namespaces with distinct selectors" and that this
  can be implemented using a "meta cache" that delegates per object and by
  extending the current multi-namespace cache
* Outline any kind of timeline for when these settings will be implemented.
  Implementation will happen gradually over time whenever someone steps up
  to do the actual work

## Proposal


```
const (
   AllNamespaces = corev1.NamespaceAll
)

type Config struct {
  // LabelSelector specifies a label selector. A nil value allows to
  // default this.
  LabelSelector labels.Selector

  // FieldSelector specifics a field selector. A nil value allows to
  // default this.
  FieldSelector fields.Selector

  // Transform specifies a transform func. A nil value allows to default
  // this.
  Transform     toolscache.TransformFunc

  // UnsafeDisableDeepCopy specifies if List and Get requests against the
  // cache should not DeepCopy. A nil value allows to default this.
  UnsafeDisableDeepCopy *bool
}


type ByObject struct {
  // Namespaces maps a namespace name to cache setting. If set, only the
  // namespaces in this map will be cached.
  //
  // Settings in the map value that are unset because either the value as a
  // whole is nil or because the specific setting is nil will be defaulted.
  // Use an empty value for the specific setting to prevent that.
  //
  // It is possible to have specific Config for just some namespaces
  // but cache all namespaces by using the AllNamespaces const as the map key.
  // This wil then include all namespaces that do not have a more specific
  // setting.
  //
  // A nil map allows to default this to the cache's DefaultNamespaces setting.
  // An empty map prevents this.
  //
  // This must be unset for cluster-scoped objects.
  Namespaces map[string]Config

  // Config will be used for cluster-scoped objects and to default
  // Config in the Namespaces field.
  //
  // It gets defaulted from the cache'sDefaultLabelSelector, DefaultFieldSelector,
  // DefaultUnsafeDisableDeepCopy and DefaultTransform.
  Config *Config
}

type Options struct {
  // ByObject specifies per-object cache settings. If unset for a given
  // object, this will fall through to Default* settings.
  ByObject map[client.Object]ByObject

  // DefaultNamespaces maps namespace names to cache settings. If set, it
  // will be used for all objects that have a nil Namespaces setting.
  //
  // It is possible to have a specific Config for just some namespaces
  // but cache all namespaces by using the `AllNamespaces` const as the map
  // key. This wil then include all namespaces that do not have a more
  // specific setting.
  //
  // The options in the Config that are nil will be defaulted from
  // the respective Default* settings.
  DefaultNamespaces map[string]Config

  // DefaultLabelSelector is the label selector that will be used as
  // the default field label selector for everything that doesn't
  // have one configured.
  DefaultLabelSelector labels.Selector

  // DefaultFieldSelector is the field selector that will be used as
  // the default field selector for everything that doesn't have
  // one configured.
  DefaultFieldSelector fields.Selector

  // DefaultUnsafeDisableDeepCopy is the default for UnsafeDisableDeepCopy
  // for everything that doesn't specify this.
  DefaultUnsafeDisableDeepCopy *bool

  // DefaultTransform will be used as transform for all object types
  // unless they have a more specific transform set in ByObject.
  DefaultTransform toolscache.TransformFunc

  // HTTPClient is the http client to use for the REST client
  HTTPClient *http.Client

  // Scheme is the scheme to use for mapping objects to GroupVersionKinds
  Scheme *runtime.Scheme

  // Mapper is the RESTMapper to use for mapping GroupVersionKinds to Resources
  Mapper meta.RESTMapper

  // SyncPeriod determines the minimum frequency at which watched resources are
  // reconciled. A lower period will correct entropy more quickly, but reduce
  // responsiveness to change if there are many watched resources. Change this
  // value only if you know what you are doing. Defaults to 10 hours if unset.
  // there will a 10 percent jitter between the SyncPeriod of all controllers
  // so that all controllers will not send list requests simultaneously.
  //
  // This applies to all controllers.
  //
  // A period sync happens for two reasons:
  // 1. To insure against a bug in the controller that causes an object to not
  // be requeued, when it otherwise should be requeued.
  // 2. To insure against an unknown bug in controller-runtime, or its dependencies,
  // that causes an object to not be requeued, when it otherwise should be
  // requeued, or to be removed from the queue, when it otherwise should not
  // be removed.
  //
  // If you want
  // 1. to insure against missed watch events, or
  // 2. to poll services that cannot be watched,
  // then we recommend that, instead of changing the default period, the
  // controller requeue, with a constant duration `t`, whenever the controller
  // is "done" with an object, and would otherwise not requeue it, i.e., we
  // recommend the `Reconcile` function return `reconcile.Result{RequeueAfter: t}`,
  // instead of `reconcile.Result{}`.
  SyncPeriod *time.Duration

}
```


## Example usages

### Cache ConfigMaps in the `public` and `kube-system` namespaces and Secrets in the `operator` Namespace


```
cache.Options{
  ByObject: map[client.Object]cache.ByObject{
    &corev1.ConfigMap{}: {
      Namespaces: map[string]cache.Config{
        "public":      {},
        "kube-system": {},
      },
    },
    &corev1.Secret{}: {Namespaces: map[string]Config{
        "operator": {},
    }},
  },
}
```

### Cache ConfigMaps in all namespaces without selector, but have a selector for the `operator` Namespace

```
cache.Options{
  ByObject: map[client.Object]cache.ByObject{
    &corev1.ConfigMap{}: {
      Namespaces: map[string]cache.Config{
        cache.AllNamespaces: nil,                   // No selector for all namespaces...
        "operator": {LabelSelector: labelSelector}, // except for the operator namespace
      },
    },
  },
}
```


### Only cache the `operator` namespace for namespaced objects and all namespaces for Deployments

```
cache.Options{
  ByObject: map[client.Object]cache.ByObject{
    &appsv1.Deployment: {Namespaces: map[string]cache.Config{
       cache.AllNamespaces: {}},
    }},
  },
  DefaultNamespaces: map[string]cache.Config{
      "operator": {}},
  },
}
```

### Use a LabelSelector for everything except Nodes

```
cache.Options{
  ByObject: map[client.Object]cache.ByObject{
    &corev1.Node: {LabelSelector: labels.Everything()},
  },
  DefaultLabelSelector: myLabelSelector,
}
```

### Only cache namespaced objects in the `foo` and `bar` namespace

```
cache.Options{
  DefaultNamespaces: map[string]cache.Config{
    "foo": {},
    "bar": {},
  }
}
```

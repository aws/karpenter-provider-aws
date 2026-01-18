# Filter cache ListWatch using selectors

## Motivation

Controller-Runtime controllers use a cache to subscribe to events from
Kubernetes objects and to read those objects more efficiently by avoiding
to call out to the API. This cache is backed by Kubernetes informers.

The only way to filter this cache is by namespace and resource type.
In cases where a controller is only interested in a small subset of objects
(for example all pods on a node), this might end up not being efficient enough.

Requests to a client backed by a filtered cache for objects that do not match
the filter will never return anything, so we need to make sure that we properly
warn users to only use this when they are sure they know what they are doing.

This proposal sidesteps the issue of "How to we plug this into the cache-backed
client so that users get feedback when they request something that is
not matching the caches filter" by only implementing the filter logic in the
cache package. This allows advanced users to combine a filtered cache with the
already existing `NewCacheFunc` option in the manager and cluster package,
while simultaneously hiding it from newer users that might not be aware of the
implications and the associated foot-shoot potential.

The only alternative today to get a filtered cache with controller-runtime is
to build it out-of tree. Because such a cache would mostly copy the existing
cache and add just some options, this is not great for consumers.

This proposal is related to the following issue [2]

## Proposal

Add a new selector code at `pkg/cache/internal/selector.go` with common structs
and helpers

```golang
package internal

...

// SelectorsByObject associate a runtime.Object to a field/label selector
type SelectorsByObject map[client.Object]Selector

// SelectorsByGVK associate a GroupVersionResource to a field/label selector
type SelectorsByGVK map[schema.GroupVersionKind]Selector

// Selector specify the label/field selector to fill in ListOptions
type Selector struct {
	Label labels.Selector
	Field fields.Selector
}

// ApplyToList fill in ListOptions LabelSelector and FieldSelector if needed
func (s Selector) ApplyToList(listOpts *metav1.ListOptions) {
...
}
```

Add a type alias to `pkg/cache/cache.go` to internal

```golang
type SelectorsByObject internal.SelectorsByObject
```

Extend `cache.Options` as follows:

```golang
type Options struct {
	Scheme            *runtime.Scheme
	Mapper            meta.RESTMapper
	Resync            *time.Duration
	Namespace         string
	SelectorsByObject SelectorsByObject
}
```

Add new builder function that will return a cache constructor using the passed
cache.Options, users can set SelectorsByObject there to filter out cache, it
will convert SelectorByObject to SelectorsByGVK

```golang
func BuilderWithOptions(options cache.Options) NewCacheFunc {
...
}
```

is passed to informer's ListWatch and add the filtering option:

```golang

# At pkg/cache/internal/informers_map.go

ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
            ip.selectors[gvk].ApplyToList(&opts)
...
```

Here is a PR with the implementatin at the `pkg/cache` part [3]

## Example

User will override `NewCache` function to make clear that they know exactly the
implications of using a different cache than the default one

```golang
 ctrl.Options.NewCache = cache.BuilderWithOptions(cache.Options{
                            SelectorsByObject: cache.SelectorsByObject{
                                    &corev1.Node{}: {
                                        Field: fields.SelectorFromSet(fields.Set{"metadata.name": "node01"}),
                                    }
                                    &v1beta1.NodeNetworkState{}: {
                                        Field: fields.SelectorFromSet(fields.Set{"metadata.name": "node01"}),
                                        Label: labels.SelectorFromSet(labels.Set{"app": "kubernetes-nmstate})",
                                    }
                                }
                            }
                        )
```

[1] https://github.com/nmstate/kubernetes-nmstate/pull/687
[2] https://github.com/kubernetes-sigs/controller-runtime/issues/244
[3] https://github.com/kubernetes-sigs/controller-runtime/pull/1404

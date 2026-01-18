Priority Queue
===================

This document describes the motivation behind implementing a priority queue
in controller-runtime and its design details.

## Motivation

1. Controllers reconcile all objects during startup to account for changes in
   the reconciliation logic. Some controllers also periodically re-reconcile
   everything to account for out of band changes they do not get notified for,
   this is for example common for controllers managing cloud resources. In both
   these cases, the reconciliation of new or changed objects gets delayed,
   resulting in poor user experience. [Example][0]
2. There may be application-specific reason why some events are more important
   than others, [Example][1]

## Proposed changes

Implement a priority queue in controller-runtime that exposes the following
interface:

```go
type PriorityQueue[T comparable] interface {
    // AddWithOpts adds one or more items to the workqueue. Items
    // in the workqueue are de-duplicated, so there will only ever
    // be one entry for a given key.
    // Adding an item that is already there may update its wait
    // period to the lowest of existing and new wait period or
    // its priority to the highest of existing and new priority.
    AddWithOpts(o AddOpts, items ...T)

    // GetWithPriority returns an item and its priority. It allows
    // a controller to re-use the priority if it enqueues an item
    // again.
    GetWithPriority() (item T, priority int, shutdown bool)

    // workqueue.TypedRateLimitingInterface is kept for backwards
    // compatibility.
    workqueue.TypedRateLimitingInterface[T]
}

type AddOpts struct {
    // After is a duration after which the object will be available for
    // reconciliation. If the object is already in the workqueue, the
    // lowest of existing and new After period will be used.
    After time.Duration

    // Ratelimited specifies if the ratelimiter should be used to
    // determine a wait period. If the object is already in the
    // workqueue, the lowest of existing and new wait period will be
    // used.
    RateLimited bool

    // Priority specifies the priority of the object. Objects with higher
    // priority are returned before objects with lower priority. If the
    // object is already in the workqueue, the priority will be updated
    // to the highest of existing and new priority.
    //
    // The default value is 0.
    Priority int
}
```

In order to fix the issue described in point one of the motivation section,
we have to be able to differentiate events stemming from the initial list
during startup and from resyncs from other events. For events from the initial
list, the informer emits a `Create` event whereas for `Resync` it emits an `Update`
event. The suggestion is to use a heuristic for `Create` events, if the object
in there is older than one minute, it is assumed to be from the initial `List`.
For the `Resync`, we simply check if the `ResourceVersion` is unchanged.
In both these cases, we will lower the priority to `LowPriority`/`-100`.
This gives some room for use-cases where people want to use a priority that
is lower than default (`0`) but higher than what we use in the wrapper.

```go
// WithLowPriorityWhenUnchanged wraps an existing handler and will
// reduce the priority of events stemming from the initial listwatch
// or cache resyncs to LowPriority.
func WithLowPriorityWhenUnchanged[object client.Object, request comparable](u TypedEventHandler[object, request]) TypedEventHandler[object, request]{
}
```

```go
// LowPriority is the priority set by WithLowPriorityWhenUnchanged
const LowPriority = -100
```

The issue described in point two of the motivation section ("application-specific
reasons to prioritize some events") will always require implementation of a custom
handler or eventsource in order to inject the appropriate priority.

## Implementation stages

In order to safely roll this out to all controller-runtime users, it is suggested to
divide the implementation into two stages: Initially, we will add the priority queue
but mark it as experimental and all usage of it requires explicit opt-in by setting
a boolean on the manager or configuring `NewQueue` in a controllers opts. There will
be no breaking changes required for this, but sources or handlers that want to make
use of the new queue will have to use type assertions.

After we've gained some confidence that the implementation is useful and correct, we
will make it the default. Doing so entails breaking the `source.Source` and the
`handler.Handler` interfaces as well as the `controller.Options` struct to refer to
the new workqueue interface. We will wait at least one minor release after introducing
the `PriorityQueue` before doing this.


* [0]: https://youtu.be/AYNaaXlV8LQ?si=i2Pfo7Ske6rTrPLS
* [1]: https://github.com/cilium/cilium/blob/a17d6945b29c177209af3d985bd82cce49eed4a1/operator/pkg/ciliumendpointslice/controller.go#L73

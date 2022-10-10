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

package polling

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	crmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/aws/karpenter/pkg/controllers"
	"github.com/aws/karpenter/pkg/metrics"
)

// Immediate isn't exactly immediate for a reconcile result. But it should be passed to the RequeueAfter if you want
// effectively immediate re-reconciliation. This can't be 0 because otherwise controller-runtime won't treat it as a
// valid RequeueAfter value
const Immediate = time.Nanosecond

type ControllerInterface interface {
	controllers.Controller

	Builder(context.Context, manager.Manager) *controllerruntime.Builder

	Start(context.Context)
	Stop(context.Context)
	Trigger()
	Active() bool
}

// Controller is a wrapper around a controller interface that adds a trigger mechanism for enqueuing
// reconcile requests for the TriggerObject. On a new trigger, Controller will throw away old trigger calls
// by comparing the current triggerGeneration to the previous triggerGeneration.
// Controller also has an active flag that can be enabled or disabled. This serves as a mechanism to stop
// a requeue of a trigger request from the wrapped Reconcile() method of the Controller
type Controller struct {
	r    controllers.Reconciler
	uuid types.UID

	mu     sync.RWMutex
	active bool

	triggerGeneration atomic.Int64
	trigger           chan event.GenericEvent

	cancels sync.Map
}

type Object struct {
	metav1.ObjectMeta
	runtime.Object
}

func NewController(rec controllers.Reconciler) *Controller {
	return &Controller{
		r:       rec,
		uuid:    types.UID(uuid.New().String()),
		trigger: make(chan event.GenericEvent, 100),
	}
}

// WithHealth returns a decorated version of the polling controller that surfaces health information
// based on the success or failure of a reconciliation loop
func (c *Controller) WithHealth() *ControllerWithHealth {
	return NewControllerWithHealth(c)
}

// Start is an idempotent call to kick-off a single reconciliation loop. Based on the intended use of this controller,
// the Reconciler is responsible for requeuing this message back in the WorkQueue so there is a time-based reconciliation
// performed. The Trigger operation is performed to kick-off the loop.
func (c *Controller) Start(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.active {
		logging.FromContext(ctx).Infof("Starting the %s controller...", c.r.Metadata().Name)
		c.active = true
		c.Trigger()
	}
}

// Trigger triggers an immediate reconciliation by inserting a message into the event channel. We increase the trigger
// generation here to ensure that any messages that were previously re-queued are thrown away
func (c *Controller) Trigger() {
	c.triggeredCountMetric().Inc()
	obj := &Object{ObjectMeta: metav1.ObjectMeta{Generation: c.triggerGeneration.Add(1), UID: c.uuid}}
	c.trigger <- event.GenericEvent{Object: obj}
}

// Stop sets the state of the controller to active and cancel the current reconciliation contexts, if there are any
func (c *Controller) Stop(ctx context.Context) {
	logging.FromContext(ctx).Infof("Stopping the %s controller...", c.r.Metadata().Name)
	c.SetActive(false)
	c.cancels.Range(func(_ any, c any) bool {
		cancel := c.(context.CancelFunc)
		cancel()
		return true
	})
}

// Active gets whether the controller is active right now. This value is passed down to the wrapped
// Reconcile method so that the Reconciler can handle cleanup scenarios. The underlying Reconciler is responsible
// for returning a result with no RequeueAfter to stop its activity
func (c *Controller) Active() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.active
}

// SetActive sets the active flag on the controller
func (c *Controller) SetActive(active bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.active = active
	if active {
		c.activeMetric().Set(1)
	} else {
		c.activeMetric().Set(0)
	}
}

func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(c.r.Metadata().Name))
	ctx, cancel := context.WithCancel(ctx)

	// Store the cancel function for the duration of Reconcile, so we can cancel on a Stop() call
	cancelID := uuid.New()
	c.cancels.Store(cancelID, cancel)
	defer c.cancels.Delete(cancelID)

	return c.r.Reconcile(ctx, req)
}

func (c *Controller) Builder(_ context.Context, m manager.Manager) *controllerruntime.Builder {
	crmetrics.Registry.MustRegister(c.activeMetric(), c.triggeredCountMetric())
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(c.r.Metadata().Name).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			// UUID comparison is a hacky way to get around the fact that controller-runtime requires
			// us to perform a watch on some K8s object
			return obj.GetUID() == c.uuid && obj.GetGeneration() == c.triggerGeneration.Load()
		})).
		Watches(&source.Channel{Source: c.trigger}, &handler.EnqueueRequestForObject{}).
		For(&v1.Pod{}) // controller-runtime requires us to perform a watch on some object, so let's do it on a fundamental component
}

func (c *Controller) Register(ctx context.Context, m manager.Manager) error {
	return c.Builder(ctx, m).Complete(c)
}

func (c *Controller) activeMetric() prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metrics.Namespace,
			Subsystem: c.r.Metadata().MetricsSubsystem,
			Name:      "active",
			Help:      "Whether the controller is active.",
		},
	)
}

func (c *Controller) triggeredCountMetric() prometheus.Counter {
	return prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metrics.Namespace,
			Subsystem: c.r.Metadata().MetricsSubsystem,
			Name:      "trigger_count",
			Help:      "A counter of the number of times this controller has been triggered.",
		},
	)
}

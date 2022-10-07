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

	"github.com/google/uuid"
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
)

type ControllerInterface interface {
	controllers.Controller

	Start(context.Context)
	Stop(context.Context)
	Trigger()
	Active() bool
	Healthy() bool
}

// Controller is a wrapper around a controller interface that adds a trigger mechanism for enqueuing
// reconcile requests for the TriggerObject. On a new trigger, Controller will throw away old trigger calls
// by comparing the current triggerGeneration to the previous triggerGeneration.
// Controller also has an active flag that can be enabled or disabled. This serves as a mechanism to stop
// a requeue of a trigger request from the wrapped Reconcile() method of the Controller
type Controller struct {
	OnHealthy   func(context.Context)
	OnUnhealthy func(context.Context)

	r    Reconciler
	uuid types.UID

	active  bool
	healthy bool

	triggerGeneration int64
	trigger           chan event.GenericEvent

	triggerMu sync.RWMutex
	activeMu  sync.RWMutex
	healthyMu sync.RWMutex

	cancels sync.Map
}

type Reconciler interface {
	reconcile.Reconciler

	Name() string
	MetricsSubsystemName() string
}

type Object struct {
	metav1.ObjectMeta
	runtime.Object
}

func NewController(rec Reconciler) *Controller {
	return &Controller{
		r:       rec,
		uuid:    types.UID(uuid.New().String()),
		trigger: make(chan event.GenericEvent, 100),
	}
}

// Start is an idempotent call to kick-off a single reconciliation loop. Based on the intended use of this controller,
// the Reconciler is responsible for requeuing this message back in the WorkQueue so there is a time-based reconciliation
// performed. The Trigger operation is performed to kick-off the loop.
func (t *Controller) Start(ctx context.Context) {
	logging.FromContext(ctx).Infof("Starting the %s controller...", t.r.Name())
	t.activeMu.Lock()
	if !t.active {
		t.active = true
		t.activeMu.Unlock()
		t.Trigger()
	} else {
		t.activeMu.Unlock()
	}
}

// Trigger triggers an immediate reconciliation by inserting a message into the event channel. We increase the trigger
// generation here to ensure that any messages that were previously re-queued are thrown away
func (t *Controller) Trigger() {
	t.triggerMu.Lock()
	defer t.triggerMu.Unlock()

	t.triggerGeneration++
	t.triggeredCountMetric().Inc()
	obj := &Object{ObjectMeta: metav1.ObjectMeta{Generation: t.triggerGeneration, UID: t.uuid}}
	t.trigger <- event.GenericEvent{Object: obj}
}

// Stop sets the state of the controller to active and cancel the current reconciliation contexts, if there are any
func (t *Controller) Stop(ctx context.Context) {
	logging.FromContext(ctx).Infof("Stopping the %s controller...", t.r.Name())
	t.SetActive(false)
	t.cancels.Range(func(_ any, c any) bool {
		cancel := c.(context.CancelFunc)
		cancel()
		return true
	})
}

// Active gets whether the controller is active right now. This value is passed down to the wrapped
// Reconcile method so that the Reconciler can handle cleanup scenarios. The underlying Reconciler is responsible
// for returning a result with no RequeueAfter to stop its activity
func (t *Controller) Active() bool {
	t.activeMu.RLock()
	defer t.activeMu.RUnlock()
	return t.active
}

// SetActive sets the active flag on the controller
func (t *Controller) SetActive(active bool) {
	t.activeMu.Lock()
	defer t.activeMu.Unlock()

	t.active = active
	if active {
		t.activeMetric().Set(1)
	} else {
		t.activeMetric().Set(0)
	}
}

func (t *Controller) Healthy() bool {
	t.healthyMu.RLock()
	defer t.healthyMu.RUnlock()
	return t.healthy
}

func (t *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).Named(t.r.Name()))
	ctx, cancel := context.WithCancel(ctx)

	// Store the cancel function for the duration of the reconcile so we can cancel on a Stop() call
	cancelID := uuid.New()
	t.cancels.Store(cancelID, cancel)
	defer t.cancels.Delete(cancelID)

	res, err := t.r.Reconcile(ctx, req)

	t.healthyMu.Lock()
	t.healthy = err == nil // The controller is considered healthy when it successfully reconciles
	if t.healthy {
		if t.OnHealthy != nil {
			t.OnHealthy(ctx)
		}
		t.healthyMetric().Set(1)
	} else {
		if t.OnUnhealthy != nil {
			t.OnUnhealthy(ctx)
		}
		t.healthyMetric().Set(0)
	}
	t.healthyMu.Unlock()

	t.activeMu.Lock()
	if !t.active {
		return reconcile.Result{}, nil // Swallow any errors/calls at this point
	}
	t.activeMu.Unlock()
	return res, err
}

func (t *Controller) Register(_ context.Context, m manager.Manager) error {
	crmetrics.Registry.MustRegister(t.healthyMetric(), t.activeMetric(), t.triggeredCountMetric())
	return controllerruntime.
		NewControllerManagedBy(m).
		Named(t.r.Name()).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			t.triggerMu.RLock()
			defer t.triggerMu.RUnlock()

			// UUID comparison is a hacky way to get around the fact that controller-runtime requires
			// us to perform a watch on some K8s object
			return obj.GetUID() == t.uuid && obj.GetGeneration() == t.triggerGeneration
		})).
		Watches(&source.Channel{Source: t.trigger}, &handler.EnqueueRequestForObject{}).
		For(&v1.Pod{}). // controller-runtime requires us to perform a watch on some object, so let's do it on a fundamental component
		Complete(t)
}

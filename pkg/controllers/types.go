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

package controllers

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// Controller is an interface implemented by Karpenter custom resources.
type Controller interface {
	// Reconcile hands a hydrated kubernetes resource to the controller for
	// reconciliation. Any changes made to the resource's status are persisted
	// after Reconcile returns, even if it returns an error.
	Reconcile(context.Context, client.Object) (reconcile.Result, error)
	// Interval returns an interval that the controller should wait before
	// executing another reconciliation loop. If set to zero, will only execute
	// on watch events or the global resync interval.
	Interval() time.Duration
	// For returns a default instantiation of the resource and is injected by
	// data from the API Server at the start of the reconciliation loop.
	For() client.Object
	// Owns returns a slice of resources that are watched by this resources.
	// Watch events are triggered if owner references are set on the resource.
	Owns() []client.Object
	// WatchDescription returns the necessary information to create a watch
	//   a. Source: the resource that is being watched
	//   b. EventHandler: which controller objects to be reconciled
	//   c. WatchesOption: which events can be filtered out before processed
	Watches(context.Context) (source.Source, handler.EventHandler, builder.WatchesOption)
}

// NamedController allows controllers to optionally implement a Name() function which will be used instead of the
// reconciled resource's name. This is useful when writing multiple controllers for a single resource type.
type NamedController interface {
	Controller
	// Name returns the name of the controller
	Name() string
}

// Webhook implements both a handler and path and can be attached to a webhook server.
type Webhook interface {
	webhook.AdmissionHandler
	Path() string
}

// Object provides an abstraction over a kubernetes custom resource with
// methods necessary to standardize reconciliation behavior in Karpenter.
//type Object interface {
//	client.Object
//	StatusConditions() apis.ConditionManager
//}

// Manager manages a set of controllers and webhooks.
type Manager interface {
	manager.Manager
	RegisterControllers(controllers ...Controller) Manager
	RegisterWebhooks(controllers ...Webhook) Manager
}

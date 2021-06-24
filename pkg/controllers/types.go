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

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller is an interface implemented by Karpenter custom resources.
type Controller interface {
	// Reconcile hands a hydrated kubernetes resource to the controller for
	// reconciliation. Any changes made to the resource's status are persisted
	// after Reconcile returns, even if it returns an error.
	Reconcile(context.Context, client.Object) (reconcile.Result, error)
	// For returns a default instantiation of the resource and is injected by
	// data from the API Server at the start of the reconciliation loop.
	For() client.Object
}

// NamedController allows controllers to optionally implement a Name() function which will be used instead of the
// reconciled resource's name. This is useful when writing multiple controllers for a single resource type.
type NamedController interface {
	Controller
	// Name returns the name of the controller
	Name() string
}

// Manager manages a set of controllers and webhooks.
type Manager interface {
	manager.Manager
	RegisterControllers(controllers ...Controller) Manager
}

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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
)

// Controller is an interface implemented by Karpenter custom resources.
type Controller interface {
	// Reconcile hands a hydrated kubernetes resource to the controller for
	// reconciliation. Any changes made to the resource's status are persisted
	// after Reconcile returns, even if it returns an error.
	Reconcile(Resource) error
	// Interval returns an interval that the controller should wait before
	// executing another reconciliation loop. If set to zero, will only execute
	// on watch events or the global resync interval.
	Interval() time.Duration
	// For returns a default instantiation of the resource and is injected by
	// data from the API Server at the start of the reconcilation loop.
	For() Resource
	// Owns returns a slice of resources that are watched by this resources.
	// Watch events are triggered if owner references are set on the resource.
	Owns() []Resource
}

// Resource provides an abstraction over a kubernetes custom resource with
// methods necessary to standardize reconciliation behavior in Karpenter.
type Resource interface {
	runtime.Object
	metav1.Object
	StatusConditions() apis.ConditionManager
}

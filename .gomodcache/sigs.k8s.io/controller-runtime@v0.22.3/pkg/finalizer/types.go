/*
Copyright 2021 The Kubernetes Authors.
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

package finalizer

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Registerer holds Register that will check if a key is already registered
// and error out and it does; and if not registered, it will add the finalizer
// to the finalizers map as the value for the provided key.
type Registerer interface {
	Register(key string, f Finalizer) error
}

// Finalizer holds Finalize that will add/remove a finalizer based on the
// deletion timestamp being set and return an indication of whether the
// obj needs an update or not.
type Finalizer interface {
	Finalize(context.Context, client.Object) (Result, error)
}

// Finalizers implements Registerer and Finalizer to finalize all registered
// finalizers if the provided object has a deletion timestamp or set all
// registered finalizers if it does not.
type Finalizers interface {
	Registerer
	Finalizer
}

/*
Copyright 2018 The Kubernetes Authors.

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

package reconcile_test

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// This example implements a simple no-op reconcile function that prints the object to be Reconciled.
func ExampleFunc() {
	r := reconcile.Func(func(_ context.Context, o reconcile.Request) (reconcile.Result, error) {
		// Create your business logic to create, update, delete objects here.
		fmt.Printf("Name: %s, Namespace: %s", o.Name, o.Namespace)
		return reconcile.Result{}, nil
	})

	res, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "test"}})
	if err != nil || res.Requeue || res.RequeueAfter != time.Duration(0) {
		fmt.Printf("got requeue request: %v, %v\n", err, res)
	}

	// Output: Name: test, Namespace: default
}

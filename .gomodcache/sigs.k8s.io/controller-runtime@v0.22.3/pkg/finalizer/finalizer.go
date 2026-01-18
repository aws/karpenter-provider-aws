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
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type finalizers map[string]Finalizer

// Result struct holds information about what parts of an object were updated by finalizer(s).
type Result struct {
	// Updated will be true if at least one of the object's non-status field
	// was updated by some registered finalizer.
	Updated bool
	// StatusUpdated will be true if at least one of the object's status' fields
	// was updated by some registered finalizer.
	StatusUpdated bool
}

// NewFinalizers returns the Finalizers interface.
func NewFinalizers() Finalizers {
	return finalizers{}
}

func (f finalizers) Register(key string, finalizer Finalizer) error {
	if _, ok := f[key]; ok {
		return fmt.Errorf("finalizer for key %q already registered", key)
	}
	f[key] = finalizer
	return nil
}

func (f finalizers) Finalize(ctx context.Context, obj client.Object) (Result, error) {
	var (
		res     Result
		errList []error
	)
	res.Updated = false
	for key, finalizer := range f {
		if dt := obj.GetDeletionTimestamp(); dt.IsZero() && !controllerutil.ContainsFinalizer(obj, key) {
			controllerutil.AddFinalizer(obj, key)
			res.Updated = true
		} else if !dt.IsZero() && controllerutil.ContainsFinalizer(obj, key) {
			finalizerRes, err := finalizer.Finalize(ctx, obj)
			if err != nil {
				// Even when the finalizer fails, it may need to signal to update the primary
				// object (e.g. it may set a condition and need a status update).
				res.Updated = res.Updated || finalizerRes.Updated
				res.StatusUpdated = res.StatusUpdated || finalizerRes.StatusUpdated
				errList = append(errList, fmt.Errorf("finalizer %q failed: %w", key, err))
			} else {
				// If the finalizer succeeds, we remove the finalizer from the primary
				// object's metadata, so we know it will need an update.
				res.Updated = true
				controllerutil.RemoveFinalizer(obj, key)
				// The finalizer may have updated the status too.
				res.StatusUpdated = res.StatusUpdated || finalizerRes.StatusUpdated
			}
		}
	}
	return res, kerrors.NewAggregate(errList)
}

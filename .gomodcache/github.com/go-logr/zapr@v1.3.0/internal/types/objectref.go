/*
Copyright 2021 The logr Authors.

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

// Package types holds a copy of the ObjectRef type from klog for
// use in the example.
package types

import (
	"fmt"

	"github.com/go-logr/logr"
)

// ObjectRef references a Kubernetes object
type ObjectRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

func (ref ObjectRef) String() string {
	if ref.Namespace != "" {
		return fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
	}
	return ref.Name
}

// MarshalLog ensures that loggers with structured output ignore the String method.
//
// We implement fmt.Stringer for non-structured logging, but we want the
// raw struct when using structured logs.  Some logr implementations call
// String if it is present, so we want to convert this struct to something
// that doesn't have that method.
func (ref ObjectRef) MarshalLog() interface{} {
	// Methods do not survive type definitions.
	type forLog ObjectRef
	return forLog(ref)
}

var _ logr.Marshaler = ObjectRef{}

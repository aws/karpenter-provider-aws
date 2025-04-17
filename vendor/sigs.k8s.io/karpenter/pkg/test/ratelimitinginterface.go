/*
Copyright The Kubernetes Authors.

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

// nolint:staticcheck
package test

import (
	"sync"
	"time"

	"k8s.io/client-go/util/workqueue"
)

// RateLimitingInterface is copied from https://github.com/kubernetes-sigs/controller-runtime/blob/e6c3d139d2b6c286b1dbba6b6a95919159cfe655/pkg/controller/controllertest/testing.go#L48
// but is an interface{} implementation to use while the orchestration queue can't use the TypedQueue

// RateLimitingInterface implements a RateLimiting queue as a non-ratelimited queue for testing.
// This helps testing by having functions that use a RateLimiting queue synchronously add items to the queue.
type RateLimitingInterface struct {
	workqueue.Interface
	AddedRateLimitedLock sync.Mutex
	AddedRatelimited     []any
}

func NewRateLimitingInterface(queueConfig workqueue.QueueConfig) *RateLimitingInterface {
	return &RateLimitingInterface{Interface: workqueue.NewWithConfig(queueConfig)}
}

// AddAfter implements RateLimitingInterface.
func (q *RateLimitingInterface) AddAfter(item interface{}, duration time.Duration) {
	q.Add(item)
}

// AddRateLimited implements RateLimitingInterface.
func (q *RateLimitingInterface) AddRateLimited(item interface{}) {
	q.AddedRateLimitedLock.Lock()
	q.AddedRatelimited = append(q.AddedRatelimited, item)
	q.AddedRateLimitedLock.Unlock()
	q.Add(item)
}

// Forget implements RateLimitingInterface.
func (q *RateLimitingInterface) Forget(item interface{}) {}

// NumRequeues implements RateLimitingInterface.
func (q *RateLimitingInterface) NumRequeues(item interface{}) int {
	return 0
}

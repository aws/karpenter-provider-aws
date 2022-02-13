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

package test

import (
	"context"

	"github.com/aws/karpenter/pkg/controllers/termination"
	set "github.com/deckarep/golang-set"
	v1 "k8s.io/api/core/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EvictionQueue is an eviction queue that provides an additional HasContained
// method to check if an ObjectKey was ever in the queue at any point.  This is
// used to eliminate test flakiness as pods can quickly enter and be removed
// from the eviction queue before a test can check if they are in the queue.
type EvictionQueue struct {
	q            termination.EvictionQueue
	hasContained set.Set
}

func (e *EvictionQueue) Add(pods []*v1.Pod) {
	for _, p := range pods {
		e.hasContained.Add(client.ObjectKeyFromObject(p))
	}
	e.q.Add(pods)
}

func (e EvictionQueue) HasContained(keys ...client.ObjectKey) bool {
	for _, p := range keys {
		if !e.hasContained.Contains(p) {
			return false
		}
	}

	return true
}
func (e EvictionQueue) Contains(i ...interface{}) bool {
	return e.q.Contains(i...)
}

func NewEvictionQueue(ctx context.Context, coreV1Client corev1.CoreV1Interface) *EvictionQueue {
	return &EvictionQueue{
		hasContained: set.NewSet(),
		q:            termination.NewEvictionQueue(ctx, coreV1Client),
	}
}

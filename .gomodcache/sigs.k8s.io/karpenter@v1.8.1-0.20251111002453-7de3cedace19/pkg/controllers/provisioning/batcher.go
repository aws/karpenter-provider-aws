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

package provisioning

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"

	"sigs.k8s.io/karpenter/pkg/operator/options"
)

// Batcher separates a stream of Trigger() calls into windowed slices. The
// window is dynamic and will be extended if additional items are added up to a
// maximum batch duration.
type Batcher[T comparable] struct {
	trigger chan struct{}
	clk     clock.Clock

	mu    sync.RWMutex
	elems sets.Set[T]
}

// NewBatcher is a constructor for the Batcher
func NewBatcher[T comparable](clk clock.Clock) *Batcher[T] {
	return &Batcher[T]{
		trigger: make(chan struct{}, 1),
		clk:     clk,
		elems:   sets.New[T](),
	}
}

// Trigger causes the batcher to start a batching window, or extend the current batching window if it hasn't reached the
// maximum length.
func (b *Batcher[T]) Trigger(elem T) {
	// Don't trigger if we've already triggered for this element
	b.mu.RLock()
	if b.elems.Has(elem) {
		b.mu.RUnlock()
		return
	}
	b.mu.RUnlock()
	// The trigger is idempotently armed. This statement never blocks
	select {
	case b.trigger <- struct{}{}:
	default:
	}
	b.mu.Lock()
	b.elems.Insert(elem)
	b.mu.Unlock()
}

// Wait starts a batching window and continues waiting as long as it continues receiving triggers within
// the idleDuration, up to the maxDuration
func (b *Batcher[T]) Wait(ctx context.Context) bool {
	// Ensure that we always reset our tracked elements at the end of a Wait() statement
	defer func() {
		b.mu.Lock()
		b.elems.Clear()
		b.mu.Unlock()
	}()

	timeout := b.clk.NewTimer(time.Second)
	select {
	case <-b.trigger:
		// start the batching window after the first item is received
		timeout.Stop()
	case <-timeout.C():
		// If no pods, bail to the outer controller framework to refresh the context
		return false
	}
	timeout = b.clk.NewTimer(options.FromContext(ctx).BatchMaxDuration)
	idle := b.clk.NewTimer(options.FromContext(ctx).BatchIdleDuration)
	defer func() {
		timeout.Stop()
		idle.Stop()
	}()

	for {
		select {
		case <-b.trigger:
			// correct way to reset an active timer per docs
			if !idle.Stop() {
				<-idle.C()
			}
			idle.Reset(options.FromContext(ctx).BatchIdleDuration)
		case <-timeout.C():
			return true
		case <-idle.C():
			return true
		}
	}
}

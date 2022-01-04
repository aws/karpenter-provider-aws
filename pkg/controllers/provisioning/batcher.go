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

package provisioning

import (
	"context"
	"sync"
	"time"
)

var (
	MaxBatchDuration  = time.Second * 10
	BatchIdleDuration = time.Second * 1
	// MaxItemsPerBatch limits the number of items we process at one time to avoid using too much memory
	MaxItemsPerBatch = 2_000
)

// Batcher separates a stream of Add(item) calls into windowed slices. The
// window is dynamic and will be extended if additional items are added up to a
// maximum batch duration or maximum items per batch.
type Batcher struct {
	sync.RWMutex
	running context.Context
	queue   chan interface{}
	gate    context.Context
	flush   context.CancelFunc
}

// NewBatcher is a constructor
func NewBatcher(running context.Context) *Batcher {
	gate, flush := context.WithCancel(running)
	return &Batcher{
		running: running,
		queue:   make(chan interface{}),
		gate:    gate,
		flush:   flush,
	}
}

// Add an item to the batch, returning the next gate which the caller may block
// on. The gate is protected by a read-write mutex, and may be modified by
// Flush(), which makes a new gate.
//
// In rare scenarios, if a goroutine hangs after enqueueing but before acquiring
// the gate lock, the batch could be flushed, resulting in the pod waiting on
// the next gate. This will be flushed on the next batch, and may result in
// delayed retries for the individual pod if the provisioning loop fails. In
// practice, this won't be encountered because this time window is O(seconds).
func (b *Batcher) Add(item interface{}) <-chan struct{} {
	select {
	case b.queue <- item:
	case <-b.running.Done():
	}
	b.RLock()
	defer b.RUnlock()
	return b.gate.Done()
}

// Flush all goroutines blocking on the current gate and create a new gate.
func (b *Batcher) Flush() {
	b.Lock()
	defer b.Unlock()
	b.flush()
	b.gate, b.flush = context.WithCancel(b.running)
}

// Wait starts a batching window and returns a slice of items when closed.
func (b *Batcher) Wait() (items []interface{}, window time.Duration) {
	// Start the batching window after the first item is received
	items = append(items, <-b.queue)
	start := time.Now()
	defer func() {
		window = time.Since(start)
	}()
	timeout := time.NewTimer(MaxBatchDuration)
	idle := time.NewTimer(BatchIdleDuration)
	for {
		if len(items) >= MaxItemsPerBatch {
			return
		}
		select {
		case item := <-b.queue:
			idle.Reset(BatchIdleDuration)
			items = append(items, item)
		case <-timeout.C:
			return
		case <-idle.C:
			return
		}
	}
}

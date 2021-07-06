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

package allocation

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

const (
	BatchCacheTTL             = 1 * time.Hour
	BatchCacheCleanupInterval = 2 * time.Hour
)

type Batcher struct {
	// MaxBatchPeriod is the maximum amount of time to batch incoming pods before flushing
	MaxBatchPeriod time.Duration
	// IdlePeriod is the amount of time to wait to flush a batch when there are no incoming pods but the batch is not empty
	// It should be a smaller duration than MaxBatchPeriod
	IdlePeriod time.Duration

	// batches keeps a mapping of a key (like a provisioner name and namespace) to a specific Batch
	batches *cache.Cache
}

// NewBatcher creates a new batch manager to start multiple batch windows
func NewBatcher(maxBatchPeriod time.Duration, idlePeriod time.Duration) *Batcher {
	batchCache := cache.New(BatchCacheTTL, BatchCacheCleanupInterval)
	batchCache.OnEvicted(func(key string, val interface{}) {
		batch := val.(*Batch)
		batch.close()
	})
	return &Batcher{
		MaxBatchPeriod: maxBatchPeriod,
		IdlePeriod:     idlePeriod,
		batches:        batchCache,
	}
}

// Add starts a batching window or updates an existing one based on a batching key
func (m *Batcher) Add(key string) {
	batch, ok := m.batches.Get(key)
	if !ok {
		batch = &Batch{
			Batcher: m,
			start:   make(chan bool, 1),
			updates: make(chan bool, 1),
			end:     make(chan bool, 1),
		}
		m.batches.SetDefault(key, batch)
	}
	// Updates expiration
	m.batches.SetDefault(key, batch)
	batch.(*Batch).Add()
}

// Complete blocks until a specific batching window ends based on the batching key
func (m *Batcher) Complete(key string) {
	batch, ok := m.batches.Get(key)
	if !ok {
		return
	}
	batch.(*Batch).Complete()
}

// Batch implements a single batching window based on a max timeout and a progress period
type Batch struct {
	// start is a channel to coordinate window starts
	start chan bool
	// updates is a channel to communicate progress in filling a batch window
	updates chan bool
	// end is a channel to signal a window close
	end chan bool

	sync.Mutex
	*Batcher
}

// Add starts a batching window or adds to an existing in-progress window
func (b *Batch) Add() {
	b.Lock()
	defer b.Unlock()
	select {
	// Start a window when the channel is not blocked
	case b.start <- true:
		go b.monitor()
	// If the channel is blocked, then it's already processing a window, so send an update signal
	case b.updates <- true:
	// If both are blocked, then no need to start or send an update signal
	default:
	}
}

// Complete blocks until a batching window ends
// If the batch is empty, it will block until something is added or the window times out
func (b *Batch) Complete() {
	// block until window end signal is received from the window monitor
	<-b.end
}

// monitor kicks off a batch window and updates a bool when the window completes
func (b *Batch) monitor() {
	b.waitForWindowEnd()
	b.Lock()
	defer b.Unlock()
	b.end <- true
	// release start window
	<-b.start
}

// waitForWindowEnd will block until MaxBatchPeriod or the IdlePeriod is reached between Add operations
func (b *Batch) waitForWindowEnd() {
	ticker := time.NewTicker(b.IdlePeriod)
	timer := time.NewTimer(b.MaxBatchPeriod)
	for {
		select {
		// any progress resets the ticker
		case <-b.updates:
			ticker.Reset(b.IdlePeriod)
		// if ticker goes off, then end window early since no progress is being made
		case <-ticker.C:
			return
		// block until MaxBatchPeriod timer goes off
		case <-timer.C:
			return
		}
	}
}

// close will close all channels. If channels are in use, then this could panic.
// close should only be called by the batcher
func (b *Batch) close() {
	close(b.start)
	close(b.updates)
	close(b.end)
}

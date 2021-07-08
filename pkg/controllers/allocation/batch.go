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
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Batcher is a batch manager for multiple objects
type Batcher struct {
	// MaxBatchPeriod is the maximum amount of time to batch incoming pods before flushing
	MaxBatchPeriod time.Duration
	// IdlePeriod is the amount of time to wait to flush a batch when there are no incoming pods but the batch is not empty
	// It should be a smaller duration than MaxBatchPeriod
	IdlePeriod time.Duration

	// windows keeps a mapping of a key (like a provisioner name and namespace) to a specific object's batch window
	windows map[string]*Window
	// updates is a stream of obj keys that can start or provide a progress signal to an object's batch window
	updates chan string
	// removals is a stream of obj keys that correspond to a window that should be removed
	removals chan string
	// isMonitorRunning indicates if the monitor go routine has been started
	isMonitorRunning bool
}

// Window is an individual batch window
type Window struct {
	lastUpdated time.Time
	started     time.Time
	closed      chan bool
}

// NewBatcher creates a new batch manager to start multiple batch windows
func NewBatcher(maxBatchPeriod time.Duration, idlePeriod time.Duration) *Batcher {
	return &Batcher{
		MaxBatchPeriod: maxBatchPeriod,
		IdlePeriod:     idlePeriod,
		windows:        map[string]*Window{},
		updates:        make(chan string, 1000),
		removals:       make(chan string, 100),
	}
}

// Start should be called before Add or Wait
// It is not safe to call Start concurrently
// but Start can be called synchronously multiple times w/ no effect
func (b *Batcher) Start(ctx context.Context) {
	if !b.isMonitorRunning {
		go b.monitor(ctx)
		b.isMonitorRunning = true
	}
}

// Add starts a batching window or adds to an existing in-progress window
// Add is safe to be called concurrently
func (b *Batcher) Add(obj metav1.Object) {
	select {
	case b.updates <- b.keyFrom(obj):
	// Do not block if the channel is full
	default:
	}
}

// Wait blocks until a batching window ends
// If the batch is empty, it will block until something is added or the window times out
// Wait should not be called concurrently for the same object but can be called concurrently for different objects
func (b *Batcher) Wait(obj metav1.Object) {
	batch, ok := b.windows[b.keyFrom(obj)]
	if !ok {
		return
	}
	<-batch.closed
}

// Remove will cause the batch window for the passed in obj to stop being monitored
// Remove should only be called if there are no Add calls or Wait calls happening concurrently
// After a Remove call for an object, a subsequent Add for the same object will recreate the window
func (b *Batcher) Remove(obj metav1.Object) {
	select {
	case b.removals <- b.keyFrom(obj):
	// Do not block if the channel is full
	default:
	}
}

// monitor is a synchronous loop that controls the window start, update, and end
// monitor should be executed in one go routine and will handle all object batch windows
func (b *Batcher) monitor(ctx context.Context) {
	defer func() { b.isMonitorRunning = false }()
	ticker := time.NewTicker(time.Second * 1)
	for {
		select {
		// Wake and check for any timed out batch windows
		case <-ticker.C:
			for _, batch := range b.windows {
				b.checkForWindowEndAndNotify(batch)
			}
		// Start a new window or update progress on a window
		case key := <-b.updates:
			b.startOrUpdateWindow(key)
		// Remove a window by key
		case key := <-b.removals:
			delete(b.windows, key)
		// Stop monitor routine on shutdown
		case <-ctx.Done():
			return
		}
	}
}

// checkForWindowEndAndNotify checks if a window has timed out due to inactivity (IdlePeriod) or has reached the MaxBatchPeriod.
// If the batch window has ended, then the batch closed channel will be notified and the window will be reset
func (b *Batcher) checkForWindowEndAndNotify(window *Window) {
	if window.started.IsZero() {
		return
	}
	if time.Since(window.lastUpdated) >= b.IdlePeriod || time.Since(window.started) >= b.MaxBatchPeriod {
		select {
		case window.closed <- true:
			window.started = time.Time{}
		default:
		}
	}
}

// startOrUpdateWindow starts a new window for the object key if one does not already exist
// if a window already exists for the object key, then the lastUpdate time is set
func (b *Batcher) startOrUpdateWindow(key string) {
	if window, ok := b.windows[key]; ok {
		window.lastUpdated = time.Now()
		if window.started.IsZero() {
			window.started = time.Now()
		}
	} else {
		b.windows[key] = &Window{lastUpdated: time.Now(), started: time.Now(), closed: make(chan bool, 1)}
	}
}

// keyFrom takes an object and outputs a unique key
func (b *Batcher) keyFrom(obj metav1.Object) string {
	return fmt.Sprintf("%s/%s", obj.GetName(), obj.GetNamespace())
}

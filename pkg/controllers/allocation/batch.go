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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	opAdd  = "add"
	opWait = "wait"
)

// Batcher is a batch manager for multiple objects
type Batcher struct {
	// MaxPeriod is the maximum amount of time to batch incoming pods before flushing
	MaxPeriod time.Duration
	// IdlePeriod is the amount of time to wait to flush a batch when there are no incoming pods but the batch is not empty
	// It should be a smaller duration than MaxPeriod
	IdlePeriod time.Duration

	// windows keeps a mapping of a key (like a provisioner name and namespace) to a specific object's batch window
	windows map[types.UID]*window
	// ops is a stream of add and wait operations on a batch window
	ops chan *batchOp
	// isMonitorRunning indicates if the monitor go routine has been started
	isMonitorRunning bool
}

type batchOp struct {
	kind    string
	key     types.UID
	waitEnd chan bool
}

// window is an individual batch window
type window struct {
	lastUpdated time.Time
	started     time.Time
	closed      []chan bool
}

// NewBatcher creates a new batch manager to start multiple batch windows
func NewBatcher(maxPeriod time.Duration, idlePeriod time.Duration) *Batcher {
	return &Batcher{
		MaxPeriod:  maxPeriod,
		IdlePeriod: idlePeriod,
		windows:    map[types.UID]*window{},
	}
}

// Start should be called before Add or Wait
// It is not safe to call Start concurrently
// but Start can be called synchronously multiple times w/ no effect
func (b *Batcher) Start(ctx context.Context) {
	if !b.isMonitorRunning {
		b.ops = make(chan *batchOp, 1000)
		go b.monitor(ctx)
		b.isMonitorRunning = true
	}
}

// Add starts a batching window or adds to an existing in-progress window
// Add is safe to be called concurrently
func (b *Batcher) Add(obj metav1.Object) {
	select {
	case b.ops <- &batchOp{kind: opAdd, key: obj.GetUID()}:
	// Do not block if the channel is full
	default:
	}
}

// Wait blocks until a batching window ends
// If the batch is empty, it will block until something is added or the window times out
func (b *Batcher) Wait(obj metav1.Object) {
	waitBatchOp := &batchOp{kind: opWait, key: obj.GetUID(), waitEnd: make(chan bool, 1)}
	timeout := time.NewTimer(b.MaxPeriod)
	select {
	case b.ops <- waitBatchOp:
		<-waitBatchOp.waitEnd
	// if the ops channel is full (should be very rare), allow wait to block until the MaxPeriod
	case <-timeout.C:
	}
}

// monitor is a synchronous loop that controls the window start, update, and end
// monitor should be executed in one go routine and will handle all object batch windows
func (b *Batcher) monitor(ctx context.Context) {
	defer func() { b.isMonitorRunning = false }()
	ticker := time.NewTicker(b.IdlePeriod / 2)
	for {
		select {
		// Wake and check for any timed out batch windows
		case <-ticker.C:
			for key, batch := range b.windows {
				b.checkForWindowEndAndNotify(key, batch)
			}
		// Process window operations
		case op := <-b.ops:
			switch op.kind {
			// Start a new window or update progress on a window
			case opAdd:
				b.startOrUpdateWindow(op.key)
			// Register a waiter and start a window if no window has been started
			case opWait:
				window, ok := b.windows[op.key]
				if !ok {
					window = b.startOrUpdateWindow(op.key)
				}
				window.closed = append(window.closed, op.waitEnd)
			}
		// Stop monitor routine on shutdown
		case <-ctx.Done():
			for key, window := range b.windows {
				b.endWindow(key, window)
			}
			return
		}
	}
}

// checkForWindowEndAndNotify checks if a window has timed out due to inactivity (IdlePeriod) or has reached the MaxBatchPeriod.
// If the batch window has ended, then the batch closed channel will be notified and the window will be removed
func (b *Batcher) checkForWindowEndAndNotify(key types.UID, window *window) {
	if time.Since(window.lastUpdated) < b.IdlePeriod && time.Since(window.started) < b.MaxPeriod {
		return
	}
	b.endWindow(key, window)
}

// endWindow signals the end of a window to all wait consumers and deletes the window
func (b *Batcher) endWindow(key types.UID, window *window) {
	for _, end := range window.closed {
		select {
		case end <- true:
			close(end)
		default:
		}
	}
	delete(b.windows, key)
}

// startOrUpdateWindow starts a new window for the object key if one does not already exist
// if a window already exists for the object key, then the lastUpdate time is set
func (b *Batcher) startOrUpdateWindow(key types.UID) *window {
	batchWindow, ok := b.windows[key]
	if !ok {
		batchWindow = &window{lastUpdated: time.Now(), started: time.Now()}
		b.windows[key] = batchWindow
		return batchWindow
	}
	batchWindow.lastUpdated = time.Now()
	if batchWindow.started.IsZero() {
		batchWindow.started = time.Now()
	}
	return batchWindow
}

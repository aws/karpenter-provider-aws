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

package parallel

import (
	"context"
	"time"
)

type BatchOptions struct {
	IdleDuration time.Duration
	MaxDuration  time.Duration
	MaxSize      int
}

// Batch objects across an expanding time window
type Batch struct {
	options   BatchOptions
	startTime time.Time
	// Input won't be closed, and won't accept items after the batch is closed.
	input chan interface{}
	// Output will be closed when the batch closes.
	output chan interface{}
	// The batch is open until idle, timeout, or max items. Items added after
	// the batch is closed won't be included in output.
	open  context.Context
	close context.CancelFunc
	// The batch is running until Stop is called. Callers of Add() will release.
	running context.Context
	stop    context.CancelFunc
}

func NewBatch(ctx context.Context, options BatchOptions) *Batch {
	open, close := context.WithCancel(ctx)
	running, stop := context.WithCancel(ctx)
	return &Batch{
		options: options,
		input:   make(chan interface{}),
		output:  make(chan interface{}, options.MaxSize),
		open:    open,
		close:   close,
		running: running,
		stop:    stop,
	}
}

// Next returns a channel that can be ranged upon to recieve the batch. The
// channel will close when the batch is closed.
func (b *Batch) Next() chan interface{} {
	return b.output
}

// Start the batch and close after idle, timeout, or max items.
// Returns a function that closes the batch and releases callers of Add()
func (b *Batch) Start() {
	go func() {
		b.startTime = time.Now()
		timeout := time.NewTimer(b.options.MaxDuration)
		idle := time.NewTimer(b.options.IdleDuration)
		for i := 0; i < b.options.MaxSize; i++ {
			if b.open.Err() != nil {
				break
			}
			select {
			case item := <-b.input:
				b.output <- item
			case <-timeout.C:
				b.close()
			case <-idle.C:
				b.close()
			}
		}
		close(b.output)
	}()
}

func (b *Batch) Stop() {
	b.stop()
}

// Add a item to the batch and block until the batch is complete. Returns true
// if the item was added to the batch, or false if the batch was closed.
func (b *Batch) Add(ctx context.Context, item interface{}) bool {
	// Enqueue
	select {
	case b.input <- item:
	case <-b.open.Done():
		return false
	case <-ctx.Done():
		return false
	}
	// Release
	select {
	case <-b.running.Done():
	case <-ctx.Done():
	}
	return true
}

func (b *Batch) Duration() time.Duration {
	return time.Since(b.startTime)
}

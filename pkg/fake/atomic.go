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

package fake

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"sync"
)

// AtomicPtr is intended for use in mocks to easily expose variables for use in testing.  It makes setting and retrieving
// the values race free by wrapping the pointer itself in a mutex.  There is no Get() method, but instead a Clone() method
// deep copies the object being stored by serializing/de-serializing it from JSON.  This pattern shouldn't be followed
// anywhere else but is an easy way to eliminate races in our tests.
type AtomicPtr[T any] struct {
	mu    sync.Mutex
	value *T
}

func (a *AtomicPtr[T]) Set(v *T) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.value = v
}

func (a *AtomicPtr[T]) IsNil() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.value == nil
}

func (a *AtomicPtr[T]) Clone() *T {
	a.mu.Lock()
	defer a.mu.Unlock()
	return clone(a.value)
}

func clone[T any](v *T) *T {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		log.Fatalf("encoding %T, %s", v, err)
	}
	dec := json.NewDecoder(&buf)
	var cp T
	if err := dec.Decode(&cp); err != nil {
		log.Fatalf("encoding %T, %s", v, err)
	}
	return &cp
}

func (a *AtomicPtr[T]) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.value = nil
}

type AtomicError struct {
	mu  sync.Mutex
	err error

	calls    int
	maxCalls int
}

func (e *AtomicError) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.err = nil
	e.calls = 0
	e.maxCalls = 0
}

func (e *AtomicError) IsNil() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.err == nil
}

// Get is equivalent to the error being called, so we increase
// number of calls in this function
func (e *AtomicError) Get() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.calls >= e.maxCalls {
		return nil
	}
	e.calls++
	return e.err
}

func (e *AtomicError) Set(err error, opts ...AtomicErrorOption) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.err = err
	for _, opt := range opts {
		opt(e)
	}
	if e.maxCalls == 0 {
		e.maxCalls = 1
	}
}

type AtomicErrorOption func(atomicError *AtomicError)

func MaxCalls(maxCalls int) AtomicErrorOption {
	// Setting to 0 is equivalent to allowing infinite errors to API
	if maxCalls <= 0 {
		maxCalls = math.MaxInt
	}
	return func(e *AtomicError) {
		e.maxCalls = maxCalls
	}
}

// AtomicPtrSlice exposes a slice of a pointer type in a race-free manner. The interface is just enough to replace the
// set.Set usage in our previous tests.
type AtomicPtrSlice[T any] struct {
	mu     sync.RWMutex
	values []*T
}

func (a *AtomicPtrSlice[T]) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.values = nil
}

func (a *AtomicPtrSlice[T]) Add(input *T) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.values = append(a.values, clone(input))
}

func (a *AtomicPtrSlice[T]) Len() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.values)
}

func (a *AtomicPtrSlice[T]) Pop() *T {
	a.mu.Lock()
	defer a.mu.Unlock()

	last := a.values[len(a.values)-1]
	a.values = a.values[0 : len(a.values)-1]
	return last
}

func (a *AtomicPtrSlice[T]) ForEach(fn func(*T)) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, t := range a.values {
		fn(clone(t))
	}
}

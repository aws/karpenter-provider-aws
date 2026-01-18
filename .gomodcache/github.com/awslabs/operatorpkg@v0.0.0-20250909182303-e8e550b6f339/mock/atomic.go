package mock

import (
	"bytes"
	"encoding/json"
	"log"
	"sync"
)

// atomicPtr is intended for use in mocks to easily expose variables for use in testing.  It makes setting and retrieving
// the values race free by wrapping the pointer itself in a mutex.  There is no Get() method, but instead a Clone() method
// deep copies the object being stored by serializing/de-serializing it from JSON.  This pattern shouldn't be followed
// anywhere else but is an easy way to eliminate races in our tests.
type atomicPtr[T any] struct {
	mu    sync.Mutex
	value *T
}

func (a *atomicPtr[T]) Set(v *T) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.value = v
}

func (a *atomicPtr[T]) IsNil() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.value == nil
}

func (a *atomicPtr[T]) Clone() *T {
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

func (a *atomicPtr[T]) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.value = nil
}

type atomicError struct {
	mu  sync.Mutex
	err error

	calls    int
	maxCalls int
}

func (e *atomicError) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.err = nil
	e.calls = 0
	e.maxCalls = 0
}

func (e *atomicError) IsNil() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.err == nil
}

// Get is equivalent to the error being called, so we increase
// number of calls in this function
func (e *atomicError) Get() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.calls >= e.maxCalls {
		return nil
	}
	e.calls++
	return e.err
}

func (e *atomicError) Set(err error, opts ...atomicErrorOption) {
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

type atomicErrorOption func(atomicError *atomicError)

// atomicPtrSlice exposes a slice of a pointer type in a race-free manner. The interface is just enough to replace the
// set.Set usage in our previous tests.
type atomicPtrSlice[T any] struct {
	mu     sync.RWMutex
	values []*T
}

func (a *atomicPtrSlice[T]) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.values = nil
}

func (a *atomicPtrSlice[T]) Add(input *T) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.values = append(a.values, clone(input))
}

func (a *atomicPtrSlice[T]) Len() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.values)
}

func (a *atomicPtrSlice[T]) Pop() *T {
	a.mu.Lock()
	defer a.mu.Unlock()
	last := a.values[len(a.values)-1]
	a.values = a.values[0 : len(a.values)-1]
	return last
}

func (a *atomicPtrSlice[T]) ForEach(fn func(*T)) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, t := range a.values {
		fn(clone(t))
	}
}

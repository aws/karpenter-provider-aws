package mock

import (
	"sync/atomic"
)

type Function[I any, O any] struct {
	Output          atomicPtr[O]      // Output to return on call to this function
	CalledWithInput atomicPtrSlice[I] // Slice used to keep track of passed input to this function
	Error           atomicError       // Error to return a certain number of times defined by custom error options

	successfulCalls atomic.Int32 // Internal construct to keep track of the number of times this function has successfully been called
	failedCalls     atomic.Int32 // Internal construct to keep track of the number of times this function has failed (with error)
}

// Reset must be called between tests otherwise tests will pollute
// each other.
func (m *Function[I, O]) Reset() {
	m.Output.Reset()
	m.CalledWithInput.Reset()
	m.Error.Reset()

	m.successfulCalls.Store(0)
	m.failedCalls.Store(0)
}

func (m *Function[I, O]) Invoke(input *I, defaultTransformer func(*I) (*O, error)) (*O, error) {
	err := m.Error.Get()
	if err != nil {
		m.failedCalls.Add(1)
		return nil, err
	}
	m.CalledWithInput.Add(input)

	if !m.Output.IsNil() {
		m.successfulCalls.Add(1)
		return m.Output.Clone(), nil
	}
	out, err := defaultTransformer(input)
	if err != nil {
		m.failedCalls.Add(1)
	} else {
		m.successfulCalls.Add(1)
	}
	return out, err
}

func (m *Function[I, O]) Calls() int {
	return m.SuccessfulCalls() + m.FailedCalls()
}

func (m *Function[I, O]) SuccessfulCalls() int {
	return int(m.successfulCalls.Load())
}

func (m *Function[I, O]) FailedCalls() int {
	return int(m.failedCalls.Load())
}
